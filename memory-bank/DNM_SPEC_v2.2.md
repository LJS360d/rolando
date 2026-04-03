# PROJECT SPECIFICATION: ROLANDO — DISTRIBUTED NEURAL MESH (ELIXIR/BEAM)
## Version 2.2 — Architectural Blueprint

**Codename:** Rolando

---

## 1. MODEL ARCHITECTURE & MEMORY MODEL

The core memory problem in any per-guild neural system is the **embedding table and output projection layer**, which scale with vocabulary size and dominate memory cost — not the recurrent weights. The solution is to split the model into a **shared system-wide component** and a **per-guild component**.

### 1a. Shared Layers (initialized once at system startup, permanently frozen)

**What they are:**

* **Input Embedding Table** — a matrix of shape `[vocab_size × embedding_dim]`. Each row is a dense vector representing one token in the vocabulary. When the tokenizer converts a raw text string to a sequence of integer IDs (e.g., `"hello world"` → `[4312, 995]`), each integer is used to index into this table and retrieve its corresponding vector. These vectors are what the GRU actually processes — it never sees raw text or token integers directly.

* **Output Projection Layer** — a weight matrix of shape `[hidden_dim × vocab_size]`. After the GRU produces a hidden state vector for the last token in a sequence, this layer projects it back into a distribution over the entire vocabulary, producing the logits from which next-token prediction is made.

**These layers are permanently frozen.** They are initialized exactly once at system startup using Xavier uniform random initialization, serialized to the `shared_weights` DB table, and never modified again by any process. No guild's training step produces gradients that flow into these layers — ever. They are fixed infrastructure, equivalent to a deterministic projection function.

**Why frozen, and why random:**

The alternative — updating shared layers from guild training data — would require gradient contributions from multiple guilds to flow into a single shared component. This creates an irreconcilable conflict: any update to the shared embedding from guild A's messages changes the vector space that guild B's GRU is operating in, silently altering guild B's model behavior without guild B's knowledge or consent. There is no correct weighting scheme that resolves this, because guilds are not comparable data sources — they differ in language, size, activity rate, and content domain. Shared layer updates are therefore categorically prohibited.

The absence of a pretrained embedding matrix is equally intentional. Loading a pretrained embedding (e.g., from a general-purpose English language model) would embed a strong prior for one language and one distribution of text into the fixed foundation that every guild's GRU is built on. A guild that communicates primarily in Portuguese, Japanese, or internet-specific vernacular would train its GRU against an embedding space that was never built for its content. Rolando's identity is that each guild's model emerges entirely from that guild's own messages. A pretrained foundation contradicts this at the architectural level.

**What frozen random embeddings actually provide:**

A randomly initialized and frozen embedding table is a fixed random projection from the space of token IDs into a continuous vector space. The GRU does not require this projection to be semantically meaningful — it is expressive enough to learn which directions in that fixed vector space correlate with which output patterns, purely from the guild's own data. The embedding table's job is to give the GRU something to differentiate tokens by; a stable, consistent random projection is sufficient for this. The semantic structure of the guild's language emerges in the GRU weights, not in the embedding table.

**Initialization and recovery:**

At first startup, if no `shared_weights` entry exists in the DB, Xavier uniform initialization is applied and the result is persisted immediately. On all subsequent startups, the frozen matrices are loaded from the DB unchanged. This ensures that all guild GRU weights trained against a given frozen projection remain valid across system restarts. The frozen matrices must never be regenerated unless all guild weights are also reset — a new random projection invalidates all existing per-guild GRU weights.

**The tokenizer is also fixed and version-pinned.** It is a pre-built BPE model (sentencepiece or HuggingFace `tokenizers`), loaded once at startup and never modified. The vocabulary size of the tokenizer defines the first dimension of both the embedding table and the output projection. Changing the tokenizer after deployment requires reinitializing all shared and per-guild weights.

---

### 1b. Per-Guild Component (one GenServer per Discord server)

* **GRU weight matrices only.** A standard GRU has six weight matrices:
  * Three input-to-hidden matrices (`W_z`, `W_r`, `W_h`), each of shape `[embedding_dim × hidden_dim]`
  * Three hidden-to-hidden matrices (`U_z`, `U_r`, `U_h`), each of shape `[hidden_dim × hidden_dim]`
  * Six bias vectors of length `hidden_dim`
  * Total parameters: `3 × hidden_dim × (embedding_dim + hidden_dim + 1) × 2` (for the input and hidden halves, plus biases)

* **Optional per-guild output bias** — a single vector of length `vocab_size`. Added to the shared output projection's logits before softmax. Acts as a lightweight vocabulary frequency adjustment: the guild's model learns to boost tokens that appear often in its specific chat, without retaining the full output projection. Cost: `vocab_size × 2 bytes` (FP16) = 16KB on Minimal tier.

