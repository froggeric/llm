# Roadmap

This is a living document. It organizes everything **localvision** is heading
toward — the scattered `v0.1.1` / `v0.2` notes that used to live in source
comments and Known-Limitations lists, plus the feature ideas that drive the
next several releases.

New ideas are added here first, sequenced and scoped, before any code. If an
item isn't in this file, it isn't planned.

## How to read this

- Work is grouped into **themes** (A–H), each anchored to a target version.
- Each item carries an **effort** guess (`XS / S / M / L / XL`) and a
  **status**: ✅ done · 🚧 next up · 📋 planned · 🔬 needs design.
- Versions are advisory, not commitments. Items move between them freely.
- Until v1.0, minor versions may break compatibility (see `CHANGELOG.md`).

## Where we are now

`v0.1.0` shipped 2026-06-18 (Apple Silicon MVP, old 3-model catalog). On `main`
today, **unreleased**:

- The catalog was refreshed to the **v0.2.0** set — winners of the v6 benchmark
  (30 images × 3 runs × 24 variants): `qwen3-vl-8b` (Q8_0), `qwen3.5-4b`
  (nothink), `qwen3.6-27b` (nothink).
- `chat_template_kwargs` plumbing (`enable_thinking=false`) and **WEBP** support
  were added.
- The directory was renamed `local-vision-mcp` → `localvision`.
- The v6 benchmark was checked into the monorepo at `benchmark/vlm/`.

What **works** today: 9 MCP tools, hardware tier detection on Apple Silicon,
spawn-on-demand subprocess lifecycle with keep-warm + crash recovery, SHA256
verification on every model load, HEIC conversion, Claude Code plugin manifest
+ skill, `go install` / `curl|sh` install.

What's **not** working yet: CI is red (see A1), no release has ever been cut,
the `llama-server` binary is downloaded at runtime without an integrity pin,
and the binary is MCP-only (no shell/CLI usage).

---

## Theme A — Release hygiene (unblock shipping) → `v0.2.0`

The foundation. Nothing downstream (releases, Homebrew, marketplace) is
trustworthy until CI is green and the shipped `llama-server` is pinned. This
theme is the gate for the first real `v0.2.0` tag.

### A1. Fix the CI build ✅ `S` — *(done in v0.2.0)*

**Root cause (diagnosed):** `internal/llama/client_test.go` has two tests —
`TestBuildChatRequestBodyImageURLs` (~line 280) and
`TestBuildChatRequestBodyContainsNoShell` (~line 364) — that pass fake paths
like `/tmp/foo.png`. Production code changed: `buildChatRequestBody`
(`client.go:439/484`) now inlines images as `data:` URIs (it `os.ReadFile`s
each path) because recent `llama-server` rejects `file://` unless `--media-path`
is set. The fixtures don't exist on disk, so both tests fail. Build + vet pass;
`go test -race` is the red step.

**Fix:** write fixtures into `t.TempDir()` before calling
`buildChatRequestBody`, and assert the data-URI JSON shape instead of the
stale `file://` substrings at `client_test.go:292-293`. Failure predates the
rename — it is not a path/import issue.

**Blocks:** every release-dependent item (B1, B2).

### A2. Acquire `llama-server` safely ✅ `M` — *(done in v0.2.0)*

Dropped the `TODO-PHASE3` placeholder. localvision now **prefers a
user-installed `llama-server` on `$PATH`** (e.g. `brew install llama.cpp`;
warned as unverified); if absent it downloads a **pinned official llama.cpp
release tag** and verifies the **archive SHA256** before extracting the dylib
bundle. No custom build or self-hosting. See `internal/llama/binary.go`.

**Pairs with A5** (how the binary reaches the user).

### A3. Unify naming ✅ `XS` — *(this update)*

The server was keyed three different ways across the repo (`"local-vision"` in
`plugin/plugin.json` and `docs/INSTALL.md`, `"localvision"` in `scripts/install.sh`
and the README). All are now standardized on **`localvision`**.

