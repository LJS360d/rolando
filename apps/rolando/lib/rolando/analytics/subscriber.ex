defmodule Rolando.Analytics.Subscriber do
  @moduledoc false
  use GenServer

  alias Rolando.Analytics
  alias Rolando.Analytics.Sync
  require Logger

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(_opts) do
    Phoenix.PubSub.subscribe(Rolando.PubSub, Sync.topic())
    {:ok, %{}}
  end

  @impl true
  def handle_info({:analytics_event, attrs}, state) do
    case Analytics.persist_event(attrs) do
      {:ok, _} ->
        :ok

      {:error, changeset} ->
        Logger.warning("Analytics persist failed: #{inspect(changeset.errors)}")
    end

    {:noreply, state}
  end

  @impl true
  def handle_info(_, state), do: {:noreply, state}
end
