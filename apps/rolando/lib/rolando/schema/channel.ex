defmodule Rolando.Schema.Channel do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :string, autogenerate: false}
  schema "channels" do
    field :name, :string
    field :reply_rate, :float, default: 0.0
    field :reaction_rate, :float, default: 0.0
    field :vc_join_rate, :float, default: 0.0
    belongs_to :guild, Rolando.Schema.Guild, type: :string
    timestamps()
  end

  def changeset(channel, attrs) do
    channel
    |> cast(attrs, [:id, :guild_id, :name, :reply_rate, :reaction_rate])
    |> validate_required([:id, :guild_id, :name])
    |> validate_number(:spawn_rate, greater_than_or_equal_to: 0, less_than_or_equal_to: 100)
    |> foreign_key_constraint(:guild_id)
  end
end
