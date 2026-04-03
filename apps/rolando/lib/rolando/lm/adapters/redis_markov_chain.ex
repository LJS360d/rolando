defmodule Rolando.LM.Adapters.RedisMarkovChain do
  @moduledoc """
  Redis-backed Markov chain adapter.
  See `container/redis_markov.lua` for the Redis loaded functions.

  Key layout
  ----------
    markov:{guild_id}:state:{prefix}   – HASH   next_word → weight
    markov:{guild_id}:prefixes         – SET    all known prefix strings
    stats:{guild_id}:unique_prefixes   – STRING integer
    stats:{guild_id}:message_count     – STRING integer
    config:{guild_id}                  – HASH   n_gram_size
  """

  @behaviour Rolando.LM.Adapter

  @connection :redix_client

  @default_n_gram_size 2
  @default_length 20

  # ---------------------------------------------------------------------------
  # Behaviour callbacks
  # ---------------------------------------------------------------------------

  @impl true
  def train(guild_id, text, opts \\ []) do
    if String.starts_with?(text, "https://") do
      :ok
    else
      guild_id_str = to_string(guild_id)
      n_gram_size = Keyword.get(opts, :n_gram_size, @default_n_gram_size)
      tokens = tokenize(text)

      if length(tokens) >= n_gram_size do
        ngram_commands =
          tokens
          |> Enum.chunk_every(n_gram_size, 1, :discard)
          |> Enum.map(fn chunk ->
            {prefix_tokens, [next_word]} = Enum.split(chunk, n_gram_size - 1)
            prefix_key = Enum.join(prefix_tokens, " ")
            ["FCALL", "train_markov", "1", guild_id_str, prefix_key, next_word]
          end)

        config_command = [
          "HSETNX",
          config_key(guild_id_str),
          "n_gram_size",
          to_string(n_gram_size)
        ]

        counter_command = ["FCALL", "count_message", "1", guild_id_str]

        case Redix.pipeline(@connection, ngram_commands ++ [config_command, counter_command]) do
          {:ok, _} -> :ok
          {:error, reason} -> {:error, reason}
        end
      else
        :ok
      end
    end
  end

  @impl true
  def train_batch(guild_id, messages, opts \\ []) when is_list(messages) do
    guild_id_str = to_string(guild_id)
    n_gram_size = Keyword.get(opts, :n_gram_size, @default_n_gram_size)

    config_command = ["HSETNX", config_key(guild_id_str), "n_gram_size", to_string(n_gram_size)]

    message_commands =
      messages
      |> Enum.reject(&String.starts_with?(&1, "https://"))
      |> Enum.flat_map(fn text ->
        tokens = tokenize(text)

        if length(tokens) >= n_gram_size do
          ngram_cmds =
            tokens
            |> Enum.chunk_every(n_gram_size, 1, :discard)
            |> Enum.map(fn chunk ->
              {prefix_tokens, [next_word]} = Enum.split(chunk, n_gram_size - 1)
              prefix_key = Enum.join(prefix_tokens, " ")
              ["FCALL", "train_markov", "1", guild_id_str, prefix_key, next_word]
            end)

          counter_cmd = ["FCALL", "count_message", "1", guild_id_str]
          ngram_cmds ++ [counter_cmd]
        else
          []
        end
      end)

    if message_commands == [] do
      :ok
    else
      case Redix.pipeline(@connection, [config_command | message_commands]) do
        {:ok, _} -> :ok
        {:error, reason} -> {:error, reason}
      end
    end
  end

  @impl true
  def change_tier(guild_id, new_size, messages) when new_size >= 2 do
    guild_id_str = to_string(guild_id)

    with {:ok, _} <- Redix.command(@connection, ["FCALL", "clear_guild", "1", guild_id_str]),
         {:ok, _} <-
           Redix.command(@connection, [
             "HSET",
             config_key(guild_id_str),
             "n_gram_size",
             to_string(new_size)
           ]) do
      train_batch(guild_id_str, messages, n_gram_size: new_size)
    end
  end

  def change_tier(_guild_id, _new_size, _messages), do: {:error, :invalid_n_gram_size}

  @impl true
  def generate(guild_id) do
    generate(guild_id, nil, @default_length)
  end

  @impl true
  def generate(guild_id, seed) do
    generate(guild_id, seed, @default_length)
  end

  @impl true
  def generate(guild_id, seed, length) do
    guild_id_str = to_string(guild_id)
    length = length || @default_length

    with {:ok, n_gram_size} <- fetch_n_gram_size(guild_id_str),
         {:ok, starting_prefix} <- find_starting_prefix(guild_id_str, seed || ""),
         {:ok, text} <- run_generate(guild_id_str, starting_prefix, length, n_gram_size) do
      case String.trim(text) do
        "" -> {:error, :no_data}
        result -> {:ok, result}
      end
    end
  end

  @impl true
  def get_stats(guild_id) do
    case Redix.command(@connection, ["FCALL", "get_stats_markov", "1", to_string(guild_id)]) do
      {:ok, [up, mc]} -> {:ok, %{unique_prefixes: parse_int(up), message_count: parse_int(mc)}}
      {:ok, _unexpected} -> {:ok, %{unique_prefixes: 0, message_count: 0}}
      {:error, reason} -> {:error, reason}
    end
  end

  @impl true
  def delete_message(guild_id, text) do
    if String.starts_with?(text, "https://") do
      :ok
    else
      guild_id_str = to_string(guild_id)

      with {:ok, n_gram_size} <- fetch_n_gram_size(guild_id_str) do
        tokens = tokenize(text)

        if length(tokens) >= n_gram_size do
          commands =
            tokens
            |> Enum.chunk_every(n_gram_size, 1, :discard)
            |> Enum.map(fn chunk ->
              {prefix_tokens, [next_word]} = Enum.split(chunk, n_gram_size - 1)
              prefix_key = Enum.join(prefix_tokens, " ")
              ["FCALL", "delete_markov", "1", guild_id_str, prefix_key, next_word]
            end)

          case Redix.pipeline(@connection, commands) do
            {:ok, _} -> :ok
            {:error, reason} -> {:error, reason}
          end
        else
          :ok
        end
      end
    end
  end

  @impl true
  def delete_guild(guild_id) do
    case Redix.command(@connection, ["FCALL", "clear_guild", "1", to_string(guild_id)]) do
      {:ok, _} -> :ok
      {:error, reason} -> {:error, reason}
    end
  end

  @doc """
  Gets the current n_gram_size (cohesion) value for a guild from Redis.
  """
  def get_n_gram_size(guild_id) do
    fetch_n_gram_size(to_string(guild_id))
  end

  @doc """
  Updates the n_gram_size (cohesion) value for a guild in Redis.
  """
  def update_n_gram_size(guild_id, n_gram_size) when n_gram_size >= 2 and n_gram_size <= 10 do
    guild_id_str = to_string(guild_id)

    case Redix.command(@connection, [
           "HSET",
           config_key(guild_id_str),
           "n_gram_size",
           to_string(n_gram_size)
         ]) do
      {:ok, _} -> {:ok, n_gram_size}
      {:error, reason} -> {:error, reason}
    end
  end

  def update_n_gram_size(_guild_id, _n_gram_size), do: {:error, :invalid_n_gram_size}

  # ---------------------------------------------------------------------------
  # Private helpers
  # ---------------------------------------------------------------------------

  defp find_starting_prefix(guild_id, seed) do
    case Redix.command(@connection, ["FCALL", "find_prefix", "1", guild_id, seed]) do
      {:ok, nil} -> {:error, :no_data}
      {:ok, ""} -> {:error, :no_data}
      {:ok, prefix} -> {:ok, prefix}
      {:error, reason} -> {:error, reason}
    end
  end

  defp run_generate(guild_id, starting_prefix, length, n_gram_size) do
    Redix.command(@connection, [
      "FCALL",
      "generate_markov",
      "1",
      guild_id,
      starting_prefix,
      to_string(length),
      to_string(n_gram_size)
    ])
  end

  def fetch_n_gram_size(guild_id) do
    case Redix.command(@connection, ["HGET", config_key(guild_id), "n_gram_size"]) do
      {:ok, nil} -> {:ok, @default_n_gram_size}
      {:ok, val} -> {:ok, String.to_integer(val)}
      {:error, _} -> {:ok, @default_n_gram_size}
    end
  end

  defp tokenize(text) do
    text
    |> String.split(~r/\s+/, trim: true)
    |> Enum.filter(&(String.length(&1) > 0))
  end

  defp parse_int(nil), do: 0
  defp parse_int(val) when is_integer(val), do: val
  defp parse_int(val) when is_binary(val), do: String.to_integer(val)

  defp config_key(guild_id), do: "config:#{guild_id}"
end
