#!/usr/bin/env python3
"""SearXNG local MCP setup — install, configure, and test.

Run from anywhere:
    python3 searxng-setup-local-mcp.py          # full install + configure + test
    python3 searxng-setup-local-mcp.py --test   # just run tests
    python3 searxng-setup-local-mcp.py --uninstall  # remove service, keep repo

Idempotent — safe to run multiple times. Restores modified files from git
before re-applying config, so it always produces a clean result.
"""

import json
import os
import platform
import re
import shutil
import subprocess
import sys
import time
import urllib.request
import urllib.error

# ── Configuration ──────────────────────────────────────────────────────────

SEARXNG_DIR = os.path.expanduser("~/searxng")
REPO_URL = "https://github.com/searxng/searxng.git"
SEARXNG_PORT = 8888
SEARXNG_URL = f"http://127.0.0.1:{SEARXNG_PORT}"
LAUNCHD_LABEL = "com.searxng"
LAUNCHD_PLIST = os.path.expanduser(f"~/Library/LaunchAgents/{LAUNCHD_LABEL}.plist")
SETTINGS_FILE = "searx/settings.yml"
WIKIDATA_FILE = "searx/engines/wikidata.py"

# Standard MCP server config (most harnesses use this format)
_STDIO_MCP_CONFIG = {
    "command": "npx",
    "args": ["-y", "mcp-searxng"],
    "env": {"SEARXNG_URL": SEARXNG_URL},
}

MCP_HARNESSES = [
    # ── CLI tools (detected by command) ──
    {
        "name": "Claude Code",
        "config_file": "~/.claude.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["claude"],
        "detect_paths": [],
    },
    {
        "name": "Gemini CLI",
        "config_file": "~/.gemini/settings.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["gemini"],
        "detect_paths": [],
    },
    {
        "name": "iFlow",
        "config_file": "~/.iflow/settings.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["iflow"],
        "detect_paths": [],
    },
    {
        "name": "Qwen Code",
        "config_file": "~/.qwen/settings.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["qwen"],
        "detect_paths": [],
    },
    # ── Desktop/IDE tools (detected by config directory) ──
    {
        "name": "Claude Desktop",
        "config_file": "~/Library/Application Support/Claude/claude_desktop_config.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": [],
        "detect_paths": ["~/Library/Application Support/Claude"],
    },
    {
        "name": "Cursor",
        "config_file": "~/.cursor/mcp.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["cursor"],
        "detect_paths": ["~/.cursor"],
    },
    {
        "name": "Windsurf",
        "config_file": "~/.codeium/windsurf/mcp_config.json",
        "mcp_key": "mcpServers",
        "server_config": _STDIO_MCP_CONFIG,
        "detect_cmds": ["windsurf"],
        "detect_paths": ["~/.codeium/windsurf"],
    },
    # OpenCode uses a different format: command as array, "environment" not "env", "type"/"enabled" required
    {
        "name": "OpenCode",
        "config_file": "~/.config/opencode/opencode.json",
        "mcp_key": "mcp",
        "server_config": {
            "type": "local",
            "command": ["npx", "-y", "mcp-searxng"],
            "environment": {"SEARXNG_URL": SEARXNG_URL},
            "enabled": True,
        },
        "detect_cmds": ["opencode"],
        "detect_paths": [],
    },
]


# ── Helpers ────────────────────────────────────────────────────────────────

def run(cmd, **kwargs):
    """Run a command, return CompletedProcess. Prints on failure."""
    r = subprocess.run(cmd, capture_output=True, text=True, **kwargs)
    return r

def run_ok(cmd, **kwargs):
    """Run a command, return True on success."""
    return run(cmd, **kwargs).returncode == 0

def banner(text):
    print(f"\n{'='*60}")
    print(f"  {text}")
    print(f"{'='*60}\n")

def section(text):
    print(f"\n── {text} ──")

def ok(text):
    print(f"  ok  {text}")

def skip(text):
    print(f"  --  {text} (skipped)")

def warn(text):
    print(f"  !!  {text}")

def changed(text):
    print(f"  >>  {text}")


# ── Step 1: Prerequisites ─────────────────────────────────────────────────

