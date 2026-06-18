# Changelog

All notable changes to `local-vision-mcp` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches v1.0.0. Until then, minor changes may break compatibility.

Tags for this subdirectory follow the Go module convention
`mcp/local-vision-mcp/v<MAJOR>.<MINOR>.<PATCH>`.

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
- **CLI**: `local-vision-mcp run` (default; stdio server), `doctor`
  (diagnostics), `version`.
- **Catalog overlays**: drop TOML files in `~/.local-vision-mcp/catalog.d/`
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
  `~/.local-vision-mcp/bin/`. v0.1.1 will pin a real hash.
- HEIC conversion requires `sips` (macOS built-in). Linux/Windows users must
  convert HEIC images manually.
- No streaming responses to the MCP client. Each `tools/call` blocks until
  the inference completes (30–70 s typical). v0.2 may add
  `notifications/progress`.
- Tool names are unprefixed. If you have another vision MCP installed, you
  may see name collisions in Claude Code's tool list.
- `InternVL3.5 8B` was considered but dropped from v0.1 — no clean upstream
  GGUF source and it ranked last in our 7-model benchmark.

## [Unreleased]

### Notes for the lead (do not ship verbatim)

- This `v0.1.0` entry is a placeholder written by Track A (skeleton + build
  + release infrastructure). The lead updates it before tagging
  `mcp/local-vision-mcp/v0.1.0`, once Tracks B/C/D/E have landed their
  implementations and the Phase 3 smoke test has produced real model SHA256s.

## [v0.1.0] — Phase 0 scaffold + 5 implementation tracks

### Added — Phase 0 contract (lead)

- Go module at `mcp/local-vision-mcp/` with its own `go.mod`, `LICENSE`,
  and CI workflow, so it can be built and released independently from the
  rest of the monorepo.
- Stub entry point at `cmd/local-vision-mcp/main.go` that exits non-zero
  with a "not yet implemented" message; supports a `version` subcommand.
- Interface stubs in `internal/version`, `internal/llama/lifecycle.go`,
  `internal/models/catalog.go`, `internal/tools/tool.go`, and
  `internal/config/config.go` returning `errors.New("not implemented")`.
- `internal/models/builtin.toml` catalog scaffold with the v0.1 model set
  (qwen3-vl-4b/8b, gemma4-26b-a4b, internvl35-8b). SHA256 fields are
  placeholders pending Phase 3.
- `LICENSE` (PolyForm Noncommercial 1.0.0) and `ARCHITECTURE.md`.

### Added — Track A (this release)

- `Makefile` with `build`, `test`, `test-race`, `lint`, `vet`, `clean`,
  `install`, `release-snapshot`, `release`, `doctor`, `version` targets.
- `.goreleaser.yaml` configured for darwin/arm64 (Apple Silicon) MVP, with
  stub configuration for five additional targets to be enabled in v0.2.
  Ships the `scripts/install.sh` `curl|sh` installer as a release asset.
  Tag convention: `mcp/local-vision-mcp/v0.1.0`.
- `scripts/build-llama-cpp.sh` — builds `llama-server` for a single target
  platform, with Metal support on darwin-arm64 and clear error messages
  for the five stubbed targets. Produces a SHA256 sidecar.
- `scripts/install.sh` — `curl|sh` installer (source of truth). Detects
  OS/arch via `uname`, rejects unsupported combinations with a clear
  message, downloads the right release tarball, verifies SHA256, extracts
  to `/usr/local/bin` (falling back to `~/.local/bin` when no sudo).
  Supports `--dry-run`. Uploaded to each GitHub Release as a top-level
  `install.sh` artifact by goreleaser.
- `scripts/release.sh` — thin wrapper around `goreleaser release` run from
  the subdirectory.
- `COMMERCIAL-LICENSING.md` explaining PolyForm Noncommercial implications
  and the contact process for commercial licenses.
- `THIRD_PARTY_LICENSES.md` listing MIT notices for `go-sdk`, BurntSushi
  TOML, `testify`, and the auto-downloaded `llama.cpp` binary.
- `README.md` (subdirectory-level) covering install, quick start, privacy,
  license, latency expectations, and Ollama coexistence.
- This `CHANGELOG.md`.
- `.github/workflows/vision-mcp-ci.yml` — scoped CI on macOS 14 (Apple
  Silicon). `paths: ['mcp/local-vision-mcp/**']` filter.
- `.github/workflows/vision-mcp-release.yml` — goreleaser-driven release
  on `mcp/local-vision-mcp/v*` tag pushes.
- One-line addition to the root `README.md` pointing at this subdirectory.

### Deviations from PLAN-v3

- The plan literal says "`dist/install.sh` — `curl|sh` installer." That
  path is unworkable because goreleaser wipes `dist/` on every release
  (`--clean`), which would delete the script before it could be uploaded.
  The source of truth is `scripts/install.sh`; goreleaser uploads it to
  each GitHub Release as a top-level `install.sh` artifact. The user-
  facing `latest/download/install.sh` URL still resolves correctly.

### Pending — Tracks B, C, D, E

- Track B: MCP server shell (`internal/mcpserver`, `internal/logging`).
- Track C: `llama.cpp` lifecycle, binary downloader, HTTP client
  (`internal/llama`).
- Track D: models catalog, hardware detection, downloader
  (`internal/models`).
- Track E: 9 tool implementations (`internal/tools`).

### Pending — Phase 2 (lead)

- `cmd/local-vision-mcp/main.go` subcommand dispatch (`run`, `doctor`,
  `version`).
- `plugin/plugin.json` + `plugin/SKILL.md`.
- `docs/{INSTALL,TOOLS,MODELS,CONFIGURATION,CONTRIBUTING,SECURITY,TROUBLESHOOTING}.md`.

### Pending — Phase 3 (lead)

- Real model SHA256s computed and committed to `builtin.toml`.
- End-to-end smoke test against the smallest model.
- Tag `mcp/local-vision-mcp/v0.1.0`.

[Unreleased]: https://github.com/froggeric/llm/compare/HEAD
[v0.1.0]: https://github.com/froggeric/llm/releases/tag/mcp%2Flocal-vision-mcp%2Fv0.1.0
