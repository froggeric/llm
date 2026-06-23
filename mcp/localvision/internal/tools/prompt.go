package tools

// Task-tuned system prompts for each of the 10 tools.
//
// These prompts are the entire reason we have 10 tools instead of one generic
// "describe image" tool: each prompt steers the model toward a specific output
// shape (Markdown tables, fenced code, terse bullet lists) so downstream
// consumers (text-only LLMs) get a structure they can reason about.
//
// Guidelines applied across all prompts:
//   - Short and specific. The model has 800-4096 output tokens per tool;
//     a verbose prompt wastes tokens on instruction-following overhead.
//   - Always specify output format ("Markdown headings", "fenced code block").
//   - Always specify what NOT to output ("No explanation before or after").
//   - "Verbatim" appears in extract_* prompts to discourage paraphrasing.
//
// Per-tool max_tokens are encoded in the Tool.MaxTokens() method, not here.

// promptReadImage is the generic "describe this image" prompt.
const promptReadImage = `Describe this image in detail. Include visible text (verbatim), objects, people, layout, colors, and notable features. Use Markdown headings. Do not invent details that are not visible. If text is illegible, say so rather than guessing.`

// promptExtractText is OCR: extract every visible character verbatim.
const promptExtractText = `Extract ALL text from the image verbatim. Preserve formatting, indentation, line breaks, and punctuation exactly as they appear. Output the text only — no commentary, no Markdown, no fences. If multiple distinct text blocks are visible, separate them with a blank line.`

// promptExtractCode detects language and emits a fenced block. ParseOutput
// strips any prose outside the fence.
const promptExtractCode = `You are an expert code reader. Extract the code from the image verbatim. Detect the programming language. Output ONLY the code in a fenced code block (triple-backtick followed by the language name on the opening line). Preserve all indentation exactly. No explanation before or after the fence.`

// promptExtractTable produces Markdown tables. The prompt instructs the model
// to preserve column alignment, which matters for downstream diff tools.
const promptExtractTable = `Extract tables from the image. Output each table as a Markdown table (header row, separator row, data rows). Preserve column alignment. If multiple tables are present, separate them with a horizontal rule (---). If a cell spans multiple lines, join them with a space. Do not include any text outside the Markdown tables.`

// promptDescribeUI lists every visible component for use by an agent
// navigating the application.
const promptDescribeUI = `Describe this UI screenshot. List:
(1) Overall layout (regions, panels, navigation).
(2) Every visible component with its text label verbatim.
(3) Any feedback, status, or error messages.
(4) Interactive elements (buttons, inputs, links) and their state.
Use Markdown headings for each section. Be terse — one bullet per component.`

// promptDescribeDiagram handles architecture / flowchart / ER / sequence
// diagrams. The "diagram type" first line lets downstream consumers route
// to the right reasoning tool.
const promptDescribeDiagram = `Describe this technical diagram. Report:
(1) Diagram type (architecture, flowchart, ER, sequence, state machine, etc.).
(2) All named components (boxes, nodes, classes, tables).
(3) All connections with their labels and protocols.
(4) A one-sentence summary of what the diagram represents.
Use Markdown headings for each section.`

// promptDescribeDiagramMermaid emits editable Mermaid markup (G5) — an
// image → round-trippable diagram.
const promptDescribeDiagramMermaid = `You are an expert at reading technical diagrams. Convert the diagram in this image into Mermaid markup that reproduces it.
Rules:
- Detect the diagram kind and emit the matching Mermaid type: flowchart TD/LR (boxes and arrows), sequenceDiagram (actors and interactions across lifelines), classDiagram (UML classes), erDiagram (entities and relationships), stateDiagram-v2 (state machines), or graph.
- Output ONLY a fenced mermaid code block (triple-backtick then "mermaid") containing valid Mermaid. No prose before or after.
- Preserve every named component and every labeled connection. Use the exact labels from the image as node IDs and edge labels (quote them if they contain spaces or special characters).
- Keep node and edge text verbatim. If the image is not a diagram, emit an empty mermaid block with a one-line %% comment explaining why.`

// diagramPromptFor returns the system prompt for describe_diagram given the
// requested output mode. "prose" (the default) is the canonical
// promptDescribeDiagram returned by SystemPrompt(); mermaid is the G5 mode.
func diagramPromptFor(mode string) string {
	switch mode {
	case "mermaid":
		return promptDescribeDiagramMermaid
	default:
		return promptDescribeDiagram
	}
}

