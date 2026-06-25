# froggeric/llm

**Give your local AI coding agent vision and web search — fully on your own
machine. No cloud, no API keys, no telemetry.**

A monorepo of on-device, privacy-first tools for working with LLMs and coding
agents (Claude Code, Codex CLI, Gemini CLI, Cursor, Windsurf, and friends): MCP
servers that add capabilities text-only models lack, drop-in Claude Code skills,
a curated prompt library, and the open-weights VLM benchmark that backs the model
choices.

![release](https://img.shields.io/github/v/release/froggeric/llm?color=brightgreen)
![platforms](https://img.shields.io/badge/platforms-mac%2Flinux%2Fwin-blue)
![license](https://img.shields.io/badge/license-PolyForm%20NC-orange)

---

## 🔭 localvision — vision for text-only coding LLMs

The headline tool. `localvision` is a Go MCP server that wraps a local
`llama.cpp` subprocess so **text-only** coding agents — Claude Code, Codex CLI,
Gemini CLI free tier, non-vision Claude variants — can finally **see images**.

- **Private by design.** Images are processed on-device; `llama-server` binds to
  `127.0.0.1` only. No telemetry, ever. The only outbound traffic is the one-time
  model download from `huggingface.co/froggeric/`.
- **11 task-tuned tools** — OCR, code/UI/diagram/chart extraction, PDF ingestion,
  image-to-prompt, image diff — each routed to its benchmark-best model.
- **Cross-platform:** macOS (Apple Silicon/Intel), Linux, Windows (x86_64 + arm64).
- **Use it two ways** — as a one-shot shell command *or* an MCP server.

**Install:**

```bash
# macOS (Homebrew)
brew tap froggeric/homebrew-tap && brew trust froggeric/tap && brew install localvision

# any OS, no Homebrew:
curl -fsSL https://github.com/froggeric/llm/releases/latest/download/install.sh | bash

# or with Go:
go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest
```

**Use it from Claude Code** (or any MCP client):

```json
{ "mcpServers": { "localvision": { "command": "localvision", "args": ["run"] } } }
```

…then ask: *"Use the `read_image` tool to describe ./screenshot.png."*
Or one-shot from the shell: `localvision img.png --type ocr`.

➡️ Full docs, latency expectations, and the model catalog:
[`mcp/localvision/README.md`](./mcp/localvision/README.md)

---

## 🌐 SearXNG MCP — free, unlimited web search for your AI tools

A local [SearXNG](https://searxng.org) metasearch + MCP server that gives your
coding agent **web search and URL reading** — free, unlimited, no API key, no
Docker. One setup script auto-detects your tools (Claude Code, Gemini CLI,
Cursor, Windsurf, Claude Desktop, iFlow, Qwen Code, OpenCode) and wires in the
MCP server; a macOS launchd job keeps it running.

➡️ [`mcp/searxng/readme.md`](./mcp/searxng/readme.md)

---

## 🧠 Claude Code skills

Drop-in skills for Claude Code (copy a folder into your skills directory):

- **`prompt-enhancer`** — turns a vague request (*"fix the bug"*, *"make it
  better"*) into a precise, executable prompt before acting.
- **`intelligent-versioning`** — release prep on autopilot: semantic-version
  bumps, changelog, and documentation sync.
- **`agentic-translation`** — an agentic workflow for translating documents of
  5K+ words with consistency across passes.

➡️ [`claude/skills/`](./claude/skills/)

---

## 📝 Prompt library

Curated, shell-centric prompt templates for coding tasks — a prompt-engineer
meta-prompt, an introspection step, a competitive code-review prompt, two
"Elements of Style" editions, and a web-project starter. Copy-paste ready and
easy to combine with shell scripts and CLI clients.

➡️ [`prompts/coding/`](./prompts/coding/)

---

## 📊 VLM benchmark (for researchers)

The open-weights Vision-Language Model benchmark used to choose `localvision`'s
catalog: **30 images × 15 model variants × 3 runs** at Q4_K_M, plus a Q8_0
comparison. If you're picking a local vision model for your own hardware, the
ranking, per-model analysis, and hardware-tier recommendations are a great
starting point.

➡️ [`benchmark/vlm/BENCHMARK-REPORT-v5.md`](./benchmark/vlm/BENCHMARK-REPORT-v5.md)
· [`SUMMARY.md`](./benchmark/vlm/SUMMARY.md)

---

## License & privacy

- **Source-available under [PolyForm Noncommercial 1.0.0](./LICENSE)** — read,
  modify, and redistribute for non-commercial use; commercial use requires a
  separate license. The Claude skills and prompt templates are provided as-is for
  you to adapt.
- **Everything runs on your machine.** No telemetry; the only outbound traffic is
  the one-time model/binary download over HTTPS to `huggingface.co/froggeric/`.
  `llama-server` binds to `127.0.0.1` only.

---

## Repository layout

```
froggeric/llm
├── mcp/
│   ├── localvision/   # vision MCP server (the flagship) — Go, 6 platforms
│   └── searxng/       # local web-search MCP server
├── claude/skills/     # Claude Code skills: prompt-enhancer, versioning, translation
├── prompts/coding/    # curated prompt templates
└── benchmark/vlm/     # open-weights VLM benchmark + reports
```