def check_prerequisites():
    section("Checking prerequisites")

    required = {
        "python3": "3.10",
        "node": "18",
        "git": "2",
    }
    optional = {
        "uv": None,
    }
    missing = []

    for cmd, min_ver in required.items():
        r = run([cmd, "--version"])
        if r.returncode != 0:
            missing.append(cmd)
            warn(f"{cmd} not found")
        else:
            ver_line = (r.stdout or r.stderr).strip().split()[-1]
            ok(f"{cmd} {ver_line}")

    for cmd, _ in optional.items():
        if shutil.which(cmd):
            ok(f"{cmd} found")
        else:
            skip(f"{cmd} not found (optional, for MCP Fetch)")

    if missing:
        warn(f"Missing: {', '.join(missing)}")
        warn("Install with: brew install " + " ".join(missing))
        return False
    return True


# ── Step 2: Install / Update ──────────────────────────────────────────────

def install_or_update():
    """Clone if needed, pull if behind, set up venv."""

    # 2a: Clone
    if not os.path.isdir(os.path.join(SEARXNG_DIR, ".git")):
        section("Cloning SearXNG repository")
        r = run(["git", "clone", REPO_URL, SEARXNG_DIR])
        if r.returncode != 0:
            warn(f"git clone failed: {r.stderr}")
            return False
        ok(f"Cloned to {SEARXNG_DIR}")
    else:
        section("Updating SearXNG repository")
        r = run(["git", "fetch", "origin"], cwd=SEARXNG_DIR)
        if r.returncode != 0:
            warn(f"git fetch failed: {r.stderr}")
            # non-fatal, continue with local copy

        # Check if behind
        r = run(["git", "rev-list", "--count", "HEAD..origin/master"], cwd=SEARXNG_DIR)
        if r.returncode == 0 and r.stdout.strip():
            behind = int(r.stdout.strip())
            if behind > 0:
                # Stash any local changes, pull, then we'll re-apply config
                run(["git", "stash"], cwd=SEARXNG_DIR)
                r2 = run(["git", "pull", "origin", "master"], cwd=SEARXNG_DIR)
                if r2.returncode == 0:
                    changed(f"Updated: {behind} new commit(s) pulled from upstream")
                else:
                    warn(f"git pull failed: {r2.stderr}")
            else:
                ok("Already up to date")
        else:
            ok("Could not check remote (offline?)")

    # 2b: Venv + dependencies
    section("Setting up virtual environment")
    venv_dir = os.path.join(SEARXNG_DIR, "venv")
    venv_python = os.path.join(venv_dir, "bin", "python")
    venv_pip = os.path.join(venv_dir, "bin", "pip")

    if not os.path.isfile(venv_python):
        changed("Creating virtual environment...")
        r = run(["python3", "-m", "venv", venv_dir], cwd=SEARXNG_DIR)
        if r.returncode != 0:
            warn(f"venv creation failed: {r.stderr}")
            return False
    else:
        skip("venv already exists")

    # Upgrade pip (always — cheap and avoids version issues)
    r = run([venv_python, "-m", "pip", "install", "-q", "--upgrade", "pip"])
    if r.returncode == 0:
        ok("pip up to date")
    else:
        warn(f"pip upgrade had issues: {r.stderr}")

    # Install requirements (always — pip skips already-installed packages)
    changed("Installing/updating dependencies...")
    r = run([venv_pip, "install", "-q", "-r", "requirements.txt"], cwd=SEARXNG_DIR)
    if r.returncode != 0:
        warn(f"pip install failed: {r.stderr}")
        return False
    ok("Dependencies installed")

    return True


# ── Step 3: Configure ─────────────────────────────────────────────────────

def read_file(path):
    with open(path, "r") as f:
        return f.read()

def write_file(path, content):
    with open(path, "w") as f:
        f.write(content)

def restore_from_git(path):
    """Restore original from git."""
    run(["git", "checkout", "--", path], cwd=SEARXNG_DIR)

