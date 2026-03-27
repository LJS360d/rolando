defmodule RolandoDiscord.CommandsResyncSubscriber do
  use GenServer
  alias RolandoDiscord.Commands
  require Logger

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Phoenix.PubSub.subscribe(Rolando.PubSub, "commands_resync")
    {:ok, %{}}
  end

  @impl true
  def handle_info(:resync, state) do
    Logger.info("Commands resync requested via PubSub.")

    case Commands.sync_force() do
      {:ok, _} -> Logger.info("Commands resynced successfully.")
      {:error, reason} -> Logger.error("Commands resync failed: #{inspect(reason)}")
    end

    {:noreply, state}
  end
end
