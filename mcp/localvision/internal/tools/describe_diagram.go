package tools

// describe_diagram handles architecture, flowchart, ER, sequence, and
// similar technical diagrams. Emits a structured Markdown report. Served by
// the default qwen3-vl-8b.
type describeDiagramTool struct{}

func (describeDiagramTool) ID() string { return idDescribeDiagram }

func (describeDiagramTool) Description() string {
	return "Describe a technical diagram (architecture, flowchart, ER, sequence, state machine). Reports diagram type, named components, labeled connections, and a one-sentence summary. Use for understanding system design from a single image." + latencyHint
}

func (describeDiagramTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2000 — enough for a complex diagram with many boxes and arrows.
// Per F4.10.
func (describeDiagramTool) MaxTokens() int { return 2000 }

func (describeDiagramTool) SystemPrompt() string { return promptDescribeDiagram }

func (t describeDiagramTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

func (describeDiagramTool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