def fix_settings(content):
    changes = []

    # 1. Secret key
    if 'secret_key: "ultrasecretkey"' in content:
        content = content.replace('secret_key: "ultrasecretkey"', 'secret_key: "my-local-searxng-2026"')
        changes.append("secret_key changed from default")
    else:
        changes.append("secret_key already set")

    # 2. JSON API
    if re.search(r"formats:\s*\n\s+- html\s*\n\s+- json\b", content):
        changes.append("JSON API already enabled")
    elif "formats: [html, json]" in content:
        changes.append("JSON API already enabled")
    else:
        content = re.sub(
            r"(  formats:\s*\n    - html\s*)\n(\n|\z)",
            r"\1\n    - json\n\2",
            content
        )
        if re.search(r"formats:\s*\n\s+- html\s*\n\s+- json\b", content):
            changes.append("JSON API: added json to formats")
        else:
            changes.append("JSON API: could not find formats block")

    # 3. Tor engines — inactive (not disabled — disabled doesn't prevent loading)
    for name, sc in [("ahmia", "ah"), ("torch", "tch")]:
        content, c = set_engine_state(content, sc, name)
        changes.append(c)

    # 4. Karmasearch engines
    for sc in ["ka", "kai", "kav", "kan"]:
        content, c = set_engine_state(content, sc, f"karmasearch ({sc})")
        changes.append(c)

    # 5. HTTP/1.1
    if 'http_protocol_version: "1.0"' in content:
        content = content.replace('http_protocol_version: "1.0"', 'http_protocol_version: "1.1"')
        changes.append("http_protocol_version: 1.0 -> 1.1")
    else:
        changes.append("http_protocol_version already 1.1")

    # 6. Max request timeout
    if "# max_request_timeout: 10.0" in content:
        content = content.replace("# max_request_timeout: 10.0", "max_request_timeout: 10.0")
        changes.append("max_request_timeout: 10.0 (uncommented)")
    else:
        changes.append("max_request_timeout already set")

    # 7. Base URL
    for old in [
        'base_url: false  # "http://example.com/location"',
        "base_url: false",
    ]:
        if old in content:
            content = content.replace(old, 'base_url: "http://127.0.0.1:8888"', 1)
            changes.append("base_url set to http://127.0.0.1:8888")
            break
    else:
        if 'base_url: "http://127.0.0.1:8888"' in content:
            changes.append("base_url already set")
        else:
            changes.append("base_url: no matching pattern")

    # 8. Disable ahmia_filter plugin
    if "searx.plugins.ahmia_filter.SXNGPlugin:\n    active: true" in content:
        content = content.replace(
            "searx.plugins.ahmia_filter.SXNGPlugin:\n    active: true",
            "searx.plugins.ahmia_filter.SXNGPlugin:\n    active: false"
        )
        changes.append("ahmia_filter plugin: disabled")
    else:
        changes.append("ahmia_filter plugin: already disabled")

    # 9. Suspension times
    for old, new, desc in [
        ("cf_SearxEngineCaptcha: 1296000", "cf_SearxEngineCaptcha: 86400", "Cloudflare CAPTCHA: 15d -> 1d"),
        ("cf_SearxEngineAccessDenied: 86400", "cf_SearxEngineAccessDenied: 3600", "Cloudflare denied: 1d -> 1h"),
        ("recaptcha_SearxEngineCaptcha: 604800", "recaptcha_SearxEngineCaptcha: 86400", "reCAPTCHA: 7d -> 1d"),
    ]:
        if old in content:
            content = content.replace(old, new)
            changes.append(f"suspension: {desc}")

    # 10. Niche engines
    for sc, name in [
        ("kc", "kickass"), ("tpb", "piratebay"), ("solid", "solidtorrents"),
        ("bt4g", "bt4g"), ("chef", "chefkoch"), ("mc", "mixcloud"),
        ("od", "odysee"), ("rb", "radio browser"), ("pdb", "pdbe"),
        ("leco", "lemmy.ca"), ("leus", "lemmy.world"),
        ("lepo", "lemmy.ml"), ("lecom", "lemmy.dbzer0"),
    ]:
        content, _ = set_engine_state(content, sc, name)

    return content, changes

def set_engine_state(content, shortcut, name):
    """Set an engine to inactive: true. Returns (content, description)."""
    if re.search(rf"shortcut: {shortcut}\n\s+inactive: true", content):
        return content, f"{name} ({shortcut}): already inactive"
    if re.search(rf"shortcut: {shortcut}\n\s+disabled: true", content):
        content = re.sub(
            rf"(shortcut: {shortcut}\n)\s+disabled: true",
            r"\1    inactive: true",
            content
        )
        return content, f"{name} ({shortcut}): disabled -> inactive"
    content = re.sub(rf"(shortcut: {shortcut}\n)", rf"\1    inactive: true\n", content)
    return content, f"{name} ({shortcut}): set inactive"

