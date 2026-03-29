defmodule Rolando.Cache do
  @moduledoc """
  Cache facade that handles both adapter delegation AND
  distributed synchronization via PubSub.
  """
  @behaviour Rolando.Cache.Adapter

  alias Rolando.Cache.Sync

  defp adapter, do: Application.get_env(:rolando, :cache_adapter, Rolando.Cache.ETSAdapter)

  @impl true
  def init() do
    adapter().init()
  end

  @impl true
  def get(table, key), do: adapter().get(table, key)

  @impl true
  def put(table, key, value) do
    with :ok <- adapter().put(table, key, value) do
      Sync.broadcast_update(table, key, value)
      :ok
    end
  end

  @impl true
  def delete(table, key) do
    with :ok <- adapter().delete(table, key) do
      Sync.broadcast_delete(table, key)
      :ok
    end
  end
end
