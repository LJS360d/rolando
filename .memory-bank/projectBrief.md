# Project brief — Rolando (Elixir umbrella)

## Purpose

Replace the legacy Go Discord bot (`rolando-go`, discordgo + Gin + SQLite + in-process Markov state) with an Elixir umbrella: **Core** domain logic, **Discord** interface (Nostrum), **Web** interface (Phoenix). Preserve **end-user-visible behavior and features** from the Go bot and the **Vuetify admin/marketing client**, without copying the Go process architecture (single binary, shared SQLite, unbounded in-memory `chainsMap`, concurrent fetch/update races).

## Success criteria

- **Dev ergonomics**: runnable with minimal system dependencies (prefer pure Elixir/Erlang + optional local SQLite; avoid requiring Redis/Postgres/ClickHouse for `mix phx.server` / bot dev unless explicitly opted in).
- **Production scale**: Docker Compose (or equivalent) supporting **bot sharding**, **multiple Core OTP nodes**, **Redis** for Markov/distributed state where needed, **Postgres** for relational data, a **column-oriented FOSS store** for high-volume message text/analytics-oriented workloads, with clear boundaries between adapters.
- **Libraries**: Discord via **Nostrum** (stability/maintainability vs discordgo). HTTP client: **Req** per project rules.
- **Frontend**: Port `rolando-go/client` (Vue + Vuetify) capabilities to **Phoenix LiveView** (and associated assets), feature parity over time.

## Explicit non-goals (for this rewrite)

- Reimplement the Go monolith layout (one DB connection string, one in-memory map of all guild chains, fetch pipeline that fires `go` updates to chain + DB without coordinated back-pressure).
- Commit to Vosk-from-day-one for STT; treat STT as a pluggable boundary (see `techDecisions.md`).

## Source of truth for “what existed”

Legacy codebase path: `/home/luca-lencioni/Documents/other/rolando-go` (Go bot + `client/` SPA).

## Priority order (stakeholder)

1. **Core**: message ingestion from Discord semantics, persistence behind a **scalable store in prod** with a **dev adapter** (single SQLite DB acceptable for local dev).
2. **Core**: Markov chain engine + persistence/sync strategy: **Horde** for local/cluster process placement; **optional sharded Redis** for production-scale shared state.
3. Later: full Discord command/event parity, web parity, voice/STT/TTS, premium gates, etc. (see `migration-inventory.md`).