def fix_wikidata(content):
    old = """\
    for result in jsonresponse.get('results', {}).get('bindings', {}):
        name = result['name']['value']
        lang = result['name']['xml:lang']"""

    new = """\
    for result in jsonresponse.get('results', {}).get('bindings', {}):
        name_field = result.get("name")
        if not name_field:
            continue
        name = result['name']['value']
        lang = result['name']['xml:lang']"""

    if old in content:
        content = content.replace(old, new)
        return content, "wikidata KeyError fix applied (#5982)"
    if "name_field = result.get" in content:
        return content, "wikidata fix already applied"
    return content, "wikidata: target block not found"

def configure():
    """Apply all configuration changes."""
    section("Configuring SearXNG")

    # Restore originals to get clean base
    restore_from_git(SETTINGS_FILE)
    restore_from_git(WIKIDATA_FILE)

    # settings.yml
    path = os.path.join(SEARXNG_DIR, SETTINGS_FILE)
    content = read_file(path)
    content, changes = fix_settings(content)
    write_file(path, content)
    for c in changes:
        changed(f"  {c}")

    # wikidata.py
    path = os.path.join(SEARXNG_DIR, WIKIDATA_FILE)
    content = read_file(path)
    content, desc = fix_wikidata(content)
    write_file(path, content)
    changed(f"  {desc}")

    # System config
    config_file = "/etc/searxng/limiter.toml"
    if os.path.exists(config_file):
        skip(f"{config_file} exists")
    else:
        changed(f"Creating {config_file} (may need sudo)")
        try:
            os.makedirs("/etc/searxng", exist_ok=True)
            open(config_file, "w").close()
            ok(f"Created {config_file}")
        except PermissionError:
            r = run(["sudo", "mkdir", "-p", "/etc/searxng"])
            r = run(["sudo", "touch", config_file])
            if r.returncode == 0:
                ok(f"Created {config_file}")
            else:
                warn(f"Could not create {config_file}: {r.stderr}")


# ── Step 4: macOS Service ─────────────────────────────────────────────────

def install_service():
    """Install or update the launchd plist for auto-start."""
    if platform.system() != "Darwin":
        skip("macOS service (not on macOS)")
        return

    section("Installing macOS launchd service")

    venv_python = os.path.join(SEARXNG_DIR, "venv", "bin", "python")
    plist_content = f"""<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{LAUNCHD_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{venv_python}</string>
        <string>-c</string>
        <string>from searx.webapp import run; run()</string>
    </array>
    <key>WorkingDirectory</key>
    <string>{SEARXNG_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/searxng.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/searxng.err</string>
</dict>
</plist>
"""

    # Unload existing service if loaded
    run(["launchctl", "unload", LAUNCHD_PLIST])

    # Write plist
    os.makedirs(os.path.dirname(LAUNCHD_PLIST), exist_ok=True)
    with open(LAUNCHD_PLIST, "w") as f:
        f.write(plist_content)
    changed(f"Written {LAUNCHD_PLIST}")

    # Load service
    r = run(["launchctl", "load", LAUNCHD_PLIST])
    if r.returncode == 0:
        ok("Service loaded (SearXNG auto-starts at login)")
    else:
        warn(f"launchctl load failed: {r.stderr}")


def uninstall_service():
    """Remove the launchd service."""
    if not os.path.exists(LAUNCHD_PLIST):
        skip("Service not installed")
        return

    section("Uninstalling macOS service")

    # Unload
    run(["launchctl", "unload", LAUNCHD_PLIST])
    changed("Service unloaded")

    # Remove plist
    os.remove(LAUNCHD_PLIST)
    changed(f"Removed {LAUNCHD_PLIST}")


# ── Step 5: Test ──────────────────────────────────────────────────────────

def wait_for_searxng(timeout=30):
    """Wait for SearXNG to respond. Returns True if ready."""
    start = time.time()
    while time.time() - start < timeout:
        try:
            resp = urllib.request.urlopen(
                f"{SEARXNG_URL}/search?q=test&format=json", timeout=3
            )
            if resp.status == 200:
                return True
        except (urllib.error.URLError, ConnectionRefusedError, OSError):
            pass
        time.sleep(1)
    return False

def is_searxng_running():
    """Check if SearXNG is already running."""
    try:
        resp = urllib.request.urlopen(f"{SEARXNG_URL}/search?q=health&format=json", timeout=3)
        return resp.status == 200
    except Exception:
        return False

