# Progress tracker

Legend: not started | in progress | done

## Phase 0 — Tooling & docs

| Item | Status |
|------|--------|
| `.memory-bank/` agent workflow | done |
| Umbrella config pattern (Dotenvy runtime, Ueberauth, Nostrum, release) | done |

## Phase 1 — Core: messages

| Item | Status |
|------|--------|
| Schema/migrations for guild messages (IDs, guild_id, content, inserted_at, source metadata if needed) | not started |
| Repo behaviour: append, batch insert, paginated read, count, delete guild, delete by content, delete containing substring | not started |
| Dev adapter: SQLite (single file / single DB) | not started |
| Prod adapter: Postgres + column store writer/reader (design + interface) | not started |
| Backfill/fetch orchestration (Discord API batching, rate limits, channel iteration) — lives in Discord app calling Core | not started |
| Tests for repo + boundary contracts | not started |

## Phase 2 — Core: Markov + media

| Item | Status |
|------|--------|
| Pure Markov module (n-gram state, generation, delete, filtered output) | not started |
| Media URL sets + validation strategy (Req HEAD) | not started |
| Chain metadata (reply_rate, reaction_rate, n_gram_size, pings, tts_language, trained_at, premium flags) — Ecto/Postgres | not started |
| Horde: registry/supervision per guild chain worker | not started |
| Redis-backed state sync (optional compile/runtime config) | not started |
| Analytics/complexity metrics parity with `MarkovChainAnalyzer` | not started |

## Phase 3 — Discord interface (Nostrum)

| Item | Status |
|------|--------|
| Slash commands parity (`commands.go` registry) | in progress — global command maps in `RolandoDiscord.Commands.commands/0`; handlers not wired |
| Message create handler (learn, mention reply, random reply, reactions) | not started |
| Events: GUILD_CREATE/UPDATE/DELETE, presence | not started |
| Voice: join/leave/speak, STT pipeline abstraction | not started |
| Buttons (train confirmation flows) | not started |
| Sharding / multi-node | not started |

## Phase 4 — Web (Phoenix)

| Item | Status |
|------|--------|
| Auth: Discord OAuth / session parity with Go `auth` | not started |
| REST or LiveView JSON API parity with Gin routes under `/auth`, `/analytics`, `/data`, `/bot` | not started |
| Marketing home, premium, legal pages | not started |
| Admin dashboard (guild list, analytics, broadcast, resources) | not started |
