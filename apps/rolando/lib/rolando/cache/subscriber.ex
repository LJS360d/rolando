defmodule Rolando.Cache.Subscriber do
  @moduledoc """
  Listens for cache synchronization events from other nodes and
  updates the local adapter directly to avoid broadcast loops.
  """
  use GenServer
  alias Rolando.Cache.Sync
  require Logger

  # Move your adapter selection logic here or use a helper
  defp adapter, do: Application.get_env(:rolando, :cache_adapter, Rolando.Cache.ETSAdapter)

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    # Note: Ensure your Adapter.init() creates the table if it's ETS
    adapter().init()
    Phoenix.PubSub.subscribe(Rolando.PubSub, Sync.topic())
    {:ok, %{}}
  end

  # Handle the generic update message
  @impl true
  def handle_info({:update, table, key, value}, state) do
    # IMPORTANT: call adapter() directly, NOT Rolando.Cache.put(), it would cause an infinite loop
    adapter().put(table, key, value)
    {:noreply, state}
  end

  @impl true
  def handle_info({:delete, table, key}, state) do
    adapter().delete(table, key)
    {:noreply, state}
  end

  @impl true
  def handle_info(_msg, state), do: {:noreply, state}
end
