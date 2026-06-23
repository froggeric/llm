# Troubleshooting

Start with `localvision doctor` — it prints hardware, catalog validation, model selection, and binary status. Most issues show up there.

## Common issues

### 1. `catalog failed validation: gguf_sha256 is missing or placeholder`

A model entry in the catalog (usually one you added via an overlay in `~/.localvision/catalog.d/`) is missing its `gguf_sha256`. The catalog loads but that model can't be used until the hash is present. (Built-in models ship with their hashes.)

**Fix**: compute the hash and add it to your overlay:

```bash
# Download the model file
curl -L "https://huggingface.co/froggeric/Qwen3-VL-8B-Instruct-GGUF/resolve/main/Qwen3-VL-8B-Instruct-Q8_0.gguf" -o /tmp/model.gguf

# Compute the hash
shasum -a 256 /tmp/model.gguf

# Add to an overlay
cat > ~/.localvision/catalog.d/local.toml <<'EOF'
schema_version = 1
[models.qwen3-vl-8b]
gguf_sha256 = "<hash from above>"
EOF
```

### 2. `no model in the catalog fits the detected hardware`

Your Mac has less unified memory than the smallest model in the catalog requires (`min_vram_gb = 3` for Qwen3.5 4B).

**Fix**: either upgrade your Mac, or add a smaller model via a catalog overlay (e.g. Qwen3-VL 0.5B if you can find one). Or use a cloud vision MCP instead.

### 3. `llama-server binary unavailable`

The first-run download of `llama-server` failed (network, GitHub down, SHA mismatch).

**Fix**:

```bash
# Option A: install llama.cpp manually and let the MCP find it on $PATH
brew install llama.cpp

# Option B: clear the cache and retry
rm -rf ~/.localvision/bin
localvision doctor   # should report missing
localvision run      # will redownload on first tool call
```

### 4. `subprocess failed to become healthy in time`

`llama-server` started but didn't respond to `/health` within `startup_timeout` (default 2m). Common causes:

- Model file is corrupt (re-download).
- Disk is slow (model loading takes >2m on a HDD).
- Another process is using all GPU memory.

**Fix**:

```bash
# Raise the startup timeout
cat >> ~/.localvision/config.toml <<'EOF'
startup_timeout = "5m"
EOF

# Check for memory pressure
vm_stat | head -5
ps aux | sort -nk 4 | tail -5  # top memory consumers
```

### 5. Tool call returns 503 or "connection refused"

`llama-server` died mid-call. The lifecycle manager should respawn it on the next call. If it persists:

```bash
# Check for orphans
pgrep -fa llama-server

# Kill them
pkill -fa llama-server

# Retry
localvision doctor
```

### 6. Coexistence with Ollama

If you run `ollama serve` on `:11434`, it may grab GPU memory that `llama-server` needs.

**Fix**:

```bash
# Pause Ollama while using localvision
brew services stop ollama   # or: pkill ollama

# Or run Ollama with a memory limit (Ollama-specific; see its docs)
```

`localvision` never touches port 11434.

### 7. Two `llama-server` processes are running

You see two `llama-server` processes in `ps`. One is a stale orphan; the other is the current one.

**Fix**:

```bash
# Identify the older PID
ps aux | grep llama-server | sort -k2 -n

# Kill the older one
kill <older-pid>
```

The lifecycle manager uses port-sampling via `net.Listen("127.0.0.1:0")`, so two processes won't bind the same port — but both will compete for GPU memory.

### 8. `tools/list` returns 10 tools but `tools/call` always fails

The catalog validates (SHA256s are correct), the binary is present, but every tool call returns an error.

**Diagnostic**:

```bash
localvision doctor   # check the "Default model" line
```

If `Default model: NONE FITS`, see issue #2.

If `Default model: <name>`, try the smallest model:

```bash
# Override per-tool selection to force the smallest model
cat > ~/.localvision/catalog.d/force-small.toml <<'EOF'
schema_version = 1
[models.qwen3-vl-8b]
preferred_for = []  # opt Qwen3-VL 8B out of all preferred lists
EOF
# Now ModelFor falls back to qwen3-vl-4b (preferred_for=["read_image"])
```

### 9. Logs are too verbose / not verbose enough

```bash
# More verbose
localvision run --verbose

# Or persistently
cat >> ~/.localvision/config.toml <<'EOF'
log_level = "debug"
EOF

# Less verbose
cat >> ~/.localvision/config.toml <<'EOF'
log_level = "warn"
EOF
```

### 10. The `compare_images` tool says the two images are identical when they're not

The model didn't see the differences. Try:

1. Use a higher-tier model (override per-tool to `gemma4-26b-a4b`).
2. Pass a specific question: `compare_images({ images: [...], question: "Compare the navigation bars specifically" })`.
3. Crop the images to the relevant region first.

Local VLMs are not frontier-class. Cross-check critical comparisons.

## Reporting bugs

Run `localvision doctor` and paste the full output. Also include:

- macOS version (`sw_vers`).
- Output of `localvision version`.
- Output of `~/.localvision/logs/mcp.log` if you set `log_file`.
- The exact tool call and error message.
- Whether Ollama or any other local LLM tool is running.

Open an issue at https://github.com/froggeric/llm/issues and tag it `component: localvision`.
