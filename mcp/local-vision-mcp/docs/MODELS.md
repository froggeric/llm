# Models

The catalog is a TOML file that describes every model the MCP can serve. It lives at `internal/models/builtin.toml` (embedded in the binary) and can be extended via overlays at `~/.local-vision-mcp/catalog.d/*.toml`.

## Built-in catalog (v0.2.0)

Based on the v6 benchmark (30 images, 3 runs, 24 variants, hybrid scoring).

| Model | Display name | Tier | Min VRAM | Preferred for |
|---|---|---|---|---|
| `qwen3-vl-8b` | Qwen3-VL 8B (Q8_0) | constrained | 6 GB | all tools |
| `qwen3.5-4b` | Qwen3.5 4B (nothink) | constrained | 3 GB | `read_image` (fallback) |
| `qwen3.6-27b` | Qwen3.6 27B (nothink) | mainstream | 17 GB | all tools |

**How selection works**:

- On 4–8 GB Macs: `qwen3.5-4b` is the only model that fits. Runs with `enable_thinking=false` (chat_template_kwargs).
- On 12–16 GB Macs: `qwen3-vl-8b` (Q8_0) is preferred — the only 100%-reliable Q8 model in the benchmark (0 timeouts, σ=0.33).
- On 24+ GB Macs: `qwen3.6-27b` is preferred — the benchmark champion (79.6/100, σ=0.24, 0 failures).
- On 48+ GB Macs: no upgrade. `qwen3.6-27b` remains the best model in the study; larger models (Qwen3.6-35B-A3B, Gemma 4 31B) tested worse despite the bigger footprint.

Run `local-vision-mcp doctor` to see which model applies to your hardware.

### Why these models

The v6 benchmark tested 11 base models × multiple quants and thinking modes. Key findings:

- **Thinking mode hurts vision**: all Qwen hybrid thinkers scored higher with `enable_thinking=false`. Vision is perception, not reasoning. This is why `chat_template_kwargs = {enable_thinking = false}` is set for both Qwen3.5 and Qwen3.6 entries.
- **Q8 is asymmetric**: Q8_0 is a strict win for Qwen3-VL 8B (0 timeouts, lower σ) but cripples Qwen3.5 thinkers. Only Qwen3-VL 8B-Q8 earned a recommendation.
- **MoE size is misleading**: Qwen3.6 35B-A3B (3B active per token) ties much smaller dense models on quality despite being 7× larger. Not worth the footprint.
- **Gemma 4 12B has hallucination flips** at Q4 (same image → different results across runs). Q8 fixes the variance but introduces 22% timeout rate. Excluded.

For the full benchmark report, see `local-vlm-research/BENCHMARK-REPORT-v5.md` (in the parent repo).

## Adding a model

Add a block to `internal/models/builtin.toml` (built-in) or to a new `~/.local-vision-mcp/catalog.d/<name>.toml` (overlay).

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
| `preferred` | bool | yes | `true` if this is the default for its tier. Invariant: exactly one preferred per tier. |
| `preferred_for` | array of strings | yes (can be empty) | Tool IDs this model is best for. Empty = never auto-picked. |
| `hardware_tier` | string | yes | One of `constrained`, `mainstream`, `high_end`. |
| `bench_toks` | float | yes (can be 0) | Throughput from the benchmark, in tokens/sec. Informational. |
| `notes` | string | no | Free-form notes shown in `doctor` output. |
| `chat_template_kwargs` | map | no | Forwarded as `chat_template_kwargs` in the chat-completion request. Use for `enable_thinking = false` on hybrid thinking models (Qwen3.5/3.6). |

### Computing SHA256

```bash
shasum -a 256 path/to/model.gguf | awk '{print $1}'
```

Or use `local-vision-mcp doctor --compute-hashes` (planned for v0.2). For v0.1, populate by hand.

### Uploading to HuggingFace

The catalog URLs point at `huggingface.co/froggeric/`. To add a model:

1. Download the GGUF and mmproj locally.
2. Compute their SHA256 hashes.
3. Create a HuggingFace repo under the `froggeric` namespace (e.g., `froggeric/Your-Model-12B-GGUF`).
4. Upload both files via `huggingface-cli upload` or the web UI.
5. Add the catalog entry with the resolved URLs + SHA256s.
6. Run `local-vision-mcp doctor` to verify the entry loads and validates.

## Overlay catalog files

User overlays live at `~/.local-vision-mcp/catalog.d/*.toml`. They deep-merge into the built-in catalog (per-field, last-write-wins by lexical filename). Use them to:

- Override a built-in model's `min_vram_gb` (e.g., you've measured it fits in less).
- Point a model at a different HF repo (e.g., your fork).
- Add new models without rebuilding the binary.

Example overlay `~/.local-vision-mcp/catalog.d/local-experiments.toml`:

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

1. Compute `available_vram = total_memory - 4 GB safety margin - 1 GB resident llama-server`.
2. Filter to models where `min_vram_gb <= available_vram`.
3. **Default model**: the `preferred=true` entry whose `hardware_tier` matches the user's tier.
4. **Per-tool model**: among fitting models, those whose `preferred_for` contains the tool ID; tie-break by smallest `min_vram_gb`, then by `display_name` lexically.
5. If nothing fits, the catalog returns `ErrNoFittingModel` and the MCP surfaces a structured error to the client (never crashes).

See `internal/models/selection.go` for the implementation.

## Quality caveats

Models in the catalog are sourced from the open-weight ecosystem. Known weaknesses from the v6 benchmark:

- **All models miss medical findings**: the X-ray rib fracture was missed by all 24 variants. Do not use for clinical work.
- **Dense scenes are hard**: Where's Waldo, complex spritesheets — no model locates hidden characters reliably.
- **Qwen3.5-4B (nothink)** is the best small model but has σ=0.48 (some run-to-run variance on hard images).
- **Qwen3-VL 8B (Q8)** is the most reliable mid-tier (σ=0.33) but slightly lower quality than the 27B.
- **Qwen3.6-27B (nothink)** is the champion but needs 24+ GB RAM for comfortable operation.

For full benchmark results, see `local-vlm-research/BENCHMARK-REPORT-v5.md` (in the parent repo).
