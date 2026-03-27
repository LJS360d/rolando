defmodule Rolando.Schema.UserIdentity do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :binary_id, autogenerate: true}
  @foreign_key_type :binary_id

  schema "user_identities" do
    field :platform, :string, default: "discord"
    field :platform_id, :string
    field :username, :string

    belongs_to :user, Rolando.Schema.User

    timestamps()
  end

  def changeset(identity, attrs) do
    identity
    |> cast(attrs, [:platform, :platform_id, :username, :user_id])
    |> validate_required([:platform, :platform_id, :username, :user_id])
    |> unique_constraint([:platform, :platform_id])
    |> foreign_key_constraint(:user_id)
  end
end
