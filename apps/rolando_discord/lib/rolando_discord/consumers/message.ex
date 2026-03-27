defmodule RolandoDiscord.Consumers.Message do
  @moduledoc """
  Handles `MESSAGE_CREATE` (and related message events when added).

  Heavy work should be delegated to `Rolando.TaskSupervisor` or a dedicated pool so
  this process stays responsive across all guilds/shards.
  """
  use Nostrum.Consumer

  def handle_event({:MESSAGE_CREATE, _msg, _ws_state}) do
    :noop
  end

  def handle_event(_event) do
    :noop
  end
end