### A4. Reconcile documentation ✅ `M` — *(this update)*

Fix the v0.1 → v0.2 drift across README, CHANGELOG, ARCHITECTURE, TOOLS, and
the plugin SKILL (catalog names, tier list, benchmark wording, stale
"Phase 0" / "stub" language).

### A5. `llama-server` delivery ✅ `M` — *(done in v0.2.0)*

The release pipeline ships only the `localvision` wrapper (goreleaser,
`CGO_ENABLED=0`); `llama-server` is **not** bundled — it is resolved at the
first tool call per A2 (prefer `$PATH`, else download a pinned official
tar.gz). `scripts/build-llama-cpp.sh` is retained as a dev/convenience way to
build a local `llama-server` for `$PATH` (not part of the release).

---

## Theme B — Distribution (get it into users' hands) → `v0.2.0`–`v0.3.0`

### B1. GitHub Releases 🚧 `S` — *(idea 9)*

goreleaser is already wired (`.goreleaser.yaml` builds `darwin/arm64`, emits
`checksums.txt`, ships `install.sh` as a release asset). No release exists yet
(`gh release list` is empty). Once **A1** is green, tag
`mcp/localvision/v0.2.0` and the release publishes automatically.

### B2. Homebrew 🚧 `S–M` — *(idea 8)*

`.goreleaser.yaml:139` is `brews: []`; `HOMEBREW_TAP_GITHUB_TOKEN` is already
plumbed through the release workflow. To enable: create a tap repo
(`froggeric/homebrew-tap`), set the secret with `repo` scope on the tap, and
replace `brews: []` with a `brews:` stanza (goreleaser auto-generates the
formula). Depends on **B1** (artifacts to point at).

### B3. Claude Code marketplace plugin 📋 `M` — *(idea 6)*

The current `plugin/` dir holds a `plugin.json` + `SKILL.md` but is **not** in
marketplace layout — there is no `.claude-plugin/marketplace.json`. Today users
install the binary then hand-paste an `mcpServers` snippet. Add the marketplace
manifest and repo structure so localvision is one-click installable from the
Claude Code plugin marketplace.

### B4. Auto-detect & install into AI coding harnesses 📋 `M–L` — *(idea 7)*

Detect installed coding agents (Claude Code, Codex CLI, Gemini CLI, Cursor,
Continue, etc.) and offer to wire `localvision` into each one's MCP/config
automatically. This is the onboarding half of the **C1** setup wizard — when a
user runs `localvision` with no args, the interactive setup (C1) should find
their harnesses (B4) and configure them.

---

## Theme C — Standalone CLI (the flagship shift) → `v0.3.0`

Today `localvision` is **MCP-only**: the only inference path is
MCP-client → JSON-RPC → `CatalogExecutor`. Theme C turns the same binary into a
first-class **shell tool** you can call directly — `localvision describe
shot.png`, pipeable, scriptable, batchable. This is the largest capability
expansion in the roadmap and unlocks ideas 2–5, 10.

The reuse is clean: `tools.Tool.BuildRequest` + `Executor.Run` +
`Tool.ParseOutput` (`internal/mcpserver/executor.go:71`, `internal/tools/tool.go`)
are already **MCP-agnostic**. A new CLI subcommand can construct the executor
exactly as `main.go:119-130` does and skip the MCP SDK entirely.

### C1. Single executable: interactive setup + one-shot queries 📋 `L` — *(idea 2)*

- **No args** → interactive setup/configuration mode (TUI or prompted): pick a
  model, set paths, detect hardware, optionally install into a harness (B4),
  write `~/.localvision/config.toml`.
- **With args** → one-shot image query against a tool, prints to stdout.

`main.go` currently hand-rolls a `run`/`doctor`/`version` switch on plain
`flag`. Add a query/one-shot subcommand. This is the spine the rest of Theme C
hangs off.

### C2. `--type` query parameter with optimized prompts 📋 `S–M` — *(idea 5)*

