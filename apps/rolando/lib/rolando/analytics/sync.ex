defmodule Rolando.Analytics.Sync do
  @topic "analytics_sync"
  @ui_topic "operator:analytics"

  def topic, do: @topic

  def ui_topic, do: @ui_topic

  def broadcast_event(attrs),
    do: Phoenix.PubSub.broadcast(Rolando.PubSub, topic(), {:analytics_event, attrs})

  def broadcast_ui_refresh do
    Phoenix.PubSub.broadcast(Rolando.PubSub, ui_topic(), :analytics_updated)
  end
end
