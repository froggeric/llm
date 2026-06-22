# Changelog

All notable changes to `localvision` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches v1.0.0. Until then, minor changes may break compatibility.

Tags for this subdirectory follow the Go module convention
`mcp/localvision/v<MAJOR>.<MINOR>.<PATCH>`.

## [Unreleased]

## [0.2.1] - 2026-06-22

### Fixed

- **`$PATH`-discovered `llama-server` now spawns** (v0.2.0 regression): the
  recommended setup (`brew install llama.cpp` so `llama-server` is on `$PATH`)
  discovered the binary but `validateBinaryPath` rejected it at spawn as
  "outside bin cache dir", so every tool call (MCP and CLI) failed. Path
  validation now accepts any absolute path (discovery-trusted, never
  config-controlled); relative paths are still rejected.

## [0.2.0] - 2026-06-22

Foundation & first real distribution. The v6 benchmark catalog refresh lands,
CI is green again, `llama-server` is acquired safely, and the first GitHub
Release + Homebrew formula ship. See [`ROADMAP.md`](./ROADMAP.md) Themes A/B + C6.

### Added

- **First GitHub Release + Homebrew formula** (B1/B2):
  `brew tap froggeric/homebrew-tap && brew trust froggeric/tap && brew install localvision`.
  darwin/arm64 only.
- `chat_template_kwargs` catalog field, carrying `enable_thinking=false` for the
  hybrid-thinking models (Qwen3.5/3.6) — "thinking mode hurts vision" was a key
  v6 finding.
- **WEBP** image support.
- The v6 **benchmark** is checked into the monorepo at `benchmark/vlm/` (report,
  scoring code, test images, raw results).

### Changed

- **Catalog refreshed to v0.2.0** — winners of the v6 benchmark
  (`benchmark/vlm/`: 30 images × 3 runs × 24 variants, hybrid scoring). Three
  models across two tiers:
  - `qwen3-vl-8b` (Q8_0) — constrained tier, preferred for all tools; the only
    100%-reliable Q8 model in the benchmark (σ=0.33, 0 timeouts across 90 cells).
  - `qwen3.5-4b` (nothink) — constrained fallback for 4–8 GB Macs where the 8B
    does not fit.
  - `qwen3.6-27b` (nothink) — mainstream tier; benchmark champion (79.6/100,
    σ=0.24, 0 failures).
  Dropped: Qwen3-VL 4B and Gemma 4 26B-A4B. The `high_end` tier is gone — the
  mainstream 27B is reused on 48+ GB hardware via deterministic fallback.
- **`llama-server` acquisition reframed** (A2/A5): the `TODO-PHASE3` placeholder
  is gone. localvision now **prefers a user-installed `llama-server` on `$PATH`**
  (e.g. `brew install llama.cpp`; WARN-logged as unverified); if absent it
  downloads a **pinned official llama.cpp release** (`b9758`) and verifies the
  archive SHA256 before extracting the dylib bundle. PATH-first is a behavior
  change for users who previously relied on the cached placeholder download.
- **Benchmark-faithful llama.cpp parameters** (C6): sampling now sends
  `top_p` 0.95 and `top_k` 64 (alongside the existing `temperature` 0.1), and
  `llama-server` launches with `-np 1 -b 4096 -ub 4096` (batch sizes large enough
  that image tokens never split across physical batches — a quality fix from the
  v6 benchmark).
- **Server key unified** to `localvision` everywhere (`plugin/plugin.json`,
  `docs/INSTALL.md`).
- **Release tooling**: goreleaser OSS can't parse Go subdirectory-module tags,
  so the release workflow normalizes `mcp/localvision/vX.Y.Z` to a local bare
  `vX.Y.Z` for goreleaser (the subdir tag stays on the remote for `go install`),
  and `.goreleaser.yaml` uses `gomod.proxy: false`. (`--snapshot` had masked
  this — v0.1.0 was never released through goreleaser.)

### Fixed

- **CI green again** (A1): the two `internal/llama` tests that broke when image
  handling switched to data-URI inlining now use real fixtures and assert the
  data-URI shape.

