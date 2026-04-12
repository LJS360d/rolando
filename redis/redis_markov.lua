#!lua name=markov
---@meta
---@diagnostic disable: undefined-global

-- ---------------------------------------------------------------------------
-- Key helpers
-- ---------------------------------------------------------------------------
local function state_key(guild_id, prefix) return "markov:" .. guild_id .. ":state:" .. prefix end
local function prefixes_key(guild_id) return "markov:" .. guild_id .. ":prefixes" end
local function stats_prefix_key(guild_id) return "stats:" .. guild_id .. ":unique_prefixes" end
local function stats_msg_key(guild_id) return "stats:" .. guild_id .. ":message_count" end
local function stats_bytes_key(guild_id) return "stats:" .. guild_id .. ":estimated_bytes" end
local function media_key(guild_id, kind) return "media:" .. guild_id .. ":" .. kind end
local function config_key(guild_id) return "config:" .. guild_id end

-- ---------------------------------------------------------------------------
-- split_tokens  (used only by generate_markov; tokenisation lives in Go)
-- ---------------------------------------------------------------------------
local function split_tokens(str)
  local tokens = {}
  for token in string.gmatch(str, "%S+") do
    table.insert(tokens, token)
  end
  return tokens
end

-- ---------------------------------------------------------------------------
-- train_batch  KEYS[1]=guild_id
--              ARGV[1]=max_size_bytes  (0 = unlimited)
--              ARGV[2]=message_count   (number of messages in this batch)
--              ARGV[3..N] = pairs packed as "prefix\0next_word"
--
-- Ingests an entire pre-tokenised batch in a single FCall.
-- Returns 1=written, 0=size limit already reached (nothing written).
-- ---------------------------------------------------------------------------
local function train_batch(keys, args)
  local guild_id       = keys[1]
  local max_size_bytes = tonumber(args[1]) or 0
  local message_count  = tonumber(args[2]) or 0

  -- Fast size-limit pre-check via the cheap counter
  if max_size_bytes > 0 then
    local current = tonumber(redis.call('GET', stats_bytes_key(guild_id)) or "0") or 0
    if current >= max_size_bytes then
      return 0
    end
  end

  local pk           = prefixes_key(guild_id)
  local added_bytes  = 0
  local new_prefixes = 0

  for i = 3, #args do
    local pair = args[i]
    local sep  = string.find(pair, "\0", 1, true)

    -- Replace 'goto continue' with a standard if check
    if sep then
      local prefix    = string.sub(pair, 1, sep - 1)
      local next_word = string.sub(pair, sep + 1)
      local sk        = state_key(guild_id, prefix)
      local is_new    = redis.call('EXISTS', sk) == 0

      redis.call('HINCRBY', sk, next_word, 1)

      if is_new then
        new_prefixes = new_prefixes + 1
        redis.call('SADD', pk, prefix)
        added_bytes = added_bytes + #sk + 64      -- key name + Redis key overhead
      end
      added_bytes = added_bytes + #next_word + 16 -- hash field + integer value
    end
  end

  -- Commit aggregated counters in bulk
  if new_prefixes > 0 then
    redis.call('INCRBY', stats_prefix_key(guild_id), new_prefixes)
  end
  if message_count > 0 then
    redis.call('INCRBY', stats_msg_key(guild_id), message_count)
  end
  if added_bytes > 0 then
    redis.call('INCRBY', stats_bytes_key(guild_id), added_bytes)
  end

  return 1
end

-- ---------------------------------------------------------------------------
-- train_markov  KEYS[1]=guild_id  ARGV[1]=prefix  ARGV[2]=next_word
--               ARGV[3]=max_size_bytes  (0 = unlimited)
--
-- Single-pair write for real-time ingestion of one message.
-- Returns 1=written, 0=size limit hit.
-- ---------------------------------------------------------------------------
local function train_markov(keys, args)
  local guild_id       = keys[1]
  local prefix         = args[1]
  local next_word      = args[2]
  local max_size_bytes = tonumber(args[3]) or 0

  if max_size_bytes > 0 then
    local current = tonumber(redis.call('GET', stats_bytes_key(guild_id)) or "0") or 0
    if current >= max_size_bytes then
      return 0
    end
  end

  local sk     = state_key(guild_id, prefix)
  local is_new = redis.call('EXISTS', sk) == 0

  redis.call('HINCRBY', sk, next_word, 1)

  local added = #next_word + 16
  if is_new then
    redis.call('INCR', stats_prefix_key(guild_id))
    redis.call('SADD', prefixes_key(guild_id), prefix)
    added = added + #sk + 64
  end
  redis.call('INCRBY', stats_bytes_key(guild_id), added)

  return 1
end

-- ---------------------------------------------------------------------------
-- count_message  KEYS[1]=guild_id
-- ---------------------------------------------------------------------------
local function count_message(keys, _args)
  redis.call('INCR', stats_msg_key(keys[1]))
  return 1
end

