package tools

import "strings"

// extract_table emits Markdown tables. ParseOutput strips any prose outside
// the tables so the result is a clean pasteable block. Served by the default
// qwen3-vl-8b.
type extractTableTool struct{}

func (extractTableTool) ID() string { return idExtractTable }

func (extractTableTool) Description() string {
	return "Extract tables from an image as Markdown tables. Preserves column alignment. Use for screenshots of spreadsheets, financial reports, comparison tables, and any gridded numeric or text data." + latencyHint
}

func (extractTableTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2048 — wide tables can produce many characters per row.
// Per F4.10.
func (extractTableTool) MaxTokens() int { return 2048 }

func (extractTableTool) SystemPrompt() string { return promptExtractTable }

func (t extractTableTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

// ParseOutput strips non-table prose. If the model emitted nothing that
// looks like a Markdown table (no pipe characters), returns the raw text
// verbatim — better to surface the model's "I couldn't find a table"
// message than an empty string.
func (extractTableTool) ParseOutput(raw string) (any, error) {
	tables := extractMarkdownTables(raw)
	if strings.TrimSpace(tables) == "" {
		return raw, nil
	}
	return tables, nil
}
