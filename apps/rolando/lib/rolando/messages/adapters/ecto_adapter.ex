defmodule Rolando.Messages.EctoAdapter do
  @moduledoc """
  Ecto adapter for message storage using SQLite.
  """
  @behaviour Rolando.Messages.Adapter

  import Ecto.Query

  alias Rolando.Repo
  alias Rolando.Schema.Message

  @impl true
  def create(attrs) do
    %Message{}
    |> Message.changeset(attrs)
    |> Repo.insert()
  end

  @impl true
  def create_many(message_list) when is_list(message_list) do
    Repo.insert_all(Message, message_list, returning: true)
  end

  @impl true
  def list_by_guild(guild_id, opts \\ []) do
    limit = Keyword.get(opts, :limit, 1000)
    offset = Keyword.get(opts, :offset, 0)

    Message
    |> where([m], m.guild_id == ^guild_id)
    |> order_by([m], asc: :inserted_at)
    |> limit(^limit)
    |> offset(^offset)
    |> Repo.all()
  end

  @impl true
  def count_by_guild(guild_id) do
    Message
    |> where([m], m.guild_id == ^guild_id)
    |> Repo.aggregate(:count, :id)
  end

  @impl true
  def delete_by_guild(guild_id) do
    Message
    |> where([m], m.guild_id == ^guild_id)
    |> Repo.delete_all()
  end

  @impl true
  def get_random_messages(guild_id, count) do
    # SQLite doesn't have true random, so we use order_by random via fragment
    # For production with other DBs, this could use ORDER BY RANDOM()
    Message
    |> where([m], m.guild_id == ^guild_id)
    |> limit(^count)
    |> Repo.all()
    |> Enum.shuffle()
    |> Enum.take(count)
  end
end
