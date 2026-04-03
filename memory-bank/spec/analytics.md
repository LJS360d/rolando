# Analytics

## Purpose

Record **operational and product events** durably and expose them to **operators** (and limited **guild-scoped** views via slash command). Analytics are **orthogonal** to chat-platform transport metrics.

## Scope boundaries

**In scope:** Event shape, pluggable persistence, optional cluster fan-out, **slash** vs **operator** read models.

**Out of scope alone:** Exact SQL DDL; chart library choices.

## Event contract

Each occurrence includes:

- **Event type** — Non-empty string (e.g. `slash_command`, `train_complete`, `broadcast_sent`).
- **Scope** — Optional **guild id**, **channel id**, **user id** as strings.
- **Metadata** — Structured map (command name, error strings, counts, durations).
- **Severity** — Optional classification for log routing before/at persistence.

Stored rows include at minimum: type, optional scopes, metadata, **created at**; implementations may add severity columns.

## Pluggable persistence

Facade delegates to a **configured adapter** (relational table default; optional external SaaS/stream). Callers remain unaware of storage tech.

## Cluster fan-out

Optional **pub/sub** topic so each node’s subscriber writes once; **avoid duplicate logical rows** when mixing direct insert + broadcast without coordination.

## Emission sources (categories)

- **Guild registry** — create/update/delete/cache events.
- **Slash commands** — invoked, failed, completed (with command name).
- **Training** — started, progress milestones, completed, failed (exception text in metadata).
- **Generation** — optional sampling (throttled) for quality monitoring—not full text bodies by default.
- **Broadcast** — operator-initiated sends (target ids, correlation id, outcome).
- **Voice** — join/leave/STT errors (no raw audio in metadata).

## Read models: two audiences

### Guild-scoped slash (`/analytics`)

- **Audience:** Guild members with command access (policy-defined).
- **Content:** **Aggregates and metadata** for **that guild only**: message counts, media bucket counts, last trained time, configured rates, approximate model size, command usage counters—**no** full message listing, **no** other guilds’ data.

### Operator web (authenticated)

- **Audience:** **Allowlisted** operators after OAuth.
- **Content:** **Event list** with filters, **time-series** views suitable for **graphs**, correlation with guild id. May include **system** telemetry exposed alongside product events (e.g. **memory usage bar** for the node or BEAM—implementation-specific but **operator-visible**).

## Non-functional

- Subscriber failures **log** and **continue**; do not crash the loop.
- High volume → batch/async sinks.

## Open questions

- Retention policy and PII **redaction** in metadata for compliance.
- Whether slash **analytics** and operator views share the **same** underlying aggregates or separate rollups for cost control.
