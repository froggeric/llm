# localvision

A Go-based MCP (Model Context Protocol) server that wraps a local
`llama.cpp` subprocess to give text-only coding LLMs **vision** — Claude
Code, Codex CLI, and similar agents that cannot natively see images.

It runs entirely on your machine. There is no telemetry, no outbound HTTP
except to `huggingface.co/froggeric/` for the initial model download, and
the `llama-server` subprocess binds to `127.0.0.1` only.

> ⚠️ **Linux & Windows users: please report issues.** v0.4 is the first
> cross-platform release. The maintainer develops and tests on **macOS only** —
> there is **no real Linux or Windows GPU hardware** to validate against. GPU
> detection (CUDA/ROCm), model selection, and the HEIC converter chain are
> unit-tested and pass CI on Linux/Windows, but real-world behavior on actual
> NVIDIA/AMD GPUs is **unverified**. If anything mis-detects, mis-selects a
> model, or fails to convert an image, please open an issue with your `doctor`
> output and `--verbose` logs:
> **[github.com/froggeric/llm/issues](https://github.com/froggeric/llm/issues)**.
> As a workaround, `default_model` in the config overrides any misdetection.

> **Status:** **v0.7.0** — per-tool model routing + multi-sample consensus + 2
> new models, plus a round of correctness/UX fixes. Cross-
> platform: macOS (Apple Silicon/Intel), Linux, Windows (x86_64 + arm64). One-
> shot CLI (`localvision img.png --type ocr`), `--format`, batch
> (`--output-dir --meta`), a `setup` wizard, a cross-platform HEIC/WEBP converter
> chain, 11 vision tools — including `read_document` (PDF ingestion) and
> `image_to_prompt`. Charts and diagrams return structured output
> (`describe_chart --output-mode csv|json`, `describe_diagram --output-mode
> mermaid`). Inference and downloads stream progress (CLI spinner + MCP
> `notifications/progress`). The catalog routes each tool to its benchmark-best
> model (`qwen3-vl-8b` for most, `qwen3.5-4b-q8` for code/UI/diagram/error; the
> 27B champion and the MoE are opt-in via `--model`). Model files no longer
> re-download on every model switch.

---

## What it is

`localvision` exposes a small set of vision tools over the standard
MCP protocol:

- `read_image` — describe an image in natural language
- `read_document` — rasterize a PDF and summarize it (needs a rasterizer on `$PATH`)
- `extract_text` — OCR / transcription
- `extract_code` — extract source code from a screenshot
- `extract_table` — structured table extraction
- `describe_ui` — describe a UI for accessibility or replay
- `describe_diagram` — explain an architecture or flow diagram
- `describe_chart` — read a chart back as data
- `diagnose_error` — explain an error dialog or stack trace
- `image_to_prompt` — generate a text-to-image prompt to recreate the image
- `compare_images` — diff two images

Each tool takes a local file path (preferred), a `data:` URI, or a
`file://` URI. Remote `http(s)://` URLs are **rejected** because the
underlying `llama-server` is localhost-only.

Behind the scenes, the server spawns `llama-server` (from `llama.cpp`),
loads the smallest fitting model from a built-in catalog, runs inference,
and tears the subprocess down after a 5-minute idle timer.

## Install

macOS (Apple Silicon/Intel), Linux, and Windows are all supported. Three options
(Windows users: use Option 3, `go install`).

### Option 1 — Homebrew (recommended)

```bash
brew tap froggeric/homebrew-tap
brew trust froggeric/tap          # one-time: current Homebrew requires trusting new taps
brew install localvision
```

### Option 2 — `curl | sh`

If you do not have Homebrew, use the installer that ships with each GitHub
release:

```bash
curl -fsSL https://github.com/froggeric/llm/releases/latest/download/install.sh | bash
```

The installer (source: [`scripts/install.sh`](./scripts/install.sh)):

1. Detects your OS and architecture via `uname`.
2. Supports darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 (tar.gz).
   Windows is detected and the user is pointed to `go install` (Option 3),
   since native-Windows zip+exe handling isn't in the bash installer.
3. Downloads the release tarball from GitHub Releases.
4. Verifies the SHA256 sidecar.
5. Extracts `localvision` into `/usr/local/bin` if you have `sudo`,
   otherwise `~/.local/bin`.

### Option 3 — `go install`

If you have the Go toolchain:

```bash
go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest
```

The resulting binary lands in `$(go env GOPATH)/bin`. Make sure that
directory is on your `PATH`.

### What happens on first run

Run `localvision setup` (it also starts automatically on `localvision` with no
args in a terminal) to pick a model and write `~/.localvision/config.toml`. Then
the first real query will:

1. Download a pinned `llama-server` build (≈ 80 MB) to
   `~/.localvision/bin/` and verify its SHA256 — unless a `llama-server` is
   already on your `$PATH` (e.g. `brew install llama.cpp`), which is used as-is.
2. Download the model you selected (or the smallest that fits your hardware)
   from `huggingface.co/froggeric/`.
3. Run the inference. In MCP mode, `tools/list` keeps working while this
   happens; tools are marked unavailable until the model is ready.

This first-run download takes 3–15 minutes on a fast connection. You can
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

## Standalone CLI

`localvision` is also a normal shell tool — you don't need an MCP client. Run a
one-shot query directly:

```bash
localvision setup                          # first run: pick a model, write config
localvision img.png                        # describe an image
localvision shot.png --type ocr            # extract text (OCR)
localvision err.png  --type error          # diagnose an error/stack trace
localvision chart.png --type chart --output-mode csv      # chart → CSV
localvision chart.png --type chart --output-mode json | jq .
localvision arch.png --type diagram --output-mode mermaid # diagram → Mermaid
localvision paper.pdf --type doc           # summarize a PDF
localvision a.png b.png --type compare     # diff two images
localvision *.png --type ocr --output-dir out/ --format json --meta
```

`--type` selects one of eleven tools (`ocr|code|table|ui|diagram|chart|error|prompt|
doc|compare|describe`). Inputs may be file paths, globs, directories (`--recursive`),
or `-` for stdin (`find . -name '*.png' | localvision - --type ocr`).
`--model` overrides the auto-selected model; `--output`/`--output-dir` write
results to files; `--meta` writes a `.json` telemetry sidecar per output.
`--output-mode` requests a structured representation from `chart` (`csv`/`json`)
or `diagram` (`mermaid`); default `prose`.

With no arguments, `localvision` runs `setup` in a terminal and the MCP server
when invoked over a pipe (i.e. by an MCP client). Run `localvision --help` for
the full flag reference.

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

This is documented as a known limitation; a future release may add automatic
coordination via Ollama's API (ROADMAP Theme E5).

## Roadmap

**v0.7.0** adds **per-tool model routing**
(each tool → its benchmark-best model: `qwen3.5-4b-q8` for code/UI/diagram/error,
`qwen3-vl-8b` for the rest; the MoE opt-in for `read_image` coverage), two new
mirrored models, `[tools.<id>]` per-tool config (model + sampling method), a
setup-wizard routing prompt, and the **multi-sample consensus** scaffold
(`--sample N` / `method = "union@N"`, off by default). **v0.6.0** shipped tools
& UX: an 11th tool `read_document` (PDF ingestion via a `$PATH`-discovered
rasterizer chain), structured chart output (`describe_chart --output-mode
csv|json`) and diagram markup (`describe_diagram --output-mode mermaid`), and
**streaming progress** for inference + downloads (CLI spinner + MCP
`notifications/progress`; real token-by-token SSE is deferred). **v0.5.x** added
`image_to_prompt`, an MCP temp-file leak fix, `qwen3-vl-8b` as the default, and a
per-model cache. Earlier: **v0.4.0** (cross-platform) and **v0.3.0** (the
standalone CLI). Next: **constrained decoding (GBNF)** + **SSE output
streaming**, then **v0.8+ new modalities & a native GUI** (UI→code, video,
grounding). A background daemon / HTTP service was evaluated and **declined**.
Every item, its effort, and its target is in
[`ROADMAP.md`](./ROADMAP.md) (themes A–H,
prioritized into release tiers).

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