* **Context ring buffer** — the last N token IDs from recent messages, held in RAM as a sequence to feed the GRU. This is not a "context window" in the Transformer sense. The GRU compresses all history into its fixed-size hidden state vector; the ring buffer simply provides the unrolled input sequence for the next forward or training pass (see Section 10 for GRU memory span caveats).

---

### 1c. Configurable Tiers

Each parameter's memory impact is labeled: **(S)** = affects Shared memory, **(G)** = affects per-Guild memory.

| Parameter | Minimal (default) | Standard | Full | Memory impact |
|---|---|---|---|---|
| Vocabulary Size | 8,192 | 16,384 | 32,768 | **(S)** Scales embedding table AND output projection linearly |
| Embedding Dim | 64 | 128 | 256 | **(S)** Scales embedding table · **(G)** Scales GRU input-to-hidden matrices |
| GRU Hidden Dim | 128 | 256 | 512 | **(G)** Scales GRU hidden-to-hidden matrices quadratically · **(S)** Scales output projection linearly |
| Active Precision | FP16 | FP16 | FP32 | **(S+G)** Doubles all memory when moving FP16→FP32 |
| Context Buffer | 64 tokens | 128 tokens | 256 tokens | **(G)** Negligible (integers, ~512B–2KB) |

**Breakdown of actual memory costs per tier (derived figures):**

Shared memory formula:
* Embedding table: `vocab_size × embed_dim × bytes_per_param`
* Output projection: `hidden_dim × vocab_size × bytes_per_param`

Per-guild GRU formula:
* `3 × hidden_dim × (embed_dim + hidden_dim + 2) × bytes_per_param`

| | Minimal (FP16) | Standard (FP16) | Full (FP32) |
|---|---|---|---|
| Embedding table | 8192×64×2 = **1.0MB** | 16384×128×2 = **4.0MB** | 32768×256×4 = **32MB** |
| Output projection | 128×8192×2 = **2.0MB** | 256×16384×2 = **8.0MB** | 512×32768×4 = **64MB** |
| **Shared total** | **~3MB** | **~12MB** | **~96MB** |
| Per-guild GRU | 3×128×(64+128+2)×2 = **~149KB** | 3×256×(128+256+2)×2 = **~591KB** | 3×512×(256+512+2)×4 = **~4.5MB** |
| Per-guild output bias (optional) | 8192×2 = **~16KB** | 16384×2 = **~32KB** | 32768×4 = **~128KB** |
| **Per-guild total (active)** | **~165KB** | **~623KB** | **~4.6MB** |
| Ternary snapshot (hibernated) | ~19KB | ~74KB | ~565KB |

The dominant driver of **shared memory** is `vocab_size`. The dominant driver of **per-guild memory** is `hidden_dim` (quadratic) followed by `embed_dim` (linear in the input-to-hidden matrices). Halving `hidden_dim` reduces per-guild memory by ~4× for the hidden-to-hidden matrices alone.

---

## 2. PRECISION & QUANTIZATION STRATEGY

Two precision modes are available and selected per-deployment via config.

* **Mode A — Standard (FP16 Active / 1.58-bit Persisted):**
  * Master weights are maintained in FP32 during training for gradient stability. For active inference, they are cast to FP16.
  * On checkpointing, FP32 masters are quantized to ternary representation (-1, 0, 1) using **stochastic rounding** rather than round-to-nearest. Stochastic rounding is unbiased in expectation and reduces the systematic error accumulation that causes progressive drift over many checkpoint cycles.
  * On recovery, ternary weights are dequantized back to FP32 masters.
  * **Drift monitoring:** The system computes a before/after quantization loss delta at each checkpoint. If the delta exceeds a configurable threshold (default: 15% relative increase), the checkpoint is flagged in telemetry and the active learning rate is temporarily halved until the next clean checkpoint.
  * FP16 active training requires gradient clipping (Section 4) to be unconditional. Loss scaling is not implemented — if FP16 underflow is detected (gradients collapse to zero), the system automatically falls back to FP32 for that guild's update step and logs a warning.

* **Mode B — BitNet 1.58 (native ternary training, recommended for long-running deployments):**
  * The model maintains **FP32 latent weights** alongside a derived ternary representation. During each forward pass, the latent weights are ternarized on the fly: each weight is mapped to `sign(w)` scaled by the mean absolute value of its weight matrix. Computation proceeds in ternary.
  * Gradients are computed with respect to the latent FP32 weights via a straight-through estimator. Only the latent weights are updated by the optimizer.
  * Because the model **trains against quantized weights from the start**, it learns representations that are inherently robust to quantization. There is no precision mismatch between training time and persistence time — they are the same.
  * Persistence: save the FP32 latent weights at checkpoints. Hibernation stores the derived ternary snapshot. On wake, the ternary snapshot is loaded and latent weights are initialized from it.
  * The tradeoff is slightly higher computational cost per forward pass due to on-the-fly ternarization and the need to maintain both latent and ternary representations simultaneously.

