defmodule Rolando.Neural.SharedWeights do
  @moduledoc """
  GenServer: loads frozen embedding + output projection at startup, never updated.
  """
  use GenServer

  # Client API

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  # Server Callbacks

  @impl true
  def init(_opts) do
    {:ok, %{}}
  end
end
