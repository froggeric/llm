# Models

The catalog is a TOML file that describes every model the MCP can serve. It lives at `internal/models/builtin.toml` (embedded in the binary) and can be extended via overlays at `~/.local-vision-mcp/catalog.d/*.toml`.

## Built-in catalog (v0.1.0)

| Model | Display name | Tier | Min VRAM | Preferred for |
|---|---|---|---|---|
| `qwen3-vl-4b` | Qwen3-VL 4B | constrained | 5 GB | `read_image` |
| `qwen3-vl-8b` | Qwen3-VL 8B | mainstream | 8 GB | `read_image`, `extract_text`, `extract_code`, `describe_ui`, `diagnose_error` |
| `gemma4-26b-a4b` | Gemma 4 26B-A4B (MoE) | high_end | 24 GB | `describe_chart`, `describe_diagram`, `extract_table`, `compare_images` |
| `internvl35-8b` | InternVL3.5 8B | mainstream | 8 GB | (none — explicit selection only) |

The default model for your hardware is auto-selected at startup. Run `local-vision-mcp doctor` to see which one applies.

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

Models in the catalog are sourced from the open-weight ecosystem. They have known weaknesses:

- **InternVL3.5 8B** called a QR code a "maze-like pattern" in our 20-image benchmark. Not in any `preferred_for` list.
- **Gemma 4 26B-A4B** is the strongest on dense content but takes 17 GB of disk.
- **Qwen3-VL 8B** is the recommended default for 16–32 GB Macs.
- **Qwen3-VL 4B** is the recommended default for ≤16 GB Macs; quality drops on busy images.

For full benchmark results, see `local-vlm-research/BENCHMARK-SUMMARY.md` (in the parent repo).
