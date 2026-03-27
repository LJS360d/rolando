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
| Schema/migrations: `training_messages`, `guild_chains`, `guilds` bootstrap, `analytics_events` + `users` | done |
| Batch insert training lines (`Rolando.Messages.insert_training_lines/3`); delete guild training rows on re-train | done (subset of full repo behaviour) |
| Dev adapter: SQLite (single file / single DB) | done |
| Prod adapter: Postgres + column store writer/reader (design + interface) | not started |
| Backfill/fetch orchestration (`RolandoDiscord.Train`) | done for `/train` |
| Tests for repo + boundary contracts | partial (`Rolando.MarkovTest`) |

## Phase 2 — Core: Markov + media

| Item | Status |
|------|--------|
| Pure Markov module (n-gram ingest + JSON snapshot; no generation yet) | in progress |
| Media URL sets + validation strategy (Req HEAD) | not started |
| Chain metadata (`guild_chains` table + Ecto schema) | done |
| Per-guild chain worker (`Registry` + `DynamicSupervisor` + `GuildChainServer`; Horde optional later) | done |
| Redis-backed state sync (optional compile/runtime config) | not started |
| Analytics/complexity metrics parity with `MarkovChainAnalyzer` | not started |

## Phase 3 — Discord interface (Nostrum)

| Item | Status |
|------|--------|
| Slash commands parity | in progress — `/train`, `/channels` implemented; rest still unhandled |
| Message create handler (learn, mention reply, random reply, reactions) | not started |
| Events: GUILD_CREATE/UPDATE/DELETE, presence | not started |
| Voice: join/leave/speak, STT pipeline abstraction | not started |
| Buttons (train confirmation flows) | done (`confirm-train`, `confirm-train-again`) |
| Sharding / multi-node | not started |

## Phase 4 — Web (Phoenix)

| Item | Status |
|------|--------|
| Auth: Discord OAuth / session parity with Go `auth` | not started |
| REST or LiveView JSON API parity with Gin routes under `/auth`, `/analytics`, `/data`, `/bot` | not started |
| Marketing home, premium, legal pages | not started |
| Admin dashboard (guild list, analytics, broadcast, resources) | not started |
