# Contributing

Contributions are welcome, with caveats — this project is PolyForm Noncommercial, so all contributors retain their copyright but grant rights under the same terms.

## Project layout

```
local-vision-mcp/                # this subdirectory of froggeric/llm
├── cmd/local-vision-mcp/main.go # entry point
├── internal/
│   ├── version/                 # build-time metadata
│   ├── mcpserver/               # MCP protocol glue (uses go-sdk)
│   ├── llama/                   # subprocess lifecycle, binary mgmt, HTTP client
│   ├── models/                  # catalog, hardware detection, downloader
│   ├── tools/                   # 9 tool implementations
│   ├── config/                  # user config
│   └── logging/                 # slog setup
├── plugin/                      # Claude Code plugin manifest + SKILL.md
├── scripts/                     # build-llama-cpp.sh, release.sh
├── docs/                        # these docs
└── .goreleaser.yaml             # release automation
```

## Development setup

```bash
git clone https://github.com/froggeric/llm.git
cd llm/mcp/local-vision-mcp

# Build
make build

# Test (race detector required for concurrency code)
make test-race

# Vet
make vet

# Lint (requires golangci-lint)
make lint

# Local binary
./bin/local-vision-mcp version
./bin/local-vision-mcp doctor
```

Requires Go 1.23+ (we use `log/slog`, range-over-func, and other 1.23+ features).

## Test philosophy

- Unit tests cover every public function.
- Concurrency tests use `-race` and run under load.
- Integration tests (real `llama-server` + real model) are gated behind `//go:build integration` and not run in CI.
- The benchmark suite (`local-vlm-research/` in the parent repo) is the source of truth for model quality claims in the catalog.

## Adding a new tool

1. Pick an ID (lowercase snake_case). Avoid collisions with existing 9 tools.
2. Create `internal/tools/<your_tool>.go` implementing the `Tool` interface.
3. Add a task-tuned system prompt to `internal/tools/prompt.go`.
4. Add the tool to `allTools()` in `internal/tools/registry.go`.
5. Add the ID constant to `internal/tools/constants.go`.
6. Add tests to `internal/tools/tools_test.go` (table-driven).
7. Update `docs/TOOLS.md` and `plugin/SKILL.md` with the new tool.
8. Update `internal/models/builtin.toml` if a model should be preferred for the new tool (add the ID to `preferred_for`).

## Adding a new model

See [MODELS.md](./MODELS.md). Briefly:

1. Upload GGUF + mmproj to HuggingFace under `froggeric/` namespace.
2. Compute SHA256 of both files.
3. Add a block to `internal/models/builtin.toml`.
4. Run `local-vision-mcp doctor` to verify it loads and validates.

## Code style

- Match existing style. The codebase uses `gofmt` + `goimports`.
- Comments: explain *why*, not *what*. The code itself shows what.
- Public functions get doc comments. Unexported helpers don't unless non-obvious.
- Error wrapping: `fmt.Errorf("doing X: %w", err)` for context, sentinel errors via `errors.Is`.
- No panics in production code paths. Panics in `tools.Register` are intentional (programming error).

## Pull request checklist

- [ ] Branch is up to date with `main`.
- [ ] `make build && make test-race && make vet && make lint` all pass locally.
- [ ] New code has tests.
- [ ] Public API changes are documented.
- [ ] If touching the catalog, you've run `local-vision-mcp doctor` to verify it loads.
- [ ] Commit messages are descriptive (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`).

## Licensing your contribution

By contributing, you agree your changes are licensed under the [PolyForm Noncommercial License 1.0.0](../LICENSE). You retain copyright; the project maintainer (Frederic Guigand) is the licensor.

If your contribution includes code from a third party (e.g. an upstream library), include the original license in `THIRD_PARTY_LICENSES.md`.

## Release process (maintainer)

1. Update `CHANGELOG.md` with the new version's changes.
2. Update `internal/version/version.go` defaults if needed (rare; `make build` injects real values via ldflags).
3. Commit and tag: `git tag mcp/local-vision-mcp/v0.X.0`. Push the tag.
4. `.github/workflows/vision-mcp-release.yml` runs `goreleaser` on tag push and publishes GitHub Releases.
5. Confirm the release artifacts appear at `https://github.com/froggeric/llm/releases/tag/mcp%2Flocal-vision-mcp%2Fv0.X.0`.

The tag prefix `mcp/local-vision-mcp/v*` matches Go's subdirectory-module convention so `go install` resolves correctly.

## Getting help

- Open an issue: https://github.com/froggeric/llm/issues (tag `component: local-vision-mcp`).
- Email: `frederic@guigand.com` for security issues (see [SECURITY.md](./SECURITY.md)).
