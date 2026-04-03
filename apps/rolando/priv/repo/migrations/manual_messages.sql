-- Add missing columns (nullable first)
ALTER TABLE messages ADD COLUMN channel_id text;
ALTER TABLE messages ADD COLUMN author_id text;
ALTER TABLE messages ADD COLUMN inserted_at datetime;
ALTER TABLE messages ADD COLUMN updated_at datetime;

-- Set default values for existing rows
UPDATE messages SET channel_id = 'unknown' WHERE channel_id IS NULL;
UPDATE messages SET inserted_at = created_at WHERE inserted_at IS NULL;
UPDATE messages SET updated_at = inserted_at WHERE updated_at IS NULL;

-- Recreate table with proper NOT NULL constraints
CREATE TABLE messages_new (
  id integer PRIMARY KEY AUTOINCREMENT,
  guild_id text NOT NULL,
  channel_id text NOT NULL,
  author_id text,
  content text NOT NULL,
  inserted_at datetime,
  updated_at datetime
);

-- Copy data
INSERT INTO messages_new SELECT id, guild_id, channel_id, author_id, content, inserted_at, updated_at FROM messages;

-- Drop old table and rename
DROP TABLE messages;
ALTER TABLE messages_new RENAME TO messages;

-- Recreate indexes
CREATE INDEX idx_messages_guild_id ON messages(guild_id);
CREATE INDEX idx_messages_channel_id ON messages(channel_id);
CREATE INDEX idx_messages_author_id ON messages(author_id);

SELECT 'Migrated ' || COUNT(*) || ' messages' as status FROM messages;
