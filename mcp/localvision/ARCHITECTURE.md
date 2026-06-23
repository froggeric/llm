# localvision

A Go-based MCP server that wraps a local `llama.cpp` subprocess to provide
vision-language model tools to text-only coding LLMs (Claude Code, Codex CLI,
etc.).

**Status:** v0.1.0 shipped; `main` carries the unreleased v0.2 catalog refresh.
All interface contracts are implemented — the 11 tools, the `llama.cpp`
lifecycle, the model catalog, and hardware detection are live. For the forward
plan see [`ROADMAP.md`](./ROADMAP.md); for shipped state see
[`CHANGELOG.md`](./CHANGELOG.md).

## Where you are

This subdirectory lives inside [`froggeric/llm`](https://github.com/froggeric/llm),
which holds prompts, skills, and shell-first workflows for working with LLMs.
The MCP is one of several LLM-related artifacts in that repo. It has its own
`go.mod`, `LICENSE`, and CI workflow so it can be built and released
independently.

## Module path

```
github.com/froggeric/llm/mcp/localvision
```

## Layout

```
mcp/localvision/
├── go.mod, go.sum, LICENSE, .gitignore
├── cmd/localvision/main.go    # entry point: run (MCP server), doctor, version
├── internal/
│   ├── version/                    # build-time metadata
│   ├── mcpserver/                  # MCP protocol glue (uses go-sdk)
│   ├── llama/                      # subprocess lifecycle, binary mgmt, HTTP client
│   ├── models/                     # catalog, hardware detection, downloader
│   ├── tools/                      # 11 tool implementations
│   ├── config/                     # user config
│   └── logging/                    # slog setup
├── plugin/                         # Claude Code plugin manifest + SKILL.md
├── scripts/                        # build-llama-cpp.sh, install.sh, release.sh
├── docs/                           # INSTALL, TOOLS, MODELS, etc.
└── dist/                           # release artifacts (gitignored)
```

## Build

```bash
cd mcp/localvision
go build ./...
go test ./...
```

## Locked dependencies (do not change without lead approval)

- `github.com/modelcontextprotocol/go-sdk` v1.6.1 — official MCP SDK
- `github.com/BurntSushi/toml` v1.6.0 — catalog parsing
- `github.com/stretchr/testify` v1.11.1 — test assertions

## License

[PolyForm Noncommercial 1.0.0](./LICENSE). Source-available; commercial use
requires a separate license. See `COMMERCIAL-LICENSING.md`.

## Pre-Implementation Plan

The original design + review notes lived in `/Volumes/ssd/github/misc/localvision/`
(`PLAN-v2.md`, `PLAN-v3-monorepo-deltas.md`). Those scratch docs sit outside
this repo and may be stale following the rename. The current source of truth
for intent is this file plus [`ROADMAP.md`](./ROADMAP.md).
