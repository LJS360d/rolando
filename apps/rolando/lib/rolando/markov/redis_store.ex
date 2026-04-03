defmodule Rolando.Markov.RedisStore do
  @moduledoc false

  @prefix "rolando:mv"

  def child_spec(url) when is_binary(url) do
    {Redix, {url, [name: Rolando.Redix]}}
  end

  def reset(guild_id) do
    pattern = "#{@prefix}:#{guild_id}:*"

    case Redix.command(Rolando.Redix, ["KEYS", pattern]) do
      {:ok, []} ->
        :ok

      {:ok, keys} when is_list(keys) ->
        _ = Redix.command(Rolando.Redix, Enum.concat(["DEL"], keys))
        :ok

      {:error, _} = e ->
        e
    end
  end

  def incr_pairs(guild_id, pairs) when is_list(pairs) do
    cmds =
      Enum.map(pairs, fn {from, to} ->
        fk = Rolando.Markov.Shared.from_hash(from)
        field = Rolando.Markov.Shared.field_token(to)
        ["HINCRBY", out_key(guild_id, fk), field, "1"]
      end)

    case cmds do
      [] -> {:ok, :noop}
      _ -> Redix.pipeline(Rolando.Redix, cmds)
    end
  end

  def successors(guild_id, from_word) do
    fk = Rolando.Markov.Shared.from_hash(from_word)

    case Redix.command(Rolando.Redix, ["HGETALL", out_key(guild_id, fk)]) do
      {:ok, []} -> {:ok, %{}}
      {:ok, list} when is_list(list) -> {:ok, pairs_to_map(list)}
      {:error, _} = e -> e
    end
  end

  def mark_ready(guild_id) do
    Redix.command(Rolando.Redix, ["SET", ready_key(guild_id), "1"])
  end

  def ready?(guild_id) do
    case Redix.command(Rolando.Redix, ["EXISTS", ready_key(guild_id)]) do
      {:ok, n} when n in [1, "1"] -> true
      _ -> false
    end
  end

  defp out_key(guild_id, from_hash), do: "#{@prefix}:#{guild_id}:o:#{from_hash}"

  defp ready_key(guild_id), do: "#{@prefix}:#{guild_id}:ready"

  defp pairs_to_map([]), do: %{}

  defp pairs_to_map([k, v | rest]) do
    n = String.to_integer(v)
    Map.put(pairs_to_map(rest), k, n)
  end
end
