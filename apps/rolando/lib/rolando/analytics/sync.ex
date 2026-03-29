defmodule Rolando.Analytics.Sync do
  @topic "analytics_sync"

  def topic, do: @topic

  def broadcast_event(attrs),
    do: Phoenix.PubSub.broadcast(Rolando.PubSub, topic(), {:analytics_event, attrs})
end
