defmodule Rolando.Messages do
  @moduledoc """
  Messages facade. Delegates to the configured adapter so dev can use SQLite
  and prod can use a high-throughput store (e.g., TimescaleDB, ClickHouse).
  """
  @behaviour Rolando.Messages.Adapter
  @default_adapter Rolando.Messages.EctoAdapter
  require Logger

  defp adapter do
    Application.get_env(:rolando, :messages_adapter, @default_adapter)
  end

  @impl true
  def create(attrs) do
    adapter().create(attrs)
  end

  @impl true
  def create_many(message_list) when is_list(message_list) do
    adapter().create_many(message_list)
  end

  @impl true
  def list_by_guild(guild_id, opts \\ []) do
    adapter().list_by_guild(guild_id, opts)
  end

  @impl true
  def count_by_guild(guild_id) do
    adapter().count_by_guild(guild_id)
  end

  @impl true
  def delete_by_guild(guild_id) do
    adapter().delete_by_guild(guild_id)
  end

  @impl true
  def get_random_messages(guild_id, count) do
    adapter().get_random_messages(guild_id, count)
  end
end