---

## 3. MEDIA EXTRACTION MIDDLEWARE

URL content — images, GIFs, videos, and generic links — is stripped from all messages **before they enter the training pipeline**. This is a hard preprocessing stage, not optional. URLs embedded in training sequences damage speech pattern coherence by inserting long, unpredictable character strings into the next-token prediction objective. They also dominate the token budget for short messages.

**Process (executes on every incoming message before any ML step):**

* Step 1 — URL Scan: Regex sweep for all `http://` / `https://` URLs, Discord CDN patterns (`cdn.discordapp.com`, `media.discordapp.net`), and known media shortlink domains (tenor, giphy, imgur, etc.).
* Step 2 — Classification: Each URL is classified into `image`, `gif`, `video`, or `generic` by extension or domain heuristic.
* Step 3 — Context Hash: A lightweight hash is computed over the non-URL tokens in the message. This links the media URL to its conversational context without storing full message text.
* Step 4 — MediaStore Write: All extracted URLs are written to the MediaStore with their classification, guild ID, context hash, and timestamp. This write is fire-and-forget — async and non-blocking to the training pipeline.
* Step 5 — Message Rewrite: URLs are removed from the string. The cleaned text proceeds to the Quality Gate (Section 5).

**URL-only messages:** After stripping, if the cleaned message is empty or whitespace-only, the message is consumed entirely by MediaStore. The training pipeline receives nothing.

**MediaStore** is a logically standalone service with its own DB table and query interface. It is independent of the ML pipeline and can be queried separately — e.g., to surface past media for a guild by context, type, or date range.

---

## 4. LIVE TRAINING PIPELINE (BATCH-BUFFERED)

Training does not occur on every individual message. Messages are buffered per guild, and a gradient step is computed over the batch. Batch size 1 produces gradient estimates too noisy for reliable learning, especially on short, real-world chat text.

**Per-guild message ring buffer:**
* Capacity: configurable, default 16 sequences (Minimal tier), 32 (Standard/Full).
* Each slot holds one tokenized, cleaned sequence.
* A flush timer (default: 60 seconds of inactivity) triggers a step if the buffer has at least 4 sequences. Buffers with fewer than 4 sequences at flush time are discarded.

**Very Long Messages:** A message longer than the context buffer length (e.g., 64 tokens on Minimal) is handled by **sliding window chunking** rather than simple truncation:
* The message is split into overlapping windows of `context_buffer_size` tokens with a stride of `context_buffer_size / 2`.
* Each window is added as a separate sequence to the training buffer, up to a hard cap of **4 chunks per message** to prevent a single paste from flooding the buffer.
* Any tokens beyond the 4-chunk cap are discarded.
* This approach extracts more training signal from long messages (code pastes, copypasta, long explanations) while bounding their contribution.
* For inference, the same sliding window is applied with the GRU hidden state carried forward from the end of one chunk to seed the next.

**Training Step (fires when buffer is full or flush timer triggers):**
* Step 1 — Batch Assembly: Sequences are padded/truncated to a fixed length and assembled into a batch tensor. Optional per-sequence loss weighting applied if `weighted_loss: true` (longer sequences weighted up to `min(len / 16, 1.0)`; off by default).
* Step 2 — Forward Pass: Shared embedding layer encodes token IDs. GRU processes the sequence with per-guild weights. Shared output projection produces logits.
* Step 3 — Loss: Cross-entropy next-token prediction loss over all non-padding positions.
* Step 4 — Backprop: Gradients flow **only** to the per-guild GRU weights and the per-guild output bias. The shared embedding table and output projection are frozen and receive no gradient updates from any guild, ever. Backpropagation stops at the boundary between the per-guild and shared components.
* Step 5 — Gradient Clipping: Global gradient norm clamped to `1.0` unconditionally. Not configurable.
* Step 6 — Weight Update: Adam optimizer (or SGD with momentum, configurable). Default LR: `1e-4`.
* Step 7 — Buffer Clear: Ring buffer resets. Per-guild volatile state updated in RAM.

No persistence on every training step. Hot weights live in RAM between checkpoints.

---

## 5. NOISE & MESSAGE QUALITY HANDLING

