# Changelog

All notable changes to `localvision` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches v1.0.0. Until then, minor changes may break compatibility.

Tags for this subdirectory follow the Go module convention
`mcp/localvision/v<MAJOR>.<MINOR>.<PATCH>`.

## [Unreleased]

_Nothing yet._

## [0.7.0] - 2026-06-25

Per-tool model routing + multi-sample consensus, two new mirrored models, and a
round of correctness/UX fixes from a code review. See [`ROADMAP.md`](./ROADMAP.md)
(F5, F6).

### Added

- **Per-tool model routing (F6)** — the v6 benchmark crowns a different best
  model per tool, so the catalog now routes per tool via `preferred_for`:
  `qwen3.5-4b-q8` serves `extract_code` / `describe_ui` / `describe_diagram` /
  `diagnose_error` (it edges the 8B there, at ~58 tok/s and 4.2 GB), and
  `qwen3-vl-8b` serves the rest. Hardware fit still gates selection (on hardware
  where the 8B doesn't fit, those tools fall back; the `qwen3.5-4b` Q4 remains
  the sub-4 GB fallback). `doctor` now prints the per-tool routing table.
- **Two new catalog models, mirrored to huggingface.co/froggeric**:
  `qwen3.5-4b-q8` (Q8_0, 4.2 GB) and `qwen3.6-35b-a3b` (the sparse MoE, 21 GB,
  opt-in via `--model` — the benchmark's `read_image` coverage pick *with*
  `--sample`; on a single call it ties much smaller models, so it is not a
  default).
- **Per-tool config** — `[tools.<id>]` tables in `config.toml` override the
  model (`model = "..."`) and the sampling method (`method = "union@N"`) per
  tool. Layering: `--model` flag (forces all) > `[tools.<id>].model` > catalog
  routing > `default_model` (now a **fallback**, no longer a forced override —
  it no longer defeats per-tool routing). Method: `off` (default) / `union@N`.
- **Setup wizard** — after picking a default model, offers to write the
  benchmark's recommended per-tool routing as explicit `[tools.<id>].model`
  tables (default-off; flagged that mixed-tool sessions then switch models).

### Added (experimental)

- **Multi-sample consensus (F5, off by default)** — `--sample N` (or
  `[tools.<id>].method = "union@N"`) runs N warm calls at the tool's sampling
  temperature and fuses them via a text-only merge pass. Driven by the v6
  benchmark category report: `read_image` / `describe_ui` / `describe_chart` at
  0.7, `extract_text` at 0.4. Any tool can be opted in (the per-tool recipe
  supplies the temp; single-mode tools use 0.7 when forced). Single calls stay
  at 0.1 (temp is the "gate" — at 0.1 the N runs are ~identical). A/B on the 8B:
  union@3 surfaced more detail at ~2.1× latency.

### Changed

- The executor's `default_model` (config) is now a **fallback**, not a forced
  override — per-tool catalog routing takes precedence. `--model` is still a
  forced override (bypasses routing).
- `doctor`'s Tools section shows the per-tool routing for the detected hardware.

### Fixed

- **Windows idle-kill no longer orphans `llama-server`.** The untagged,
  `syscall`-based signaling compiled on Windows but silently no-op'd there
  (`os.Process.Signal` returns `EWINDOWS`), so an idle `llama-server.exe` was
  never killed — leaking VRAM until the MCP server exited. Signals are now split
  per-OS (`signals_unix.go` / `signals_windows.go`); Windows probes liveness via
  `OpenProcess` + `GetExitCodeProcess`, so the SIGKILL escalation now fires.
- **`safety_margin_gb` now actually influences model selection.** The config knob
  was parsed, validated, and shown by `doctor` but never reached selection (which
  always used the hardcoded 4 GB default). It's now threaded through
  `DefaultModel` / `ModelFor` / `Fits`.
