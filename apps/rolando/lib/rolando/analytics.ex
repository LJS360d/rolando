defmodule Rolando.Analytics do
  @moduledoc """
  Analytics facade. Delegates to the configured adapter so dev can use SQL
  and prod can use a scalable backend (event stream, external service, etc.).
  """
  @default_adapter Rolando.Analytics.SQLAdapter
  @pubsub_topic "analytics_events"

  defp adapter do
    Application.get_env(:rolando, :analytics_adapter, @default_adapter)
  end

  def guilds_count do
    adapter().guilds_count()
  end

  def track_event(attrs) when is_map(attrs) do
    Phoenix.PubSub.broadcast(Rolando.PubSub, @pubsub_topic, {:analytics_event, attrs})
  end

  def pubsub_topic, do: @pubsub_topic
end
