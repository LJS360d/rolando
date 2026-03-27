defmodule Rolando.Schema.GuildChain do
  use Ecto.Schema
  import Ecto.Changeset

  @primary_key {:guild_id, :string, autogenerate: false}
  schema "guild_chains" do
    field :name, :string
    field :reply_rate, :integer, default: 10
    field :reaction_rate, :integer, default: 30
    field :vc_join_rate, :integer, default: 100
    field :max_size_mb, :integer, default: 25
    field :ngram_size, :integer, default: 2
    field :tts_language, :string, default: "en"
    field :pings, :boolean, default: true
    field :premium, :boolean, default: false
    field :trained_at, :utc_datetime
    field :chain_state, :string
    timestamps()
  end

  def changeset(chain, attrs) do
    chain
    |> cast(attrs, [
      :guild_id,
      :name,
      :reply_rate,
      :reaction_rate,
      :vc_join_rate,
      :max_size_mb,
      :ngram_size,
      :tts_language,
      :pings,
      :premium,
      :trained_at,
      :chain_state
    ])
    |> validate_required([:guild_id])
  end
end
