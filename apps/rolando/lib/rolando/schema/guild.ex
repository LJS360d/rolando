defmodule Rolando.Schema.Guild do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :string, autogenerate: false}
  schema "guilds" do
    field :name, :string
    field :platform, :string, default: "discord"
    field :image_url, :string

    # Neural network associations - uses shared primary key for belongs_to
    belongs_to :config, Rolando.Schema.GuildConfig,
      references: :guild_id,
      foreign_key: :config_id,
      type: :string,
      on_replace: :delete

    timestamps()
  end

  def changeset(guild, attrs) do
    guild
    |> cast(attrs, [:id, :name, :platform, :image_url])
    |> validate_required([:id, :name])
  end
end