Each tool already has a task-tuned, benchmark-informed system prompt
(`internal/tools/prompt.go`). Expose them as a `--type`/`--tool` flag on the
one-shot path: `--type ocr`, `--type diagram`, `--type chart`, `--type code`,
`--type ui`, `--type error`, `--type compare`, default `describe`. No new
prompts to write — reuse the 9 existing ones.

### C3. `--format` output parameter 📋 `M–L` — *(idea 3)*

`json | markdown | yaml | text | xml | exif`. Today the output format is
**baked into each prompt constant** ("Output ONLY the code in a fenced block",
"Output the text only — no Markdown"). To support a real format flag:

1. Strip the format clauses from the 9 prompts in `prompt.go`.
2. Expand `Tool.ParseOutput(raw string)` → `ParseOutput(raw, opts ParseOptions)`
   so the requested format flows in (`internal/tools/tool.go:53`,
   `internal/mcpserver/tools.go:190`).
3. Add format converters (the post-processing primitives `extractCodeBlock` /
   `extractMarkdownTables` in `base.go` already exist as a starting point).

**`exif` is special** — it writes the result back as metadata *into the image*
sidecar, not as stdout text; treat it as a distinct output sink.

Note: a clean structural format (JSON/YAML/XML) from a model that wants to emit
prose may need either a converter pass or a format-aware prompt suffix. Markdown
/ text are near-free; structured formats are more work.

### C4. Output to file + batch processing 📋 `M` — *(idea 4)*

`--output <file>` and `--output-dir <dir>` plus glob/directory input for batch
runs. Single natural hook point: `buildResult` (`internal/mcpserver/tools.go:217`)
sees both parsed and raw output and is the last Go-side transform before the
result leaves the process. The image normalizer already handles N images
(`normalizeImages`, `tools.go:260`), so batch is mostly a CLI + glob layer
(`filepath.Glob`) on top. A `.meta.json` sidecar (model, tokens, elapsed) is
possible but requires threading `ChatResponse.TokensIn/Out/ElapsedMs`
(currently dropped at `executor.go:141`) back through `Executor.Run`.

### C5. Manual model override 📋 `S–M` — *(idea 10)*

`--model <id>` per invocation, plus wiring the **currently-unused**
`Config.DefaultModel` field (`config.go:47`) into the executor. Today
`CatalogExecutor.Run` calls `catalog.ModelFor(toolID, hw)` and ignores
`cfg.DefaultModel` entirely — it's a latent field. Add: explicit `--model`
override → honor it; else honor `default_model` if set; else catalog autoselect.

### C6. Benchmark-faithful `llama.cpp` parameters ✅ `S–M` — *(done in v0.2.0)*

The v6 benchmark produced its quality with a specific `llama-server` invocation
(`benchmark/vlm/code/benchmark_llamaserver.py:139`). Reproduce it exactly so
shipped quality matches the benchmark:

**Sampling (HTTP request body):**
`temperature 0.1`, `top_p 0.95`, `top_k 64`, `max_tokens 16384`. (Temperature
0.1 is already hardcoded at `executor.go:117`; the rest need adding.)
`chat_template_kwargs.enable_thinking=false` is already carried per-model in
`builtin.toml`.

**Subprocess launch flags:**
`-np 1` (single-slot), `-b 4096 -ub 4096` (batch sizes large enough that image
tokens — up to ~2240 — never split across physical batches; the default `-ub
512` splits a 548-token image into two passes and degrades quality). `-ngl` /
`-c` are already per-model in the catalog (`ctx=32768`, `gpu_layers=-1`).

Low risk, no API change, high value for existing MCP users — **candidate to
pull forward into `v0.2.0`** alongside A1–A4.

---

## Theme D — Cross-platform → `v0.4.0`

`v0.1`/`v0.2` are Apple Silicon only. Linux/Windows hardware detection returns
`BackendUnsupported` (`internal/models/hardware_linux.go`,
`hardware_windows.go`); goreleaser and `build-llama-cpp.sh` reject every other
target with a clear "v0.2" message.

