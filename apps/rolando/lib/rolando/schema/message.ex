defmodule Rolando.Schema.Message do
  @moduledoc """
  Schema for storing message content for GRU training.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :binary_id, autogenerate: true}
  @foreign_key_type :binary_id
  schema "messages" do
    field :guild_id, :string
    field :channel_id, :string
    field :author_id, :string
    field :content, :string
    field :message_hash, :string
    timestamps()
  end

  def changeset(message, attrs) do
    message
    |> cast(attrs, [:guild_id, :channel_id, :author_id, :content, :message_hash])
    |> validate_required([:guild_id, :channel_id, :content])
    |> validate_length(:content, min: 1)
    |> unique_constraint(:message_hash, name: :messages_guild_hash_index)
  end
end
