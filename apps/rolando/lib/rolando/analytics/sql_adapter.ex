defmodule Rolando.Analytics.SQLAdapter do
  @behaviour Rolando.Analytics.Adapter
  alias Rolando.Repo
  alias Rolando.Schema.{AnalyticsEvent}
  import Ecto.Query

  @impl true
  def guilds_count do
    Repo.aggregate(Guild, :count, :id)
  end
end
