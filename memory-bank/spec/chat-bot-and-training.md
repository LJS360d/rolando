# Chat bot runtime, ingestion, training, and slash commands

## Purpose

Define how the **chat adapter** ingests messages, runs **bulk train** and **live** learning, exposes **slash commands**, handles **media** for random gif/image/video replies, applies **data-driven** outbound choices (text, reaction, sticker, voice), and integrates **voice** with abstract **STT** and **TTS**.

## Actors and permissions

- **Bot user** — Ignores self-authored messages for learning loops; may still log analytics.
- **Human users** — Source text and media references.
- **Guild admin / owner / deployment-defined role** — Commands that mutate training data, re-fetch history, or show sensitive channel lists require **elevated** permission consistent with the chat platform’s admin signals.
- **Operator allowlist user** — May bypass **train cooldown** (deployment-configured); not automatically the guild owner.

## Core behaviors: inbound messages

### Eligibility

- Ignore DMs if the product is guild-only.
- Ignore messages from bot accounts (including self).
- Respect channel visibility: only learn/respond where the bot has **read** and **send** (and **history**) as required by policy.

### Learning (live)

- Messages that pass quality gates are persisted for training and **media extraction**.
- **Attachments** and **embedded media URLs** resolve to **stable CDN URLs** where the platform exposes them; store **type** (gif / image / video heuristic) for later slash commands.
- **Concurrent** updates: heavy work must not block the gateway consumer (supervised tasks).

### Outbound decision (data-driven)

A **policy engine** (configuration + rolls) chooses among:

1. **No action** (most messages).
2. **Generate text** — When **mention** or **reply-rate** roll succeeds; uses **core text generation** with optional seed from message or recent context.
3. **Reply shape** — When sending generated text stochastically: **reply** (thread/reference) vs **standalone** channel message; reference implementation used a secondary split (~10% reply vs ~90% standalone) independent of reply rate.
4. **React** — With **reaction rate**, add **Unicode emoji** and/or **guild custom emoji** from allowed pools.
5. **Sticker** — Optionally send a **guild sticker** when sticker pool non-empty and policy selects sticker path.
6. **Media instead of text** — A unified **roll** may choose gif vs image vs video **from stored pools** before falling back to text when empty (reference bands: skew strongly toward text; exact integers documented as deployment constants).

**Mention path:** If the bot is mentioned, **always** attempt text generation (subject to core errors), not only stochastic paths.

Configuration **reply rate**, **reaction rate**, **cohesion**, **toggle pings**, and **VC join/speak rates** are **per guild** (some VC options may be **per voice channel** where the command allows a channel parameter).

## Slash commands (complete product set)

All listed commands **must** be implemented end-to-end (registration, handler, core side effects, user-visible outcomes).

| Command | Purpose |
|---------|---------|
| **train** | Start bulk history import and model build (see below). |
| **gif** / **image** / **video** | Return a **random** URL of that type from the guild’s **learned pool** (from training + live ingest); fallback behavior if pool empty. |
| **analytics** | Show **numeric / summary** statistics **about the bot in this guild**: counts, rates, last trained time, corpus/message counts, media bucket sizes, model metadata **sizes**—**not** raw training text. |
| **togglepings** | Flip guild flag: strip or allow mention patterns in **generated** output. |
| **replyrate** | Optional integer argument: **view** current value if omitted, **set** if provided. |
| **reactionrate** | Same pattern for reaction stochastic rate. |
| **cohesion** | Optional integer **2–10**: view or set coherence bias for generation. |
| **opinion** | Required seed string: generate text from core using seed (**opinion about**). |
| **wipe** | Required string **data**: remove matching material from **stored** and **live** training state (implementation: substring match, hash match, or documented matching—must be **specified per deployment**; destructive—confirm dangerous cases if needed). |
| **channels** | Show, in a **rich embed** (or platform equivalent), which text channels are **usable** for fetch/learn vs **inaccessible** (permission denied), with legend and guidance on fixing permissions. |
| **vc** | Subcommands (guild-only): **join**, **leave**, **speak**, **joinrate**, **language** — see Voice section. |

