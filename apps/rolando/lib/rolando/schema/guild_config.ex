defmodule Rolando.Schema.GuildConfig do
  @moduledoc """
  Schema for per-guild neural network configuration.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:guild_id, :string, autogenerate: false}
  schema "guild_config" do
    field :batch_size, :integer, default: 32
    field :learning_rate, :float, default: 0.001
    field :weighted_loss, :boolean, default: false
    field :emoji_skip, :boolean, default: true
    field :filter_pings, :boolean, default: true
    field :filter_bot_authors, :boolean, default: true
    field :tokenizer_model, :string
    field :vector_augment, :boolean, default: false
    field :precision_mode, Ecto.Enum, values: [:standard, :bitnet], default: :standard
    field :tier, Ecto.Enum, values: [:minimal, :standard, :full], default: :standard
    field :trained_at, :utc_datetime
    timestamps(updated_at: false)
  end

  def changeset(guild_config, attrs) do
    guild_config
    |> cast(attrs, [
      :guild_id,
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
      :trained_at
    ])
    |> validate_required([:guild_id])
    |> validate_number(:batch_size, greater_than: 0)
    |> validate_number(:learning_rate, greater_than: 0)
    |> validate_inclusion(:precision_mode, [:standard, :bitnet])
    |> validate_inclusion(:tier, [:minimal, :standard, :full])
  end
end
