defmodule Rolando.Neural.GuildSupervisor do
  @moduledoc """
  DynamicSupervisor for guild GenServers.
  """
  use DynamicSupervisor

  # Client API

  def start_link(opts \\ []) do
    DynamicSupervisor.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def start_guild(guild_id) do
    spec = {Rolando.Neural.GuildModel, guild_id}
    DynamicSupervisor.start_child(__MODULE__, spec)
  end

  def stop_guild(guild_id) do
    case Registry.lookup(Rolando.Neural.GuildRegistry, guild_id) do
      [{pid, _}] ->
        DynamicSupervisor.terminate_child(__MODULE__, pid)

      [] ->
        :ok
    end
  end

  def list_guilds do
    DynamicSupervisor.which_children(__MODULE__)
    |> Enum.map(fn {_, pid, _, _} -> pid end)
  end

  # Server Callbacks

  @impl true
  def init(opts) do
    DynamicSupervisor.init(strategy: :one_for_one, extra_arguments: opts)
  end
end
