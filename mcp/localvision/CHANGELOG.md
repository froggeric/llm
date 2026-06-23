# Changelog

All notable changes to `localvision` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches v1.0.0. Until then, minor changes may break compatibility.

Tags for this subdirectory follow the Go module convention
`mcp/localvision/v<MAJOR>.<MINOR>.<PATCH>`.

## [0.5.1] - 2026-06-23

Models + selection. Two fixes driven by user-reported pain: model files kept
re-downloading, and a re-analysis of the v6 benchmark (quality + speed together)
changed the default model.

### Changed

- **`qwen3-vl-8b` (Q8) is now the default for all tools**, everywhere it fits.
  A re-analysis of the v6 benchmark picked the 8B-Q8 as the best all-round model:
  the only 100%-reliable Q8 in the study (0 timeouts, σ=0.33), 74.4/100 (within
  ~5 pts of the 27B champion's 79.6), and ~3× faster (26 s vs 70 s per image).
  `qwen3.6-27b` (the champion) becomes **opt-in** (`--model qwen3.6-27b`); never
  auto-selected. `qwen3.5-4b` remains the fallback on 4–8 GB Macs where the 8B
  does not fit. See [`docs/MODELS.md`](./docs/MODELS.md).

### Fixed

- **Model files no longer re-download on every model switch.** All three catalog
  models ship a projector named `mmproj-F16.gguf`; the v0.4 flat cache
  (`~/.localvision/models/<basename>`) stored them at one path, so each model
  switch failed the next model's SHA check and re-downloaded the ~0.4–1.5 GB
  projector. Model files are now cached **per-model**
  (`~/.localvision/models/<model-id>/`), eliminating the collision. On first load
  after the upgrade, any already-cached files are **migrated** (SHA-verified,
  renamed into the right subdirectory) — no re-download of your existing models.

### Known limitations

- On 49+ GB Macs, `doctor`'s "Default model" line may show `qwen3.6-27b` (the
  generic default falls back to the largest fitting model), though actual tool
  calls still use `qwen3-vl-8b`. Display-only; pass `--model` to override.

## [0.5.0] - 2026-06-23

Breadth & polish: a new 10th tool plus a reliability fix. See
[`ROADMAP.md`](./ROADMAP.md) (G8, E6).

### Added

- **`image_to_prompt` tool** (G8): reverse-engineers an image into a
  text-to-image (diffusion) prompt that could recreate it — subject, medium/style,
  composition and camera details, lighting, color palette, mood, plus a
  paste-ready comma-separated tag line. Usable with Midjourney, SDXL, Flux,
  DALL·E, etc.; the optional `question` steers it toward a specific generator or
  style. Reach it via `localvision img.png --type prompt` or the
  `image_to_prompt` MCP tool. Tool count is now **10**.

### Fixed

- **MCP `image_data` temp-file leak** (E6): the MCP server path decoded
  `image_data` (data: URI) inputs to temp files via a private helper that never
  registered them for cleanup, leaking one temp file per `image_data` MCP call.
  The MCP path now reuses the canonical `tools.ParseImageRef` (which registers
  the temp) and reaps it with `CleanupImageRefs` after each call — matching the
  CLI one-shot path, which never leaked. A partially-malformed `images` array
  (valid element followed by an invalid one) also no longer leaks the earlier
  elements' temp files. ~80 lines of duplicated decode helpers removed; `Source`
  is now redacted on the MCP path too (privacy, matching the CLI).

### Deferred

