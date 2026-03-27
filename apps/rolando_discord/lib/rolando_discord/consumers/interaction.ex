defmodule RolandoDiscord.Consumers.Interaction do
  use Nostrum.Consumer
  alias Nostrum.Api
  require Logger

  def handle_event(_), do: :noop

end