- **D1.** Linux hardware detection — CUDA + ROCm. `M`
- **D2.** Windows hardware detection — CUDA + DirectML. `M`
- **D3.** CI matrix: add Linux + Windows runners. `S`
- **D4.** goreleaser cross-compile: uncomment `linux`, `windows`, `amd64`
  (`.goreleaser.yaml:42-46`); the windows→zip `format_override` is already
  staged. `S`
- **D5.** Cross-platform HEIC: ImageMagick fallback (today HEIC relies on macOS
  `sips`). `S`

D3/D4 depend on D1/D2 (don't ship builds for platforms you can't detect).

---

## Theme E — Hardening & polish → ongoing / `v0.5.0+`

The long tail of known limitations, mostly small, mostly independent.

- **E1.** Streaming `notifications/progress` for long tool calls (today each
  `tools/call` blocks 30–70 s). *(from v0.2 known-limitation)* `M`
- **E2.** Auto-reap orphan `llama-server` subprocesses on startup (today:
  manual via `doctor`). `S`
- **E3.** `doctor --compute-hashes` to populate catalog SHA256s automatically
  (today: by hand). `S`
- **E4.** Configurable tool-name prefix to avoid MCP collisions (tool names are
  unprefixed today). `S`
- **E5.** Automatic Ollama coordination (unified-memory contention on Apple
  Silicon; today `doctor` only warns if `:11434` is occupied). `M`
- **E6.** Wire temp-file cleanup — `tools.CleanupImageRef` is defined but never
  called in the production path; data-URI temp files leak to `os.TempDir`. `S`
- **E7.** Pin `goreleaser` to a known-good major version in the release
  workflow (today `brew install goreleaser` is unpinned) and add a `lint`
  (golangci-lint) step to CI. `S`

---

## Theme F — Reach & power → `v0.6.0+`

Make localvision faster, more reliable, and reachable far beyond a single MCP
client. Several items here are small and can be pulled forward into any release.

- **F1. localhost HTTP/REST API** — `localvision serve --http :7665` exposing
  `/v1/ocr`, `/v1/describe`, etc. Loopback-only. MCP stays the agent path; HTTP
  opens up `curl`, scripts, cron, and any language. `M`
- **F2. OpenAI-compatible `/v1/chat/completions` with vision** — expose the same
  protocol you already speak to `llama-server`, upstream. Any OpenAI-compatible
  client can then use localvision as a **local vision backend**. `M`
- **F3. Content-addressed result cache** — reuse the per-image SHA256 (already
  computed for logging) as a cache key in `~/.localvision/cache/`. Same image +
  tool + model → instant replay. Big win for batches and re-runs. **Opt-in,
  text-only, never stores image bytes** (privacy-conscious). `S`
- **F4. Constrained decoding (GBNF grammars)** — constrain `llama-server` output
  to valid JSON/CSV/table shape so structured formats are *guaranteed*
  well-formed, not prompt-begged. The real engine behind a reliable `--format`
  (reinforces C3). `M`
- **F5. Multi-sample consensus + confidence** — run N=3 on hard images,
  majority-vote, surface a `confidence`/disagreement field. Attacks the
  run-to-run variance the benchmark documented (σ up to 0.48). On-mission
  ("reliability > peak score"). `S/M`
- **F6. Cascade / difficulty routing** — cheap model first; escalate to the 27B
  only when confidence is low. Saves resources, reserves the big gun for hard
  cases. `M`
- **F7. Self-verification pass** — for `extract_text`/`extract_code`, a second
  cheap call re-checks the extraction against the image and flags mismatches.
  Catches hallucinations. `S/M`
- **F8. `doctor --selftest`** — run a handful of `benchmark/vlm/test-images`
  against the installed model and sanity-check output against `GROUND-TRUTH.md`.
  One-command "does my install actually work," reusing assets you already ship. `S`
