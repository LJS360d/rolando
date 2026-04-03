defmodule Rolando.Broadcast do
  @moduledoc false

  @topic "operator:broadcast"

  def topic, do: @topic

  def publish(envelope) when is_map(envelope) do
    Phoenix.PubSub.broadcast(Rolando.PubSub, @topic, {:operator_broadcast, envelope})
  end
end
