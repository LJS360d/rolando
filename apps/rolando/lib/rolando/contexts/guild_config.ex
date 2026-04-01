defmodule Rolando.Contexts.GuildConfig do
  @moduledoc """
  Context for managing guild neural network configuration.
  """

  import Ecto.Query

  alias Rolando.Repo
  alias Rolando.Schema.GuildConfig

  @spec get(guild_id :: String.t()) :: {:ok, %GuildConfig{}} | {:error, :not_found}
  def get(guild_id) do
    case Repo.get(GuildConfig, guild_id) do
      nil ->
        {:error, :not_found}

      config ->
        {:ok, config}
    end
  end

  @spec get!(guild_id :: String.t()) :: %GuildConfig{} | nil
  def get!(guild_id) do
    Repo.get(GuildConfig, guild_id)
  end

  @spec get_or_default(guild_id :: String.t()) :: %GuildConfig{}
  def get_or_default(guild_id) do
    case get(guild_id) do
      {:ok, config} -> config
      {:error, :not_found} -> %GuildConfig{guild_id: guild_id}
    end
  end

  @spec create(guild_id :: String.t(), attrs :: map()) ::
          {:ok, %GuildConfig{}} | {:error, Ecto.Changeset.t()}
  def create(guild_id, attrs \\ %{}) do
    attrs = Map.put(attrs, "guild_id", guild_id)

    %GuildConfig{guild_id: guild_id}
    |> GuildConfig.changeset(attrs)
    |> Repo.insert()
  end

  @spec get_or_create(guild_id :: String.t()) ::
          {:ok, %GuildConfig{}} | {:error, Ecto.Changeset.t()}
  def get_or_create(guild_id) do
    case get(guild_id) do
      {:ok, config} ->
        {:ok, config}

      {:error, :not_found} ->
        create(guild_id, %{})
    end
  end

  @spec upsert(guild_id :: String.t(), attrs :: map()) ::
          {:ok, %GuildConfig{}} | {:error, Ecto.Changeset.t()}
  def upsert(guild_id, attrs) do
    attrs = Map.put(attrs, "guild_id", guild_id)

    %GuildConfig{guild_id: guild_id}
    |> GuildConfig.changeset(attrs)
    |> Repo.insert(
      on_conflict:
        {:replace,
         [
           :batch_size,
           :learning_rate,
           :weighted_loss,
           :emoji_skip,
           :filter_pings,
           :filter_bot_authors,
           :tokenizer_model,
           :vector_augment,
           :precision_mode,
           :tier,
           :trained_at,
           :reply_rate,
           :reaction_rate,
           :updated_at
         ]},
      conflict_target: :guild_id,
      returning: true
    )
  end

  @spec update_batch_size(guild_id :: String.t(), batch_size :: pos_integer()) ::
          {:ok, %GuildConfig{}} | {:error, :not_found}
  def update_batch_size(guild_id, batch_size) do
    case get(guild_id) do
      {:ok, config} ->
        config
        |> GuildConfig.changeset(%{batch_size: batch_size, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_learning_rate(guild_id :: String.t(), learning_rate :: float()) ::
          {:ok, %GuildConfig{}} | {:error, :not_found}
  def update_learning_rate(guild_id, learning_rate) do
    case get(guild_id) do
      {:ok, config} ->
        config
        |> GuildConfig.changeset(%{learning_rate: learning_rate, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_precision_mode(guild_id :: String.t(), precision_mode :: atom()) ::
          {:ok, %GuildConfig{}} | {:error, :not_found}
  def update_precision_mode(guild_id, precision_mode) do
    case get(guild_id) do
      {:ok, config} ->
        config
        |> GuildConfig.changeset(%{
          precision_mode: precision_mode,
          updated_at: DateTime.utc_now()
        })
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_tier(guild_id :: String.t(), tier :: atom()) ::
          {:ok, %GuildConfig{}} | {:error, :not_found}
  def update_tier(guild_id, tier) do
    case get(guild_id) do
      {:ok, config} ->
        config
        |> GuildConfig.changeset(%{tier: tier, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_trained_at(guild_id :: String.t(), trained_at :: DateTime.t() | nil) ::
          {:ok, %GuildConfig{}} | {:error, :not_found}
  def update_trained_at(guild_id, trained_at) do
    case get(guild_id) do
      {:ok, config} ->
        config
        |> GuildConfig.changeset(%{trained_at: trained_at, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec delete(guild_id :: String.t()) :: {:ok, %GuildConfig{}} | {:error, :not_found}
  def delete(guild_id) do
    case get(guild_id) do
      {:ok, config} ->
        Repo.delete(config)

      error ->
        error
    end
  end

  @spec list_all :: [%GuildConfig{}]
  def list_all do
    Repo.all(GuildConfig)
  end

  @spec list_by_tier(tier :: atom()) :: [%GuildConfig{}]
  def list_by_tier(tier) do
    GuildConfig
    |> where([c], c.tier == ^tier)
    |> Repo.all()
  end

  @spec count :: non_neg_integer()
  def count do
    Repo.aggregate(GuildConfig, :count, :guild_id)
  end
end