- **F9. Clipboard in/out** — read a screenshot from the clipboard, copy the
  result text back out. Zero file management for the #1 input ("the screenshot
  I just took"). `XS`
- **F10. Watch mode** — `localvision watch ~/Screenshots --type ocr` processes
  every new file automatically. Hands-off pipelines. `S`
- **F11. `doctor --fix`** — let `doctor` auto-remediate (clear corrupt cache,
  re-download, re-pin), not just diagnose. `XS`
- **F12. Shell completions + examples-rich help** — bash/zsh/fish/PowerShell
  completions and a great `--help`. Cheap polish, big feel. `XS`
- **F13. Model management subcommands** — `localvision models list/pull/remove/
  switch`, pre-fetch. Make the catalog first-class instead of auto-magic only. `S`
- **F14. MCP resources & prompts** — expose the catalog, benchmark summary, and
  tool-selection guidance as MCP `resources`, and the 9 task prompts as reusable
  `prompts`, so agents can introspect "which model and why" without a tool call. `S`
- **F15. Local usage stats** — `localvision stats` (calls, models, tokens, time),
  purely local and opt-in. `S`
- **F16. Local web UI** — `localvision ui` serves a drag-drop page (pick a tool,
  see results) on top of the F1 HTTP API. Democrafts localvision to
  non-developers. `L`

---

## Theme G — Expanded tool set & new modalities → `v0.6.0+`

New tools and richer output, motivated by a comparison against a peer server.

### Tool landscape vs. `zai_mcp_server`

`zai_mcp_server` exposes 8 vision tools. localvision covers 5 of them 1:1 and
has 2 dedicated tools zai lacks. Two real gaps:

| `zai_mcp_server` | localvision | status |
|---|---|---|
| `analyze_image` | `read_image` | ✅ covered |
| `extract_text_from_screenshot` | `extract_text` | ✅ covered |
| `diagnose_error_screenshot` | `diagnose_error` | ✅ covered |
| `understand_technical_diagram` | `describe_diagram` | ✅ covered |
| `analyze_data_visualization` | `describe_chart` | ✅ covered (angle: numbers vs insights) |
| `ui_diff_check` | `compare_images` | ◐ partial — generic vs UI-regression |
| `ui_to_artifact` | `describe_ui` (prose only) | ✗ **gap** — no code/spec artifact (G2) |
| `analyze_video` | — | ✗ **gap** — no video (G1) |
| — | `extract_code` | ★ localvision advantage (zai has none) |
| — | `extract_table` | ★ localvision advantage (zai has none) |

### Items

- **G1. Video analysis** (`analyze_video`) — extract keyframes via `ffmpeg`,
  analyze + summarize. Fills the `analyze_video` gap; new modality. `L`
- **G2. UI → artifact** (`ui_to_artifact`) — from a UI screenshot, emit
  **code** (HTML/Tailwind/React), a **design spec**, a **rebuild prompt**, or a
  description. Fills the `ui_to_artifact` gap; `describe_ui` becomes the
  description mode of a more powerful tool. `M/L`
- **G3. PDF / multi-page / document ingestion** — rasterize PDF/DOCX/slides
  pages (e.g. `pdftoppm`/`mutool`), process per-page, summarize. Extends scope
  from images to documents. `M`
- **G4. `describe_chart` → structured data** — return the underlying numbers as
  CSV/JSON, not just prose. "Get this graph into my spreadsheet." `S`
- **G5. `describe_diagram` → editable markup** — emit Mermaid/PlantUML/graph
  JSON. Image → round-trippable, editable diagram. `S`
- **G6. Coordinate grounding** — have `describe_ui`/`describe_diagram` return
  bounding boxes (Qwen3-VL supports grounding) so agents can crop/click/replay. `M`
- **G7. `compare_images` → UI regression + visual diff** — specialize the
  expected-vs-actual case and optionally emit a highlighted diff image. `S/M`

**Preserve as differentiators:** `extract_code` and `extract_table` — dedicated
tools zai has no equivalent for. Keep them best-in-class.

