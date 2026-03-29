defmodule Rolando.Contexts.SharedWeights do
  @moduledoc """
  Context for managing system-wide shared neural network weights.
  These are the frozen embedding and output projection initialized once at startup.
  """

  alias Rolando.Repo
  alias Rolando.Schema.SharedWeights

  @shared_weights_id 1

  @spec get :: {:ok, %SharedWeights{}} | {:error, :not_found}
  def get do
    case Repo.get(SharedWeights, @shared_weights_id) do
      nil ->
        {:error, :not_found}

      weights ->
        {:ok, weights}
    end
  end

  @spec get! :: %SharedWeights{} | nil
  def get! do
    Repo.get(SharedWeights, @shared_weights_id)
  end

  @spec exists? :: boolean()
  def exists? do
    not is_nil(get!())
  end

  @spec initialize(embedding_data :: binary(), projection_data :: binary(), tier :: atom()) ::
          {:ok, %SharedWeights{}} | {:error, Ecto.Changeset.t()}
  def initialize(embedding_data, projection_data, tier \\ :standard) do
    case get() do
      {:ok, existing} ->
        {:ok, existing}

      {:error, :not_found} ->
        %SharedWeights{id: @shared_weights_id}
        |> SharedWeights.changeset(%{
          id: @shared_weights_id,
          embedding_data: embedding_data,
          projection_data: projection_data,
          tier: tier,
          inserted_at: DateTime.utc_now(),
          updated_at: DateTime.utc_now()
        })
        |> Repo.insert()
    end
  end

  @spec update(embedding_data :: binary(), projection_data :: binary()) ::
          {:ok, %SharedWeights{}} | {:error, :not_found}
  def update(embedding_data, projection_data) do
    case get() do
      {:ok, weights} ->
        weights
        |> SharedWeights.changeset(%{
          embedding_data: embedding_data,
          projection_data: projection_data,
          updated_at: DateTime.utc_now()
        })
        |> Repo.update()

      error ->
        error
    end
  end

  @spec get_embedding :: {:ok, binary()} | {:error, :not_found}
  def get_embedding do
    case get() do
      {:ok, weights} ->
        {:ok, weights.embedding_data}

      error ->
        error
    end
  end

  @spec get_projection :: {:ok, binary()} | {:error, :not_found}
  def get_projection do
    case get() do
      {:ok, weights} ->
        {:ok, weights.projection_data}

      error ->
        error
    end
  end

  @spec get_tier :: {:ok, atom()} | {:error, :not_found}
  def get_tier do
    case get() do
      {:ok, weights} ->
        {:ok, weights.tier}

      error ->
        error
    end
  end

  @spec delete :: :ok | {:error, term()}
  def delete do
    case get() do
      {:ok, weights} ->
        Repo.delete(weights)

      {:error, :not_found} ->
        :ok
    end
  end
end
