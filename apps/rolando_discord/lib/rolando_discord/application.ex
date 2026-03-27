defmodule RolandoDiscord.Application do
  use Application

  @impl true
  def start(_type, _args) do
    children = [
      # TODO: figure out if we can use Horde in dev (local) for MarvokChain or we need a redis for markov chains
      # Horde Registry
      # {Horde.Registry, [name: RolandoDiscord.HordeRegistry, keys: :unique, members: :auto]},
      #  Horde Supervisor
      # {Horde.DynamicSupervisor,
      # [name: RolandoDiscord.HordeSupervisor, strategy: :one_for_one, members: :auto]},

      # Subscriber for resyncing commands
      RolandoDiscord.CommandsResyncSubscriber,

      # Discord Consumers
      # RolandoDiscord.Consumers.Message,   # on message listener
      RolandoDiscord.Consumers.Interaction, # slash commands
      # RolandoDiscord.Consumers.Component  # button components
      # RolandoDiscord.Consumers.Events,    # other discord ws events
    ]

    opts = [strategy: :one_for_one, name: RolandoDiscord.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
