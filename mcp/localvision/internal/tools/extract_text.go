package tools

// extract_text is OCR. The model is instructed to emit every visible
// character verbatim with no commentary. Preferred on qwen3-vl-8b.
type extractTextTool struct{}

func (extractTextTool) ID() string { return idExtractText }

func (extractTextTool) Description() string {
	return "Extract ALL text from an image verbatim (OCR). Preserves formatting, indentation, and line breaks. Use for screenshots of documents, terminals, and any text-heavy image." + latencyHint
}

func (extractTextTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2048 — long enough for a full terminal screenshot or
// document page. Per F4.10.
func (extractTextTool) MaxTokens() int { return 2048 }

func (extractTextTool) SystemPrompt() string { return promptExtractText }

func (t extractTextTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

func (extractTextTool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
