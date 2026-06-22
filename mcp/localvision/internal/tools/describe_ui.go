package tools

// describe_ui produces a structured description of a UI screenshot. Used by
// agents that need to navigate the application (find a button, read an error
// message, verify state). Preferred on qwen3-vl-8b.
type describeUITool struct{}

func (describeUITool) ID() string { return idDescribeUI }

func (describeUITool) Description() string {
	return "Describe a UI screenshot: layout, every visible component with text labels, feedback or error messages, and interactive elements. Use for navigating applications, verifying state, and reading on-screen messages." + latencyHint
}

func (describeUITool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["question"] = map[string]any{
		"type":        "string",
		"description": "Optional specific question about the UI (e.g. 'where is the submit button?').",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2000 — enough for a busy dashboard with many components.
// Per F4.10.
func (describeUITool) MaxTokens() int { return 2000 }

func (describeUITool) SystemPrompt() string { return promptDescribeUI }

func (t describeUITool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, true), []string{ref.LocalPath}, nil
}

func (describeUITool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
