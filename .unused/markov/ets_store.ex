defmodule Rolando.Markov.ETSStore do
  @moduledoc false

  @table :rolando_markov_edges

  def ensure_table do
    case :ets.whereis(@table) do
      :undefined ->
        :ets.new(@table, [:named_table, :public, :set])

      _ ->
        @table
    end
  end

  def reset(guild_id) do
    ensure_table()

    :ets.select_delete(@table, [
      {{{:edge, :"$1", :_, :_}, :_}, [{:==, :"$1", guild_id}], [true]}
    ])

    :ets.delete(@table, {:ready, guild_id})
    :ok
  end

  def incr_pairs(guild_id, pairs) when is_list(pairs) do
    ensure_table()

    Enum.each(pairs, fn {from, to} ->
      fk = Rolando.Markov.Shared.from_hash(from)
      field = Rolando.Markov.Shared.field_token(to)
      key = {:edge, guild_id, fk, field}

      case :ets.lookup(@table, key) do
        [] -> :ets.insert(@table, {key, 1})
        [{_, n}] -> :ets.insert(@table, {key, n + 1})
      end
    end)

    {:ok, :ok}
  end

  def successors(guild_id, from_word) do
    ensure_table()
    fk = Rolando.Markov.Shared.from_hash(from_word)

    counts =
      :ets.match_object(@table, {{:edge, guild_id, fk, :_}, :_})
      |> Enum.reduce(%{}, fn {{:edge, _, _, field}, n}, acc ->
        w = Rolando.Markov.Shared.field_decode(field)
        Map.update(acc, w, n, &(&1 + n))
      end)

    {:ok, counts}
  end

  def mark_ready(guild_id) do
    ensure_table()
    :ets.insert(@table, {{:ready, guild_id}, true})
    {:ok, "OK"}
  end

  def ready?(guild_id) do
    ensure_table()

    case :ets.lookup(@table, {:ready, guild_id}) do
      [{{:ready, ^guild_id}, true}] -> true
      _ -> false
    end
  end
end
