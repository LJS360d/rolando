defmodule Rolando.Repo.Migrations.AddGuildChainsAndTrainingMessages do
  use Ecto.Migration

  def up do
    create_if_not_exists table(:users, primary_key: false) do
      add :id, :binary_id, primary_key: true
      add :display_name, :string
      add :role, :string, default: "regular"
      timestamps()
    end

    create_if_not_exists table(:analytics_events, primary_key: false) do
      add :id, :binary_id, primary_key: true
      add :event_type, :string, null: false
      add :guild_id, :string
      add :channel_id, :string
      add :user_id, :binary_id
      add :meta, :map, default: %{}
      timestamps()
    end

    create_if_not_exists table(:guilds, primary_key: false) do
      add :id, :string, primary_key: true
      add :name, :string, null: false
      add :platform, :string, default: "discord"
      add :image_url, :string
      timestamps()
    end

    create table(:guild_chains, primary_key: false) do
      add :guild_id, references(:guilds, column: :id, type: :string, on_delete: :delete_all),
        primary_key: true
      add :name, :string
      add :reply_rate, :integer, default: 10
      add :reaction_rate, :integer, default: 30
      add :vc_join_rate, :integer, default: 100
      add :max_size_mb, :integer, default: 25
      add :ngram_size, :integer, default: 2
      add :tts_language, :string, default: "en"
      add :pings, :boolean, default: true
      add :premium, :boolean, default: false
      add :trained_at, :utc_datetime
      add :chain_state, :text
      timestamps()
    end

    create index(:guild_chains, [:trained_at])

    create table(:training_messages) do
      add :guild_id, :string, null: false
      add :channel_id, :string, null: false
      add :content, :text, null: false
      timestamps(updated_at: false)
    end

    create index(:training_messages, [:guild_id])
  end

  def down do
    drop index(:training_messages, [:guild_id])
    drop table(:training_messages)
    drop index(:guild_chains, [:trained_at])
    drop table(:guild_chains)
    drop_if_exists table(:analytics_events)
    drop_if_exists table(:users)
    drop_if_exists table(:guilds)
  end
end
