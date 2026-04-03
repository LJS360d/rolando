defmodule Rolando.Schema.GuildWeights do
  @moduledoc """
  Schema for guild neural network weights (GRU).
  Replaces the markov chain state storage.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:guild_id, :string, autogenerate: false}
  schema "guild_weights" do
    field :weight_data, :binary
    field :codebook_blob, :binary
    field :version, :integer, default: 1
    field :perplexity, :float
    timestamps()
  end

  def changeset(guild_weights, attrs) do
    guild_weights
    |> cast(attrs, [
      :guild_id,
      :weight_data,
      :codebook_blob,
      :version,
      :perplexity,
      :inserted_at,
      :updated_at
    ])
    |> validate_required([:guild_id])
    |> validate_number(:version, greater_than: 0)
    |> validate_number(:perplexity, greater_than_or_equal_to: 0)
  end
end
