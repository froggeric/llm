# localvision

A Go-based MCP (Model Context Protocol) server that wraps a local
`llama.cpp` subprocess to give text-only coding LLMs **vision** — Claude
Code, Codex CLI, and similar agents that cannot natively see images.

It runs entirely on your machine. There is no telemetry, no outbound HTTP
except to `huggingface.co/froggeric/` for the initial model download, and
the `llama-server` subprocess binds to `127.0.0.1` only.

> **Status:** v0.1.0 shipped (Apple Silicon MVP). `main` carries the unreleased
> v0.2 catalog refresh — not yet tagged. Apple Silicon only; Linux and Windows
> are planned (see [`ROADMAP.md`](./ROADMAP.md), Theme D).

---

## What it is

`localvision` exposes a small set of vision tools over the standard
MCP protocol:

- `read_image` — describe an image in natural language
- `extract_text` — OCR / transcription
- `extract_code` — extract source code from a screenshot
- `extract_table` — structured table extraction
- `describe_ui` — describe a UI for accessibility or replay
- `describe_diagram` — explain an architecture or flow diagram
- `describe_chart` — read a chart back as data
- `diagnose_error` — explain an error dialog or stack trace
- `compare_images` — diff two images

Each tool takes a local file path (preferred), a `data:` URI, or a
`file://` URI. Remote `http(s)://` URLs are **rejected** because the
underlying `llama-server` is localhost-only.

Behind the scenes, the server spawns `llama-server` (from `llama.cpp`),
loads the smallest fitting model from a built-in catalog, runs inference,
and tears the subprocess down after a 5-minute idle timer.

## Install

You have two options.

### Option 1 — `go install`

The canonical install path uses the Go toolchain and pulls from the
canonical module path:

```bash
go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest
```

The resulting binary lands in `$(go env GOPATH)/bin`. Make sure that
directory is on your `PATH`.

### Option 2 — `curl | sh`

If you do not have Go installed, you can use the installer that ships with
each GitHub release:

```bash
curl -fsSL https://github.com/froggeric/llm/releases/latest/download/install.sh | bash
```

The installer (source: [`scripts/install.sh`](./scripts/install.sh)):

1. Detects your OS and architecture via `uname`.
2. Rejects unsupported combinations (in v0.1: only macOS Apple Silicon is
   supported; Linux and Windows fail with a clear message).
3. Downloads the release tarball from GitHub Releases.
4. Verifies the SHA256 sidecar.
5. Extracts `localvision` into `/usr/local/bin` if you have `sudo`,
   otherwise `~/.local/bin`.

### What happens on first run

On the first `localvision run`, the binary will:

1. Download a pinned `llama-server` build (≈ 80 MB) to
   `~/.localvision/bin/` and verify its SHA256.
2. Download the smallest model that fits your hardware from
   `huggingface.co/froggeric/` (the v0.2 smallest is Qwen3.5 4B, ~3 GB).
3. Surface install progress to the MCP client (`tools/list` still works
   while this happens; tools are marked unavailable).

This first-run experience takes 5–15 minutes on a fast connection. You can
speed it up by pre-running `localvision doctor` after install.

## Quick start

After install, point Claude Code (or any MCP client) at the binary. In
Claude Code's MCP config:

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

Verify the install:

```bash
localvision version     # prints version, commit, build date
localvision doctor      # checks llama-server, models, hardware tier
```

Then ask Claude Code something like:

> Use the `read_image` tool to describe ./screenshot.png.

Claude will call the tool, the MCP server will spawn `llama-server`, load
the right model, and stream back the description (typically within
30–70 seconds; see *Latency* below).

## Privacy

- **No telemetry, ever.** No anonymous stats, no crash reports that include
  image content, no "phone home" of any kind.
- **No outbound HTTP except to `huggingface.co/froggeric/`.** The
  `llama-server` subprocess binds to `127.0.0.1` only; nothing it does
  leaves your machine.
- **Images are processed in memory.** They are never written to disk unless
  you opt in for debugging via `--keep-temp-images`.
- **Logs reference images by hash-of-bytes**, never by content, so you can
  attach a log to a bug report without leaking PII.

## License

This project is licensed under the
[PolyForm Noncommercial License 1.0.0](https://polyformproject.org/licenses/noncommercial/1.0.0).

In short: it is **source-available** — you can read, modify, and
redistribute the code for non-commercial purposes. Commercial use requires
a separate commercial license. See [`COMMERCIAL-LICENSING.md`](./COMMERCIAL-LICENSING.md)
for details, or contact <frederic@guigand.com>.

The bundled `llama.cpp` binary that `localvision` downloads at first
run is MIT-licensed. See [`THIRD_PARTY_LICENSES.md`](./THIRD_PARTY_LICENSES.md)
for the full list of dependencies and their licenses.

## Latency expectations

Each tool call spawns (or reuses) a `llama-server` subprocess, runs a
single vision inference, and returns the result. Expect:

- **Cold start** (first call after install or after the 5-minute idle
  kill): 30–70 seconds — most of which is loading the model into unified
  memory.
- **Warm calls** (within the 5-minute idle window): 5–25 seconds depending
  on the model and image complexity.

If your MCP client has a tool-call timeout (some approval pipelines do),
set it to **at least 120 seconds** to avoid spurious failures on cold
starts. If you are using a separate approval gate (e.g. an
auto-approve pipeline), tune its timeout to match.

## Coexistence with Ollama

If you already run `ollama serve` on port `11434`, **this MCP does not
touch that port**. They can coexist.

The catch is unified memory: on Apple Silicon, both `llama-server` and
Ollama compete for the same pool of wired memory. If Ollama has a model
loaded, our `llama-server` may fail to fit its own model. The `doctor`
subcommand checks for a running Ollama and warns; on cold starts the server
emits a stderr log line if it detects `:11434` is occupied.

If you hit memory pressure, the simplest fix is to stop Ollama while you
use vision tools:

```bash
ollama stop                 # or: pkill -TERM ollama
```

This is documented as a known limitation; v0.2 may add automatic
coordination via Ollama's API.

## Roadmap

The near-term plan: green CI and a real pinned release (GitHub Releases +
Homebrew), then a standalone shell CLI — `localvision describe img.png` with
`--type`, `--format`, `--output`, and `--model` — then cross-platform
(Linux/Windows + GPU backends), then hardening. Beyond that: a localhost HTTP
API and OpenAI-compatible endpoint, richer tools (video, PDF, UI→code), and
far-future research ideas. Every item, its effort, and its target version is in
[`ROADMAP.md`](./ROADMAP.md) (themes A–H).

## Project layout

```
mcp/localvision/
├── go.mod, go.sum, LICENSE, .gitignore, Makefile
├── ARCHITECTURE.md                 # high-level overview
├── README.md                       # this file
├── CHANGELOG.md                    # per-subdirectory changelog
├── ROADMAP.md                      # forward plan, themes, target versions
├── COMMERCIAL-LICENSING.md
├── THIRD_PARTY_LICENSES.md
├── cmd/localvision/main.go    # entry point
├── internal/                       # implementation (server, llama, models, tools, config)
├── plugin/                         # Claude Code plugin manifest + SKILL.md
├── scripts/                        # build-llama-cpp.sh, install.sh, release.sh
├── docs/                           # INSTALL, TOOLS, MODELS, etc.
└── dist/                           # release artifacts (gitignored, goreleaser output)
```

This subdirectory lives inside [`froggeric/llm`](https://github.com/froggeric/llm)
and is built / released independently of the rest of the repo.
