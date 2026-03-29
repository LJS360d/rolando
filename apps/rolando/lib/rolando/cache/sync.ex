defmodule Rolando.Cache.Sync do
  @topic "cache_sync"

  def topic, do: @topic

  def broadcast_update(table, key, value),
    do: Phoenix.PubSub.broadcast(Rolando.PubSub, @topic, {:update, table, key, value})

  def broadcast_delete(table, key),
    do: Phoenix.PubSub.broadcast(Rolando.PubSub, @topic, {:delete, table, key})
end
