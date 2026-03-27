defmodule RolandoDiscord.Consumers.Event do
  @moduledoc """
  Handles lifecycle and guild/channel events (`READY`, `GUILD_CREATE`, etc.).

  Slash commands and components use dedicated consumers; add `handle_event/1` clauses here.
  """
  use Nostrum.Consumer

  def handle_event(_event) do
    :noop
  end
end
