# Operator web UI, public site, and access control

## Purpose

Provide a **browser** experience with:

1. **Public** pages (unauthenticated).
2. **Authenticated** operator tools (Discord OAuth in this product line).

Use **interactive server-connected pages** (e.g. Phoenix LiveView) for dashboards: **lists**, **graphs**, **forms**, and **live refresh** via **pub/sub**—not a JSON-first public API for operators.

## Public pages (mandatory)

| Page | Content |
|------|---------|
| **Marketing / landing** | Product description, invite or CTA as policy allows. |
| **Privacy policy** | Data collected (messages for training, analytics events, OAuth tokens handling), retention, third parties (STT/TTS if used), contact. |
| **Terms of service** | Acceptable use, liability limits as counsel approves, bot behavior disclaimers. |

These are **static or server-rendered**; no operator data required.

## Authentication

- **Discord OAuth** establishes **server-side session** (cookie).
- **Scopes** are the minimum needed for operator identification and any advertised “add to server” flows—exact scope list is deployment-defined.
- **CSRF** protection on state-changing routes.

## Authorization

- **Operator allowlist** — Platform user ids configured in deployment environment; only these users access `/operator` (or equivalent) routes.
- Non-allowlisted authenticated users → **403**.
- Unauthenticated → redirect to login or **401** page for protected routes.

## Operator surfaces (behavioral)

### Analytics dashboard

- **Event stream** or paginated table: filter by type, guild, time range.
- **Graphs** — time-series for key metrics (commands, trains, errors).
- **System memory** — a **visual bar** (or equivalent) showing **process / node memory usage** for operational awareness (values from server introspection, not from client guesswork).

**Realtime:** Subscribe to pub/sub topics for new events so tables/charts **update without full page reload**.

### Guild directory

- **Paginated** table of **all guilds** known to the core (persisted registry).
- Columns: identifiers, name snapshot, member count if cached, **last seen**, training status, artifact presence, flags—**everything needed for ops** without opening each guild’s message corpus inline.

### Broadcast

- **Composer:** message body (length-limited), optional **rich** features per platform capability.
- **Targeting UI:** select **one or more guilds** and/or **channels** by id; optional **user** dm targets where platform permits.
- **Execution:** On submit, server **validates** allowlist and payload, then **publishes** a **broadcast envelope** to the **pub/sub** topic consumed by the **chat bot** application.
- **Bot** receives event, **re-validates** (defense in depth), performs sends, emits **analytics** (`broadcast_sent` / per-target failures).

**No** direct REST from browser to Discord; **all** sends go **server → bus → bot**.

## Non-functional

- **Secure** cookies (HttpOnly, Secure, SameSite per deployment).
- **Rate limit** broadcast actions per operator session.
- **Audit:** store who broadcast what (operator user id + timestamp) in analytics or audit table.

## Open questions

- **2FA** or org SSO beyond Discord for high-assurance operators.
- **Multi-region** pub/sub semantics if web and bot are not co-located.