-- ---------------------------------------------------------------------------
-- find_prefix  KEYS[1]=guild_id  ARGV[1]=seed
-- ---------------------------------------------------------------------------
local function find_prefix(keys, args)
  local guild_id = keys[1]
  local seed     = args[1] or ""
  local pk       = prefixes_key(guild_id)

  if seed ~= "" then
    if redis.call('EXISTS', state_key(guild_id, seed)) == 1 then
      return seed
    end

    local cursor   = "0"
    local matching = {}
    repeat
      local res = redis.call('SSCAN', pk, cursor, "COUNT", 100)
      cursor = res[1]
      for _, key in ipairs(res[2]) do
        if string.find(key, seed, 1, true) then
          table.insert(matching, key)
          if #matching >= 200 then
            cursor = "0"
            break
          end
        end
      end
    until cursor == "0"

    if #matching > 0 then
      math.randomseed(tonumber(redis.call('TIME')[1]) + tonumber(redis.call('TIME')[2]))
      return matching[math.random(1, #matching)]
    end
  end

  return redis.call('SRANDMEMBER', pk) or ""
end

-- ---------------------------------------------------------------------------
-- generate_markov  KEYS[1]=guild_id
--                  ARGV[1]=start_prefix  ARGV[2]=max_length  ARGV[3]=n_gram_size
-- ---------------------------------------------------------------------------
local function generate_markov(keys, args)
  local guild_id     = keys[1]
  local start_prefix = args[1] or ""
  local max_length   = tonumber(args[2]) or 20
  local n_gram_size  = tonumber(args[3]) or 2
  local window       = n_gram_size - 1

  if start_prefix == "" then return "" end

  math.randomseed(tonumber(redis.call('TIME')[1]) + tonumber(redis.call('TIME')[2]))

  local generated      = split_tokens(start_prefix)
  local current_prefix = start_prefix

  for _ = 1, max_length do
    local backoff    = split_tokens(current_prefix)
    local next_words = {}
    local found      = false

    while #backoff > 0 do
      local bk     = table.concat(backoff, " ")
      local result = redis.call('HGETALL', state_key(guild_id, bk))
      if #result > 0 then
        next_words = result
        found      = true
        break
      end
      table.remove(backoff, 1)
    end

    if not found then break end

    local total_weight = 0
    for j = 2, #next_words, 2 do
      total_weight = total_weight + tonumber(next_words[j])
    end
    if total_weight == 0 then break end

    local target     = math.random(1, total_weight)
    local cumulative = 0
    local chosen     = nil
    for j = 1, #next_words, 2 do
      cumulative = cumulative + tonumber(next_words[j + 1])
      if target <= cumulative then
        chosen = next_words[j]
        break
      end
    end
    if not chosen then break end

    table.insert(generated, chosen)

    local new_prefix = {}
    local start_idx  = math.max(1, #generated - window + 1)
    for k = start_idx, #generated do
      table.insert(new_prefix, generated[k])
    end
    current_prefix = table.concat(new_prefix, " ")
  end

  return table.concat(generated, " ")
end

-- ---------------------------------------------------------------------------
-- delete_markov  KEYS[1]=guild_id  ARGV[1]=prefix  ARGV[2]=next_word
-- ---------------------------------------------------------------------------
local function delete_markov(keys, args)
  local guild_id  = keys[1]
  local prefix    = args[1]
  local next_word = args[2]
  local sk        = state_key(guild_id, prefix)

  if redis.call('EXISTS', sk) == 0 then return 0 end

  local current = tonumber(redis.call('HGET', sk, next_word) or "0")
  if current <= 0 then return 0 end

  local new_val = redis.call('HINCRBY', sk, next_word, -1)

  local freed = #next_word + 16
  if new_val <= 0 then
    redis.call('HDEL', sk, next_word)
    if redis.call('HLEN', sk) == 0 then
      redis.call('DEL', sk)
      redis.call('SREM', prefixes_key(guild_id), prefix)
      redis.call('DECR', stats_prefix_key(guild_id))
      freed = freed + #sk + 64
    end
  end

  -- Clamp byte counter to 0
  local cur_bytes = tonumber(redis.call('GET', stats_bytes_key(guild_id)) or "0") or 0
  redis.call('SET', stats_bytes_key(guild_id), math.max(0, cur_bytes - freed))

  return 1
end

-- ---------------------------------------------------------------------------
-- get_stats_markov  KEYS[1]=guild_id
-- Returns {unique_prefixes, message_count, estimated_bytes}
-- ---------------------------------------------------------------------------
local function get_stats_markov(keys, _args)
  local guild_id = keys[1]
  return {
    tonumber(redis.call('GET', stats_prefix_key(guild_id)) or "0") or 0,
    tonumber(redis.call('GET', stats_msg_key(guild_id)) or "0") or 0,
    tonumber(redis.call('GET', stats_bytes_key(guild_id)) or "0") or 0,
  }
end

-- ---------------------------------------------------------------------------
-- reconcile_bytes  KEYS[1]=guild_id
-- Performs the expensive O(n-prefixes) MEMORY USAGE walk and resets the
-- counter to the true value.  Never call on the hot path.
-- Returns the corrected byte count.
-- ---------------------------------------------------------------------------
local function reconcile_bytes(keys, _args)
  local guild_id    = keys[1]
  local total_bytes = 0

  local function add_size(key)
    local usage = redis.call('MEMORY', 'USAGE', key, 'SAMPLES', 0)
    if usage then total_bytes = total_bytes + usage end
  end

  local pk     = prefixes_key(guild_id)
  local cursor = "0"
  repeat
    local res = redis.call('SSCAN', pk, cursor, 'COUNT', 100)
    cursor = res[1]
    for _, prefix in ipairs(res[2]) do
      add_size(state_key(guild_id, prefix))
    end
  until cursor == "0"

  add_size(pk)
  add_size(stats_prefix_key(guild_id))
  add_size(stats_msg_key(guild_id))
  add_size(stats_bytes_key(guild_id))
  add_size(media_key(guild_id, "gif"))
  add_size(media_key(guild_id, "image"))
  add_size(media_key(guild_id, "video"))

  redis.call('SET', stats_bytes_key(guild_id), total_bytes)
  return total_bytes
end

-- ---------------------------------------------------------------------------
-- clear_guild  KEYS[1]=guild_id
-- ---------------------------------------------------------------------------
local function clear_guild(keys, _args)
  local guild_id = keys[1]
  local pk       = prefixes_key(guild_id)

  local cursor   = "0"
  repeat
    local res = redis.call('SSCAN', pk, cursor, "COUNT", 200)
    cursor = res[1]
    for _, prefix in ipairs(res[2]) do
      redis.call('DEL', state_key(guild_id, prefix))
    end
  until cursor == "0"

  redis.call('DEL', pk)
  redis.call('SET', stats_prefix_key(guild_id), 0)
  redis.call('SET', stats_msg_key(guild_id), 0)
  redis.call('SET', stats_bytes_key(guild_id), 0)
  redis.call('DEL', media_key(guild_id, "gif"))
  redis.call('DEL', media_key(guild_id, "image"))
  redis.call('DEL', media_key(guild_id, "video"))
  return 1
end

-- ---------------------------------------------------------------------------
-- Config cache  (Redis hash at config:<guild_id>)
-- Fields are snake_case strings mirroring ChainConfig.
-- ---------------------------------------------------------------------------

-- set_config  KEYS[1]=guild_id  ARGV = flat field/value pairs
local function set_config(keys, args)
  local ck = config_key(keys[1])
  -- HSET accepts variadic field/value pairs in Redis 4+
  local hset_args = { ck }
  for i = 1, #args do
    table.insert(hset_args, args[i])
  end
  redis.call(table.unpack({ "HSET", table.unpack(hset_args) }))
  return 1
end

-- get_config  KEYS[1]=guild_id → flat {field, value, ...} or {} on miss
local function get_config(keys, _args)
  return redis.call('HGETALL', config_key(keys[1]))
end

-- delete_config  KEYS[1]=guild_id
local function delete_config(keys, _args)
  redis.call('DEL', config_key(keys[1]))
  return 1
end

-- ---------------------------------------------------------------------------
-- Media helpers (interface unchanged)
-- ---------------------------------------------------------------------------
local function add_media(keys, args)
  redis.call('SADD', media_key(keys[1], args[1]), args[2])
  return 1
end

local function remove_media(keys, args)
  redis.call('SREM', media_key(keys[1], args[1]), args[2])
  return 1
end

local function get_random_media(keys, args)
  return redis.call('SRANDMEMBER', media_key(keys[1], args[1])) or ""
end

local function get_media_counts(keys, _args)
  local guild_id = keys[1]
  return {
    redis.call('SCARD', media_key(guild_id, "gif")),
    redis.call('SCARD', media_key(guild_id, "image")),
    redis.call('SCARD', media_key(guild_id, "video")),
  }
end

-- Backwards-compat alias; prefer reconcile_bytes for new callers.
local function get_guild_memory_usage(keys, args)
  return reconcile_bytes(keys, args)
end

-- ---------------------------------------------------------------------------
-- Registration
-- ---------------------------------------------------------------------------
redis.register_function('train_markov', train_markov)
redis.register_function('train_batch', train_batch)
redis.register_function('count_message', count_message)
redis.register_function('find_prefix', find_prefix)
redis.register_function('generate_markov', generate_markov)
redis.register_function('delete_markov', delete_markov)
redis.register_function('get_stats_markov', get_stats_markov)
redis.register_function('reconcile_bytes', reconcile_bytes)
redis.register_function('clear_guild', clear_guild)
redis.register_function('set_config', set_config)
redis.register_function('get_config', get_config)
redis.register_function('delete_config', delete_config)
redis.register_function('add_media', add_media)
redis.register_function('remove_media', remove_media)
redis.register_function('get_random_media', get_random_media)
redis.register_function('get_media_counts', get_media_counts)
redis.register_function('get_guild_memory_usage', get_guild_memory_usage)