- **`describe_chart` / `describe_diagram` honor the requested output mode.**
  `ParseOutput` content-sniffed, so a prose-mode report that happened to be valid
  JSON was returned as structured output. It now gates on the requested mode
  (`csv` / `json` / `mermaid` / `prose`); also fixes `outputMode` erroring on an
  explicit `output = "prose"`.
- **Subcommand `--help` exits 0, not 2.** `-h` / `--help` on `run`, `doctor`,
  `setup`, and the one-shot form now match the top-level `--help` (shared
  `parseFlags` helper).
- **Overlay TOML can now set / override `chat_template_kwargs`** (the overlay
  merge silently dropped it before).
- **`mcpProgressSink.Progress` is safe to call after `close()`** — a send on the
  closed progress channel could panic the server if a producer outlived the call;
  it's now recovered.
- **Multi-sample `--sample N` accounts the merge pass** in the `--meta` token /
  timing sidecar (the merge call's tokens were previously dropped).
- **Greedy decoding is expressible** — `ChatRequest.Temperature` / `TopP` are now
  `*float64`, so an explicit `0.0` is honored instead of silently replaced with
  the `0.1` default.
- **`setup --non-interactive` / `--yes`** writes a config from
  `LOCALVISION_SETUP_MODEL` / `_ROUTING` / `_FORMAT` env vars, for CI and scripts
  (the interactive wizard cancels on closed stdin).
- Smaller: `health` backoff resets once `llama-server` starts answering; the
  downloaded-archive handle is closed exactly once; a SHA-mismatched model file
  invalidates the integrity cache (avoids a stale-hash false mismatch); a dead
  `loadErr` field and an unreachable `.webp` MIME branch were removed.

### Known limitations

- **Per-tool model routing switches models between tools** in a mixed MCP
  session (a cold reload per switch). The wizard defaults to single-model +
  opt-in routing to preserve warm reuse; per-tool routing is a deliberate choice.
- **4B-Q8 reliability** — the benchmark flags Q8 as ~87%-ok (slightly
  timeout-prone); the rock-solid 8B-Q8 remains the safe default for mixed use.
- **MoE = 21 GB**; only useful for `read_image` coverage with `--sample`; opt-in.
- **Windows runtime not validated on real hardware** — the per-OS signal split
  (which fixes the idle-kill orphan) cross-compiles clean on all six targets and
  its nil-guard is covered by a test that runs on Windows, but the actual
  load → idle → `llama-server.exe` exit + flat handle-count behavior was not
  verified from the macOS dev box. Report discrepancies.

## [0.6.0] - 2026-06-24

Tools & UX: a new document tool, structured chart/diagram output, and streaming
progress. See [`ROADMAP.md`](./ROADMAP.md) (G3, G4, G5, E1).

### Added

- **`read_document` tool** (G3): rasterize a PDF and summarize it in one
  inference — document summary, per-page highlights (critical text transcribed),
  tables/figures called out. New 11th tool (`localvision paper.pdf --type doc`).
  Rasterization uses an opportunistic, CLI-only, `$PATH`-discovered chain
  (`pdftoppm` → `mutool` → `magick`/`convert` → `gs`); none is bundled. Up to 20
  pages (longer documents are truncated, noted in the output). Magic-byte
  detection means a `data:`-URI PDF written to a `.bin` temp is recognized too.
  Requires a rasterizer on `$PATH` (poppler/mupdf/imagemagick/ghostscript).
- **Structured chart output** (G4): `describe_chart` gains an optional `output`
  argument — `csv` (underlying numbers, paste into a spreadsheet) or `json` (a
  JSON object of the data, returned as MCP structured content). Default `prose`
  is unchanged. CLI: `--output-mode csv|json`.
- **Editable diagram markup** (G5): `describe_diagram` gains `output=mermaid` —
  editable Mermaid markup that reproduces the diagram. CLI: `--output-mode mermaid`.
- **Streaming progress** (E1): the CLI spinner and MCP `notifications/progress`
  now report real progress instead of going silent for 30–70 s. Downloads
  (model files, the ~80 MB `llama-server` binary) stream bytes (`%` + MiB);
  inference reports phase transitions and a climbing elapsed heartbeat.
  MCP clients opt in by sending `_meta.progressToken`; no token = no
  notifications (byte-for-byte today). Fire-and-forget with a 2 s timeout so a
  slow client can never stall a call. (Real token-by-token SSE output streaming
  is deferred to v0.7.)

### Changed

- Tool count is now **11**. The `read_document` tool is added to the
  `qwen3-vl-8b` catalog entry's `preferred_for`.
- `describe_chart`/`describe_diagram` `BuildRequest` return a mode-specific
  prompt when a non-prose `output` is requested; the default (`prose`) still
  returns `SystemPrompt()` (backward compatible).

### Internal

- New `internal/progress` package: a context-carried, nil-safe progress Sink
  (`Update`/`WithSink`/`SinkFrom`/`Report`/`Heartbeat`/`Throttled`). Threaded
  through the lifecycle (downloads + phases), the executor (inference
  heartbeat), the MCP `callTool` (per-request `NotifyProgress` sink), and the
  CLI spinner — without changing the `tools.Executor` interface or its mocks.
- New `tools.Expander` interface + `ExpandInput` + `RegisterTemp`: an optional
  hook a tool implements to turn one document input into multiple image refs
  (`read_document` rasterizes PDFs). The 10 existing tools pay nothing.
- `llama.LifecycleManager.Phase` now takes a `context.Context` so per-request
  progress sinks reach the download/load/ready transitions. `Options.PhaseHook`
  is retained for back-compat (the CLI spinner still uses it for per-phase timing).
- `tools.CleanupImageRef` now also reaps a now-empty rasterizer out dir (never
  the shared `os.TempDir()`), so PDF page temps don't leak their parent dir.

### Known limitations

- `read_document` needs a PDF rasterizer on `$PATH`; on a host with none, the
  tool errors with installable options named. The rasterizer argv is
  cross-platform by inspection; CI exercises it only via fakes (no real
  rasterizer in the 3-OS matrix).
- Multi-page PDFs are sent to the model in one inference; very long documents
  are capped at 20 pages (per-page batching is a future enhancement).
- Inference progress is an elapsed heartbeat, not real token streaming (SSE is
  a v0.7 item).

## [0.5.2] - 2026-06-23

Small follow-up to v0.5.1.

### Fixed

- **`doctor` now shows the model tools actually use on large machines.** On
  49+ GB Macs, v0.5.1's `doctor` reported `qwen3.6-27b` as the "Default model"
  (the generic default fell back to the largest fitting model), even though
  tool calls used `qwen3-vl-8b`. The default-selection fallback now prefers a
  fitting model that lists tools (`preferred_for` non-empty) over a larger
  opt-in model that lists none — so the displayed default matches real tool
  selection on every hardware tier. (`--model` still selects any catalog model.)

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

[Unreleased]: https://github.com/froggeric/llm/compare/v0.7.0
[0.7.0]: https://github.com/froggeric/llm/releases/tag/v0.7.0
[v0.6.0]: https://github.com/froggeric/llm/releases/tag/v0.6.0
[v0.5.2]: https://github.com/froggeric/llm/releases/tag/v0.5.2
[v0.5.1]: https://github.com/froggeric/llm/releases/tag/v0.5.1
[v0.5.0]: https://github.com/froggeric/llm/releases/tag/v0.5.0
[v0.4.0]: https://github.com/froggeric/llm/releases/tag/v0.4.0
[v0.3.0]: https://github.com/froggeric/llm/releases/tag/v0.3.0
[v0.2.2]: https://github.com/froggeric/llm/releases/tag/v0.2.2
[v0.2.1]: https://github.com/froggeric/llm/releases/tag/v0.2.1
[v0.2.0]: https://github.com/froggeric/llm/releases/tag/v0.2.0
[v0.1.0]: https://github.com/froggeric/llm/releases/tag/mcp%2Flocalvision%2Fv0.1.0