def start_searxng_temp():
    """Start SearXNG in background for testing. Returns Popen or None."""
    venv_python = os.path.join(SEARXNG_DIR, "venv", "bin", "python")
    proc = subprocess.Popen(
        [venv_python, "-c", "from searx.webapp import run; run()"],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        cwd=SEARXNG_DIR,
    )
    return proc

def stop_searxng_temp(proc):
    """Stop a temp SearXNG process."""
    if proc and proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()

def test_installation():
    """Run automated tests. Returns True if all pass."""
    section("Running automated tests")

    started_ourselves = False
    proc = None
    all_passed = True

    # Test 1: SearXNG is running (or start it)
    if is_searxng_running():
        ok("SearXNG is already running")
    else:
        changed("Starting SearXNG for testing...")
        proc = start_searxng_temp()
        started_ourselves = True
        if wait_for_searxng(timeout=20):
            ok("SearXNG started successfully")
        else:
            warn("SearXNG failed to start within 20s")
            # Show last log lines
            err_file = "/tmp/searxng.err"
            if os.path.exists(err_file):
                with open(err_file) as f:
                    lines = f.readlines()
                    for line in lines[-10:]:
                        print(f"       {line.rstrip()}")
            all_passed = False

    if all_passed:
        # Test 2: JSON API works
        try:
            resp = urllib.request.urlopen(
                f"{SEARXNG_URL}/search?q=python+programming&format=json", timeout=15
            )
            data = json.loads(resp.read())
            n_results = len(data.get("results", []))
            if n_results > 0:
                ok(f"Search works: {n_results} results returned")
            else:
                warn("Search returned 0 results")
                all_passed = False
        except Exception as e:
            warn(f"Search test failed: {e}")
            all_passed = False

        # Test 3: No config/startup errors
        # Engine-level errors (rate limits, timeouts, parse failures) are
        # transient and expected — we only flag structural config errors.
        err_file = "/tmp/searxng.err"
        config_error_lines = []
        if os.path.exists(err_file):
            with open(err_file) as f:
                for line in f:
                    if "ERROR" not in line:
                        continue
                    # Only flag structural errors (missing config, broken imports)
                    if "Missing engine config attribute" in line:
                        config_error_lines.append(line.strip())
                    elif "Cannot load engine" in line:
                        config_error_lines.append(line.strip())
                    elif "ambiguous name" in line or "ambiguous shortcut" in line:
                        config_error_lines.append(line.strip())
        if config_error_lines:
            warn(f"Found {len(config_error_lines)} config ERROR(s):")
            for line in config_error_lines[:5]:
                print(f"       {line}")
            all_passed = False
        else:
            ok("No config errors in log")

        # Test 4: JSON format in response
        try:
            resp = urllib.request.urlopen(
                f"{SEARXNG_URL}/search?q=test&format=json", timeout=10
            )
            data = json.loads(resp.read())
            if "results" in data and "query" in data:
                ok("JSON API response structure valid")
            else:
                warn("JSON response missing expected keys")
                all_passed = False
        except Exception as e:
            warn(f"JSON API test failed: {e}")
            all_passed = False

        # Test 5: launchd service (macOS only)
        if platform.system() == "Darwin":
            if os.path.exists(LAUNCHD_PLIST):
                ok(f"launchd service installed at {LAUNCHD_PLIST}")
            else:
                warn("launchd service not installed")
                # Not a hard failure for testing

    # Cleanup
    if started_ourselves and proc:
        stop_searxng_temp(proc)
        # If service is installed, SearXNG will restart via launchd
        if os.path.exists(LAUNCHD_PLIST):
            changed("Waiting for launchd to restart SearXNG...")
            time.sleep(2)
            if wait_for_searxng(timeout=10):
                ok("SearXNG restarted via launchd")

    return all_passed


# ── Step 6: MCP Server Setup ──────────────────────────────────────────────

def detect_harness(harness):
    """Check if a coding tool is installed."""
    for cmd in harness.get("detect_cmds", []):
        if shutil.which(cmd):
            return True
    for path in harness.get("detect_paths", []):
        if os.path.exists(os.path.expanduser(path)):
            return True
    return False

def _has_searxng_config(existing):
    """Check if an existing MCP entry looks like our SearXNG config."""
    cmd = existing.get("command")
    if isinstance(cmd, list):
        return "mcp-searxng" in cmd
    if cmd == "npx":
        return "mcp-searxng" in existing.get("args", [])
    return False

