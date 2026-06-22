package tools

import "strings"

// extract_code pulls code out of an image. The model is instructed to emit
// a fenced block; ParseOutput strips any prose outside the fence so the
// returned text is always directly pasteable. Preferred on qwen3-vl-8b.
type extractCodeTool struct{}

func (extractCodeTool) ID() string { return idExtractCode }

func (extractCodeTool) Description() string {
	return "Extract source code from an image. Detects the programming language and returns the code in a fenced block, preserving indentation. Use for screenshots of code editors, terminals showing code, or printed source listings." + latencyHint
}

func (extractCodeTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 4096 — generous to accommodate a full screen of dense code.
// Per F4.10.
func (extractCodeTool) MaxTokens() int { return 4096 }

func (extractCodeTool) SystemPrompt() string { return promptExtractCode }

func (t extractCodeTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

// ParseOutput strips prose outside the fenced block. Returns a small struct
// with the language tag and the code body so structured consumers can route
// by language; text consumers see the code body via a TextContent fallback.
//
// If the model didn't emit a fence (e.g. it returned the code as plain
// text), ParseOutput returns the trimmed input as the "code" field with an
// empty language — better than failing entirely.
func (extractCodeTool) ParseOutput(raw string) (any, error) {
	lang, code := extractCodeBlock(raw)
	if code == "" {
		// Defensive: empty code body is almost always a parsing bug or a
		// model that refused to produce output. Return whatever we got so
		// the caller can see it.
		code = strings.TrimSpace(raw)
	}
	return map[string]any{
		"language": lang,
		"code":     code,
	}, nil
}
