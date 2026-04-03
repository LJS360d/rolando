defmodule Rolando.Contexts.Guilds do
  import Ecto.Query
  alias Rolando.Repo
  alias Rolando.Cache
  alias Rolando.Analytics
  alias Rolando.Schema.{Guild, GuildConfig, GuildWeights}

  @cache_table :guild_cache

  @spec get_or_create(guild :: %Guild{}) :: {:ok, %Guild{}} | {:error, Ecto.Changeset.t()}
  def get_or_create(guild) do
    # Try to get from cache first
    guild_id = to_string(guild.id)

    case Cache.get(@cache_table, guild_id) do
      {:ok, cached_guild} ->
        {:ok, cached_guild}

      {:error, :not_found} ->
        # Not in cache, try DB
        case Repo.get(Guild, guild_id) do
          nil ->
            # Not in DB, create it
            guild_attrs =
              if is_map(guild) and not is_struct(guild), do: guild, else: Map.from_struct(guild)

            case Guild.changeset(%Guild{}, guild_attrs)
                 |> Repo.insert(returning: true) do
              {:ok, new_guild} ->
                Cache.put(@cache_table, to_string(new_guild.id), new_guild)
                Analytics.track("guild_created", %{id: to_string(new_guild.id)})
                {:ok, new_guild}

              {:error, changeset} ->
                Analytics.track("guild_create_failed", %{id: guild.id, reason: :validation_error})
                {:error, changeset}
            end

          existing_guild ->
            # Found in DB, put in cache and return
            Cache.put(@cache_table, to_string(existing_guild.id), existing_guild)
            Analytics.track("guild_retrieved_from_db", %{id: to_string(existing_guild.id)})
            {:ok, existing_guild}
        end
    end
  end

  @spec upsert(guild :: %Guild{}) :: {:ok, %Guild{}} | {:error, Ecto.Changeset.t()}
  def upsert(%Guild{} = guild) do
    fields_to_replace = Guild.__schema__(:fields) -- [:id, :inserted_at]

    case Repo.insert(
           %Guild{}
           |> Guild.changeset(Map.from_struct(guild))
           |> Repo.insert(
             on_conflict: {:replace, fields_to_replace},
             conflict_target: :id,
             returning: true
           )
         ) do
      {:ok, updated_guild} ->
        Cache.put(@cache_table, updated_guild.id, updated_guild)
        Analytics.track("guild_upserted", %{id: updated_guild.id})
        {:ok, updated_guild}

      {:error, changeset} ->
        Analytics.track("guild_upsert_failed", %{id: guild.id, reason: :validation_error})
        {:error, changeset}
    end
  end

  @spec get(guild_id :: String.t()) :: {:ok, %Guild{}} | {:error, :not_found}
  def get(guild_id) do
    case Cache.get(@cache_table, guild_id) do
      {:ok, cached_guild} ->
        Analytics.track("guild_retrieved_from_cache", %{id: guild_id})
        {:ok, cached_guild}

      {:error, :not_found} ->
        case Repo.get(Guild, guild_id) do
          nil ->
            Analytics.track("guild_not_found", %{id: guild_id})
            {:error, :not_found}

          guild ->
            Cache.put(@cache_table, guild_id, guild)
            Analytics.track("guild_retrieved_from_db", %{id: guild_id})
            {:ok, guild}
        end
    end
  end

  @spec list_directory_page(pos_integer(), pos_integer()) :: [%{}]
  def list_directory_page(page, page_size)
      when is_integer(page) and page >= 1 and is_integer(page_size) and page_size >= 1 do
    offset = (page - 1) * page_size

    from(g in Guild,
      left_join: c in GuildConfig,
      on: c.guild_id == g.id,
      left_join: w in GuildWeights,
      on: w.guild_id == g.id,
      order_by: [desc: g.updated_at],
      offset: ^offset,
      limit: ^page_size,
      select: %{
        id: g.id,
        name: g.name,
        platform: g.platform,
        image_url: g.image_url,
        updated_at: g.updated_at,
        trained_at: c.trained_at,
        has_weights: not is_nil(w.guild_id)
      }
    )
    |> Repo.all()
  end

  @spec count_guilds() :: non_neg_integer()
  def count_guilds do
    Repo.aggregate(Guild, :count, :id)
  end

  @spec delete(guild_id :: String.t()) ::
          {:ok, %Guild{}} | {:error, Ecto.Changeset.t()} | {:error, :not_found}
  def delete(guild_id) do
    case Repo.get(Guild, guild_id) do
      nil ->
        Analytics.track("guild_delete_not_found", %{id: guild_id})
        {:error, :not_found}

      guild ->
        case Repo.delete(guild) do
          {:ok, deleted_guild} ->
            Cache.delete(@cache_table, guild_id)
            Analytics.track("guild_deleted", %{id: guild_id})
            {:ok, deleted_guild}

          {:error, changeset} ->
            Analytics.track("guild_delete_failed", %{id: guild_id, reason: :database_error})
            {:error, changeset}
        end
    end
  end
end
