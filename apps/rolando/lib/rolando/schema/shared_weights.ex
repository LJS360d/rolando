defmodule Rolando.Schema.SharedWeights do
  @moduledoc """
  Schema for system-wide shared neural network weights (frozen embedding + output projection).
  Initialized once at first startup, never updated.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :id, autogenerate: false}
  schema "shared_weights" do
    field :embedding_data, :binary
    field :projection_data, :binary
    field :tier, Ecto.Enum, values: [:minimal, :standard, :full], default: :standard
    timestamps(updated_at: false)
  end

  def changeset(shared_weights, attrs) do
    shared_weights
    |> cast(attrs, [
      :id,
      :embedding_data,
      :projection_data,
      :tier,
      :inserted_at,
      :updated_at
    ])
    |> validate_required([:id, :embedding_data, :projection_data])
    |> validate_inclusion(:tier, [:minimal, :standard, :full])
  end
end
