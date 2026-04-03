# Rolando — specification index

This folder holds an implementation-agnostic product and behavior specification for **Rolando**: systems that learn per-community text style and produce synthetic chat-like output, with interfaces including a team-chat bot and an operator web application.

## Scope of evidence

Specifications are grounded in **product intent** (including analyst-authored requirements) and may be cross-checked against implementations. Where **analyst prompt** and prior spec text conflict, the **prompt wins**; major thematic supersessions are noted in domain files where helpful.

## Domain documents

| Document | Contents |
|----------|----------|
| [platform-architecture.md](platform-architecture.md) | Core platform: persistence, analytics, **pluggable cache** (process-local vs external), pub/sub, chat and web boundaries, operator broadcast path to the bot. |
| [text-generation.md](text-generation.md) | Interchangeable generators (e.g. n-gram / recurrent), storage artifacts, training, **ping stripping**, cohesion, seeded “opinion” generation—**isolated from chat transport**. |
| [chat-bot-and-training.md](chat-bot-and-training.md) | Slash commands, train confirmations, ingestion, **live and bulk** learning, media URL storage, stochastic replies/reactions/stickers, **voice channel** STT/TTS and training loop. |
| [analytics.md](analytics.md) | Event contract, persistence adapter, cluster fan-out; **guild-scoped slash “analytics”** vs **operator** analytics views. |
| [operator-web-ui-and-access.md](operator-web-ui-and-access.md) | Public marketing and legal pages; **Discord-authenticated** operator surfaces: analytics, guild directory, **broadcast** composer with pub/sub delivery to the bot runtime. |

## Cross-cutting concepts

- **Logical community** is modeled as a distinct **guild** with its own configuration and learned artifacts.
- **Text generation** is **per guild**; the **algorithm** (Markov-class, recurrent neural LM, or other supported backend) is selected and executed **inside the core** behind a stable generation contract—**not** inside chat-specific UI code.
- **Cache** for hot metadata (guild config, resolved permissions summaries, etc.) uses a **configured adapter**: in-process memory, or an external key/value service (e.g. memcached-class), with the same behavioral contract.
- **Operator allowlist** — Deployment-configured user identities for privileged web flows and optional train-cooldown bypass; distinct from a guild’s native “owner” unless configured to match.
- **Pub/sub backbone** — Named topics for cache coherence, analytics fan-out, interactive web refreshes, and **operator-initiated broadcasts** consumed by the bot application.

## Where reference numbers live

- **Bot runtime, rates, train cadence:** [chat-bot-and-training.md](chat-bot-and-training.md) — reference-style parameters where fixed for parity (ingest thresholds, cooldown, paging, 429 handling).
- **Analytics and cluster messaging:** [analytics.md](analytics.md), [platform-architecture.md](platform-architecture.md).

## Open questions (global)

- Whether a single deployment must support **runtime switching** of text backends per guild or only **deploy-time / operator** choice.
- Standard **topic naming** and **envelope shapes** for UI vs operator-broadcast vs internal-only messages across deployments.
- Fine-grained **operator roles** beyond binary allowlist (if needed later).

## Supersedes (analyst pass)

- **Channels command** is specified as an **embed** (or equivalent rich presentation) listing trainable vs blocked channels, not plain message text alone.
- **Web surface** must include **public** marketing plus **privacy policy** and **terms of service**, plus **Discord-authenticated** operator tools including **broadcast** with selection UI and pub/sub to the bot.