After media extraction, messages pass a lightweight quality gate. The goal is to pass as much signal as possible — guild identity lives in short, idiomatic messages — while discarding content with no learnable structure.

* **URL-only messages:** Consumed by MediaStore entirely. No training involvement.
* **Single-word messages:** Pass through unconditionally. Part of guild identity. The batch-buffered approach means they contribute 1/16th–1/32nd of one gradient step.
* **Emoji-only messages:** Configurable (`emoji_skip`, default: true). Skipped if no non-emoji characters remain after URL stripping. Emoji alone provide near-zero next-token structure. Threshold configurable via `min_non_emoji_chars` (default: 1).
* **Bot command invocations:** Messages beginning with a configured prefix list (e.g., `!`, `/`) are skipped. These are not natural language.
* **The bot's own generated messages:** Messages authored by Rolando's own Discord user ID are **always** filtered from training. Without this, the model enters a feedback loop that reinforces its own outputs, progressively detaching from the guild's natural language.
* **Other bots' messages:** Filtered by default using Discord's `author.bot` flag (`filter_bot_authors: true`, configurable). Guild admins may selectively allow specific bot IDs if desired.
* **Sequences >50% `<UNK>` tokens:** Discarded after tokenization. These represent text in scripts or character sets outside the tokenizer's vocabulary and contribute no learnable structure.

---

## 6. CHECKPOINTING & PERSISTENCE

* **Trigger:** Every 100 messages processed by a guild, OR every 30 minutes of active training — whichever fires first.

* **Standard Mode:** Freeze master weights → stochastic-rounding quantize to ternary → serialize binary → write to `guild_weights` with version increment and perplexity snapshot.

* **BitNet Mode:** Serialize FP32 latent weights directly for full-fidelity checkpoints. For hibernation specifically, derive and store the ternary snapshot for compact storage, accepting a small precision loss.

* **Recovery:** Standard: load ternary → dequantize to FP32. BitNet: load FP32 latent if available; fall back to ternary snapshot. The `version` field enables rollback to a previous checkpoint if the current one is flagged as drifted.

---

## 7. COLD START, VALIDATION & GUILD LIFECYCLE

