defmodule Rolando.Schema.AnalyticsEvent do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:id, :binary_id, autogenerate: true}
  @foreign_key_type :binary_id
  schema "analytics_events" do
    field :event_type, :string
    field :guild_id, :string
    field :channel_id, :string
    field :meta, :map, default: %{}
    belongs_to :user, Rolando.Schema.User
    timestamps()
  end

  def changeset(analytics_event, attrs) do
    analytics_event
    |> cast(attrs, [:event_type, :guild_id, :channel_id, :user_id, :meta])
    |> validate_required([:event_type])
    |> foreign_key_constraint(:user_id)
  end
end