- **E2 (auto-reap orphan `llama-server` on startup)** moved to a later release.
  Investigation showed no orphan-detection code exists, ports are ephemeral, and
  there is no PID file or argv marker — so safe reaping needs a marker plus a
  parent-liveness check (to avoid killing a different live instance's subprocess),
  which is larger than its roadmap "S" tag and carries automatic-kill risk. The
  manual workaround (`pkill -fa llama-server`) remains documented.

## [0.4.0] - 2026-06-23

Cross-platform: localvision now builds and runs on Linux and Windows (x86_64
and arm64) in addition to macOS Apple Silicon/Intel. The wrapper is pure Go
(CGO off), so all six targets cross-compile from one runner. See
[`ROADMAP.md`](./ROADMAP.md) Theme D.

### Added

- **Linux hardware detection** (D1): system RAM from `/proc/meminfo`; NVIDIA
  CUDA and AMD ROCm GPUs detected via `nvidia-smi` / `rocm-smi` (VRAM parsed
  from their output). When a discrete GPU is found, model selection sizes
  against VRAM, not host RAM.
- **Windows hardware detection** (D2): total RAM via `GlobalMemoryStatusEx`
  (kernel32; `x/sys/windows` v0.46 doesn't wrap it, so it's called directly);
  NVIDIA CUDA via `nvidia-smi`. DirectML detection is deferred — set
  `default_model` manually on DirectML-only hardware.
- **VRAM-aware model selection**: `HardwareInfo.VramGB` + `effectiveMemoryGB`
  — on a discrete GPU the model must fit in VRAM; on Apple Silicon (unified
  memory) behavior is unchanged. Fixes a latent bug where a 16 GB GPU on a
  64 GB host would have been sized against host RAM.
- **Cross-platform HEIC/WEBP conversion** (D5): a first-wins converter chain
  — `sips → magick/convert → heif-convert → ffmpeg` — CLI-only and
  `$PATH`-discovered, so it runs headless (MCP server, batch, cron). Replaces
  the macOS-only `sips` path. No decoder is bundled (HEVC patents + freeware
  redistribution limits); when no converter is found the error names what to
  install.
- **CI matrix** (D3): `vision-mcp-ci.yml` now runs `go vet` / `go test -race` /
  `go build` on ubuntu, windows, and macos, catching platform regressions and
  exercising the Linux/Windows detection paths on real OSes.

### Changed

- **Cross-compile all six targets** (D4): `.goreleaser.yaml` builds
  darwin/linux/windows × arm64/amd64 from a single macos runner (CGO off).
- **Per-OS `isTerminal` and disk-space** (foundation): the BSD-only
  `unix.TIOCGETA` and `unix.Statfs` calls that blocked Linux/Windows builds
  are now build-tagged per OS (`term_*.go`, `disk_*.go`).

### Known limitations

- Linux/Windows GPU detection is unit-tested (parsers + selection logic) and
  runs clean on GPU-less CI machines, but is **not validated on real CUDA/ROCm
  hardware** from the dev machine (macOS). Report discrepancies; `default_model`
  overrides any misdetection.
- DirectML (DirectX 12) GPU detection is not implemented on Windows.
- The v0.4 feature packages (models/setup/format/cmd) are fully tested on
  Windows, but ~14 **pre-existing** tests asserting Unix-only behavior (exec-bit
  preservation, `#!/bin/sh` subprocesses, SIGTERM/SIGKILL, Unix path literals)
  skip on Windows. They still run on macOS + Linux. Full Windows hardening of
  those paths is tracked follow-up.
- Homebrew remains macOS-only (Linux/Windows users use `curl | sh` or
  `go install`); the formula's `brews:` stanza uses a goreleaser-deprecated
  (but functional) field — tracked as ROADMAP Theme E7.

## [0.3.0] - 2026-06-22

The standalone CLI is complete: one-shot queries, output formats, batch
processing, and a first-run setup wizard. `localvision` is now a usable shell
tool, not just an MCP server. See [`ROADMAP.md`](./ROADMAP.md) Theme C.

### Added

- **One-shot shell CLI** (Theme C Phase 1, C1/C2/C5): `localvision img.png
  --type ocr --model qwen3.6-27b`. Positional images + interspersed flags;
  `--type` maps to the 9 tools; `--model` override + config `default_model`;
  `--question`; `--models-dir` / `--cache-dir` to redirect storage. Existing
  `run`/`doctor`/`version` subcommands unchanged.
- **Per-phase CLI progress**: the one-shot shows animated phase-by-phase status
  — ⬇ Downloading → ⚙ Loading model → ↻ Inferring — each with elapsed time.
  The summary line includes the model name + llama.cpp build number. Interactive
  mode is quiet (slog suppressed; `--verbose` recovers). The result is rendered
  markdown-ish in a TTY and plain when piped; errors are broken into indented
  lines.
- **Output formats** (Theme C Phase 2, C3): `--format text|markdown|json|yaml|xml`
  encodes a result for machine consumption (`localvision img.png --format json |
  jq .`). Machine formats wrap the result in `{tool, result}` and are always
  structurally valid; `Config.default_format` sets a default. *Limitation:*
  without constrained decoding (ROADMAP Theme F4), JSON wraps the model's natural
  output rather than imposing a per-tool schema — `extract_code`'s
  `{language, code}` is the one structured result today.
- **Batch processing** (Theme C Phase 3, C4): positional inputs expand over
  globs (`*.png`), directories (`--recursive`), and stdin (`-`, one path/line).
  `--output FILE` writes a single result; `--output-dir DIR` writes one file per
  input (extension follows `--format`); `--meta` writes a `.meta.json` telemetry
  sidecar (model, tokens, elapsed) beside each file output. `--type compare`
  groups inputs into consecutive pairs. A warm `llama-server` is reused across
  the batch (cold first item, warm rest).
- **First-run setup wizard** (Theme C Phase 4, C1): `localvision setup` — also
  the default for `localvision` with no args in a terminal — walks through
  hardware detection, model selection, llama-server status, and storage paths,
  then writes `~/.localvision/config.toml`. **Zero new dependencies**: built
  with the standard library and the project's existing ANSI helpers. (A richer
  bubbletea TUI was evaluated and deferred to keep the dependency tree lean; the
  v0.2 tree ran on four direct deps and v0.3 keeps that discipline.)