* **New guild — cold start:** GRU matrices are initialized with Xavier uniform. The per-guild output bias is initialized to zero. No global seed checkpoint is used — outputs will be incoherent from the start and that is expected and acceptable.

  To calibrate expectations: a typical guild of ~10,000 lifetime messages will have roughly 30% or more consumed by the quality gate as single-word noise, media-only messages, command invocations, and emoji-only content. Of the remaining ~7,000, the sliding window chunker and `<UNK>` filters remove further marginal content. A realistic usable training pool is ~5,000–6,000 sequences. At the default batch size of 16, this yields roughly 300–375 training steps before the guild's model has seen all its historical data.

  In practice, coherent sentencing patterns begin to emerge after approximately **1,000 quality messages** (roughly 60 training steps at batch size 16). Before that threshold, outputs are effectively noise — grammatically broken, topically arbitrary. This is a deliberate design position: Rolando learns exclusively from the guild's own data, from zero, which means the cost of isolation is a cold start period. There is no shortcut that does not compromise guild independence.

  The `Cold` lifecycle state (Section 7) exists precisely to track and communicate this — downstream consumers (the bot's response logic) should check whether a guild has crossed the coherence threshold before attempting to use model output conversationally.

* **Validation (in-training rolling metric):**
  * Rolling perplexity is computed over the last 10 training batches.
  * If rolling perplexity increases monotonically over 5 consecutive checkpoints, a `learning_degraded` event is emitted to telemetry. The only automatic response is the LR halving from Section 2. Human review is expected beyond that.

* **Guild lifecycle states:**
  * `Active`: messages arriving, buffer accumulating, full weights in RAM.
  * `Idle`: no messages recently, weights still in RAM, buffer paused. Transitions to `Hibernated` after configurable inactivity (default: 10 minutes).
  * `Hibernated`: weights serialized and freed from RAM. GenServer exits or enters minimal watch mode. Wakes on next message with latency of one DB read plus dequantization.
  * `Cold`: guild has never accumulated enough quality messages to cross the coherence threshold (~1,000 quality messages). Weights are at initialization values. GRU outputs are incoherent and should not be used conversationally. The guild's GenServer tracks a `quality_message_count` field and transitions out of `Cold` automatically once the threshold is reached. The threshold is configurable (`cold_threshold`, default: 1000).

---

## 8. SYSTEM LOAD BALANCING

* **CPU throttling:** A `NimblePool` or `PartitionSupervisor` limits simultaneously executing training steps to `N` workers (default: `num_cores × 2`). Inference (forward pass only) bypasses the pool — it is fast enough that queuing it would add more latency than it prevents.

* **Hibernation as the primary scaling mechanism:** At any moment the majority of guilds are hibernated. Active guild count is RAM-bounded, not guild-count-bounded. Example: 2,500 guilds, 100 active, 2,400 hibernated on Minimal tier: `3MB + (100 × 165KB) + (2,400 × 19KB) ≈ 3 + 16 + 46 = ~65MB`.

* **Backpressure:** If the pool is saturated and a guild's primary buffer is full, incoming messages overflow to a secondary buffer (`2×` primary capacity). If that also fills, the oldest messages are evicted (FIFO). Message loss under extreme load is preferable to OOM or pipeline stalls.

---

## 9. THE NIF BACKEND

Pure-Elixir code runs on the BEAM virtual machine, which is optimized for concurrency and fault tolerance — not for numerical computation. Matrix multiplication in pure Elixir will be 50–200× slower than equivalent native code, making the training latency targets unachievable without a native code layer.

**NIFs (Native Implemented Functions)** are functions written in C or Rust, compiled to a shared library (`.so` on Linux), and called from Elixir as if they were regular functions. The BEAM loads the library at startup and routes calls to it directly, bypassing the bytecode interpreter for those operations.

For Rolando, the NIF layer is implemented using **Rustler** — a framework for writing safe Elixir NIFs in Rust. The Rust code uses the `ndarray` crate for tensor operations and optionally `blas-src` + `openblas-src` for multi-threaded BLAS-accelerated matrix multiplication.

**What the NIF provides:**
* Access to CPU SIMD instructions (AVX2, AVX-512) via the Rust compiler's auto-vectorization and explicit intrinsics.
* Multi-threaded BLAS routines (`dgemm`, `sgemm`) for matrix-matrix multiplication — the dominant GRU operation.
* Controlled memory layout (row-major/column-major, cache-aligned allocation) for optimal throughput.
* Expected speedup over pure-Elixir: **10–100× for matrix operations**, bringing a 16-sequence Minimal tier training step from ~1–3 seconds to target <300ms.

**Tradeoffs and risks:**

* **A panic or segfault in a NIF crashes the entire BEAM VM**, not just the calling process. This is the most significant operational risk.
  * Mitigation: all NIF calls are dispatched via **dirty CPU schedulers** (`dirty_cpu` flag in Rustler). Dirty schedulers run on separate OS threads isolated from the main BEAM scheduler pool. They do not prevent a crash from killing the VM, but they prevent long-running NIF calls from blocking BEAM's cooperative scheduler, maintaining system responsiveness during training.
  * Additional mitigation: NIF calls are made from dedicated `NimblePool` worker processes, not from guild GenServers directly. A worker crash is caught by the pool supervisor, not the guild.
* **Compilation complexity:** Building the NIF requires a Rust toolchain on every build and CI machine. Cross-compilation (e.g., building on macOS for Linux deployment) requires explicit target configuration. This must be treated as a build system dependency, not an afterthought.
* **Debugging is harder:** No BEAM process introspection for NIF code. Crashes surface as VM exits or opaque error tuples, not as Elixir exceptions with stack traces. Rust-native tooling (lldb, `cargo test`) is required for debugging the NIF layer.

**Alternative path — Nx + EXLA:**
Elixir's `Nx` library with the `EXLA` backend (wrapping Google's XLA compiler) provides hardware-accelerated tensor operations without writing any Rust. XLA JIT-compiles tensor programs to native code with AVX and optional GPU support. The tradeoff is a significantly larger dependency (XLA is a large build), less predictable memory behavior, and tighter coupling to the Nx/EXLA API surface. This is a valid alternative if the team prefers to stay within the Elixir ecosystem and avoid Rust entirely. The `Axon` library (Elixir's neural network framework, built on Nx) provides GRU implementations out of the box and may eliminate the need to implement GRU forward/backward passes manually.

---

## 10. VECTOR DATABASE — ASSESSMENT

**Is a vector database appropriate for Rolando's core training loop?** No. The GRU weights are themselves a compressed representation of the guild's language patterns. Injecting retrieved vectors into the training process would conflate two fundamentally different approaches to memory (parametric vs. retrieval-augmented) in a way that complicates the system without a clear benefit.

**Where a vector database does add genuine value:**

* **Inference augmentation (optional, significant quality uplift):** Before generating a response, retrieve the K most semantically similar past messages from the guild's vector store and inject them as a context prefix to the GRU's input sequence. This is RAG applied to inference only — the model's weights are unchanged. For a GRU with a short effective memory span (~20–40 tokens), retrieved context meaningfully extends topical coherence beyond what the hidden state alone can carry.

* **MediaStore semantic search:** Rather than querying past media by exact context hash, a vector index over the context embeddings enables queries like "find past images related to this topic" — the context hash becomes a vector.

**FOSS options:**

* **pgvector** — A PostgreSQL extension that adds a native vector column type and approximate nearest-neighbor index (`ivfflat`, `hnsw`). If Rolando already uses PostgreSQL in production, this is the lowest-complexity option: no additional service, no new infrastructure, queried via Ecto with a small extension to the adapter. Recommended for most deployments.

* **Qdrant** — A standalone vector database written in Rust, self-hostable, with an HTTP and gRPC API. Qdrant is better suited than pgvector when the vector index is large (millions of entries) and query latency needs to be sub-5ms. It adds infrastructure complexity (another service to deploy and monitor) but offers a cleaner separation of concerns. An Elixir HTTP client is sufficient to interact with it. Recommended if inference augmentation is a first-class feature rather than an experiment.

**In both cases, the vector DB is an optional inference-tier enhancement**, not part of the core training pipeline. It is excluded from base memory targets. If enabled, the inference path in each guild GenServer gains a retrieval step before the GRU forward pass.

---

## 11. EDGE CASES

* **Guild with zero natural language** — entirely emoji, commands, or bot output. After quality filtering, the training buffer never fills. The guild remains in `Cold` state with initialized weights. Outputs are incoherent. Acceptable — the bot has nothing to learn from.

* **Multilingual guild** — a single tokenizer serves all guilds. A multilingual sentencepiece model handles this at the cost of slightly less efficient tokenization for any single language. A `tokenizer_model` config key allows swapping to a language-specific tokenizer, though this changes the vocabulary and invalidates all existing weights.

* **Sudden activity spike** — viral event, large event, raid. Thousands of messages in minutes. The NimblePool saturates; the overflow buffer handles the backlog. If even the overflow fills, the oldest messages are evicted. The model misses some training during the peak but does not crash.

* **Weight corruption or incompatible checkpoint** — the stored binary is corrupt or was written by a different tier/schema version. Recovery: delete the guild's weight entry and reinitialize from scratch (Xavier uniform). The guild re-enters the `Cold` state and must re-accumulate training data. The `version`, `tier`, and `precision_mode` columns in `guild_weights` allow schema incompatibility to be detected before attempting deserialization.

* **Tier change mid-life** — changing a guild's tier (e.g., Minimal → Standard) changes GRU matrix shapes. Existing weights are **incompatible** and cannot be migrated. A tier change is therefore a destructive operation that resets the guild's learned memory. This must be surfaced clearly in the admin UI and confirmed explicitly before applying.

* **Shared layer poisoning** — a single very active guild with unusual patterns could gradually pull the shared embedding and output projection toward its own idiolect, degrading output quality for all other guilds. The `0.1×` LR multiplier and tighter gradient clip (`0.1`) on shared layers are the primary defenses. A second defense is a per-guild contribution cap: if a single guild contributes more than X% of all training steps in a rolling window, its shared-layer gradient contribution is further down-weighted for that window (configurable, off by default).

* **Extremely long message (full code paste, essay, novel excerpt)** — the 4-chunk sliding window cap applies. Tokens beyond the cap are discarded. No special handling needed beyond what Section 4 already defines.

* **Token ID out of vocabulary** — characters outside the tokenizer's training distribution (unusual unicode, scripts not in the BPE model) are mapped to `<UNK>`. Sequences that are >50% `<UNK>` after tokenization are discarded (Section 5).

* **Rolando responding to itself** — if Rolando generates a message in a channel and that message is fed back into the training pipeline, the model enters a self-reinforcing feedback loop. Mitigation: the bot's own Discord user ID is registered in config and filtered unconditionally at the Discord consumer layer before any preprocessing occurs.

* **Discord shard failover / reconnect** — the Nostrum consumer may emit duplicate message events or miss events during reconnect. Duplicate training is acceptable (minor) and gracefully handled by the batch buffer — a duplicated message occupies one extra buffer slot. Missed messages are unrecoverable and acceptable as training signal loss. Persistence is checkpoint-based, not event-log-based, so there is no replay mechanism.

* **Guild deletion or bot removal** — the guild's GenServer must be stopped and its DB entries optionally purged. A `GuildMonitor` process (subscribed to Nostrum's guild leave/delete events) handles cleanup. Data retention policy (purge immediately vs. keep for configurable duration) is configurable.

