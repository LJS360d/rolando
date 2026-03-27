defmodule Rolando.Schema.Guild do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :string, autogenerate: false}
  schema "guilds" do
    field :name, :string
    field :platform, :string, default: "discord"
    field :image_url, :string
    has_many :channels, Rolando.Schema.Channel
    timestamps()
  end

  def changeset(guild, attrs) do
    guild
    |> cast(attrs, [:id, :name, :platform, :image_url])
    |> validate_required([:id, :name])
  end
end