// promptDescribeChart is for data visualizations. Terse, structured output
// for downstream numerical reasoning.
const promptDescribeChart = `You are a data analyst. Describe the chart in this image. Report:
(1) Chart type (bar, line, pie, scatter, heatmap, etc.).
(2) Axes with units and ranges.
(3) Data series with names.
(4) Notable values, outliers, and inflection points (with numbers).
(5) Overall trend.
Use Markdown headings. Be terse.`

// promptDescribeChartCSV emits the underlying numbers as CSV (G4). The fenced
// ```csv block is designed to paste straight into a spreadsheet.
const promptDescribeChartCSV = `You are a data analyst. Extract the underlying data of the chart in this image as CSV.
Output ONLY a fenced csv code block (triple-backtick then "csv") containing:
- A header row: the first column is the category or x-axis label, followed by one column per data series named exactly as in the legend.
- One data row per category/point. Numeric values only — no units, no thousands separators, no currency symbols, no percent signs inside the number.
Estimate any value that must be read off the axes. Preserve the left-to-right / category order shown. Do not include any prose, explanation, or commentary outside the csv block.`

// promptDescribeChartJSON emits the underlying numbers as a JSON object (G4),
// returned as real structured content for machine consumers.
const promptDescribeChartJSON = `You are a data analyst. Extract the underlying data of the chart in this image as a single JSON object.
Output ONLY valid JSON — no code fence, no prose, nothing before or after — with this shape:
{"chart_type": "bar|line|pie|scatter|heatmap", "title": "", "axes": {"x": {"label": "", "unit": ""}, "y": {"label": "", "unit": ""}}, "series": [{"name": "", "points": [["x", 0]]}]}
Rules: numeric values are JSON numbers (not strings). "title" is the visible chart title or "". Estimate any value read off the axes. Each series has a name and a points array of [x, y] pairs. Keep the order shown in the chart.`

// chartPromptFor returns the system prompt for describe_chart given the
// requested output mode. "prose" (the default) is the canonical
// promptDescribeChart returned by SystemPrompt(); csv/json are the G4 modes.
func chartPromptFor(mode string) string {
	switch mode {
	case "csv":
		return promptDescribeChartCSV
	case "json":
		return promptDescribeChartJSON
	default:
		return promptDescribeChart
	}
}

// promptDiagnoseError is for stack-trace / exception screenshots. The
// constrained output budget (800 tokens) forces the model to be terse.
const promptDiagnoseError = `You are a debugging assistant. This image shows an error or stack trace. Report:
(1) Error type (exception class / error code).
(2) Root-cause message verbatim.
(3) Most relevant file:line in the stack trace.
(4) One-sentence likely cause.
Use Markdown. Be terse.`

// promptImageToPrompt reverses an image into a text-to-image (diffusion)
// prompt that could recreate it. Output is structured into five Markdown
// sections ending with a paste-ready comma-separated tag line. Steering
// toward a specific generator (Midjourney, SDXL, Flux, DALL·E) is done via
// the optional question argument; the default prompt is generator-agnostic.
const promptImageToPrompt = `You are an expert prompt engineer for text-to-image diffusion models (Midjourney, SDXL, Flux, DALL·E). Analyze the image and produce a detailed prompt that could recreate it. Report:
(1) Subject and scene — what is depicted.
(2) Medium and art style (e.g. photograph, 3D render, oil painting, anime, pixel art).
(3) Composition, framing, and camera/render details (lens, angle, lighting, depth of field).
(4) Color palette and mood.
(5) A single comma-separated tag line ready to paste into a diffusion model, distilling the points above.
Use Markdown headings for each section. Do not invent elements that are not visible in the image. Output only the five sections — no preamble, no closing remarks.`

// promptCompareImages is the only multi-image tool. The prompt makes the
// "focus on differences" intent explicit so the model does not produce two
// unrelated descriptions.
const promptCompareImages = `You are given two images. Describe what is DIFFERENT between them. Focus on visible changes: added or removed text, layout shifts, color or style changes, new or missing components, and repositioned elements. If the two images are identical, say so explicitly. Use Markdown bullets, one bullet per change. Do not describe the images separately.`