---

## 12. DATABASE SCHEMA & ADAPTERS

**Tables:**

`guild_weights`
| Column | Type | Notes |
|---|---|---|
| guild_id | string (PK) | Discord guild snowflake |
| weight_data | binary | Serialized ternary snapshot or FP32 latent blob |
| precision_mode | enum | `standard` or `bitnet` |
| tier | enum | `minimal`, `standard`, `full` — used to detect incompatible recovery |
| version | integer | Monotonically increasing per guild |
| perplexity | float | Rolling perplexity at checkpoint time |
| updated_at | datetime | |

`media_store`
| Column | Type | Notes |
|---|---|---|
| id | bigint (PK) | Auto |
| guild_id | string | |
| url | string | |
| media_type | enum | `image`, `gif`, `video`, `generic` |
| context_hash | string | Hash (or optionally vector, if pgvector enabled) of surrounding tokens |
| inserted_at | datetime | |

`guild_config`
| Column | Type | Notes |
|---|---|---|
| guild_id | string (PK) | |
| batch_size | integer | Training buffer capacity |
| learning_rate | float | |
| weighted_loss | boolean | |
| emoji_skip | boolean | |
| filter_pings | boolean| |
| 
| filter_bot_authors | boolean | |
| tokenizer_model | string | Path override for guild-specific tokenizer |
| vector_augment | boolean | Enable inference-time RAG augmentation |

