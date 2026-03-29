defmodule Rolando.Analytics.EctoAdapter do
  @behaviour Rolando.Analytics.Adapter
  alias Rolando.Repo
  alias Rolando.Schema.{AnalyticsEvent, Guild}

  @impl true
  def guilds_count do
    Repo.aggregate(Guild, :count, :id)
  end

  @impl true
  def persist_event(attrs) do
    %AnalyticsEvent{}
    |> AnalyticsEvent.changeset(attrs)
    |> Repo.insert()
  end
end
