# Active context

Last updated: 2025-03-27

## Current repo state (`rolando_umbrella`)

- Umbrella with apps including `:rolando` (Ecto + SQLite in `mix.exs`), `:rolando_web`, `:rolando_discord` — early scaffolding; **no feature parity** with Go yet.
- **Config**: `config/config.exs` Ueberauth Discord provider; `dev.exs` / `test.exs` `:assets_base_url`; `runtime.exs` uses **Dotenvy** (`source!` + `env!`) for Nostrum, Ueberauth OAuth, owner IDs, `PORT`, prod `DATABASE_PATH`. Root `mix.exs` defines **`rolando_umbrella` release**.
- Project rules: Phoenix 1.8 patterns, `Req` for HTTP, `mix precommit` at end of change batches.

## Discord slash commands

- `apps/rolando_discord/lib/rolando_discord/commands.ex`: `commands/0` mirrors `rolando-go/cmd/idiscord/commands/commands.go` (names, descriptions, options, `vc-language` choices). Sync still via `bulk_overwrite_global_commands` — **handlers/interaction routing not implemented**.

## Immediate focus (next implementation phase)

1. **Messages pipeline (Core)**
   - Port *behavior* of `MessagesRepository` + `DataFetchService` concepts: guild-scoped text + attachment URLs, pagination, bulk insert, delete by exact content / substring (used for dead media cleanup), counts.
   - **Do not** mirror Go’s unbounded `GetAllGuildMessages` for large guilds in production; design streaming/batch APIs and storage layout suitable for column store + Postgres metadata.

2. **Markov + media (Core)**
   - Port `model.MarkovChain` (n-grams, backoff generation, `Talk` / seed / `TalkFiltered`, `Delete`, ping stripping) and `MediaUrlsStore` (gif/image/video classification, HEAD validation, DB cleanup on invalid URL).
   - Plan **Horde**-supervised chain workers per guild (or sharded keyspace); **Redis** optional backend for prod for shared/merged state across nodes.

3. **Deferred**
   - Vosk STT: investigate Elixir-friendly or sidecar **self-hosted** alternatives; keep interface abstract.
   - Phoenix UI port of Vuetify client: tracked in `migration-inventory.md`, not started until Core foundations exist.

## Open questions (tracked, not blocking memory-bank)

- Exact **column DB** choice (e.g. ClickHouse, Apache Doris, others): pick based on ops cost, Elixir driver maturity, retention policy.
- Whether message bodies live only in column store vs duplicated in Postgres for transactional workflows.
- Nostrum **shard** count vs `Horde` shard mapping.

## Files to read when resuming code work

- Legacy: `rolando-go/internal/repositories/*.go`, `internal/model/markov-chain.go`, `cmd/idiscord/services/*.go`, `cmd/idiscord/messages/on-message.go`, `internal/store/media-urls.store.go`, `cmd/ihttp/http_server.go`, `client/src/api/*.ts`.
- Umbrella: `apps/rolando/lib`, `apps/rolando_discord/lib`, `config/runtime.exs`.
