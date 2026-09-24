# SearXNG + MCP Setup Guide

Free web search and URL reading for your AI coding tools. One private SearXNG instance on your Mac — no Docker, no API keys, no search quotas — kept current by automatic daily updates.

## Quick Start

**1. Install** — one command, about two minutes:

```bash
python3 searxng-setup-local-mcp.py
```

The installer clones SearXNG to `~/searxng`, configures it, starts it, and offers to register the MCP server in each coding tool it finds.

When it finishes, you should see:

```
All tests passed. SearXNG is ready.

MCP server URL: http://127.0.0.1:8888
Service: installed (auto-starts at login)
Auto-update: installed (daily)
```

**2. Restart your coding tools** so they load the new MCP server.

**3. Verify** — in a new session, ask your tool to:

- "search the web for Claude Code MCP servers" → it returns results
- "fetch the content of https://httpbin.org/html" → it returns the page

Both work? You're done.

### Prerequisites

```bash
python3 --version    # 3.10 or newer
node --version       # 18 or newer
git --version
```

Install any that are missing: `brew install python node git`

## What the Installer Sets Up

| Piece | Detail |
|---|---|
| SearXNG instance | Cloned to `~/searxng`, serving `http://127.0.0.1:8888` |
| Service | launchd agent `com.searxng` — starts at login, restarts after a crash |
| Daily updates | launchd job `com.searxng.update` — checks upstream every day at 10:42 |
| MCP config | Registered in every coding tool it detects (see table below) |
| Logs | Service: `/tmp/searxng.log`, `/tmp/searxng.err` · Updates: `/tmp/searxng-update.log` |

Configuration changes it applies (and re-applies after every update):

- **Required fixes** — replaces the default `secret_key` (SearXNG refuses to start without this), enables the JSON API the MCP server needs, disables Tor-only and karmasearch engines (they fail without extra setup), patches the wikidata engine bug ([#5982](https://github.com/searxng/searxng/issues/5982)), and creates `/etc/searxng/limiter.toml`
- **Speed and reliability** — HTTP/1.1 keep-alive, a 10 s cap on slow engines, correct `base_url`, shorter engine suspension times (hours instead of days), and niche engines (torrents, recipes, radio) turned off

The script is idempotent — run it as many times as you like. It restores the files it manages from git, then re-applies its changes, so the result is always clean.

## Daily Updates (Automatic)

SearXNG searches by scraping Google, Bing, DuckDuckGo, and others. When those sites change, scrapers break — an outdated install quietly loses results. The update job keeps this from happening.

Every day at 10:42 (or on wake, if the Mac was asleep), the job:

1. Checks upstream for new commits — and exits if there are none
2. Pulls them, reinstalls dependencies, re-applies all local configuration
3. Restarts the service (searches pause for ~5–10 s)
4. Runs the test suite — and **rolls back** to the previous version if any test fails, then retries the next day

Check the last run:

```bash
tail /tmp/searxng-update.log
```

Run an update on demand:

```bash
python3 searxng-setup-local-mcp.py        # from anywhere; also offers MCP setup
```

Disable automatic updates:

```bash
launchctl unload ~/Library/LaunchAgents/com.searxng.update.plist
```

## Supported AI Coding Tools

The installer detects these and offers to configure the MCP server in each:

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

## Manual MCP Setup

Skip this if the installer already configured your tool. Otherwise:

**Claude Code:**

```bash
claude mcp add searxng -s user -e SEARXNG_URL=http://localhost:8888 -- npx -y mcp-searxng
```

**Most other tools** — add a `"searxng"` entry under `"mcpServers"` in the config file listed above:

```json
"searxng": {
  "command": "npx",
  "args": ["-y", "mcp-searxng"],
  "env": {
    "SEARXNG_URL": "http://localhost:8888"
  }
}
```

**OpenCode** uses a different format:

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

Restart the tool afterward.

### About MCP Fetch (not needed)

You do not need `mcp-server-fetch` alongside SearXNG:

- SearXNG's URL reader goes through its anti-bot pipeline (rotating user agents, browser headers, TLS randomization) and reaches sites that block bare HTTP clients
- Its reader extracts by section, paragraph range, or document outline — not just raw HTML-to-markdown
- It also searches; `mcp-server-fetch` cannot

`mcp-server-fetch` has one advantage: it works when SearXNG is down. To add it anyway: `claude mcp add mcp-server-fetch -s user -- uvx mcp-server-fetch`

## Commands

```bash
python3 searxng-setup-local-mcp.py                  # full install + configure + test
python3 searxng-setup-local-mcp.py --auto-update    # non-interactive update check (what the daily job runs)
python3 searxng-setup-local-mcp.py --test           # just run tests
python3 searxng-setup-local-mcp.py --uninstall      # remove service, update job, and MCP configs
```

## Troubleshooting

**SearXNG won't start**

- Check the port: `lsof -i :8888` — if stale, `kill $(lsof -ti :8888)`
- Read the log: `cat /tmp/searxng.err`

**MCP server missing from your tool**

- Confirm you edited the right config file (see the table above)
- Validate the JSON: `python3 -c "import json; json.load(open('CONFIG_FILE'))"`
- Start SearXNG before launching your tool

**Search returns no results**

Check what the engines say:

```bash
curl "http://localhost:8888/search?q=test&format=json"
```

- Confirm `json` appears in the `formats` list in `searx/settings.yml`
- Read `unresponsive_engines` in the response:
  - `CAPTCHA` or `too many requests` — those engines temporarily blocked your IP. This clears by itself; the other engines keep working
  - Many engines failing with parse errors, or returning zero results silently — the scrapers are stale. Check the last update (`tail /tmp/searxng-update.log`) and run the installer

**`X-Forwarded-For nor X-Real-IP header is set!` in the logs**

Expected without a reverse proxy. Harmless for local use.

**Engine errors in the logs (CAPTCHA, timeouts, rate limits)**

Normal for a local SearXNG. External engines throttle individual IPs from time to time; the rest cover for them.

**An update left things broken**

Updates that fail their tests roll back on their own. If the service still misbehaves, run the installer from a terminal — it rebuilds the configuration from scratch.

## How SearXNG Avoids Getting Blocked

These measures run automatically:

- **User-agent rotation** — every request carries a random Firefox user agent (Google requests draw from a pool of 2,285 real Android Chrome device strings)
- **TLS fingerprint randomization** — cipher-suite order shuffles per connection, defeating the JA3/JA4 fingerprinting behind Cloudflare and similar services
- **Browser-like headers** — realistic `Accept-Encoding`, `Accept-Language`, `Sec-Fetch-*`, `DNT`, and `Referer` values
- **HTTP/2** — traffic resembles a modern browser's
- **Auto-suspension** — engines that serve CAPTCHAs or 429/403s pause for a while (configured in hours, not days) instead of hammering back

For a single-user local instance, these suffice. The binding constraint is your single IP address. To go further, add rotating proxies under `outgoing.proxies` in `searx/settings.yml` — this requires an external proxy service.

## Uninstall

```bash
python3 searxng-setup-local-mcp.py --uninstall
```

This removes the service, the daily update job, and the MCP configs, and keeps the repo at `~/searxng`. To undo only the configuration changes: `cd ~/searxng && git checkout -- searx/settings.yml searx/engines/wikidata.py`

To remove everything: `rm -rf ~/searxng`
