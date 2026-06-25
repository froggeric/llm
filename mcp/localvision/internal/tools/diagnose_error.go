package tools

// diagnose_error parses error screenshots and stack traces. The constrained
// 800-token budget forces the model to extract only the load-bearing
// information. Preferred on qwen3-vl-8b.
type diagnoseErrorTool struct{}

func (diagnoseErrorTool) ID() string { return idDiagnoseError }

func (diagnoseErrorTool) Description() string {
	return "Diagnose an error screenshot or stack trace. Reports error type, root-cause message verbatim, most relevant file:line, and a one-sentence likely cause. Use for debugging from exception screens, crash dialogs, and terminal stack traces." + latencyHint
}

func (diagnoseErrorTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 800 — small to force a terse diagnosis. Per F4.10.
func (diagnoseErrorTool) MaxTokens() int { return 800 }

func (diagnoseErrorTool) SystemPrompt() string { return promptDiagnoseError }

func (t diagnoseErrorTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

func (diagnoseErrorTool) ParseOutput(_ ToolInput, raw string) (any, error) {
	return passthroughOutput(raw)
}
