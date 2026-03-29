defmodule Rolando.Application do
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    children = [
      # Database
      Rolando.Repo,
      {Ecto.Migrator,
       repos: Application.fetch_env!(:rolando, :ecto_repos), skip: skip_migrations?()},

      # Clustering
      {DNSCluster, query: Application.get_env(:rolando, :dns_cluster_query) || :ignore},
      {Phoenix.PubSub, name: Rolando.PubSub},

      # Neural Network - Shared weights (initialized once at startup, frozen)
      {Registry, keys: :unique, name: Rolando.Neural.GuildRegistry},
      {Registry, keys: :unique, name: Rolando.Neural.SharedWeightsRegistry},

      # Neural Network - Per-guild model supervisors
      {DynamicSupervisor, strategy: :one_for_one, name: Rolando.Neural.GuildSupervisor},

      # Training - Pool worker for CPU-intensive training steps
      {Registry, keys: :unique, name: Rolando.Training.PoolRegistry},
      {Task.Supervisor, name: Rolando.TaskSupervisor},

      # Cluster Sync Subscribers
      Rolando.Analytics.Subscriber,
      Rolando.Cache.Subscriber
    ]

    Supervisor.start_link(children, strategy: :one_for_one, name: Rolando.Supervisor)
  end

  defp skip_migrations? do
    System.get_env("RELEASE_NAME") == nil
  end
end
