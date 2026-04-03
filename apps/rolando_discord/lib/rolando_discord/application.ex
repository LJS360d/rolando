defmodule RolandoDiscord.Application do
  use Application

  @impl true
  def start(_type, _args) do
    children = [
      RolandoDiscord.OperatorBroadcast,
      # Subscriber for resyncing commands
      RolandoDiscord.CommandsResyncSubscriber,
      RolandoDiscord.Consumers.SlashCommand,
      RolandoDiscord.Consumers.Component,
      RolandoDiscord.Consumers.Message,
      RolandoDiscord.Consumers.Event
    ]

    opts = [strategy: :one_for_one, name: RolandoDiscord.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
