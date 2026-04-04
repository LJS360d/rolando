defmodule Rolando.Schema.GuildConfig do
  @moduledoc """
  Schema for per-guild neural network configuration.
  """
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:guild_id, :string, autogenerate: false}
  schema "guild_config" do
    field :tier, :integer, default: 2
    field :premium, :boolean, default: false
    field :filter_pings, :boolean, default: false
    field :filter_bot_authors, :boolean, default: true
    field :max_size_mb, :integer, default: 25
    field :trained_at, :utc_datetime
    field :reply_rate, :float, default: 0.05
    field :reaction_rate, :float, default: 0.01
    field :vc_join_rate, :float, default: 0.01
    timestamps(updated_at: false)
  end

  def changeset(guild_config, attrs) do
    guild_config
    |> cast(attrs, [
      :guild_id,
      :tier,
      :filter_pings,
      :filter_bot_authors,
      :max_size_mb,
      :trained_at,
      :reply_rate,
      :reaction_rate,
      :vc_join_rate
    ])
    |> validate_required([:guild_id])
    |> validate_number(:max_size_mb, greater_than: 0)
    |> validate_number(:reply_rate, greater_than_or_equal_to: 0, less_than_or_equal_to: 100)
    |> validate_number(:reaction_rate, greater_than_or_equal_to: 0, less_than_or_equal_to: 100)
    |> validate_number(:vc_join_rate, greater_than_or_equal_to: 0, less_than_or_equal_to: 100)
    |> validate_number(:tier, greater_than_or_equal_to: 2, less_than_or_equal_to: 255)
  end
end
