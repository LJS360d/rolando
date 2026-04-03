# Active context

## Web (Phoenix)

- Operator surface is **LiveView** at `/operator`: session Discord OAuth (Ueberauth), allowlist via `OPERATOR_USER_IDS` / `OWNER_USER_IDS` → `:owner_platform_ids`. Realtime refresh uses `Phoenix.PubSub` topic `operator:analytics` after each successful analytics insert. Dashboard includes BEAM memory bar, daily event-count chart (SQL), paginated guild directory, and a broadcast form that publishes to `operator:broadcast` for `RolandoDiscord.OperatorBroadcast` to deliver via Nostrum.
- Public routes include `/privacy` and `/terms` (server-rendered legal stubs). Site-wide footer links to both. Home uses `<Layouts.app>` with `current_scope` from `RolandoWeb.Plugs.FetchCurrentScope`.

## Analytics (core)

- `Rolando.Analytics.persist_event/1` normalizes event maps (`:event`/`:name` → `event_type`, guild id from `:guild_id` or `:id`, Discord user id folded into `meta`), persists via configured adapter, then broadcasts UI refresh. No cluster subscriber process for duplicate writes (single-node SQL path).

## Discord

- Global slash commands are only **`/train`** and **`/channels`** (registered set matches handlers).
- Deployment operator IDs (train cooldown bypass, etc.) read from `:owner_platform_ids`.
- **Text generation** defaults to a **word bigram Markov** chain (`Rolando.Markov`): training streams message batches from SQL into the store (no full-corpus RAM load); edges live in **Redis** when `REDIS_URL` is set (`:markov_store` `:redis`), otherwise **ETS**. `GuildWeights` stores a `MARKOV1` marker blob for “has model” in the DB. Set `text_generator: :gru` to use the legacy GRU/LM path.
