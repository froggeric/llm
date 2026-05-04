# SearXNG + MCP Setup Guide

Free, unlimited web search and URL reading for AI coding tools. No Docker needed.

---

## What You'll End Up With

- A local SearXNG metasearch engine running on your Mac
- SearXNG MCP server configured in your AI coding tools
- Auto-start via macOS launchd

## Supported AI Coding Tools

The setup script auto-detects installed tools and offers to configure the MCP server:

| Tool | Config file | Detection |
|------|------------|-----------|
| Claude Code | `~/.claude.json` | `claude` command |
| Gemini CLI | `~/.gemini/settings.json` | `gemini` command |
| iFlow | `~/.iflow/settings.json` | `iflow` command |
| Qwen Code | `~/.qwen/settings.json` | `qwen` command |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | config directory |
| Cursor | `~/.cursor/mcp.json` | `cursor` command |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `windsurf` command |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode` command |

## Prerequisites

Before starting, verify you have these installed:

```bash
python3 --version    # needs 3.10+
node --version       # needs 18+
git --version
```

If missing: `brew install python node git`

---

## Step 1: Install & Configure SearXNG

### 1.1 Clone and set up (one command)

```bash
git clone https://github.com/searxng/searxng.git ~/searxng
cd ~/searxng
python3 searxng-setup-local-mcp.py
```

That's it. The script handles everything:

- Creates a virtual environment and installs dependencies
- Applies all configuration changes (secret key, JSON API, engine fixes, optimizations)
- Installs the macOS launchd service for auto-start
- Detects installed AI coding tools and offers to configure MCP for each
- Starts SearXNG and runs automated tests

You should see:

```
All tests passed. SearXNG is ready.

