# Target architecture (Elixir)

## Problems in the Go version (do not replicate)

- **Single process in-memory `chainsMap`**: all guild chains loaded at startup; memory scales with total learned data; limited horizontal scaling.
- **SQLite only**: fine for dev; not the target for high-volume messages + concurrent writers in prod.
- **Tight coupling**: `DataFetchService` pushes batches into DB and chain in ad hoc goroutines; no explicit back-pressure, job queue, or idempotency keys for imports.
- **HTTP + bot in one binary**: acceptable in Go; in Elixir prefer **umbrella boundaries** — Core exposes contexts and optional GenServer/Horde APIs; Discord and Web are adapters.

## Target layering

```
┌─────────────────────────────────────────────────────────┐
│  rolando_web (Phoenix)     rolando_discord (Nostrum)     │
│         │                            │                     │
│         └────────────┬───────────────┘                     │
│                      ▼                                     │
│               rolando (Core)                               │
│   - Contexts: Messages, Chains, Media, Analytics          │
│   - Pure Markov logic                                      │
│   - Horde supervisors: per-guild chain workers (or shards) │
│   - Optional Redis sync layer for Markov state             │
│   - Repo adapters: Dev (SQLite), Prod (Postgres + column)  │
└─────────────────────────────────────────────────────────┘
```

## Data stores (intended)

| Concern | Dev | Prod |
|--------|-----|------|
| Chain **metadata** (rates, flags, guild id, trained_at) | SQLite table(s) or same Postgres | Postgres |
| **Messages** (content lines, timestamps) | Single SQLite DB via Ecto | Postgres for metadata/indexing; **column DB** for append-heavy message storage and analytics scans |
| **Markov working state** | ETS + Horde local | Horde + optional **Redis** (hash/HLL structures or serialized segments — design TBD) |

## Discord scaling

- **Nostrum sharding**: configure shard count / pool to match Discord gateway expectations; Core must not assume a single process owns all guilds — use **consistent hashing** of `guild_id` to chain workers (Horde) and to Redis keys if used.

## Observability

- Replace Go runtime `GET /bot/resources` with **Telemetry** + optional LiveDashboard; expose similar metrics for admin parity later.

## STT / voice

- Legacy uses **Vosk** with CGO and on-disk models per language. Target: **out-of-process** STT (HTTP/grpc sidecar) or future Elixir NIF/port with clear failure modes; languages: align with `data.Langs` in Go (`en`, `it`, `de`, `es`) unless product changes.
