defmodule Rolando.Schema.MediaStore do
  @moduledoc """
  Schema for media storage (images, videos, etc.) extracted from messages.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :id, autogenerate: true}
  schema "media_store" do
    field :guild_id, :string
    field :channel_id, :string
    field :url, :string
    field :media_type, Ecto.Enum, values: [:image, :gif, :video, :generic]
    field :context_hash, :string
    timestamps(updated_at: false)
  end

  def changeset(media_store, attrs) do
    media_store
    |> cast(attrs, [
      :guild_id,
      :channel_id,
      :url,
      :media_type,
      :context_hash
    ])
    |> validate_required([:guild_id, :url, :media_type])
    |> validate_length(:url, min: 1)
    |> validate_length(:context_hash, min: 1)
  end
end
