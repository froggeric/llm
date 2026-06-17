package tools

import (
	"errors"
	"fmt"
	"strings"
)

// commonSchemaProperties returns the standard image-input properties shared
// by all single-image tools. Tools add their own task-specific fields on top.
//
// F1.10: file path is primary, data: URI is fallback, file:// is alternative,
// remote http(s):// is rejected. The oneOf clause mirrors what Track B's
// mcpserver normalizer expects (any one of image_path / image_data /
// image_url) and also lets strict MCP clients validate inputs before sending.
func commonSchemaProperties() map[string]any {
	return map[string]any{
		"image_path": map[string]any{
			"type":        "string",
			"description": "Absolute path to the image file (primary format).",
		},
		"image_data": map[string]any{
			"type":        "string",
			"description": "data:image/<mime>;base64,<data> URI (fallback when the image is not on disk).",
		},
		"image_url": map[string]any{
			"type":        "string",
			"description": "file:// URI pointing at a local image (alternative to image_path).",
		},
	}
}

// commonOneOf returns the JSON-Schema oneOf clause requiring exactly one of
// the three image input forms. Strict clients use this; permissive clients
// (including ours) accept any non-empty image_* key.
func commonOneOf() []map[string]any {
	return []map[string]any{
		{"required": []string{"image_path"}},
		{"required": []string{"image_data"}},
		{"required": []string{"image_url"}},
	}
}

// singleImageUserPrompt builds a deterministic user prompt for single-image
// tools. If the tool was passed an explicit question via Extra["question"],
// it's appended so the model answers that specific question rather than
// describing the image generically. Tools that don't take a question
// (extract_*, diagnose_error) ignore the field.
func singleImageUserPrompt(extra map[string]any, allowQuestion bool) string {
	if !allowQuestion {
		return "Analyze the attached image according to the system instructions."
	}
	if q, ok := extra["question"].(string); ok && strings.TrimSpace(q) != "" {
		return "Analyze the attached image according to the system instructions, with attention to this question: " + strings.TrimSpace(q)
	}
	return "Analyze the attached image according to the system instructions."
}

// requireSingleImage extracts exactly one ImageRef from input, returning an
// error if zero or more than one is supplied. All single-image tools use
// this; only compare_images accepts a slice.
func requireSingleImage(input ToolInput) (ImageRef, error) {
	if len(input.Images) == 0 {
		return ImageRef{}, errors.New("tool requires exactly one image, got 0 (supply image_path, image_data, or image_url)")
	}
	if len(input.Images) > 1 {
		return ImageRef{}, fmt.Errorf("tool requires exactly one image, got %d (use compare_images for multi-image input)", len(input.Images))
	}
	return input.Images[0], nil
}

// passthroughOutput is the ParseOutput implementation for tools that return
// the model's raw text unchanged. Most tools use this; the few that
// post-process (extract_code, extract_table) override ParseOutput.
func passthroughOutput(raw string) (any, error) { return raw, nil }

// extractCodeBlock pulls the first fenced code block out of raw, returning
// the fenced contents (without the backticks) and the language tag, if any.
// Used by extract_code's ParseOutput. Returns the trimmed input verbatim if
// no fence is found — a model that ignored the instruction is still better
// than an empty result.
//
// The function tolerates:
//   - Leading/trailing prose outside the fence
//   - Fences with 3 or more backticks (``` or ````)
//   - Fences with a language tag (```python) or without (```)
//   - Trailing whitespace on the fence line
//   - CRLF line endings
func extractCodeBlock(raw string) (lang, code string) {
	// Normalize line endings so we can split on "\n" safely.
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	// Find the opening fence: a line starting with 3+ backticks.
	openIdx := -1
	fenceLen := 0
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(trimmed, "```") {
			backticks := 0
			for backticks < len(trimmed) && trimmed[backticks] == '`' {
				backticks++
			}
			if backticks >= 3 {
				openIdx = i
				fenceLen = backticks
				// Language tag is everything after the fence on the same line.
				lang = strings.TrimSpace(trimmed[fenceLen:])
				break
			}
		}
	}

	if openIdx < 0 {
		// No fence found. Return the trimmed input as code; lang stays empty.
		return "", strings.TrimSpace(raw)
	}

	// Find the closing fence: a line whose content is exactly fenceLen-or-more
	// backticks (and nothing else, modulo whitespace).
	closingFence := strings.Repeat("`", fenceLen)
	for i := openIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, closingFence) {
			// Verify the rest of the line is just backticks (allow longer
			// fences as closer per CommonMark).
			rest := trimmed
			for len(rest) > 0 && rest[0] == '`' {
				rest = rest[1:]
			}
			if rest == "" {
				code := strings.Join(lines[openIdx+1:i], "\n")
				return lang, code
			}
		}
	}

	// No closing fence. Take everything after the opening fence.
	code = strings.Join(lines[openIdx+1:], "\n")
	return lang, code
}

// extractMarkdownTables returns the substring of raw that consists of
// Markdown tables (lines containing |) plus the surrounding horizontal
// rules. Used by extract_table's ParseOutput to strip any prose the model
// emitted outside the tables despite instructions.
//
// The algorithm: find every line that contains a pipe; group consecutive
// such lines into table blocks; preserve horizontal rules (---) between
// them; drop everything else.
func extractMarkdownTables(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var out []string
	inTable := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		isPipe := strings.Contains(trimmed, "|")
		isRule := trimmed == "---" || trimmed == "***" || trimmed == "___"
		if isPipe {
			out = append(out, ln)
			inTable = true
			continue
		}
		if isRule && inTable {
			// Keep horizontal rules only when they sit between tables (i.e.
			// we just emitted table content). Drop them otherwise to avoid
			// leading/trailing rules.
			out = append(out, ln)
			continue
		}
		// Non-pipe, non-rule line: a gap in the table. Emit a blank line
		// to preserve paragraph structure but don't carry the prose.
		if inTable {
			out = append(out, "")
			inTable = false
		}
	}
	// Trim trailing blank/rule lines.
	for len(out) > 0 {
		t := strings.TrimSpace(out[len(out)-1])
		if t == "" || t == "---" || t == "***" || t == "___" {
			out = out[:len(out)-1]
			continue
		}
		break
	}
	return strings.Join(out, "\n")
}