---

## Theme H — Far future (research-grade) → `v1.0+`

Ambitious, far-fetched, explicitly invited for the later roadmap.

- **H1. `localvision-tiny`** — distill/fine-tune a small vision model on the v6
  benchmark ground-truth for a project-tuned, ultra-fast local model. `XL`
- **H2. Local image library + semantic search** — embed and index every analyzed
  image; "find the screenshot where…". `L`
- **H3. Live screen-region capture** — continuously watch and analyze a screen
  region for automation/accessibility. `L`
- **H4. Autonomous multi-step vision agent** — the model drives its own
  crop→zoom→re-read loop on hard images. `L`
- **H5. Multi-view scene / 3D reconstruction** — reconstruct a scene from
  several images. `XL`

---

## Sequencing & dependencies

```
v0.2.0  Foundation & first real distribution
        A1 (CI) ✅ ─┬─► B1 (GitHub Releases) ─► B2 (Homebrew)
        A2 (binary) ✅─┘  A3 (names) ✅, A4 (docs) ✅, A5 ✅
        C6 (benchmark params) — pull forward, low risk

v0.3.0  Standalone CLI + onboarding
        C1 (one-shot + setup wizard) ─┬─► C2 (--type)
                                       ├─► C3 (--format)
                                       ├─► C4 (--output/batch)
                                       └─► C5 (--model)
        C1 + B4 (harness auto-install) together = the setup wizard
        B3 (marketplace) — independent, can land any time after B1

v0.4.0  Cross-platform
        D1/D2 (detect) ─► D3 (CI matrix), D4 (cross-compile), D5 (HEIC)

v0.5.0+ Hardening
        E1–E7, mostly independent

v0.6.0+ Reach & power + expanded tools (exploratory)
        F1 (HTTP) ─► F2 (OpenAI-compat), F16 (web UI)
        F4 (grammars) reinforces C3; F3 (cache), F5 (consensus), F8 (selftest)
        F9/F11/F12 (clipboard/doctor --fix/completions) — cheap, land anytime
        G1 (video), G2 (ui→artifact), G3 (PDF) — new modalities

v1.0+   Far future / research-grade: H1–H5
```

**Critical path:** A1 → B1 → B2. Green CI is the single unlock for
distribution. C6 is the highest-value-per-effort quality item and should ride
along with the foundation release. Themes F–H are exploratory reach / power /
research items, not on the critical path; the cheap ones (F9/F11/F12) can land
in any release.

## Idea index

| # | Idea | Item | Target |
|---|---|---|---|
| 1 | Fix GitHub Actions compilation | A1 | v0.2.0 |
| 2 | Single executable: interactive + parametric queries | C1 | v0.3.0 |
| 3 | `--format` output parameter (incl. exif) | C3 | v0.3.0 |
| 4 | Output to file / batch | C4 | v0.3.0 |
| 5 | `--type` query parameter + optimized prompts | C2 | v0.3.0 |
| 6 | Claude Code marketplace deployment | B3 | v0.3.0 |
| 7 | Auto-detect & install AI coding harnesses | B4 | v0.3.0 |
| 8 | Homebrew install | B2 | v0.2.0 |
| 9 | GitHub Releases | B1 | v0.2.0 |
| 10 | Manual model override | C5 | v0.3.0 |
| 11 | Benchmark-faithful llama.cpp parameters | C6 | v0.2.0 |

Pre-existing items folded in: pin `llama-server` SHA (A2), naming (A3),
binary pipeline (A5), streaming (E1), auto-reap (E2), `--compute-hashes` (E3),
tool-name prefix (E4), Ollama coordination (E5), temp-file cleanup (E6),
release-tool pinning + lint (E7), cross-platform (D1–D5).

Themes **F** (Reach & power, F1–F16), **G** (Expanded tools, G1–G7), and **H**
(Far future, H1–H5) are proposed additions — not from the original 11 user
ideas. They are exploratory; cherry-pick freely.