`shared_weights`
| Column | Type | Notes |
|---|---|---|
| id | integer (PK) | Single row per system |
| embedding_data | binary | Serialized frozen embedding table |
| projection_data | binary | Serialized frozen output projection |
| tier | string | Must match current tier — mismatch requires full reset |
| initialized_at | datetime | Written once at first startup, never updated |

**Adapter behaviour (`StorageProvider`):**
* Dev: `SqliteAdapter` — local `.db` file, zero infrastructure.
* Prod: `PostgresAdapter` (default) or `S3Adapter` (binary blobs to object storage, metadata to Postgres). S3/object storage is preferred for weight blobs at scale; Postgres for `guild_config`, `media_store`, and `shared_weights` regardless of blob backend.

---

## 13. HONEST PERFORMANCE TARGETS

*All Minimal tier figures assume 4-core VPS, FP16 active, NIF backend enabled.*

**Shared system memory (once, regardless of guild count):**
* Minimal: ~3MB · Standard: ~12MB · Full: ~96MB

**Per-guild active RAM:**
* Minimal: ~165KB · Standard: ~623KB · Full: ~4.6MB

**Per-guild hibernated:**
* Minimal: ~19KB · Standard: ~74KB · Full: ~565KB

**2,500 guilds example (100 active / 2,400 hibernated, Minimal tier):**
* `3MB + (100 × 165KB) + (2,400 × 19KB) ≈ ~65MB` — comfortably within a 2GB VPS.

**Training batch latency (16-message batch, Minimal, NIF backend):**
* Target: `<300ms`. Without NIF: `1–3 seconds` (still acceptable — training is async and non-blocking).

**Inference latency (single forward pass, Minimal, NIF backend):**
* Target: `<50ms`. Without NIF: `<500ms`.

**GRU effective memory span:**
A GRU hidden state is a fixed-size vector, not a sliding window. It compresses all history into its hidden dimension. In practice, GRUs reliably track dependencies over roughly 20–40 tokens — signal from beyond that decays. The context ring buffer feeds the last N tokens as the unrolled input sequence, partially mitigating this, but this is not equivalent to a Transformer attention window. Coherence is topical within a short exchange, not across long threads.

---

## 14. PROJECT STRUCTURE (ROLANDO UMBRELLA)

The Elixir umbrella layout below maps functional domains to their correct app homes. The `rolando` core app owns all ML logic, persistence, and preprocessing. `rolando_discord` is a thin consumer layer. `rolando_web` provides operational visibility.

