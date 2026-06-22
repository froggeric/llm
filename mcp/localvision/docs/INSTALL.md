# Install

localvision runs on macOS (Apple Silicon is the MVP target; Linux/Windows are stubbed for v0.2). You need:

- A Mac with at least 8 GB unified memory (16 GB+ recommended).
- ~5 GB free disk for the smallest model + ~5 MB for the binary.
- Go 1.23+ if installing from source, or the prebuilt binary from GitHub Releases.

## Quick install

```bash
# Option A: curl | sh (recommended; downloads prebuilt binary)
curl -fsSL https://github.com/froggeric/llm/releases/latest/download/install.sh | sh

# Option B: go install (compiles from source)
go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest

# Option C: build from a local checkout
git clone https://github.com/froggeric/llm.git
cd llm/mcp/localvision
make build
sudo make install  # copies bin/localvision to /usr/local/bin
```

Verify:

```bash
localvision version
# localvision 0.1.0 (commit abc1234, built 2026-06-17T12:00:00Z)
```

## First-run setup

On first `run`, the binary:

1. Creates `~/.localvision/` (config dir).
2. Downloads `llama-server` (~5 MB) from `https://github.com/ggml-org/llama.cpp/releases` into `~/.localvision/bin/`. SHA256-verified.
3. On first tool call, downloads the picked model (typically `qwen3-vl-8b`, ~5 GB) from `https://huggingface.co/froggeric/`. SHA256-verified.

Total first-run disk: ~5 GB. Subsequent runs use the cache.

Run `doctor` to check what's installed:

```bash
localvision doctor
```

## Configure Claude Code

Add the plugin via Claude Code's plugin config:

```json
{
  "mcpServers": {
    "localvision": {
      "command": "localvision",
      "args": ["run"]
    }
  }
}
```

Restart Claude Code. Verify with `/mcp` — `localvision` should show 9 tools (`compare_images`, `describe_chart`, `describe_diagram`, `describe_ui`, `diagnose_error`, `extract_code`, `extract_table`, `extract_text`, `read_image`).

## Configure other MCP clients (Cursor, Cline, etc.)

The server speaks stdio JSON-RPC. Any MCP-compatible client works. Point it at the binary with `run` as the only argument.

## Upgrade

```bash
# Upgrade the binary
go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest

# Models and llama-server are cached; they upgrade only when the binary's
# pinned SHA256 changes (rare). To force-redownload:
rm -rf ~/.localvision/bin
rm -rf ~/.localvision/models
localvision doctor  # will report missing, triggers re-download on next run
```

## Uninstall

```bash
rm /usr/local/bin/localvision          # or wherever you installed
rm -rf ~/.localvision                  # cache + config + logs
```

## Troubleshooting

See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for common issues.
