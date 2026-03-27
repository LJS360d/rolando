defmodule Rolando.Schema.TrainingMessage do
  use Ecto.Schema
  import Ecto.Changeset

  schema "training_messages" do
    field :guild_id, :string
    field :channel_id, :string
    field :content, :string
    timestamps(updated_at: false)
  end

  def changeset(tm, attrs) do
    tm
    |> cast(attrs, [:guild_id, :channel_id, :content])
    |> validate_required([:guild_id, :channel_id, :content])
  end
end
