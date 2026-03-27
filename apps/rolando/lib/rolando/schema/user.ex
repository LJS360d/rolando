defmodule Rolando.Schema.User do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :binary_id, autogenerate: true}
  schema "users" do
    field :display_name, :string
    field :role, Ecto.Enum, values: [:regular, :server_admin, :owner], default: :regular

    timestamps()
  end

  def changeset(user, attrs) do
    user
    |> cast(attrs, [:display_name, :currency, :role])
    |> validate_required([])
    |> validate_number(:currency, greater_than_or_equal_to: 0)
  end
end
