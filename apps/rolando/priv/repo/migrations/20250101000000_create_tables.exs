defmodule Rolando.Repo.Migrations.CreateTables do
  use Ecto.Migration

  def change do
    # Guild neural network weights (replaces guild_chains)
    create table(:guild_weights, primary_key: false) do
      add :guild_id, :string, primary_key: true
      add :weight_data, :binary
      add :version, :integer, default: 1
      add :perplexity, :float
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:guild_weights, [:guild_id])

    # Media store for extracted images/videos
    create table(:media_store, primary_key: false) do
      add :id, :bigint, primary_key: true, autogenerate: true
      add :guild_id, :string, null: false
      add :channel_id, :string, null: false
      add :url, :string, null: false
      add :media_type, :string, null: false
      add :context_hash, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:media_store, [:guild_id])
    create index(:media_store, [:media_type])
    create index(:media_store, [:context_hash])

    # Guild-specific neural network configuration
    create table(:guild_config, primary_key: false) do
      add :guild_id, :string, primary_key: true
      add :batch_size, :integer, default: 32
      add :learning_rate, :float, default: 0.001
      add :weighted_loss, :boolean, default: false
      add :emoji_skip, :boolean, default: true
      add :filter_pings, :boolean, default: true
      add :filter_bot_authors, :boolean, default: true
      add :tokenizer_model, :string
      add :vector_augment, :boolean, default: false
      add :precision_mode, :string, default: "standard"
      add :tier, :string, default: "standard"
      add :trained_at, :utc_datetime
      add :reply_rate, :integer, default: 20
      add :reaction_rate, :integer, default: 100
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:guild_config, [:tier])

    create index(:guild_config, [:guild_id])

    # Shared system weights (frozen embedding + output projection)
    create table(:shared_weights, primary_key: false) do
      add :id, :id, primary_key: true
      add :embedding_data, :binary
      add :projection_data, :binary
      add :tier, :string, default: "standard"
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:shared_weights, [:tier])

    # Guilds table for Discord servers
    create table(:guilds, primary_key: false) do
      add :id, :string, primary_key: true
      add :name, :string, null: false
      add :platform, :string, default: "discord"
      add :image_url, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:guilds, [:id])

    # Add columns to guilds for associations (FK references removed - SQLite doesn't support FK with string PKs well)
    alter table(:guilds) do
      add :config_id, :string
      add :weights_id, :string
    end

    create index(:guilds, [:config_id])
    create index(:guilds, [:weights_id])

    # Users table for Discord users
    create table(:users, primary_key: false) do
      add :id, :string, primary_key: true
      add :username, :string
      add :global_name, :string
      add :avatar_url, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:users, [:id])

    # Analytics events for tracking
    create table(:analytics_events, primary_key: false) do
      add :id, :binary_id, primary_key: true, autogenerate: true
      add :event_type, :string, null: false
      add :guild_id, :string
      add :channel_id, :string
      add :user_id, references(:users, type: :string, on_delete: :nilify_all)
      add :meta, :map, default: %{}
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:analytics_events, [:guild_id])
    create index(:analytics_events, [:event_type])
    create index(:analytics_events, [:user_id])

    # Messages table for GRU training data
    create table(:messages, primary_key: false) do
      add :id, :binary_id, primary_key: true, autogenerate: true
      add :guild_id, :string, null: false
      add :channel_id, :string, null: false
      add :author_id, :string
      add :content, :text, null: false
      add :message_hash, :string
      add :inserted_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
      add :updated_at, :utc_datetime, default: fragment("CURRENT_TIMESTAMP")
    end

    create index(:messages, [:guild_id])
    create index(:messages, [:channel_id])
    create index(:messages, [:author_id])
    create index(:messages, [:message_hash], name: :messages_guild_hash_index)
  end
end
