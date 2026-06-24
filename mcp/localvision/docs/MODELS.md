# Models

The catalog is a TOML file that describes every model the MCP can serve. It lives at `internal/models/builtin.toml` (embedded in the binary) and can be extended via overlays at `~/.localvision/catalog.d/*.toml`.

## Built-in catalog (v0.7)

Based on the v6 benchmark (30 images, 3 runs, 24 variants, hybrid scoring). v0.7
adds **per-tool model routing**: the catalog crowns a different best model per
tool (see `benchmark/vlm/BENCHMARK-REPORT-v5.md`'s use-case table), encoded in
each model's `preferred_for`.

| Model | Display name | Min VRAM | Role (v0.7) |
|---|---|---|---|
| `qwen3-vl-8b` | Qwen3-VL 8B (Q8_0) | 6 GB | **mainstream default**; routed for `read_image` / `extract_text` / `describe_chart` / `extract_table` (+ `read_document` / `image_to_prompt` / `compare_images`) — the precision/cleanliness lead |
| `qwen3.5-4b-q8` | Qwen3.5 4B (Q8) | 4 GB | routed for `extract_code` / `describe_ui` / `describe_diagram` / `diagnose_error` — edges the 8B there at ~58 tok/s and 4.2 GB |
| `qwen3.5-4b` | Qwen3.5 4B (nothink, Q4) | 3 GB | sub-4 GB fallback (where the 8B and 4B-Q8 don't fit) |
| `qwen3.6-27b` | Qwen3.6 27B (nothink) | 17 GB | opt-in, max quality (`--model qwen3.6-27b`) |
| `qwen3.6-35b-a3b` | Qwen3.6 35B-A3B (MoE) | 17 GB | opt-in; the `read_image` coverage pick **with `--sample`** (ties smaller models on a single call despite 21 GB) |

**How selection works (v0.7 — per-tool routing)**:

Each model lists the tools it's best at in `preferred_for`; the per-tool selector
(`catalog.ModelFor`) picks the fitting model that lists the tool. Hardware fit
still gates: on a 16 GB Mac both the 8B and 4B-Q8 fit → each tool routes to its
best; on 4–8 GB only the 4B-Q4 fits → everything falls back to it.

- The 8B-Q8 remains the **safest all-rounder** (only 100%-reliable Q8: 0 timeouts, σ=0.33). It is the mainstream tier default and the fallback when a tool's routed model doesn't fit.
- The 4B-Q8 edges the 8B on code/UI/diagram/error (benchmark use-case table) at ~58 tok/s, but is ~87%-ok (slightly timeout-prone) — opt into per-tool routing deliberately.
- `qwen3.6-27b` (champion, 79.6/100) and the MoE are **opt-in only** (`--model`); never auto-selected.

Run `localvision doctor` to see the per-tool routing for your hardware. Override
per tool via `[tools.<id>]` in `config.toml` (see [CONFIGURATION.md](./CONFIGURATION.md)).

> **Tradeoff:** per-tool routing switches models between tools in a mixed MCP
> session (a cold reload per switch). `localvision setup` defaults to a single
> warm model + opt-in routing to preserve warm reuse.

### Cache layout

Model files are cached per-model under `~/.localvision/models/<model-id>/` (e.g.
`models/qwen3-vl-8b/mmproj-F16.gguf`). Upgrading from v0.5.0 or earlier migrates
any already-cached files into the right subdirectory on first load — no
re-download. (The old flat layout collided on the shared `mmproj-F16.gguf`
basename and re-downloaded the projector on every model switch; v0.5.1 fixes
that.)

### Why these models

The v6 benchmark tested 11 base models × multiple quants and thinking modes. Key findings:

- **Thinking mode hurts vision**: all Qwen hybrid thinkers scored higher with `enable_thinking=false`. Vision is perception, not reasoning. This is why `chat_template_kwargs = {enable_thinking = false}` is set for both Qwen3.5 and Qwen3.6 entries.
- **Q8 is asymmetric**: Q8_0 is a strict win for Qwen3-VL 8B (0 timeouts, lower σ) but cripples Qwen3.5 thinkers. Only Qwen3-VL 8B-Q8 earned a recommendation.
- **MoE size is misleading**: Qwen3.6 35B-A3B (3B active per token) ties much smaller dense models on quality despite being 7× larger. Not worth the footprint.
- **Gemma 4 12B has hallucination flips** at Q4 (same image → different results across runs). Q8 fixes the variance but introduces 22% timeout rate. Excluded.

For the full benchmark report, see [`../../../benchmark/vlm/BENCHMARK-REPORT-v5.md`](../../../benchmark/vlm/BENCHMARK-REPORT-v5.md).

## Adding a model

Add a block to `internal/models/builtin.toml` (built-in) or to a new `~/.localvision/catalog.d/<name>.toml` (overlay).

```toml
schema_version = 1

[models.your-model-id]
display_name = "Your Model 12B"
gguf = "https://huggingface.co/froggeric/Your-Model-12B-GGUF/resolve/main/your-model-12b-q4_k_m.gguf"
mmproj = "https://huggingface.co/froggeric/Your-Model-12B-GGUF/resolve/main/mmproj-your-model-12b-f16.gguf"
gguf_sha256 = "<64-hex-chars>"
mmproj_sha256 = "<64-hex-chars>"
ctx = 32768
gpu_layers = -1
min_vram_gb = 12
min_system_ram_gb = 16
released = "2026-06"
license = "Apache-2.0"
preferred = false
preferred_for = []
hardware_tier = "mainstream"
bench_toks = 0.0
notes = "Optional notes shown in `doctor` output."
# Optional: per-model chat_template_kwargs (e.g. enable_thinking=false)
# chat_template_kwargs = { enable_thinking = false }
```

### Field reference

| Field | Type | Required | Description |
|---|---|---|---|
| `display_name` | string | yes | Human-readable name. |
| `gguf` | string (URL) | yes | HTTPS URL to the GGUF file. Must be in `huggingface.co/<hf_user>/` namespace. |
| `mmproj` | string (URL) | yes for VLMs | HTTPS URL to the vision projector (`.bin` / `.gguf`). Omit for text-only models. |
| `gguf_sha256` | string (hex) | yes | SHA256 of the GGUF file. Verified on every load. |
| `mmproj_sha256` | string (hex) | yes for VLMs | SHA256 of the mmproj file. |
| `ctx` | int | yes | Context window in tokens. Typical: 32768. |
| `gpu_layers` | int | yes | GPU layers to offload. Use `-1` for all. |
| `min_vram_gb` | int | yes | Minimum VRAM / unified memory required to load. Used by the selection algorithm. |
| `min_system_ram_gb` | int | yes | Minimum system RAM (for CPU-only fallback). |
| `released` | string (YYYY-MM) | yes | Release date for sorting. |
| `license` | string (SPDX) | yes | SPDX license ID (Apache-2.0, MIT, etc.). |
| `preferred` | bool | yes | `true` if this is the default for its tier. Invariant: at most one preferred per tier (a tier may have none; selection then falls back to the largest fitting model). |
| `preferred_for` | array of strings | yes (can be empty) | Tool IDs this model is best for. Empty = never auto-picked. |
| `hardware_tier` | string | yes | One of `constrained`, `mainstream`, `high_end`. |
| `bench_toks` | float | yes (can be 0) | Throughput from the benchmark, in tokens/sec. Informational. |
| `notes` | string | no | Free-form notes shown in `doctor` output. |
| `chat_template_kwargs` | map | no | Forwarded as `chat_template_kwargs` in the chat-completion request. Use for `enable_thinking = false` on hybrid thinking models (Qwen3.5/3.6). |

### Computing SHA256

```bash
shasum -a 256 path/to/model.gguf | awk '{print $1}'
```

Or wait for `localvision doctor --compute-hashes` (planned, ROADMAP Theme E3). For now, populate by hand.

### Uploading to HuggingFace

The catalog URLs point at `huggingface.co/froggeric/`. To add a model:

1. Download the GGUF and mmproj locally.
2. Compute their SHA256 hashes.
3. Create a HuggingFace repo under the `froggeric` namespace (e.g., `froggeric/Your-Model-12B-GGUF`).
4. Upload both files via `huggingface-cli upload` or the web UI.
5. Add the catalog entry with the resolved URLs + SHA256s.
6. Run `localvision doctor` to verify the entry loads and validates.

## Overlay catalog files

User overlays live at `~/.localvision/catalog.d/*.toml`. They deep-merge into the built-in catalog (per-field, last-write-wins by lexical filename). Use them to:

- Override a built-in model's `min_vram_gb` (e.g., you've measured it fits in less).
- Point a model at a different HF repo (e.g., your fork).
- Add new models without rebuilding the binary.

Example overlay `~/.localvision/catalog.d/local-experiments.toml`:

```toml
schema_version = 1

[models.my-experimental-model]
display_name = "My Experimental 7B"
gguf = "https://huggingface.co/froggeric/my-experimental-7b/resolve/main/model.gguf"
mmproj = "https://huggingface.co/froggeric/my-experimental-7b/resolve/main/mmproj.bin"
gguf_sha256 = "abc123..."
mmproj_sha256 = "def456..."
ctx = 16384
gpu_layers = -1
min_vram_gb = 8
min_system_ram_gb = 16
released = "2026-06"
license = "Apache-2.0"
preferred = false
preferred_for = []
hardware_tier = "mainstream"
bench_toks = 0.0
```

Every applied overlay field is logged at startup (`slog.Info` with `overlay=file.toml model=id field=...`).

## Selection algorithm

Given the catalog and detected hardware:

1. Compute `available = effective_memory - 4 GB safety margin - 1 GB resident llama-server` (`effective_memory` = VRAM on a discrete GPU, else unified/system RAM).
2. Filter to models where `min_vram_gb <= available`.
3. **Per-tool model** (used for actual tool calls): among fitting models, those whose `preferred_for` contains the tool ID, in deterministic order — **largest `min_vram_gb` first** (most capable wins), then `display_name` lexically. If none list the tool, fall through to the default.
4. **Default model** (fallback + `doctor` display): the `preferred=true` entry whose `hardware_tier` matches the user's tier among the fitting set; if no tier-preferred fits, the largest fitting model that lists at least one tool (so the default matches what tools use); if none list a tool, the largest fitting model overall.
5. If nothing fits, the catalog returns `ErrNoFittingModel` and the MCP surfaces a structured error to the client (never crashes).

In the v0.7 catalog the `preferred_for` lists are **partitioned** across the 8B
and the 4B-Q8 (the 8B lists the 7 tools it leads; the 4B-Q8 lists the 4 it edges
the 8B on), so step 3 routes each tool to its benchmark-best fitting model. The
4B-Q4 / 27B / MoE list no tools (opt-in). Where a tool's routed model doesn't
fit (e.g. on 4–8 GB, where only the 4B-Q4 fits), step 4 lands on the 4B-Q4.
`--model` or a `[tools.<id>].model` override bypasses the routing.

See `internal/models/selection.go` for the implementation.

## Quality caveats

Models in the catalog are sourced from the open-weight ecosystem. Known weaknesses from the v6 benchmark:

- **All models miss medical findings**: the X-ray rib fracture was missed by all 24 variants. Do not use for clinical work.
- **Dense scenes are hard**: Where's Waldo, complex spritesheets — no model locates hidden characters reliably.
- **Qwen3.5-4B (nothink)** is the best small model but has σ=0.48 (some run-to-run variance on hard images).
- **Qwen3-VL 8B (Q8)** is the most reliable mid-tier (σ=0.33) but slightly lower quality than the 27B.
- **Qwen3.6-27B (nothink)** is the champion but needs 24+ GB RAM for comfortable operation.

For full benchmark results, see [`../../../benchmark/vlm/BENCHMARK-REPORT-v5.md`](../../../benchmark/vlm/BENCHMARK-REPORT-v5.md).

## PDF rasterizer (read_document)

The `read_document` tool (v0.6.0) rasterizes a PDF into page images before
inference. It uses the first available rasterizer on `$PATH` — none is bundled:

1. `pdftoppm` (poppler)
2. `mutool draw` (mupdf)
3. `magick` / `convert` (ImageMagick, via its ghostscript delegate — frequently
   policy-disabled out of the box)
4. `gs` (ghostscript)

Install any one, e.g. `brew install poppler` (macOS) or `apt install poppler-utils`
(Debian/Ubuntu). When none is found, `read_document` errors with these options
named. The rasterizer chain mirrors the HEIC/WEBP converter in `internal/llama`;
see `internal/document/rasterize.go`.