MCP server URL: http://127.0.0.1:8888
Service: installed (auto-starts at login)
```

### What the script does

**Required fixes:**
- Changes the default `secret_key` (SearXNG won't start without this)
- Enables the JSON API (required for the MCP server)
- Sets Tor-only engines (ahmia, torch) to `inactive` — they can't work without a Tor proxy
- Disables karmasearch engines (they return HTTP 403)
- Fixes the wikidata engine bug ([#5982](https://github.com/searxng/searxng/issues/5982)) — `KeyError: 'name'` at startup
- Creates `/etc/searxng/limiter.toml` to silence a startup warning

**MCP optimizations:**
- HTTP/1.1 keep-alive (faster repeated API calls)
- Caps `max_request_timeout` at 10s (prevents slow engines from blocking)
- Sets `base_url` for correct URL generation
- Disables `ahmia_filter` plugin (unnecessary without Tor)
- Reduces engine suspension times from days to hours
- Disables niche engines (torrents, recipes, radio, etc.) for faster queries

**Service:**
- Installs a macOS launchd agent that auto-starts SearXNG at login
- Logs go to `/tmp/searxng.log` and `/tmp/searxng.err`

**MCP auto-detection:**
- Scans for installed AI coding tools (Claude Code, Gemini CLI, iFlow, Qwen Code, Cursor, Windsurf, OpenCode, Claude Desktop)
- For each detected tool, asks if you want to install the SearXNG MCP server
- Writes the correct config format for each tool (OpenCode uses a different format)
- Skips tools that already have the SearXNG config

The script is idempotent — safe to run multiple times. It restores modified files from git first, then re-applies config. It also handles updates: if the repo is behind upstream, it pulls new commits and re-applies config.

### CLI flags

```bash
python3 searxng-setup-local-mcp.py              # full install + configure + test
python3 searxng-setup-local-mcp.py --test       # just run tests
python3 searxng-setup-local-mcp.py --uninstall  # remove service + MCP configs, keep repo
```

### To undo all changes

```bash
git checkout -- searx/settings.yml searx/engines/wikidata.py
```

> **Note:** The wikidata fix will be overwritten by `git pull`. Re-run `python3 searxng-setup-local-mcp.py` after updating SearXNG until [PR #5993](https://github.com/searxng/searxng/pull/5993) is merged.

---

## Step 2: Manual MCP Setup (if not using the script)

If you prefer to configure MCP manually or the script didn't detect your tool:

### Claude Code

```bash
claude mcp add searxng -s user -e SEARXNG_URL=http://localhost:8888 -- npx -y mcp-searxng
```

### Other tools

Edit the config file listed in the table above. Add a `"searxng"` entry under the `"mcpServers"` key (or `"mcp"` for OpenCode):

```json
"searxng": {
  "command": "npx",
  "args": ["-y", "mcp-searxng"],
  "env": {
    "SEARXNG_URL": "http://localhost:8888"
  }
}
```

For OpenCode, use this format instead:

```json
"searxng": {
  "type": "local",
  "command": ["npx", "-y", "mcp-searxng"],
  "environment": {
    "SEARXNG_URL": "http://localhost:8888"
  },
  "enabled": true
}
```

### Optional: MCP Fetch (not recommended)

`mcp-server-fetch` is a bare HTTP client that reads URLs directly. You generally don't need it alongside SearXNG because:

- **Bot evasion:** mcp-server-fetch makes plain HTTP requests that get blocked by Cloudflare and similar services. SearXNG's URL reader goes through its anti-bot pipeline (UA rotation, TLS fingerprinting, browser headers) — it can fetch from sites that block bare clients.
- **Structured reading:** SearXNG's reader supports extracting by section heading, paragraph range, or document outline — mcp-server-fetch only does basic HTML-to-markdown.
- **Search:** mcp-server-fetch can't search the web at all.

The one advantage of mcp-server-fetch is that it works without SearXNG running. If you want it as a minimal fallback:

```bash
claude mcp add mcp-server-fetch -s user -- uvx mcp-server-fetch
```

### Restart your tool

Quit and relaunch your AI coding tool for the new MCP server to load.

---

## Step 3: Verify

In a new session of your AI coding tool, try:

- **Search:** Ask it to "search the web for Claude Code MCP servers"
- **Read a URL:** Ask it to "fetch the content of https://httpbin.org/html"

If both work, you're done.

---

## Updating SearXNG

To update to the latest version:

```bash
cd ~/searxng
python3 searxng-setup-local-mcp.py
```

The script automatically pulls new commits from GitHub and re-applies all configuration.

---

## Troubleshooting

**SearXNG won't start:**
- Make sure port 8888 is free: `lsof -i :8888`
- Check logs: `cat /tmp/searxng.err`
- If you see `Address already in use`: `kill $(lsof -ti :8888)`

**MCP server not appearing in your tool:**
- Check the correct config file for your tool (see table above)
- Verify the JSON is valid: `python3 -c "import json; json.load(open('CONFIG_FILE'))"`
- Make sure SearXNG is running before starting your tool

**Search returns no results:**
- Verify SearXNG is running: `curl "http://localhost:8888/search?q=test&format=json"`
- Check that `json` is in the `formats` list in `searx/settings.yml`

**`X-Forwarded-For nor X-Real-IP header is set!` in logs:**
- Expected when running without a reverse proxy. Harmless for local development.

**Engine errors in logs (CAPTCHA, timeouts, rate limits):**
- These are normal for a local SearXNG. External engines (Google, DuckDuckGo, etc.) may temporarily rate-limit or block requests. Results will still be returned from other engines.

**Wikidata engine still crashes after git pull:**
- Re-run `python3 searxng-setup-local-mcp.py` to re-apply the fix until [PR #5993](https://github.com/searxng/searxng/pull/5993) is merged.

**To stop auto-starting:**
```bash
python3 searxng-setup-local-mcp.py --uninstall
```

---

## Bot Detection Evasion

SearXNG includes built-in measures to avoid being blocked by search engines. These run automatically — no configuration needed:

- **User-Agent rotation** — Each outgoing request gets a random Firefox UA (general engines) or a random Android Chrome UA from a pool of 2,285 real device strings (Google specifically)
- **TLS fingerprint randomization** — Cipher suite order is shuffled on every new connection, bypassing JA3/JA4 fingerprinting used by Cloudflare and others
- **Browser-like headers** — Requests include realistic `Accept-Encoding`, `Accept-Language`, `Sec-Fetch-*`, `DNT`, and `Referer` headers
- **HTTP/2 by default** — Modern protocol that matches real browser traffic patterns
- **Engine auto-suspension** — Engines that return CAPTCHAs or rate-limit errors (429/403) are automatically suspended, with shorter recovery times configured for local use

For a single-user local instance, these measures are sufficient. The binding constraint is the single IP address — if you need more, you can add rotating proxies in `searx/settings.yml` under `outgoing.proxies`, but this requires an external proxy service.
