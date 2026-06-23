# localvision — notes for Claude Code

A Go subdirectory-module (`mcp/localvision/`, its own go.mod) inside the
`froggeric/llm` monorepo, built and released independently. Wraps a local
`llama-server` subprocess to give text-only LLMs vision.

## Architecture

- Pure Go, **CGO off** — the wrapper spawns `llama-server` (a subprocess) and does no inference itself, so it cross-compiles freely to all 6 targets.
- **Lean deps on purpose** (toml, mcp-sdk, testify, x/sys, yaml.v3). Prefer stdlib over new deps; adding one needs justification (bubbletea was rejected for a stdlib wizard).
- Inference path is MCP-agnostic: `tools.Tool.BuildRequest` → `CatalogExecutor.Run` → `Tool.ParseOutput`. The one-shot CLI and the MCP server both use it.
- `Executor.Run` returns `(raw string, stats Stats, err error)` (v0.3+); update all call sites + mocks when changing it.
- Model selection: `effectiveMemoryGB` = VRAM for discrete GPUs (CUDA/ROCm), else system RAM (Apple Silicon = unified). `fitsModel` keys off it.

## Cross-platform (dev happens on macOS)

- Runtime-verify Linux/Windows only via the **3-OS CI matrix** (`vision-mcp-ci.yml`: ubuntu/windows/macos), not locally.
- Cross-compile check before push:
  `for t in linux/amd64 linux/arm64 windows/amd64 windows/arm64; do GOOS=${t%/*} GOARCH=${t#*/} CGO_ENABLED=0 go build ./... || break; done`
- OS-specific code lives in build-tagged files (`term_*.go`, `disk_*.go`, `hardware_*.go`). `unix.TIOCGETA`/`TCGETS` and `unix.Statfs`/`Statfs_t` are NOT cross-OS — one file per OS.
- Windows syscalls missing from x/sys (e.g. `GlobalMemoryStatusEx`) → call via `windows.NewLazySystemDLL("kernel32.dll").NewProc(name)`; verify the Go struct matches the Win32 layout (byte-exact) on amd64 + arm64.
- Tests with hardcoded Unix paths (`/tmp/`, `/opt/`) fail on Windows (`filepath.Abs` makes them drive-relative). Use `t.TempDir()`-based paths, or `t.Skip` on Windows for Unix-inherent tests (exec-bit, `#!/bin/sh`, SIGTERM/SIGKILL).

## HEIC/WEBP

- Never bundle an image decoder (HEVC patents + freeware redistribution limits). Convert via the opportunistic, CLI-only, `$PATH`-discovered chain in `internal/llama/client.go` `convertImage`: `sips → magick/convert → heif-convert → ffmpeg`. No GUI tools (IrfanView/XnView) — localvision runs headless.

## Conventions

- `gofmt -w cmd/ internal/` before every commit; the tree must be gofmt-clean (reviews flag dirt). Verify with `gofmt -l cmd/ internal/` (empty = clean).
- Tests use testify (`assert`/`require`). Factor pure parsers out and use package-level `var` seams (`exec.LookPath` / subprocess overrides) for testability.
- Bool flags + interspersed positionals: detect bool-ness via the `IsBoolFlag` interface, not a hand-maintained name list (see `splitArgs`).

## Release (recurring)

- Tag `mcp/localvision/vX.Y.Z` + push → triggers `vision-mcp-release.yml`. It normalizes the subdir tag to bare `vX.Y.Z` for goreleaser (`gomod.proxy: false`); GitHub Releases are named with the bare tag.
- Verify locally first: `cd mcp/localvision && goreleaser release --snapshot --clean` (builds all 6 targets, no publish).
- Before tagging: run the `code-reviewer` agent on the diff, fix all HIGH/MEDIUM, and E2E-test headline features against a real model (e.g. `localvision img.png --type ocr --format json | jq .`).
- Homebrew formula → `froggeric/homebrew-tap` via the `HOMEBREW_TAP_GITHUB_TOKEN` secret (fine-grained PAT).
