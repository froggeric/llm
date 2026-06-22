# froggeric/llm

Prompts, skills, and shell-first workflows for working with large language models (LLMs) and agentic frameworks such as Claude Code, Gemini CLI, cline, and Kilo Code.

---

About
This repository contains curated prompt templates, small shell scripts, and recommended workflows for integrating LLMs and agentic tools into CLI-based pipelines. Files are intentionally shell-centric so you can combine them easily with shell scripts, environment variables, and CLI clients.

Repository structure
- claude/ — Claude-specific prompts, scripts, and example workflows (Anthropic)
  https://github.com/froggeric/llm/tree/main/claude
- prompts/ — generic and categorized prompt templates and examples for different tasks and agents
  https://github.com/froggeric/llm/tree/main/prompts

---

## MCP servers

- mcp/localvision/ — a Go MCP server that wraps a local `llama.cpp` subprocess to give text-only coding LLMs (Claude Code, Codex CLI, etc.) vision capabilities. Runs entirely on-device; PolyForm Noncommercial license. See [mcp/localvision/README.md](./mcp/localvision/README.md).
