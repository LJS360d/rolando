defmodule Rolando.Cache.ETSAdapter do
  @behaviour Rolando.Cache.Adapter
  @tables [:guild_cache]

  @impl true
  def init do
    for table <- @tables do
      :ets.new(table, [:set, :public, :named_table, {:read_concurrency, true}])
    end

    :ok
  end

  @impl true
  def get(table, id) do
    case :ets.lookup(table, id) do
      [{^id, val}] -> {:ok, val}
      [] -> {:error, :not_found}
    end
  end

  @impl true
  def put(table, id, val), do: :ets.insert(table, {id, val}) |> then(fn _ -> :ok end)

  @impl true
  def delete(table, id), do: :ets.delete(table, id) |> then(fn _ -> :ok end)
end
