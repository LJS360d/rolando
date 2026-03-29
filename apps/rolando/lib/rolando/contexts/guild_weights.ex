defmodule Rolando.Contexts.GuildWeights do
  @moduledoc """
  Context for managing guild neural network weights (GRU).
  """

  alias Rolando.Repo
  alias Rolando.Schema.GuildWeights

  @spec get(guild_id :: String.t()) :: {:ok, %GuildWeights{}} | {:error, :not_found}
  def get(guild_id) do
    case Repo.get(GuildWeights, guild_id) do
      nil ->
        {:error, :not_found}

      weights ->
        {:ok, weights}
    end
  end

  @spec get!(guild_id :: String.t()) :: %GuildWeights{} | nil
  def get!(guild_id) do
    Repo.get(GuildWeights, guild_id)
  end

  @spec create(guild_id :: String.t(), attrs :: map()) ::
          {:ok, %GuildWeights{}} | {:error, Ecto.Changeset.t()}
  def create(guild_id, attrs \\ %{}) do
    attrs = Map.put(attrs, "guild_id", guild_id)

    %GuildWeights{guild_id: guild_id}
    |> GuildWeights.changeset(attrs)
    |> Repo.insert()
  end

  @spec upsert(guild_id :: String.t(), attrs :: map()) ::
          {:ok, %GuildWeights{}} | {:error, Ecto.Changeset.t()}
  def upsert(guild_id, attrs) do
    attrs = Map.put(attrs, "guild_id", guild_id)

    %GuildWeights{guild_id: guild_id}
    |> GuildWeights.changeset(attrs)
    |> Repo.insert(
      on_conflict: {:replace, [:weight_data, :version, :perplexity, :updated_at]},
      conflict_target: :guild_id,
      returning: true
    )
  end

  @spec update_version(guild_id :: String.t()) :: {:ok, %GuildWeights{}} | {:error, :not_found}
  def update_version(guild_id) do
    case get(guild_id) do
      {:ok, weights} ->
        new_version = weights.version + 1

        weights
        |> GuildWeights.changeset(%{version: new_version, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_perplexity(guild_id :: String.t(), perplexity :: float()) ::
          {:ok, %GuildWeights{}} | {:error, :not_found}
  def update_perplexity(guild_id, perplexity) do
    case get(guild_id) do
      {:ok, weights} ->
        weights
        |> GuildWeights.changeset(%{perplexity: perplexity, updated_at: DateTime.utc_now()})
        |> Repo.update()

      error ->
        error
    end
  end

  @spec update_weights(guild_id :: String.t(), weight_data :: binary()) ::
          {:ok, %GuildWeights{}} | {:error, :not_found}
  def update_weights(guild_id, weight_data) do
    case get(guild_id) do
      {:ok, weights} ->
        new_version = weights.version + 1

        weights
        |> GuildWeights.changeset(%{
          weight_data: weight_data,
          version: new_version,
          updated_at: DateTime.utc_now()
        })
        |> Repo.update()

      error ->
        error
    end
  end

  @spec delete(guild_id :: String.t()) :: {:ok, %GuildWeights{}} | {:error, :not_found}
  def delete(guild_id) do
    case get(guild_id) do
      {:ok, weights} ->
        Repo.delete(weights)

      error ->
        error
    end
  end

  @spec list_all :: [%GuildWeights{}]
  def list_all do
    Repo.all(GuildWeights)
  end

  @spec count :: non_neg_integer()
  def count do
    Repo.aggregate(GuildWeights, :count, :guild_id)
  end
end
