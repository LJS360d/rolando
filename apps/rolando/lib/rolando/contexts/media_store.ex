defmodule Rolando.Contexts.MediaStore do
  @moduledoc """
  Context for managing media extracted from messages.
  """

  import Ecto.Query

  alias Rolando.Repo
  alias Rolando.Schema.MediaStore

  @spec get(id :: integer()) :: {:ok, %MediaStore{}} | {:error, :not_found}
  def get(id) do
    case Repo.get(MediaStore, id) do
      nil ->
        {:error, :not_found}

      media ->
        {:ok, media}
    end
  end

  @spec get!(id :: integer()) :: %MediaStore{} | nil
  def get!(id) do
    Repo.get(MediaStore, id)
  end

  @spec create(attrs :: map()) :: {:ok, %MediaStore{}} | {:error, Ecto.Changeset.t()}
  def create(attrs \\ %{}) do
    %MediaStore{}
    |> MediaStore.changeset(attrs)
    |> Repo.insert()
  end

  @spec create_many([map()]) :: {:ok, [%MediaStore{}]} | {:error, Ecto.Changeset.t()}
  def create_many(media_list) when is_list(media_list) do
    Repo.insert_all(MediaStore, media_list, returning: true)
  end

  @spec list_by_guild(guild_id :: String.t(), limit :: non_neg_integer()) :: [%MediaStore{}]
  def list_by_guild(guild_id, limit \\ 100) do
    MediaStore
    |> where([m], m.guild_id == ^guild_id)
    |> order_by([m], desc: m.inserted_at)
    |> limit(^limit)
    |> Repo.all()
  end

  @spec list_by_type(guild_id :: String.t(), media_type :: atom(), limit :: non_neg_integer()) ::
          [
            %MediaStore{}
          ]
  def list_by_type(guild_id, media_type, limit \\ 100) do
    MediaStore
    |> where([m], m.guild_id == ^guild_id and m.media_type == ^media_type)
    |> order_by([m], desc: m.inserted_at)
    |> limit(^limit)
    |> Repo.all()
  end

  @spec search_by_context(guild_id :: String.t(), context_hash :: String.t()) :: [%MediaStore{}]
  def search_by_context(guild_id, context_hash) do
    MediaStore
    |> where([m], m.guild_id == ^guild_id and m.context_hash == ^context_hash)
    |> Repo.all()
  end

  @spec delete(id :: integer()) :: {:ok, %MediaStore{}} | {:error, :not_found}
  def delete(id) do
    case get(id) do
      {:ok, media} ->
        Repo.delete(media)

      error ->
        error
    end
  end

  @spec delete_by_guild(guild_id :: String.t()) :: {non_neg_integer(), nil | [term()]}
  def delete_by_guild(guild_id) do
    MediaStore
    |> where([m], m.guild_id == ^guild_id)
    |> Repo.delete_all()
  end

  @spec count_by_guild(guild_id :: String.t()) :: non_neg_integer()
  def count_by_guild(guild_id) do
    MediaStore
    |> where([m], m.guild_id == ^guild_id)
    |> Repo.aggregate(:count, :id)
  end

  @spec count :: non_neg_integer()
  def count do
    Repo.aggregate(MediaStore, :count, :id)
  end
end
