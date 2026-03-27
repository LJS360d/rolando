# Migration inventory — legacy Go + Vuetify client

Source: `rolando-go` at `/home/luca-lencioni/Documents/other/rolando-go`.

## Data model (legacy)

- **messages** (SQLite/GORM): `id`, `guild_id`, `content` (text), `created_at`; indexes on `guild_id`, `(guild_id, created_at)`.
- **chains** (SQLite/GORM): `id` (guild snowflake string), `name`, `reply_rate`, `reaction_rate`, `vc_join_rate`, `max_size_mb`, `n_gram_size`, `tts_language`, `pings`, `trained_at`, `updated_at`, `premium`.

## Discord slash commands (registry in `commands.go`)

| Command | Notes |
|---------|--------|
| `train` | Admin: fetch all accessible channel messages (cap 750k/msg limit per channel loop), batch to DB + chain |
| `gif` / `image` / `video` | Return random learned media URL |
| `analytics` | Guild analytics (text) |
| `togglepings` | Admin: pings in generated text |
| `replyrate` | View/set |
| `reactionrate` | View/set |
| `cohesion` | Maps to n-gram size (2–255) — **legacy naming**; Go uses `n_gram_size` in DB |
| `opinion` | Generate from seed |
| `wipe` | Remove training line from DB |
| `channels` | List channels bot can read for training |
| `src` | Repo URL |
| `vc-join` / `vc-leave` / `vc-speak` | Premium SKU gated in Go |
| `vc-language` | en/it/de/es |
| `vc-joinrate` | Random VC join rate |

## Message behavior (`on-message.go`)

- Ignore bots; require text channel access.
- Learning: content length > 3 chars **or** attachments; updates chain only in goroutine (not always persisted to messages table on every message — **training data persistence is inconsistent vs `/train`**; product decision: Elixir should define explicit rules).
- Mention bot → reply with generated content (reference).
- `reply_rate` → random reply (10% quiet-reply with reference, else standalone).
- `reaction_rate` → random emoji (unicode pool + guild custom emojis).

## Events

- `GUILD_CREATE`: create chain, welcome on system channel, refresh presence.
- `GUILD_UPDATE`, `GUILD_DELETE`: (see source files — chain rename/delete behavior).
- `VOICE_STATE_UPDATE`: VC features + STT pipeline.

## HTTP API (`cmd/ihttp/http_server.go`)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/auth/@me` | Discord user via Bearer |
| GET | `/analytics/:chain` | Single guild analytics |
| GET | `/analytics` | Paginated |
| GET | `/analytics/all` | All chains |
| GET | `/data/:chain/all` | All message strings |
| GET | `/data/:chain` | Paginated messages |
| GET | `/bot/user` | Bot profile + invite + command list |
| GET | `/bot/guilds` | Paginated guilds + chain analytics |
| GET | `/bot/guilds/all` | All guilds |
| GET | `/bot/guilds/:guildId` | One guild |
| PUT | `/bot/guilds/:guildId` | Update chain document |
| DELETE | `/bot/guilds/:guildId` | Leave guild |
| GET | `/bot/guilds/:guildId/invite` | Invite URL |
| GET | `/bot/resources` | CPU/memory stats |
| POST | `/bot/broadcast` | Admin broadcast |

## Vuetify client — routes / features (Vue file-based routes)

- `/` — Landing: bot avatar, invite, shuffled command list, warnings.
- `/login` — Discord OAuth flow (token storage).
- `/admin` — Dashboard: memory bar, guild cards, sort, pagination, per-guild analytics, invite, leave, data link, broadcast FAB; uses `bot`, `analytics`, `resources` APIs.
- `/admin/broadcast` — Broadcast UI.
- `/data/[guildId]` — Paginated training data view.
- `/premium`, `/privacy-policy`, `/terms-of-service` — Static/marketing.

**Phoenix port**: equivalent LiveView pages + same API surface during transition (or JSON API behind session token) until fully server-rendered.

## Analytics parity

- Go `MarkovChainAnalyzer`: complexity score, gif/image/video counts, reply rate, n-gram size, word count (prefix keys), message count, approximate byte size of in-memory state.

## Config/env (legacy)

- `TOKEN`, `DATABASE_PATH`, `SERVER_ADDRESS`, `RUN_HTTP_SERVER`, `INVITE_URL`, `OWNER_IDS`, `LOG_WEBHOOK`, `GO_ENV`, paywall flags, `VOICE_CHAT_FEATURES_SKU_ID`, `PREMIUMS_PAGE_LINK`.

## Known legacy quirks to revisit in Elixir

- `cohesion` command name vs `n_gram_size` field.
- `max_size_mb` enforced in UI/analytics but not fully in chain update (TODO in Go).
- Live message learning may not mirror `/train` DB contents; define a single **source of truth** policy.
