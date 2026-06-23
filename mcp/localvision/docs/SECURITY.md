# Security & privacy

This document covers what data the MCP sees, where it goes, and what could go wrong.

## Privacy promises

1. **Image bytes never leave this machine.** Images are decoded in-memory and passed to `llama-server` via local file paths. They are not uploaded, logged, or sent anywhere off-host.
2. **No telemetry.** No anonymous stats. No crash reports that include image content. No phone-home. The binary has no analytics code.
3. **Logs do not contain image bytes.** Images are referenced by SHA256 (hash of bytes) in diagnostic output, never by content.
4. **The only outbound HTTP is to:**
   - `huggingface.co/froggeric/` (model files)
   - `github.com/ggml-org/llama.cpp/releases` (the `llama-server` binary on first run)
5. **`llama-server` binds to `127.0.0.1` only.** No external network exposure.

## Verifying these claims

A few properties are enforced by code and auditable:

```bash
# Verify no telemetry: grep for outbound HTTP in the source
grep -rn "http.Client\|http.Get\|http.Post" internal/ | grep -v _test.go
# Expected: only internal/llama/binary.go (HF + GitHub download) and
# internal/models/downloader.go (HF model download).

# Verify llama-server binds to localhost
grep -n "127.0.0.1" internal/llama/subprocess.go
# Expected: --host 127.0.0.1 in the argv builder.

# Verify no sh -c (command injection vector)
grep -rn "sh -c" internal/
# Expected: matches only in comments explaining why we don't use it.
```

## Trust boundaries

| Component | Trusted? | Why |
|---|---|---|
| `localvision` Go binary | You built it | Your code, your build. |
| `llama-server` binary | Verified via SHA256 | Pinned hash in source; downloaded binary must match. |
| Model GGUF + mmproj files | Verified via SHA256 | Per-model hashes in the catalog; verified on **every** load, not just download. |
| User overlay catalogs | User-controlled | Subject to HTTPS-only URL validation + HF namespace regex. |
| `froggeric/` HuggingFace namespace | You control it | You upload the model files. |

## Supply chain

The binary has 3 Go-module dependencies:

- `github.com/modelcontextprotocol/go-sdk` (MIT) — official MCP SDK
- `github.com/BurntSushi/toml` (MIT) — TOML parser
- `github.com/stretchr/testify` (MIT) — test assertions

Plus `golang.org/x/sys` (BSD-3-Clause) transitively for sysctl.

See `THIRD_PARTY_LICENSES.md` for full text.

The auto-downloaded `llama-server` binary is MIT-licensed. See https://github.com/ggml-org/llama.cpp/blob/master/LICENSE.

## What can go wrong

### Malicious overlay catalog

A user-supplied `~/.localvision/catalog.d/evil.toml` tries to fetch a model from a non-`froggeric/` HF namespace or from a different host entirely.

**Mitigation**: catalog validation rejects any `gguf`/`mmproj` URL that doesn't match `^https://huggingface\.co/froggeric/` (regex parameterized by `config.hf_user`). Hard error.

### Path traversal in image inputs

A tool caller passes `image_path = "/etc/passwd"`. The MCP passes the path to `llama-server` via `--mmproj`-style flags.

**Mitigation**: `llama-server` reads the file as binary and feeds it to the model as image bytes. It won't display the contents of `/etc/passwd` to the user — at worst, the model will say "I can't decode this as an image." No file-content leak.

### Command injection via catalog fields

A malicious overlay sets `gguf = "$(rm -rf ~)"`, hoping the subprocess spawner will execute it via shell.

**Mitigation**: the spawner uses `exec.Command(path, args...)` directly, never `sh -c`. The `gguf` value becomes one argv element; it's never interpreted as shell syntax.

### Compromised `froggeric/` HF account

If the HF account is compromised, an attacker uploads a malicious model file with the same name. The catalog's SHA256 catches this — the file is rejected on next load.

**Caveat**: if the attacker also updates the catalog TOML in this repo to match the new file's hash, the SHA256 check passes. Mitigation: review catalog PRs carefully; the SHA256 is the last line of defense, not the first.

### Compromised `ggml-org/llama.cpp` GitHub repo

If the upstream llama.cpp release is replaced with a malicious binary:

**Mitigation**: the `pinnedLLAMAServerSHA256` constant in `internal/llama/binary.go` is the gate. To accept a new version, a maintainer must explicitly update the constant in source. A poisoned release with a different hash is rejected.

### Subprocess fork bomb

If many tool calls arrive concurrently and the lifecycle manager doesn't serialize, multiple `llama-server` processes could spawn.

**Mitigation**: `LifecycleManager` uses `sync.Mutex` + `sync.Cond` to serialize `Acquire` calls. At most one subprocess is running at any time. Test: `TestAcquireSerializeConcurrent` (50 concurrent goroutines).

### Stale zombie `llama-server` after parent crash

If the MCP crashes hard (SIGKILL, panic), the `llama-server` child may keep running.

**Mitigation**: the `doctor` command checks for orphans via process listing. A future release will add an auto-reap on startup (ROADMAP Theme E2). For now, manual cleanup:

```bash
pkill -fa llama-server
```

## Responsible disclosure

Found a security issue? Email `frederic@guigand.com` with details. Please do not open a public GitHub issue for security vulnerabilities.

We will acknowledge within 48 hours and aim to ship a fix within 30 days for high-severity issues.
