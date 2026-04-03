defmodule Rolando.Schema.Message do
  @moduledoc """
  Schema for storing message content for GRU training.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :id, autogenerate: false}
  @foreign_key_type :id
  schema "messages" do
    field :guild_id, :string
    field :channel_id, :string
    field :author_id, :string
    field :content, :string
  end

  def changeset(message, attrs) do
    message
    |> cast(attrs, [:guild_id, :channel_id, :author_id, :content])
    |> validate_required([:guild_id, :channel_id, :content])
    |> validate_length(:content, min: 1)
  end
end
