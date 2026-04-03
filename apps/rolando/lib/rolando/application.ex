defmodule Rolando.Application do
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    Rolando.Markov.ETSStore.ensure_table()

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

      {Task.Supervisor, name: Rolando.TaskSupervisor},

      Rolando.Cache.Subscriber
    ] ++ redis_markov_children()

    Supervisor.start_link(children, strategy: :one_for_one, name: Rolando.Supervisor)
  end

  defp skip_migrations? do
    System.get_env("RELEASE_NAME") == nil
  end

  defp redis_markov_children do
    case Application.get_env(:rolando, :markov_store) do
      :redis ->
        case Application.get_env(:rolando, :redis_url) do
          url when is_binary(url) and url != "" ->
            [Rolando.Markov.RedisStore.child_spec(url)]

          _ ->
            []
        end

      _ ->
        []
    end
  end
end