### Train command flow

1. **Authorization** — Admin/owner class only.
2. **Cooldown** — If guild was trained within a **deployment window**, reject unless **operator** bypass; show **remaining time** (mm:ss).
3. **First run** — Ephemeral (or private) **confirmation** with **Confirm** button; explains scope (all accessible channels), cooldown, and how to exclude channels (revoke bot perms).
4. **Repeat run** — Warn that **all** prior fetched/training data for the guild will be **deleted** and re-fetched; **Confirm Re-train** button.
5. **Button handlers** — On confirm: defer response, enqueue **Train** job, post public channel notice with **mention**, **estimated throughput** (e.g. time per N messages), completion notice when done.
6. **Job** — Page history per channel (batch size and 429 backoff per reference parameters), persist messages + media URLs, invoke **core** to build/replace generator artifact, update `trained_at`, emit analytics.

## Media storage (gif / image / video commands)

- **Sources:** Bulk train fetch **and** live message learning.
- **Store:** CDN URLs for attachments and embeds; classify **gif** vs **static image** vs **video** by MIME, extension, or platform flags.
- **Dedup** optional but recommended to cap storage growth.

## Voice channel (vc)

### Dependencies

- Use a **library** appropriate to the platform for **voice connection**, **receive** and **send** audio streams, and **lifecycle** (join/leave/reconnect).

### STT (abstract)

- **Interface:** stream **PCM or encoded audio chunks** in → receive **partial and final transcripts** with timestamps.
- **Live:** Transcripts appended to the **same learning path** as text (subject to quality gates), attributed with guild (and optionally voice channel) scope—**no** long-term raw audio retention required unless policy enables it.
- **Provider** is **pluggable** (cloud API, local model); credentials via environment.

### TTS (abstract)

- **Interface:** text in → audio chunks out (format negotiated with voice sink).
- **Uses:** **vc speak** — generate short “words of wisdom” (text from core) then synthesize and play, then leave per command semantics; **vc join** loop — optional periodic or triggered speech driven by policy (join rate).

### Subcommands (behavioral)

| Sub | Behavior |
|-----|----------|
| **join** | Join the voice channel the invoker is in (or error if none). |
| **leave** | Leave current voice channel. |
| **speak** | Optional channel target; generate text → TTS → stream → then leave (or stay per policy). |
| **joinrate** | Optional **0–100%** rate and optional channel scope: probability (or equivalent) that the bot **auto-joins** eligible voice for training/listening. |
| **language** | Discrete set (e.g. English, Italian, German, Spanish) for **STT/TTS** locale selection; optional channel scope; view when args omitted. |

### Non-functional (voice)

- **Back-pressure:** if STT lags real time, drop or coalesce chunks with logged warnings.
- **Privacy:** voice participation implies **consent** in guild rules; product may require **visible** bot indicator when listening.

## Reference implementation parameters (portable constants)

| Parameter | Reference | Meaning |
|-----------|-----------|---------|
| Bulk page size | **100** messages | Pagination per request. |
| Train cooldown | **30** minutes | Between bulk trains unless operator bypass. |
| Min content for text n-grams (Markov line) | **> 3** chars | Skip shorter for n-grams; attachments may still count for media. |
| Stochastic reply vs standalone | **~10%** / **~90%** | When random text fires. |
| Reply/media mix roll | **4…25** integer | ≤21 text; 22–23 gif; 24 image; 25 video (reference Markov bot). |

**HTTP 429:** Sleep per server **Retry-After** or conservative default before continuing same channel.

## Error handling

- Cooldown and permission failures → **ephemeral** user messages.
- Train job partial failure → continue other channels; surface summary on completion.
- Generation failure → log; optional silent skip or short fallback string.

## Analytics (guild slash)

- The **analytics** command’s output is **curated** for the requesting guild; it does **not** dump message bodies or operator-global metrics.

## Open questions

- Exact **wipe** matching semantics (substring vs normalized token) and need for **second confirm** on large deletes.
- Whether **sticker** selection is uniform random or weighted by recency.