def install_mcp_for_harness(harness):
    """Add SearXNG MCP server to a tool's config."""
    config_path = os.path.expanduser(harness["config_file"])
    mcp_key = harness["mcp_key"]
    server_config = harness["server_config"]

    # Read existing config
    config = {}
    if os.path.isfile(config_path):
        try:
            with open(config_path) as f:
                config = json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            warn(f"Could not read {config_path}: {e}")
            return False

    # Check if already configured
    existing = config.get(mcp_key, {}).get("searxng", {})
    if _has_searxng_config(existing):
        skip(f"searxng already configured in {harness['name']}")
        return True

    # Add MCP server
    if mcp_key not in config:
        config[mcp_key] = {}
    config[mcp_key]["searxng"] = server_config

    # Write back
    os.makedirs(os.path.dirname(config_path), exist_ok=True)
    with open(config_path, "w") as f:
        json.dump(config, f, indent=2)
        f.write("\n")

    changed(f"MCP server added to {harness['name']} ({os.path.basename(config_path)})")
    return True

def install_mcp_servers():
    """Detect AI coding tools and offer to install MCP server config."""
    section("Setting up MCP servers")

    detected = [(h["name"], h) for h in MCP_HARNESSES if detect_harness(h)]

    if not detected:
        skip("No MCP-compatible coding tools detected")
        print("  Supported: " + ", ".join(h["name"] for h in MCP_HARNESSES))
        return

    ok(f"Detected: {', '.join(name for name, _ in detected)}")

    if not sys.stdin.isatty():
        skip("Interactive terminal required for MCP setup")
        print("  Re-run from a terminal to configure, or edit config files manually")
        return

    installed = 0
    for name, harness in detected:
        try:
            answer = input(f"  Install SearXNG MCP for {name}? [Y/n] ").strip().lower()
        except EOFError:
            answer = 'n'
        if answer in ('', 'y', 'yes'):
            if install_mcp_for_harness(harness):
                installed += 1
        else:
            skip(f"{name}")

    if installed:
        ok(f"Configured in {installed} tool(s) — restart them to load the MCP server")

def uninstall_mcp_servers():
    """Remove SearXNG MCP server from all configured tools."""
    section("Removing MCP server configs")

    for harness in MCP_HARNESSES:
        config_path = os.path.expanduser(harness["config_file"])
        mcp_key = harness["mcp_key"]

        if not os.path.isfile(config_path):
            continue

        try:
            with open(config_path) as f:
                config = json.load(f)
        except (json.JSONDecodeError, IOError):
            continue

        servers = config.get(mcp_key, {})
        if "searxng" not in servers:
            continue

        del config[mcp_key]["searxng"]
        if not config[mcp_key]:
            del config[mcp_key]

        with open(config_path, "w") as f:
            json.dump(config, f, indent=2)
            f.write("\n")

        changed(f"Removed from {harness['name']}")


# ── Main ───────────────────────────────────────────────────────────────────

def main():
    if "--uninstall" in sys.argv:
        banner("Uninstalling SearXNG")
        uninstall_mcp_servers()
        uninstall_service()
        print("\nService and MCP configs removed. Repo preserved at:")
        print(f"  {SEARXNG_DIR}")
        print("\nTo fully remove: rm -rf ~/searxng")
        return

    if "--test" in sys.argv:
        banner("Testing SearXNG")
        if test_installation():
            print("\nAll tests passed.")
        else:
            print("\nSome tests failed. See above for details.")
            sys.exit(1)
        return

    banner("SearXNG Local MCP Setup")

    # Step 1: Prerequisites
    if not check_prerequisites():
        sys.exit(1)

    # Step 2: Install / Update
    if not install_or_update():
        warn("Installation failed")
        sys.exit(1)

    # Step 3: Configure
    configure()

    # Step 4: Service
    install_service()

    # Step 5: MCP servers
    install_mcp_servers()

    # Step 6: Test
    banner("Testing")
    if test_installation():
        print("\nAll tests passed. SearXNG is ready.")
        print(f"\nMCP server URL: {SEARXNG_URL}")
        print(f"Service: {'installed (auto-starts at login)' if os.path.exists(LAUNCHD_PLIST) else 'not installed'}")
    else:
        print("\nSome tests failed. See above for details.")
        sys.exit(1)

if __name__ == "__main__":
    main()
