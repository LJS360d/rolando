# Progress

## Done (spec alignment — secondary stack)

- Operator web UI: Discord OAuth, allowlisted `/operator` LiveView, pub/sub–driven analytics refresh, memory bar, 7-day event histogram, guild directory pagination, operator broadcast bus to Discord runtime; public privacy/terms pages and layout footer links.
- Analytics: normalized event payloads, durable insert + UI broadcast; removed unused cluster analytics subscriber GenServer.
- Discord: slash command registration trimmed to implemented commands only; operator allowlist env documented.
- Removed unused deps/code: Horde, empty training/neural stub modules, `Training.PoolRegistry` / pool worker, `Analytics.Subscriber`.

## Follow-ups (not done)

- Guild-scoped web pages with live Discord membership checks (spec mentions; requires bot/API integration).
- Cluster-wide analytics broadcast without duplicate SQL rows (needs explicit design for shared DB).
