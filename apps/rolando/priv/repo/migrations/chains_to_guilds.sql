-- ============================================================================
-- Migrate from old 'chains' table to new 'guilds' and 'guild_config' tables
-- ============================================================================

-- The chains table schema:
-- CREATE TABLE "chains"  (
--   `id` text,
--   `name` text NOT NULL,
--   `reply_rate` integer DEFAULT 10,
--   `pings` numeric DEFAULT true,
--   `max_size_mb` integer DEFAULT 25,
--   `trained` numeric DEFAULT false,
--   `updated_at` datetime,
--   `tts_language` text DEFAULT "en",
--   `vc_join_rate` integer DEFAULT 100,
--   `premium` numeric DEFAULT false,
--   `n_gram_size` integer DEFAULT 2,
--   `reaction_rate` integer DEFAULT 30,
--   `trained_at` datetime DEFAULT null,
--   PRIMARY KEY (`id`)
-- );

-- Migration rules:
-- 1. chains.id -> guilds.id AND guild_config.guild_id
-- 2. chains.name -> guilds.name
-- 3. chains.n_gram_size -> guild_config.tier
-- 4. chains.pings -> guild_config.filter_pings
-- 5. chains.premium -> guild_config.premium
-- 6. chains.max_size_mb -> guild_config.max_size_mb
-- 7. chains.trained_at -> guild_config.trained_at
-- 8. Rate fields: new_value = CASE WHEN old_value = 0 THEN 0 ELSE MAX(0, MIN(100, 1.0 / old_value)) END
--    - chains.reply_rate -> guild_config.reply_rate
--    - chains.reaction_rate -> guild_config.reaction_rate
--    - chains.vc_join_rate -> guild_config.vc_join_rate

-- Step 1: Create temporary tables for the new data
CREATE TABLE IF NOT EXISTS guilds_new (
  id text PRIMARY KEY,
  name text NOT NULL,
  platform text DEFAULT 'discord',
  image_url text,
  config_id text,
  inserted_at datetime,
  updated_at datetime
);

CREATE TABLE IF NOT EXISTS guild_config_new (
  guild_id text PRIMARY KEY,
  tier integer DEFAULT 2,
  premium boolean DEFAULT false,
  filter_pings boolean DEFAULT false,
  filter_bot_authors boolean DEFAULT true,
  max_size_mb integer DEFAULT 25,
  trained_at datetime,
  reply_rate real DEFAULT 0.05,
  reaction_rate real DEFAULT 0.01,
  vc_join_rate real DEFAULT 0.01,
  inserted_at datetime,
  updated_at datetime
);

-- Step 2: Migrate data from chains to guilds
INSERT INTO guilds_new (id, name, platform, image_url, inserted_at, updated_at)
SELECT
  id,
  name,
  'discord' as platform,
  NULL as image_url,
  COALESCE(updated_at, datetime('now')) as inserted_at,
  COALESCE(updated_at, datetime('now')) as updated_at
FROM chains;

-- Step 3: Migrate data from chains to guild_config with rate transformation
-- Rate formula: CASE WHEN old_value = 0 THEN 0 ELSE MAX(0, MIN(100, 1.0 / old_value)) END
INSERT INTO guild_config_new (
  guild_id,
  tier,
  premium,
  filter_pings,
  filter_bot_authors,
  max_size_mb,
  trained_at,
  reply_rate,
  reaction_rate,
  vc_join_rate,
  inserted_at,
  updated_at
)
SELECT
  id as guild_id,
  COALESCE(n_gram_size, 2) as tier,
  COALESCE(premium, false) as premium,
  COALESCE(pings, true) as filter_pings,
  true as filter_bot_authors,
  COALESCE(max_size_mb, 25) as max_size_mb,
  trained_at,
  CASE
    WHEN COALESCE(reply_rate, 10) = 0 THEN 0.0
    ELSE MAX(0.0, MIN(100.0, 1.0 / reply_rate))
  END as reply_rate,
  CASE
    WHEN COALESCE(reaction_rate, 30) = 0 THEN 0.0
    ELSE MAX(0.0, MIN(100.0, 1.0 / reaction_rate))
  END as reaction_rate,
  CASE
    WHEN COALESCE(vc_join_rate, 100) = 0 THEN 0.0
    ELSE MAX(0.0, MIN(100.0, 1.0 / vc_join_rate))
  END as vc_join_rate,
  COALESCE(updated_at, datetime('now')) as inserted_at,
  COALESCE(updated_at, datetime('now')) as updated_at
FROM chains;

-- Step 4: Update guilds to reference their config
UPDATE guilds_new SET config_id = id;

-- Step 5: Drop old tables and rename new ones
DROP TABLE IF EXISTS chains;
DROP TABLE IF EXISTS guilds;
DROP TABLE IF EXISTS guild_config;

ALTER TABLE guilds_new RENAME TO guilds;
ALTER TABLE guild_config_new RENAME TO guild_config;

-- Step 6: Create indexes
CREATE INDEX IF NOT EXISTS idx_guilds_id ON guilds(id);
CREATE INDEX IF NOT EXISTS idx_guilds_config_id ON guilds(config_id);
CREATE INDEX IF NOT EXISTS idx_guild_config_tier ON guild_config(tier);
CREATE INDEX IF NOT EXISTS idx_guild_config_guild_id ON guild_config(guild_id);

-- Step 7: Verify migration
SELECT 'Migrated ' || COUNT(*) || ' guilds from chains table' as status FROM guilds;
SELECT 'Migrated ' || COUNT(*) || ' guild configs from chains table' as status FROM guild_config;
