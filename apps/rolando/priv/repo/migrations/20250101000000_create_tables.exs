defmodule Rolando.Repo.Migrations.CreateTables do
  use Ecto.Migration

  def change do
    # ── media_store ──────────────────────────────────────────────────────────
    create_if_not_exists table(:media_store, primary_key: false) do
      add :id, :bigint, primary_key: true, autogenerate: true
      add :guild_id, :string, null: false
      add :channel_id, :string, null: false
      add :url, :string, null: false
      add :media_type, :string, null: false
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:media_store, [:guild_id])
    create_if_not_exists index(:media_store, [:media_type])

    # ── guild_config ──────────────────────────────────────────────────────────
    create_if_not_exists table(:guild_config, primary_key: false) do
      add :guild_id, :string, primary_key: true
      add :tier, :integer, default: 2
      add :premium, :boolean, default: false
      add :filter_pings, :boolean, default: false
      add :filter_bot_authors, :boolean, default: true
      add :max_size_mb, :integer, default: 25
      add :trained_at, :utc_datetime
      add :reply_rate, :float, default: 0.05
      add :reaction_rate, :float, default: 0.01
      add :vc_join_rate, :float, default: 0.01
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:guild_config, [:guild_id])

    # ── guilds ────────────────────────────────────────────────────────────────
    create_if_not_exists table(:guilds, primary_key: false) do
      add :id, :string, primary_key: true
      add :name, :string, null: false
      add :platform, :string, default: "discord"
      add :image_url, :string
      add :config_id, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:guilds, [:id])
    create_if_not_exists index(:guilds, [:config_id])

    # ── users ─────────────────────────────────────────────────────────────────
    create_if_not_exists table(:users, primary_key: false) do
      add :id, :string, primary_key: true
      add :username, :string
      add :global_name, :string
      add :avatar_url, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:users, [:id])

    # ── analytics_events ──────────────────────────────────────────────────────
    create_if_not_exists table(:analytics_events, primary_key: false) do
      add :id, :binary_id, primary_key: true, autogenerate: true
      add :event_type, :string, null: false
      add :guild_id, :string
      add :channel_id, :string
      add :user_id, references(:users, type: :string, on_delete: :nilify_all)
      add :meta, :map, default: %{}
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create_if_not_exists index(:analytics_events, [:guild_id])
    create_if_not_exists index(:analytics_events, [:event_type])
    create_if_not_exists index(:analytics_events, [:user_id])

    # ── messages ──────────────────────────────────────────────────────────────
    create_if_not_exists table(:messages, primary_key: false) do
      add :id, :binary_id, primary_key: true, autogenerate: true
      add :guild_id, :string, null: false
      add :channel_id, :string, null: false
      add :author_id, :string
      add :content, :text, null: false
    end

    create_if_not_exists index(:messages, [:guild_id])
    create_if_not_exists index(:messages, [:channel_id])
    create_if_not_exists index(:messages, [:author_id])
  end
end
