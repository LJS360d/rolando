defmodule Rolando.Neural.GuildModel do
  @moduledoc """
  GenServer: per-guild GRU weights + lifecycle FSM (tracks quality_message_count).
  """
  use GenServer

  # Client API

  def start_link(guild_id, opts \\ []) do
    GenServer.start_link(__MODULE__, [guild_id: guild_id] ++ opts, name: via_tuple(guild_id))
  end

  def via_tuple(guild_id) do
    {:via, Registry, {Rolando.Neural.GuildRegistry, guild_id}}
  end

  # Server Callbacks

  @impl true
  def init(opts) do
    guild_id = Keyword.fetch!(opts, :guild_id)
    {:ok, %{guild_id: guild_id, quality_message_count: 0}}
  end
end
