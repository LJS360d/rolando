# Text generation

## Purpose

Provide **per-guild** synthetic text that statistically resembles that guild’s past messages, behind a **single abstract contract**. Chat adapters, slash commands, and voice-derived text **call the core**; they do **not** embed algorithm-specific logic.

## Scope boundaries

**In scope:** Generator selection, training/update, inference, post-processing (pings, cohesion), storage artifacts.

**Out of scope alone:** Discord API details, browser UI, moderation policy beyond output hygiene flags.

## Abstraction boundary (mandatory)

- **Upstream** passes: guild identifier, optional **seed text**, optional **generation options** (temperature analog, max length, cohesion-related parameters exposed by config).
- **Core** returns: success + text, or typed errors (untrained, empty corpus, incompatible artifact version).
- **Concrete algorithms** (word-level Markov, GRU-class recurrent LM, transformers if ever added) implement a **shared internal interface**: `fit`, `generate`, `serialize`, `deserialize`, `compatible?`.
- Swapping algorithms for a guild generally requires **new training** from stored messages; **no** silent mixing of incompatible checkpoints.

## Generation backends (interchangeable)

Deployments choose **one** active backend per guild (or globally); switching implies **re-train** unless a documented migration exists.

### Backend family A — N-gram / Markov-class

- Directed transition structure over token or word n-grams; counts update from tokenized text.
- Suitable for lightweight deployments; may differ in tokenization from neural backends.

### Backend family B — Recurrent / sequence model (e.g. GRU-class)

- Fixed tokenizer; per-guild weights (and optional codebook for word display).
- Training is **batch-oriented** after corpus ingest, not necessarily every single message in tight loops unless product policy adds online fine-tuning.

**Parity note:** Both families must honor **the same** post-processing flags (e.g. **togglepings**).

## Configuration knobs (guild-scoped where applicable)

| Knob | Effect |
|------|--------|
| **Reply rate** | Stochastic probability of “speaking” on inbound messages (see chat-bot spec); encoded as a positive integer with defined semantics (e.g. `1` = always eligible; higher = rarer). |
| **Reaction rate** | Independent roll for emoji/sticker-style reactions. |
| **Cohesion** | Bounded integer (minimum **2**, maximum **10** in product): higher values bias generation toward **more locally coherent** continuations (implementation-specific: longer context window, beam bias, or temperature schedule). |
| **Toggle pings** | When **off**, post-processing **strips** user/role/channel mention patterns from **model output** before send so generated text cannot accidentally ping. When **on**, same patterns may remain subject to platform constraints. |

## Seeded generation (“opinion”)

- **Input:** required seed string (`about` or equivalent).
- **Output:** generated continuation **conditioned** on the seed—interpreted as a “random opinion” **about** the seed topic.
- Must behave even when seed is short or OOV-heavy (fallback: sample from generic context or use tokenizer UNK policy).

## Media correlation

- Text generation does **not** replace **media pools**; gif/image/video slash commands draw from **stored URL sets** filled during ingest (see chat-bot spec). Optional **joint** behavior: generation may be skipped when the outbound strategy selects media instead.

## Data artifacts (behavioral)

- **Generator state:** opaque blob + version metadata; incompatible versions **refuse** load.
- **Optional codebook** for subword→word display for neural paths.
- **No cross-guild** weight sharing for per-guild training (shared frozen tiers, if any, are deployment-global and **not** updated from guild gradients—per target architecture in prior art).

## Training flows (core)

- **Bulk:** After historical fetch, build/update artifact from full or filtered corpus; set `trained_at`-class timestamp; emit analytics.
- **Live:** Incremental updates from accepted inbound messages (if enabled): either online n-gram updates or buffered batches for neural—product must state which is active; **both** must persist **media URLs** when present.

## Failure modes

- Untrained or empty artifact → **typed error**; adapter responds with user-safe silence or fixed message.
- **Wipe** / partial delete → rebuild or patch artifact per policy (Markov may decrement counts; neural may require partial re-train or full rebuild).

## Non-functional

- Inference must be **bounded** (token/step caps) to avoid gateway timeouts.
- Training must not **block** the process that reads chat events; use supervised tasks.

## Open questions

- Whether **A/B** two backends per guild is ever supported (default: no).
- Exact **cohesion** mapping to each backend’s parameters without overfitting small corpora.
