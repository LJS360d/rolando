#!lua name=markov

-- ---------------------------------------------------------------------------
-- Helpers
-- ---------------------------------------------------------------------------

local function split_tokens(str)
  local tokens = {}
  for token in string.gmatch(str, "%S+") do
    table.insert(tokens, token)
  end
  return tokens
end

-- Key helpers keep naming consistent across all functions.
local function state_key(guild_id, prefix) return "markov:" .. guild_id .. ":state:" .. prefix end
local function prefixes_key(guild_id) return "markov:" .. guild_id .. ":prefixes" end
local function config_key(guild_id) return "config:" .. guild_id end
local function stats_prefix_key(guild_id) return "stats:" .. guild_id .. ":unique_prefixes" end
local function stats_msg_key(guild_id) return "stats:" .. guild_id .. ":message_count" end

-- ---------------------------------------------------------------------------
-- train_markov KEYS[1]=guild_id  ARGV[1]=prefix_key  ARGV[2]=next_word
--
-- Mirrors Go's mc.State[prefixKey][nextWord]++ including the prefix SET and
-- unique-prefix counter.  Message counting is handled by count_message so
-- that the pipeline in Elixir can call it exactly once per message.
-- ---------------------------------------------------------------------------
local function train_markov(keys, args)
  local guild_id  = keys[1]
  local prefix    = args[1]
  local next_word = args[2]

  local sk        = state_key(guild_id, prefix)

  local is_new    = redis.call('EXISTS', sk) == 0

  redis.call('HINCRBY', sk, next_word, 1)

  if is_new then
    redis.call('INCR', stats_prefix_key(guild_id))
    redis.call('SADD', prefixes_key(guild_id), prefix)
  end

  return 1
end

-- ---------------------------------------------------------------------------
-- count_message KEYS[1]=guild_id
--
-- Increments the per-guild message counter once per trained message.
-- Mirrors mc.MessageCounter++ in Go's UpdateState.
-- ---------------------------------------------------------------------------
local function count_message(keys, _args)
  redis.call('INCR', stats_msg_key(keys[1]))
  return 1
end

