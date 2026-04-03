defmodule Rolando.Persistence.SqliteAdapter do
  @moduledoc """
  SQLite adapter for persisting guild weights and media.
  Uses the existing Rolando.Repo for database operations.
  """

  import Ecto.Query

  alias Rolando.Repo
  alias Rolando.Schema.{GuildWeights, MediaStore}

  @behaviour Rolando.Persistence.StorageProvider

  @impl true
  def save_weight(guild_id, data, opts \\ []) do
    precision_mode = Keyword.get(opts, :precision_mode, :standard)
    tier = Keyword.get(opts, :tier, :standard)

    case Repo.get(GuildWeights, guild_id) do
      nil ->
        %GuildWeights{guild_id: guild_id}
        |> GuildWeights.changeset(%{
          guild_id: guild_id,
          weight_data: data,
          precision_mode: precision_mode,
          tier: tier,
          version: 1,
          inserted_at: DateTime.utc_now(),
          updated_at: DateTime.utc_now()
        })
        |> Repo.insert()

      existing ->
        new_version = existing.version + 1

        existing
        |> GuildWeights.changeset(%{
          weight_data: data,
          version: new_version,
          updated_at: DateTime.utc_now()
        })
        |> Repo.update()
    end
    |> case do
      {:ok, _} -> :ok
      {:error, reason} -> {:error, reason}
    end
  end

  @impl true
  def load_weight(guild_id) do
    case Repo.get(GuildWeights, guild_id) do
      nil ->
        {:error, :not_found}

      %GuildWeights{weight_data: nil} ->
        {:error, :not_found}

      %GuildWeights{weight_data: data} ->
        {:ok, data}
    end
  end

  @impl true
  def list_weights do
    weights = Repo.all(GuildWeights)
    {:ok, Enum.map(weights, fn w -> {w.guild_id, w.weight_data} end)}
  end

  @impl true
  def delete_weight(guild_id) do
    case Repo.get(GuildWeights, guild_id) do
      nil ->
        :ok

      weight ->
        Repo.delete(weight)
        :ok
    end
  end

  @impl true
  def save_media(media) do
    %MediaStore{}
    |> MediaStore.changeset(%{
      guild_id: media.guild_id,
      url: media.url,
      media_type: media.media_type,
    })
    |> Repo.insert()
    |> case do
      {:ok, record} -> {:ok, record.id}
      {:error, reason} -> {:error, reason}
    end
  end

  @impl true
  def list_media(guild_id, opts \\ []) do
    limit = Keyword.get(opts, :limit, 100)
    media_type = Keyword.get(opts, :media_type)

    query = from m in MediaStore, where: m.guild_id == ^guild_id

    query =
      if media_type do
        from m in query, where: m.media_type == ^media_type
      else
        query
      end

    query =
      from m in query,
        order_by: [desc: m.inserted_at],
        limit: ^limit

    {:ok, Repo.all(query)}
  end

  @impl true
  def delete_media(id) do
    case Repo.get(MediaStore, id) do
      nil ->
        :ok

      media ->
        Repo.delete(media)
        :ok
    end
  end
end
