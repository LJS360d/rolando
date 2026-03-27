defmodule Rolando.Analytics.SQLAdapter do
  @behaviour Rolando.Analytics.Adapter
  alias Rolando.Repo
  alias Rolando.Schema.{AnalyticsEvent, Guild}

  @impl true
  def guilds_count do
    Repo.aggregate(Guild, :count, :id)
  end

  @impl true
  def persist_event(attrs) when is_map(attrs) do
    %AnalyticsEvent{}
    |> AnalyticsEvent.changeset(attrs)
    |> Repo.insert()
  end
end