-- ---------------------------------------------------------------------------
-- find_prefix KEYS[1]=guild_id  ARGV[1]=seed
--
-- Replicates Go's GenerateTextFromSeed prefix-resolution strategy:
--   1. O(1) exact key lookup
--   2. O(N) SSCAN of the prefix SET for a key that contains the seed
--   3. O(1) SRANDMEMBER random fallback
-- Returns the chosen prefix string, or "" when the guild has no data.
-- ---------------------------------------------------------------------------
local function find_prefix(keys, args)
  local guild_id = keys[1]
  local seed     = args[1] or ""
  local pk       = prefixes_key(guild_id)

  if seed ~= "" then
    -- 1. Exact match
    if redis.call('EXISTS', state_key(guild_id, seed)) == 1 then
      return seed
    end

    -- 2. SSCAN for a prefix that contains the seed as a substring
    local cursor   = "0"
    local matching = {}
    repeat
      local res = redis.call('SSCAN', pk, cursor, "COUNT", 100)
      cursor = res[1]
      for _, key in ipairs(res[2]) do
        if string.find(key, seed, 1, true) then
          table.insert(matching, key)
        end
      end
    until cursor == "0"

    if #matching > 0 then
      return matching[math.random(1, #matching)]
    end
  end

  -- 3. Random fallback
  local rand = redis.call('SRANDMEMBER', pk)
  return rand or ""
end

-- ---------------------------------------------------------------------------
-- generate_markov  KEYS[1]=guild_id
--                  ARGV[1]=start_prefix  ARGV[2]=max_length  ARGV[3]=n_gram_size
--
-- Mirrors Go's generateText: backoff loop + weighted stochastic choice +
-- sliding window update.  n_gram_size drives the window width so that a
-- 3-gram model keeps a 2-token context, a 2-gram keeps 1-token, etc.
-- ---------------------------------------------------------------------------
local function generate_markov(keys, args)
  local guild_id     = keys[1]
  local start_prefix = args[1] or ""
  local max_length   = tonumber(args[2]) or 20
  local n_gram_size  = tonumber(args[3]) or 2
  local window       = n_gram_size - 1  -- number of prefix tokens to keep

  if start_prefix == "" then return "" end

  local generated = split_tokens(start_prefix)
  local current_prefix = start_prefix

  for _ = 1, max_length do
    -- BACKOFF LOOP -------------------------------------------------------
    -- Mirrors Go: if the full N-1-gram prefix isn't found, drop the
    -- leftmost token and try again until we either hit or run out.
    local backoff = split_tokens(current_prefix)
    local next_words = {}
    local found = false

    while #backoff > 0 do
      local bk = table.concat(backoff, " ")
      local result = redis.call('HGETALL', state_key(guild_id, bk))
      if #result > 0 then
        next_words = result
        found = true
        break
      end
      table.remove(backoff, 1)
    end

    if not found then break end

    -- STOCHASTIC CHOICE --------------------------------------------------
    -- Mirrors Go's stochasticChoice: weighted random selection.
    local total_weight = 0
    for j = 2, #next_words, 2 do
      total_weight = total_weight + tonumber(next_words[j])
    end

    -- Go returns "" and bails when totalWeight == 0; we do the same.
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

    -- SLIDING WINDOW -----------------------------------------------------
    -- Mirrors Go: currentPrefixTokens = generatedTokens[len-window:]
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
-- delete_markov  KEYS[1]=guild_id  ARGV[1]=prefix_key  ARGV[2]=next_word
--
-- Mirrors Go's Delete: decrements the weight for a next_word entry.
-- When the weight reaches zero the field is removed; when the hash empties
-- the state key, prefix SET entry, and unique-prefix counter are cleaned up.
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

  if new_val <= 0 then
    redis.call('HDEL', sk, next_word)

    -- Clean up the prefix entirely if no next-words remain.
    -- Mirrors Go's empty-map cleanup in Delete.
    if redis.call('HLEN', sk) == 0 then
      redis.call('DEL', sk)
      redis.call('SREM', prefixes_key(guild_id), prefix)
      redis.call('DECR', stats_prefix_key(guild_id))
    end
  end

  return 1
end

-- ---------------------------------------------------------------------------
-- get_stats_markov  KEYS[1]=guild_id
--
-- Returns {unique_prefixes, message_count} as a two-element array.
-- ---------------------------------------------------------------------------
local function get_stats_markov(keys, _args)
  local guild_id = keys[1]
  local up = tonumber(redis.call('GET', stats_prefix_key(guild_id)) or "0") or 0
  local mc = tonumber(redis.call('GET', stats_msg_key(guild_id)) or "0") or 0
  return { up, mc }
end

-- ---------------------------------------------------------------------------
-- clear_guild  KEYS[1]=guild_id
--
-- Wipes all Markov state for a guild atomically:
--   - Iterates the prefix SET to DEL every state hash
--   - DELs the prefix SET itself
--   - Resets both stat counters to 0
--   - Leaves the config hash intact (n_gram_size is updated by the caller)
--
-- This is the Redis equivalent of the in-memory reset in Go's ChangeNGramSize:
--   mc.State = make(map[string]map[string]int)
--   mc.MessageCounter = 0
--
-- After calling this the caller must retrain from scratch with the new size.
-- ---------------------------------------------------------------------------
local function clear_guild(keys, _args)
  local guild_id = keys[1]
  local pk       = prefixes_key(guild_id)

  -- Walk the prefix SET in batches and delete each state hash.
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

  return 1
end

-- ---------------------------------------------------------------------------
-- Registration
-- ---------------------------------------------------------------------------
redis.register_function('train_markov', train_markov)
redis.register_function('count_message', count_message)
redis.register_function('find_prefix', find_prefix)
redis.register_function('generate_markov', generate_markov)
redis.register_function('delete_markov', delete_markov)
redis.register_function('get_stats_markov', get_stats_markov)
redis.register_function('clear_guild', clear_guild)
