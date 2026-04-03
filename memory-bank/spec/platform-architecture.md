# Platform architecture

## Product intent

Rolando is a **per-community stylistic mimic**: it learns from a guild’s messages and media references and produces new text (and optionally reactions, stickers, or media picks) that resemble that community. Architecture centers on a **core** that owns data and algorithms, **interface adapters** (e.g. a team-chat bot, HTTP for OAuth callbacks), and a **web** tier for public pages and authenticated operators.

## Core platform responsibilities

The core **must** provide:

| Concern | Behavior |
|---------|----------|
| **Relational persistence** | Guild records, per-guild configuration, message corpus (or derived training sets), learned generator artifacts, **media metadata** (URLs, attachment-derived CDN URLs, coarse type), analytics event rows. |
| **Analytics** | Append-only (or stream-backed) event recording via a **facade** and **pluggable persistence adapter**; optional cluster-wide fan-out before write (see [analytics.md](analytics.md)). |
| **Caching** | Fast reads for repeatedly accessed data (guild config, permission summaries, rate-limit bookkeeping, etc.) via a **cache adapter**: **in-process** in-memory tables **or** **external** key/value (e.g. memcached-class). Same logical API; deployment chooses backend. Invalidation follows domain rules (guild update, train complete, etc.). |
| **Text generation** | Single **abstract** API: train/load per guild, generate from seed, optional hygiene (pings, cohesion). Concrete implementation (n-gram, GRU-class LM, etc.) is **swappable** behind that API. |
| **Training orchestration** | Jobs that ingest history or batches, update stored artifacts, emit analytics; may use supervised worker pools. |

The core **does not** own Discord gateway sessions, voice codecs, or browser sessions; it receives **plain** guild/channel/user identifiers and structured payloads from adapters.

## Interface: chat bot application

- Subscribes to platform events (messages, interactions, voice-related hooks as supported).
- Maps platform structs to core calls; enforces **platform permission** checks before privileged commands.
- For **operator broadcast**: subscribes to a **named pub/sub topic** (or equivalent cluster bus). On **broadcast envelopes** validated as authorized, performs outbound messages to selected guilds/channels/users using platform APIs.

## Interface: web application (Phoenix-oriented)

- Serves **public** pages (marketing, legal).
- Establishes **session** after **Discord OAuth** (or deployment-equivalent) for operator routes.
- Operator pages query core/read models and subscribe to pub/sub for live updates.
- **Broadcast** UI publishes envelopes to the same logical topic the bot consumes (see below)—**not** by holding long-lived bot tokens in the browser.

## Pluggable cache (behavioral contract)

- **Keys** are namespaced by domain (e.g. guild-scoped) to prevent collisions.
- **TTL** may apply to derived summaries; authoritative data remains in the database.
- **Consistency:** after writes that affect cached entries, either **update** or **invalidate** so stale permission or config data does not drive sends for long windows.
- **Failure:** if external cache is unavailable, implementations may fall back to DB-only reads or in-process cache per policy—**must not** silently skip authorization.

## Distributed messaging (pub/sub)

Uses a cluster-wide bus (process group or equivalent) for:

| Concern | Role |
|---------|------|
| **Cache coherence** | Invalidate or refresh peer nodes’ local cache views when guild state changes. |
| **Analytics** | Optional fan-out so each node can persist the same logical event once (see [analytics.md](analytics.md)). |
| **Live web** | Push to interactive pages (graphs, tables) when new events or metrics arrive. |
| **Operator broadcast** | Operator web publishes **broadcast intents** (target guild id, channel id, optional user id list, message body, correlation id). Bot runtime **subscribes**, validates **authorization was already enforced server-side** on the web tier, then executes sends. **Idempotency** and **rate limits** are recommended on the bot side. |

Envelope contents are **versioned** conceptually (type tag + payload); exact schema is deployment-defined but must distinguish **broadcast** from analytics and cache invalidation.

## Boundaries and security

- **Secrets** (bot token, OAuth client secret, cache passwords, STT/TTS keys) live only in environment or secret stores; never in client bundles.
- **Core** never trusts raw client input without adapter validation; **operator** actions are checked against **allowlist** on the server before publish.
- **Cross-tenant isolation:** guild A’s data must never train or configure guild B; cache keys and DB queries remain guild-scoped.

## Scalability and operations

- Horizontal **web** and **bot** nodes share DB + pub/sub; external cache may become a hotspot—monitor connection counts and key cardinality.
- Training jobs are **CPU/GPU heavy**; isolate in worker pools or separate nodes to avoid starving gateway consumers.
- Voice pipelines (decode → STT → text → core) add **streaming** load; chunk size and back-pressure are operational tunables.

## Open questions

- Single **OTP** release vs **split services** for training at very large scale.
- Whether **broadcast** envelopes require **signed** payloads from web to bot for defense in depth when multiple services share the bus.
