# local-vision-mcp

A Go-based MCP server that wraps a local `llama.cpp` subprocess to provide
vision-language model tools to text-only coding LLMs (Claude Code, Codex CLI,
etc.).

**Status:** Phase 0 (contract). Interface stubs are committed; 5 subagent
tracks are about to be dispatched concurrently to fill in implementations.

## Where you are

This subdirectory lives inside [`froggeric/llm`](https://github.com/froggeric/llm),
which holds prompts, skills, and shell-first workflows for working with LLMs.
The MCP is one of several LLM-related artifacts in that repo. It has its own
`go.mod`, `LICENSE`, and CI workflow so it can be built and released
independently.

## Module path

```
github.com/froggeric/llm/mcp/local-vision-mcp
```

## Layout

```
mcp/local-vision-mcp/
├── go.mod, go.sum, LICENSE, .gitignore
├── cmd/local-vision-mcp/main.go    # entry point (stub; Phase 2 fills in)
├── internal/
│   ├── version/                    # build-time metadata
│   ├── mcpserver/                  # MCP protocol glue (uses go-sdk)
│   ├── llama/                      # subprocess lifecycle, binary mgmt, HTTP client
│   ├── models/                     # catalog, hardware detection, downloader
│   ├── tools/                      # 9 tool implementations
│   ├── config/                     # user config
│   └── logging/                    # slog setup
├── plugin/                         # Claude Code plugin manifest + SKILL.md
├── scripts/                        # build-llama-cpp.sh, release.sh
├── docs/                           # INSTALL, TOOLS, MODELS, etc.
└── dist/                           # release artifacts (gitignored)
```

## Build

```bash
cd mcp/local-vision-mcp
go build ./...
go test ./...
```

## Locked dependencies (do not change without lead approval)

- `github.com/modelcontextprotocol/go-sdk` v1.6.1 — official MCP SDK
- `github.com/BurntSushi/toml` v1.6.0 — catalog parsing
- `github.com/stretchr/testify` v1.11.1 — test assertions

## License

[PolyForm Noncommercial 1.0.0](./LICENSE). Source-available; commercial use
requires a separate license. See `COMMERCIAL-LICENSING.md` (to be added by
Track A).

## Pre-Implementation Plan

The full design + review is in `/Volumes/ssd/github/misc/local-vision-mcp/`:
- `PLAN-v2.md` — first review pass, 28 findings
- `PLAN-v3-monorepo-deltas.md` — second pass, 13 monorepo-specific findings

Read both before starting any track.
