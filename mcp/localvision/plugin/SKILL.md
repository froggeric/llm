---
name: image-reading
description: Use whenever the user references an image (file path, screenshot, pasted image) and you cannot see it directly. Routes to localvision's local VLM tools — no cloud, no telemetry, image bytes stay on this machine.
---

# Image reading (localvision)

The `localvision` MCP server provides 11 vision tools backed by a local `llama.cpp` subprocess. Use them when you (the assistant) need information from an image you cannot see, or when the user explicitly asks for local analysis.

## When to invoke

Invoke when **any** of these apply:

- The user references an image file (`/path/to/foo.png`, `screenshot.png`, "the screenshot I just took").
- The user pastes an image (delivered to you as a `data:` URI by the client).
- The user asks you to "look at", "describe", "read", "extract from", or "compare" an image.
- The user is on a text-only LLM (Codex CLI, Gemini CLI free tier, non-vision Claude variants) and shares an image.

Do **not** invoke when:

- You have native vision and the image is already in your context. Use your own vision directly.
- The user shares a video or audio file. These tools are image-only.
- The image is at a remote `http(s)://` URL. The MCP runs `llama-server` on localhost only — it cannot fetch remote URLs. Have the user download the image locally first.

## Tool selection

Pick the **most specific** tool that matches the user's intent. More specific tools have task-tuned prompts that produce 2-3× better output than the generic `read_image`.

| User's image is... | Tool |
|---|---|
| A **PDF document** (paper, report, slides, scan) | `read_document` |
| Code in an editor, terminal, photograph, whiteboard | `extract_code` |
| Text to transcribe verbatim (receipts, documents, signs) | `extract_text` |
| Tables (spreadsheets, CSVs, database views) | `extract_table` |
| A UI screenshot, design mock, app window | `describe_ui` |
| Architecture / ER / flowchart / sequence diagram | `describe_diagram` (pass `output: "mermaid"` for editable markup) |
| A chart, graph, plot, or data visualization | `describe_chart` (pass `output: "csv"` or `"json"` for the underlying numbers) |
| Terminal error, stack trace, exception dialog | `diagnose_error` |
| An image to reproduce as a text-to-image (diffusion) prompt | `image_to_prompt` |
| Two images to diff (before/after, regression) | `compare_images` |
| None of the above, or unclear | `read_image` |

## Image input formats

All tools accept these input shapes (pick whichever is easiest for the caller):

- **`image_path`** (preferred): an absolute path to a local file.
- **`image_data`**: a `data:image/<mime>;base64,<data>` URI. Decoded to a temp file internally.
- **`image_url`**: a `file://` URI. (Remote `http(s)://` URLs are **rejected** — `llama-server` is localhost-only.)
- **`question`** (optional): a specific question about the image. Sent to the model as the user-turn prompt.

`compare_images` takes `images` as an array of exactly two image refs (or `image_a` + `image_b` aliases).

## Latency expectations

**Budget 30–60 seconds per call.** Each call:

1. Picks a model per tool based on your hardware — the v6 benchmark crowns a
   different best model per tool, so `localvision` routes: `qwen3-vl-8b` for
   `read_image`/`extract_text`/`describe_chart`/`extract_table`, `qwen3.5-4b-q8`
   for `extract_code`/`describe_ui`/`describe_diagram`/`diagnose_error` (it edges
   the 8B there); on tiny machines where neither fits, `qwen3.5-4b`. (`doctor`
   shows the routing; `[tools.<id>]` in config overrides it; `--model` forces one.)
2. Loads the model into a `llama-server` subprocess (cold start: 5–10 s; subsequent calls on the same model: 0.1–0.5 s).
3. Runs one inference and returns text.

The subprocess stays resident for 5 minutes after the last call (configurable via `~/.localvision/config.toml` → `idle_timeout`). Subsequent calls within that window skip the load step. Note: per-tool routing means a mixed-tool session may switch models (a cold reload per switch).

## Privacy

- The image bytes never leave this machine.
- No telemetry. No anonymous stats. No crash reports that include image content.
- The MCP's only outbound HTTP is to `huggingface.co/froggeric/` (for model-file downloads on first run).
- Logs reference images by SHA256 (hash of bytes), never by content.

## Quality caveats

Local VLMs are good but not frontier-class. In our v6 benchmark (11 base models × Q4/Q8 × think/nothink, 30 images, 3 runs), every model got at least one basic-facts test wrong. Cross-check critical extractions.

If accuracy is paramount (medical, legal, security-relevant), say so to the user and suggest routing those images through Claude's native vision or another frontier model.

## After invoking

- **Cite the source.** Tell the user which model was used — e.g. "Analyzed locally with Qwen3-VL 8B" (the default), or "Qwen3.6-27B" if you passed `--model qwen3.6-27b` for max quality.
- **Don't pretend you saw the image.** You didn't — you got a description. If the user asks follow-up questions you can't answer from the description, call the tool again with the new question.
- **Surface uncertainty.** If the tool's output mentions ambiguity ("I see 5 or 6 people", "the text appears to say X"), pass that uncertainty on to the user. Don't collapse it to a confident answer.

## Examples

### Example 1 — UI screenshot

User: *"Here's a screenshot of the dashboard. Why is the deploy button greyed out? `screenshot.png`"*

Call:
```
describe_ui({ image_path: "/absolute/path/to/screenshot.png" })
```

The model returns layout, components, text labels, and crucially any error/feedback messages — which is what you need to answer the user's question.

### Example 2 — Code in an image

User: *"What does this code do? It's from a textbook."*

Call:
```
extract_code({ image_path: "/tmp/page-42.png" })
```

Returns the code as a fenced block with detected language. You then read the code and explain.

### Example 3 — Error screenshot

User: *"Why is my build failing?" + terminal screenshot*

Call:
```
diagnose_error({ image_path: "/tmp/build-error.png" })
```

Returns error type, root-cause message, and likely cause in a structured format. Use that to guide your debugging.

### Example 4 — Compare two images

User: *"Did this refactor change anything visible? Here's before.png and after.png."*

Call:
```
compare_images({ images: [
  { image_path: "/abs/path/before.png" },
  { image_path: "/abs/path/after.png" }
] })
```

Returns a bullet list of visible differences.

### Example 5 — Wrong tool

User: *"What's in this photo?"*

Wrong: `extract_text` (there may be no text).
Wrong: `describe_ui` (it's not a UI).
Right: `read_image` (generic description).

When in doubt, `read_image` is the safe default.
