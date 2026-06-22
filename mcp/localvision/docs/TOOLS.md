# Tools reference

9 tools, each with a task-tuned system prompt. Pick the **most specific** tool for the job — generic `read_image` is the fallback when nothing else fits.

## Quick reference

| Tool | Input | Output | Default model | Expected latency |
|---|---|---|---|---|
| `read_image` | 1 image, optional question | Prose description | qwen3.6-27b (mainstream) / qwen3-vl-8b (constrained) | 30–60 s |
| `extract_text` | 1 image | Verbatim OCR text | qwen3.6-27b / qwen3-vl-8b | 30–60 s |
| `extract_code` | 1 image | Fenced code block with language | qwen3.6-27b / qwen3-vl-8b | 30–60 s |
| `extract_table` | 1 image | Markdown tables | qwen3.6-27b / qwen3-vl-8b | 30–70 s |
| `describe_ui` | 1 image | Layout + components + errors | qwen3.6-27b / qwen3-vl-8b | 30–60 s |
| `describe_diagram` | 1 image | Diagram type + components + connections | qwen3.6-27b / qwen3-vl-8b | 30–70 s |
| `describe_chart` | 1 image | Chart type + axes + series + trends | qwen3.6-27b / qwen3-vl-8b | 30–70 s |
| `diagnose_error` | 1 image | Error type + root cause + file:line | qwen3.6-27b / qwen3-vl-8b | 30–60 s |
| `compare_images` | 2 images | Bullet list of differences | qwen3.6-27b / qwen3-vl-8b | 40–80 s |

"Default model" depends on detected hardware tier (see [MODELS.md](./MODELS.md)):
- **constrained** (≤16 GB unified memory): `qwen3-vl-8b` (Q8_0), preferred for all tools. On 4–8 GB Macs where it does not fit, falls back to `qwen3.5-4b` (nothink).
- **mainstream** (16+ GB): `qwen3.6-27b` (nothink), preferred for all tools; also used on 48+ GB Macs (no larger model justified by the v6 study).

Override per-tool via the catalog's `preferred_for` field, or globally via `default_model` in `~/.localvision/config.toml` (a CLI `--model` override is planned — see [ROADMAP.md](../ROADMAP.md), item C5).

## Input format

All tools accept the same input shape (except `compare_images`, which takes an array):

```json
{
  "image_path": "/absolute/path/to/image.png",
  "question": "Optional specific question about the image"
}
```

Or:

```json
{
  "image_data": "data:image/png;base64,iVBORw0KG...",
  "question": "..."
}
```

Or:

```json
{
  "image_url": "file:///absolute/path/to/image.png"
}
```

Remote `http(s)://` URLs are **rejected** with a helpful error. `llama-server` runs on localhost only.

`compare_images` takes:

```json
{
  "images": [
    { "image_path": "/abs/before.png" },
    { "image_path": "/abs/after.png" }
  ]
}
```

Or aliases `image_a` / `image_b`.

## Tool details

### `read_image`

Generic "describe what's in this image." Use when no specific tool fits.

System prompt focus: visible text (verbatim), objects, people, layout, colors, notable features. Markdown headings.

Max output: 1500 tokens.

### `extract_text`

OCR-focused. Use for receipts, documents, signs, anything where the verbatim text matters.

System prompt focus: extract ALL text verbatim. Preserve formatting, indentation, line breaks. No commentary.

Max output: 2048 tokens.

### `extract_code`

Code-focused. Detects language, preserves indentation, wraps in fenced block.

System prompt focus: detect programming language, output ONLY the code in ` ```<lang> ` fenced block, preserve all whitespace, no explanation.

Max output: 4096 tokens.

ParseOutput: strips any prose outside the fenced block. Returns `{language: "python", code: "..."}`.

### `extract_table`

Table-focused. Use for spreadsheets, CSVs, database views.

System prompt focus: output as Markdown tables, separate multiple tables with `---`, preserve column alignment.

Max output: 2048 tokens.

### `describe_ui`

UI screenshot analysis. Use for app windows, design mocks, dashboards.

System prompt focus: overall layout, every visible component with text labels, any feedback/error messages, interactive elements (buttons, links, checkboxes).

Max output: 2000 tokens.

### `describe_diagram`

Technical diagrams. Use for architecture, ER, flowchart, sequence diagrams.

System prompt focus: diagram type, all named components, all connections with labels/protocols, brief summary.

Max output: 2000 tokens.

### `describe_chart`

Charts and data visualizations. Use for bar/line/pie charts, plots, dashboards with metrics.

System prompt focus: chart type, axes with units, data series with names, notable values and outliers, trends.

Max output: 1024 tokens.

### `diagnose_error`

Error screenshots. Use for terminal errors, stack traces, exception dialogs.

System prompt focus: error type (exception class), root-cause message verbatim, most relevant file:line in the stack trace, one-sentence likely cause.

Max output: 800 tokens.

### `compare_images`

Diff two images. Use for before/after UI changes, regression detection.

System prompt focus: what's DIFFERENT between the two images, focus on visible changes (text, layout, colors, components), if identical say so.

Max output: 1500 tokens.

## Known model behaviors

From the v6 benchmark ([`../../../benchmark/vlm/SUMMARY.md`](../../../benchmark/vlm/SUMMARY.md)):

- **Qwen3.6-27B (nothink)**: benchmark champion — 79.6/100, σ=0.24, 0 failures across 90 cells. Best quality, stability, and reliability. Needs 24+ GB for comfortable operation.
- **Qwen3-VL 8B (Q8_0)**: the only 100%-reliable Q8 model (0 timeouts, σ=0.33). Slightly lower peak quality than the 27B, but rock-solid on constrained Macs. Verbose (1400–2200 tokens).
- **Qwen3.5-4B (nothink)**: best quality-per-GB (75.5/100 at ~3 GB) and the only option on 4–8 GB Macs. Higher run-to-run variance (σ=0.48) on hard images.

Models dropped after the v6 benchmark — Gemma 4 12B (Q4 hallucination flips / Q8 22% timeouts), Qwen3.6 35B-A3B (MoE size misleading), Qwen3-VL 4B, and InternVL3.5 8B — are documented in [MODELS.md](./MODELS.md).

## Tool name collisions

If you have another vision MCP installed (e.g. a cloud-based one), tool names may collide in Claude Code's tool list. Mitigations:

1. Uninstall the other vision MCP.
2. Use Claude Code's per-server namespacing if available.
3. Wrap `localvision`'s tools with a prefix decorator (requires code change; raise an issue if you need this).

For v0.1, tool names are unprefixed.
