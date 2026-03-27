# Technical decisions & options

## Discord

- **Nostrum** as the Discord gateway/API library (per project direction). Align voice/opus requirements with Nostrum docs when implementing VC.

## HTTP

- **Req** for outbound HTTP (media validation, Discord REST if not covered by Nostrum, future STT sidecars).

## Markov distribution

| Mode | Use case |
|------|----------|
| **Horde** | Supervise `ChainWorker` processes across cluster; dynamic guild membership. |
| **Redis (optional)** | When multiple nodes must share **serialized chain state** or **sharded n-gram maps** too large for single node; use key per `guild_id` or consistent-hash shard. |

Implementation note: the legacy chain is a `map[prefix]map[next]count`. Options in prod: compressed ETS per process + periodic snapshot to Redis; or store **only** training lines in DB/column store and **rebuild** chain on boot (slow) — likely unacceptable for large guilds. Expect **incremental persistence** design in Phase 2.

## Message storage

| Layer | Role |
|-------|------|
| **Postgres** | Guilds/chains/settings, users if needed, job checkpoints, foreign keys. |
| **Column-oriented FOSS DB** | High-volume `content` rows, analytics, possibly deduplicated by `(guild_id, message_hash)` if needed. |

Dev: **single SQLite** file via `ecto_sqlite3` acceptable for both messages and chain metadata until Postgres is wired.

## STT (Vosk successor track)

Legacy: Vosk C API via Go, models under `vosk/models/{lang}`.

Elixir-friendly directions (research when implementing; **no commitment in Phase 1–2**):

- **Sidecar microservice**: Whisper.cpp, Vosk REST wrapper, or Coqui — bot sends audio chunks over HTTP/grpc.
- **Self-hosted**: faster-whisper, NVIDIA NeMo, or continued Vosk in a container Elixir talks to via Port/socket.

Criterion: **same or better** language coverage for target locales; operability inside Compose.

## TTS (legacy)

- Go: `espeak` and Google TTS paths. Elixir: abstract `TTS` behaviour; espeak-ng via Port/OS process or cloud provider behind config.

## Docker Compose (future)

Sketch: `web`, `discord` (or combined release with multiple VMs), `postgres`, `redis`, `column-db`, optional `stt` service, reverse proxy; horizontal scale = duplicate discord consumers with **shard id** env + shared Redis/Postgres.

## Testing

- Core: pure functions for Markov + contract tests for repos with SQLite in test.
- Discord: integration tests with mocks or Nostrum test helpers where available.