### Renamed

- `local-vision-mcp` → `localvision` (module path, Go tags, CI/release
  workflows, install paths). Mechanical rename; no logic change.

### Known limitations

- macOS Apple Silicon only; Linux/Windows detection returns
  `BackendUnsupported` (ROADMAP Theme D).
- No streaming; each `tools/call` blocks until inference completes (ROADMAP E1).
- Tool names are unprefixed and may collide with another vision MCP
  (ROADMAP E4).
- The pinned llama.cpp release tag ages (llama.cpp releases daily); bump it
  deliberately and recompute the archive SHA. A future
  `doctor --update-llama-server` (ROADMAP Theme E) will automate this.

## [0.1.0] - 2026-06-18

First usable release. macOS Apple Silicon only (Linux/Windows stubbed for v0.2).

### Added

- **9 tools**: `read_image`, `extract_text`, `extract_code`, `extract_table`,
  `describe_ui`, `describe_diagram`, `describe_chart`, `diagnose_error`,
  `compare_images`. Each tool has a task-tuned system prompt and per-tool
  output budget.
- **3 models** mirrored to `huggingface.co/froggeric/` with pinned SHA256s:
  - Qwen3-VL 4B (constrained tier, ≤16 GB Macs)
  - Qwen3-VL 8B (mainstream tier, 16–48 GB Macs; default for most tools)
  - Gemma 4 26B-A4B MoE (high_end tier, ≥32 GB Macs; preferred for charts,
    diagrams, tables, image comparison)
- **Hardware detection** via `sysctl hw.memsize` on Apple Silicon. Tier
  classification: constrained ≤16 GB, mainstream 16–48 GB, high_end >48 GB.
- **Subprocess lifecycle**: spawn-on-demand + 5-min keep-warm. Mutex-serialized
  model switches. Crash watcher auto-respawns dead subprocesses. SHA256
  verification on every model load.
- **HEIC support**: auto-converts iPhone HEIC/HEIF images to JPEG via `sips`
  (macOS built-in) before sending to `llama-server`.
- **Plugin manifest** (`plugin/plugin.json`) and **SKILL.md** for Claude Code
  integration. Any MCP client that speaks stdio JSON-RPC works.
- **CLI**: `localvision run` (default; stdio server), `doctor`
  (diagnostics), `version`.
- **Catalog overlays**: drop TOML files in `~/.localvision/catalog.d/`
  to add models without rebuilding.
- **Docs**: INSTALL, TOOLS, MODELS, CONFIGURATION, SECURITY, TROUBLESHOOTING,
  CONTRIBUTING.
- **License**: PolyForm Noncommercial 1.0.0. Source-available; commercial use
  requires a separate license. See COMMERCIAL-LICENSING.md.

### Known limitations

- macOS Apple Silicon only. Linux and Windows hardware detection returns
  `BackendUnsupported`. v0.2 adds cross-platform support.
- `llama-server` SHA256 is not pinned in source (`TODO-PHASE3` placeholder).
  Binary discovery accepts any `llama-server` on `$PATH` or in
  `~/.localvision/bin/`. v0.1.1 will pin a real hash.
- HEIC conversion requires `sips` (macOS built-in). Linux/Windows users must
  convert HEIC images manually.
- No streaming responses to the MCP client. Each `tools/call` blocks until
  the inference completes (30–70 s typical). v0.2 may add
  `notifications/progress`.
- Tool names are unprefixed. If you have another vision MCP installed, you
  may see name collisions in Claude Code's tool list.
- `InternVL3.5 8B` was considered but dropped from v0.1 — no clean upstream
  GGUF source and it ranked last in our 7-model benchmark.

[Unreleased]: https://github.com/froggeric/llm/compare/HEAD
[v0.2.1]: https://github.com/froggeric/llm/releases/tag/v0.2.1
[v0.2.0]: https://github.com/froggeric/llm/releases/tag/v0.2.0
[v0.1.0]: https://github.com/froggeric/llm/releases/tag/mcp%2Flocalvision%2Fv0.1.0