```
rolando_umbrella/
├── apps/
│   │
│   ├── rolando/                          # Core: all ML, training, persistence, media
│   │   ├── lib/
│   │   │   ├── rolando/
│   │   │   │   ├── neural/
│   │   │   │   │   ├── shared_weights.ex       # GenServer: loads frozen embedding + output projection at startup, never updated
│   │   │   │   │   ├── guild_model.ex          # GenServer: per-guild GRU weights + lifecycle FSM (tracks quality_message_count)
│   │   │   │   │   ├── guild_supervisor.ex     # DynamicSupervisor for guild GenServers
│   │   │   │   │   ├── model.ex                # GRU forward/backward pass (delegates to NIF or Nx)
│   │   │   │   │   ├── quantizer.ex            # Ternary quant, stochastic rounding, drift delta
│   │   │   │   │   └── tokenizer.ex            # NIF wrapper for BPE tokenizer (sentencepiece)
│   │   │   │   ├── training/
│   │   │   │   │   ├── pipeline.ex             # Batch assembly, loss computation, optimizer step
│   │   │   │   │   ├── optimizer.ex            # Adam / SGD with momentum
│   │   │   │   │   └── pool_worker.ex          # NimblePool dirty-CPU worker for training steps
│   │   │   │   ├── preprocessing/
│   │   │   │   │   ├── message_cleaner.ex      # URL stripping, mention normalization
│   │   │   │   │   └── quality_gate.ex         # Noise filtering: emoji, commands, UNK threshold
│   │   │   │   ├── media/
│   │   │   │   │   ├── extractor.ex            # URL regex scan + media type classification
│   │   │   │   │   └── store.ex                # MediaStore write/query interface
│   │   │   │   ├── persistence/
│   │   │   │   │   ├── storage_provider.ex     # Behaviour: save/load/list for weights + media
│   │   │   │   │   ├── sqlite_adapter.ex       # Dev adapter
│   │   │   │   │   ├── postgres_adapter.ex     # Prod adapter
│   │   │   │   │   └── s3_adapter.ex           # Prod blob adapter (weights only)
│   │   │   │   └── telemetry.ex                # Perplexity events, drift alerts, guild health
│   │   │   └── rolando.ex                      # Application entry, supervision tree root
│   │   ├── native/
│   │   │   └── rolando_nif/                    # Rustler NIF crate (Rust)
│   │   │       ├── Cargo.toml
│   │   │       └── src/
│   │   │           ├── lib.rs                  # NIF entry points exposed to Elixir
│   │   │           ├── gru.rs                  # GRU forward + backward in ndarray / BLAS
│   │   │           └── quantize.rs             # Ternary quantization, stochastic rounding
│   │   ├── priv/
│   │   │   └── models/
│   │   │       └── tokenizer.model             # Pre-built BPE sentencepiece model (version-pinned)
│   │   ├── test/
│   │   └── mix.exs
│   │
│   ├── rolando_discord/                  # Discord interface (Nostrum)
│   │   ├── lib/
│   │   │   └── rolando_discord/
│   │   │       ├── consumer.ex                 # Nostrum event consumer (MESSAGE_CREATE etc.)
│   │   │       ├── message_handler.ex          # Entry point: raw event → preprocessing → core
│   │   │       ├── bot_filter.ex               # Filters Rolando's own messages + author.bot
│   │   │       ├── guild_monitor.ex            # Handles GUILD_DELETE / bot removal cleanup
│   │   │       └── response_sender.ex          # Sends generated text back to Discord channel
│   │   ├── test/
│   │   └── mix.exs
│   │
│   └── rolando_web/                      # Web UI (Phoenix)
│       ├── lib/
│       │   └── rolando_web/
│       │       ├── controllers/
│       │       │   └── guild_controller.ex     # Guild config CRUD, tier change, reset endpoints
│       │       ├── live/
│       │       │   ├── system_overview.ex      # LiveView: active guilds, shared memory, pool load
│       │       │   └── guild_dashboard.ex      # LiveView: per-guild perplexity, state, checkpoint log
│       │       └── router.ex
│       ├── test/
│       └── mix.exs
│
├── config/
│   ├── config.exs          # Shared: tier, precision_mode, tokenizer_model, NimblePool size
│   ├── dev.exs             # SqliteAdapter, small pool, debug logging
│   ├── prod.exs            # PostgresAdapter or S3Adapter, full pool count
│   └── runtime.exs         # Secrets: DB URL, Discord bot token, S3 credentials
│
├── mise.toml               # Toolchain versions (Elixir, Erlang, Rust for Rustler)
├── mix.exs                 # Umbrella root
├── mix.lock
└── README.md
```

**Key structural decisions:**

* The `native/rolando_nif/` Rust crate lives inside the `rolando` app (not at umbrella root) because the NIF is an implementation detail of the core ML logic, not a shared umbrella-level dependency.
* `priv/models/` holds only the pinned tokenizer as a static asset shipped with the app. The tokenizer model path is referenced in `config.exs` and loaded once at startup by `Rolando.Neural.Tokenizer`. The frozen embedding and output projection are generated at first startup and stored in the DB — they are not shipped as files.
* `rolando_discord` has no ML knowledge — it calls into `rolando` via function calls across app boundaries. The Discord layer's only responsibility is translating Nostrum events into calls to `Rolando.Preprocessing.MessageCleaner` and `Rolando.Neural.GuildModel`.
* `mise.toml` should pin the Rust toolchain version to ensure reproducible NIF compilation across dev and CI environments.

---

*Configuration defaults are chosen to be safe and cheap. Operators are expected to profile their specific guild distribution and adjust tier, batch size, and learning rate accordingly. Tier changes are destructive to guild memory and require explicit confirmation.*
