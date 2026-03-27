# Active context

Last updated: 2025-03-27

## Current repo state (`rolando_umbrella`)

- Umbrella with apps including `:rolando` (Ecto + SQLite in `mix.exs`), `:rolando_web`, `:rolando_discord`. **Train pipeline (slash + buttons + background fetch + Markov snapshot)** is implemented; other commands still stubs.
- **Config**: `config/config.exs` Ueberauth Discord provider; `dev.exs` / `test.exs` `:assets_base_url`; `runtime.exs` uses **Dotenvy** (`source!` + `env!`) for Nostrum, Ueberauth OAuth, owner IDs, `PORT`, prod `DATABASE_PATH`. Root `mix.exs` defines **`rolando_umbrella` release**.
- Project rules: Phoenix 1.8 patterns, `Req` for HTTP, `mix precommit` at end of change batches.

## Discord slash commands

- `commands/0` still mirrors Go for the full registry. **Implemented**: `/train`, `/channels`. **Routing**: slash and component logic live in `RolandoDiscord.Consumers.SlashCommand` and `RolandoDiscord.Consumers.Component` (shared helpers in `InteractionHelpers`); `Consumers.Message` and `Consumers.Event` are stubs for future parity.

## Train / data (landed)

- **DB**: `guilds`, `guild_chains` (metadata + `trained_at` + JSON `chain_state`), `training_messages` (append-only lines), `users` + `analytics_events` (for analytics subscriber). Migration: `priv/repo/migrations/*_add_guild_chains_and_training_messages.exs`.
- **Core**: `Rolando.Markov` (n-gram ingest + JSON snapshot), `Rolando.Chains` + `Rolando.Chains.GuildChainServer` (Registry + DynamicSupervisor under `Rolando.Supervisor`), `Rolando.Messages` batch insert.
- **Discord**: `RolandoDiscord.Permissions` (admin/owner, bot text-channel access = view + send + history, guild text + announcement), `RolandoDiscord.Train` (bounded `Task.async_stream` over channels, pagination, line filter matching Go).
- **Analytics**: `Rolando.Analytics.track_event/1` → PubSub `analytics_events` → `Rolando.AnalyticsSubscriber` → SQL `analytics_events` rows (`train_started`, `train_completed`, `train_failed`).

## Immediate focus (next implementation phase)

1. **Remaining slash commands** from `commands.ex` (gif, image, video, analytics, togglepings, replyrate, reactionrate, cohesion, opinion, wipe, vc subtree).

2. **Markov + media (Core)**
   - Generation (`Talk`, seed, filtered), `Delete`, ping stripping; **MediaUrlsStore** and live message learning (`on_message`).

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