- **`config.Save`** writes a minimal, `omitempty` TOML config and round-trips
  with `Load`.

### Changed

- **`Executor.Run` now returns `(raw string, stats Stats, err error)`**, where
  `Stats{Model, TokensIn, TokensOut, ElapsedMs}` carries per-inference
  telemetry. The MCP server ignores it; the CLI uses it for `--meta`. This is
  the only interface change in v0.3.0.
- **No-args dispatch**: `localvision` with no arguments now runs the setup wizard
  in an interactive terminal, and the MCP server when invoked over a non-TTY
  stdio (how MCP clients connect). MCP clients using `command: localvision` with
  no `args`, or `args: ["run"]`, are unaffected.
- When `--format` is absent for file/dir/batch output, the default is the model's
  natural markdown.

### Fixed

- **One-shot orphan prevention**: a one-shot now shuts down `llama-server` on
  exit (including mid-batch failures and SIGINT) instead of leaving it running.

## [0.2.2] - 2026-06-22

### Added

- **Configurable model storage + disk-space safety**: free-space precheck;
  `--models-dir` / `--cache-dir` flags; `doctor` shows free space.

## [0.2.1] - 2026-06-22

### Fixed

- **`$PATH`-discovered `llama-server` now spawns** (v0.2.0 regression).

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

[Unreleased]: https://github.com/froggeric/llm/compare/v0.5.1
[v0.5.1]: https://github.com/froggeric/llm/releases/tag/v0.5.1
[v0.5.0]: https://github.com/froggeric/llm/releases/tag/v0.5.0
[v0.4.0]: https://github.com/froggeric/llm/releases/tag/v0.4.0
[v0.3.0]: https://github.com/froggeric/llm/releases/tag/v0.3.0
[v0.2.2]: https://github.com/froggeric/llm/releases/tag/v0.2.2
[v0.2.1]: https://github.com/froggeric/llm/releases/tag/v0.2.1
[v0.2.0]: https://github.com/froggeric/llm/releases/tag/v0.2.0
[v0.1.0]: https://github.com/froggeric/llm/releases/tag/mcp%2Flocalvision%2Fv0.1.0
