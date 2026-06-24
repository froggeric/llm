# Configuration

The config file lives at `~/.localvision/config.toml` (or `$XDG_CONFIG_HOME/localvision/config.toml` if `XDG_CONFIG_HOME` is set).

It is optional. Every field has a safe default; you only need a config file to override defaults.

## Example

```toml
# ~/.localvision/config.toml

# How long to keep a model resident after the last tool call before killing
# the llama-server subprocess. Default: 5m. Raise if you make many calls
# in succession and want to skip the cold-start cost.
idle_timeout = "5m"

# How long to wait for llama-server to become healthy on spawn before
# declaring failure. Default: 2m. Large models (26B+) may need longer on
# slow disks.
startup_timeout = "2m"

# Log verbosity. One of: debug, info, warn, error. Default: info.
log_level = "info"

# Optional: also write structured JSON logs to this path (in addition to
# stderr). Useful for filing bug reports.
# log_file = "~/.localvision/logs/mcp.log"

# Root directory for all localvision state (models, bin, overlays).
# Default: ~/.localvision
# cache_dir = "/Volumes/ssd/llm-cache"

# Override the model-file cache location. Default: <cache_dir>/models.
# models_dir = "/Volumes/ssd/llm-cache/models"

# Override the llama-server binary cache. Default: <cache_dir>/bin.
# bin_dir = "/Volumes/ssd/llm-cache/bin"

# Override the auto-detected default model. Use a model ID from the catalog.
# Useful for testing or pinning to a specific model.
# default_model = "qwen3-vl-8b"

# How much memory to subtract from total when computing "available" for
# model loading. Default: 4.0 GB. Raise if you run other memory-hungry
# apps alongside (e.g. Docker, an IDE with many plugins).
safety_margin_gb = 4.0

# HuggingFace username model files are downloaded from. Default: froggeric.
# Change only if you're using a fork or enterprise mirror.
hf_user = "froggeric"
```

## Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `idle_timeout` | duration | `5m` | Subprocess idle timeout. |
| `startup_timeout` | duration | `2m` | Health-check timeout on spawn. |
| `log_level` | string | `info` | One of `debug`, `info`, `warn`, `error`. |
| `log_file` | string | (empty) | Optional path for JSON log output. |
| `cache_dir` | string | `~/.localvision` | Root state directory. |
| `models_dir` | string | `<cache_dir>/models` | Model-file cache. |
| `bin_dir` | string | `<cache_dir>/bin` | llama-server binary cache. |
| `default_model` | string | (auto) | Fallback model when a tool has no per-tool route and no fitting catalog model (v0.7: a fallback, not a forced override — per-tool routing takes precedence). |
| `safety_margin_gb` | float | `4.0` | Memory subtracted from total when computing availability. |
| `hf_user` | string | `froggeric` | HuggingFace namespace for downloads. |

## Per-tool model + method routing (v0.7)

The catalog routes each tool to its benchmark-best model automatically (see
`localvision doctor` for the table). You can override per tool with
`[tools.<id>]` tables — both the **model** and the **sampling method**:

```toml
[tools.read_image]
model = "qwen3.6-35b-a3b"   # route read_image to the MoE (catalog ID)
method = "union@3"           # multi-sample: 3 warm calls fused into one result

[tools.extract_code]
model = "qwen3.5-4b-q8"      # pin a specific model for this tool
```

- `model` — a catalog ID. Empty = use the catalog's per-tool routing.
- `method` — `off` (default) or `union@N` (multi-sample; experimental, ~N×
  latency). Only worth it on coverage tools (`read_image`, `describe_ui`,
  `describe_chart`, `extract_text`); see `benchmark/vlm/CATEGORY-REPORT.md`.

**Resolution order** (highest wins): `--model` flag (forces one model for every
tool) > `[tools.<id>].model` > catalog per-tool routing > `default_model`.
Note: `default_model` is a *fallback* — it no longer forces a single model, so
per-tool routing is the default behavior.

`localvision setup` can write the benchmark's recommended per-tool routing for
you (it asks). `localvision doctor` prints the resolved model per tool for your
hardware.

> **Tradeoff:** per-tool model routing means a mixed-tool MCP session switches
> models between calls (a cold reload per switch). The wizard defaults to a
> single warm model + opt-in routing to preserve warm reuse.

## Path expansion

Paths support:
- `~` for home directory (`~/.localvision`)
- Environment variables (`$HOME/foo`, `${XDG_CACHE_HOME}/mcp`)
- Relative paths (resolved against CWD; not recommended)

## Overlays

Catalog overlays live at `~/.localvision/catalog.d/*.toml`. See [MODELS.md](./MODELS.md) for the schema and merge semantics.

## Flags vs config

Command-line flags override config:

```bash
localvision run --verbose              # forces log_level = debug
localvision run --log-file /tmp/x.log # forces log_file = /tmp/x.log
localvision run --config /custom/path.toml  # uses this file instead of default
```

## Environment variables

| Variable | Purpose |
|---|---|
| `XDG_CONFIG_HOME` | Overrides the config lookup root. If set, the config file is `$XDG_CONFIG_HOME/localvision/config.toml`. |
| `HOME` | Used for `~` expansion. |

That's it. No `LOCAL_VISION_MCP_*` knobs — keep the surface small.
