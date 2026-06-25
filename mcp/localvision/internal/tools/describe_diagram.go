package tools

import "strings"

// describe_diagram handles architecture, flowchart, ER, sequence, and similar
// technical diagrams. By default it emits a structured Markdown report; pass
// output="mermaid" to get editable Mermaid markup that reproduces the diagram
// (G5). Served by the default qwen3-vl-8b.
type describeDiagramTool struct{}

func (describeDiagramTool) ID() string { return idDescribeDiagram }

func (describeDiagramTool) Description() string {
	return "Describe a technical diagram (architecture, flowchart, ER, sequence, state machine). Reports diagram type, named components, labeled connections, and a one-sentence summary. Set output=\"mermaid\" to get editable Mermaid markup that reproduces the diagram. Use for understanding system design from a single image." + latencyHint
}

func (describeDiagramTool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["output"] = map[string]any{
		"type":        "string",
		"enum":        []string{"prose", "mermaid"},
		"description": "Output representation: prose (default — a structured description) or mermaid (editable Mermaid markup that reproduces the diagram).",
		"default":     "prose",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2000 — enough for a complex diagram with many boxes and arrows.
// Per F4.10.
func (describeDiagramTool) MaxTokens() int { return 2000 }

// SystemPrompt returns the prose prompt (the default mode); the mermaid mode
// uses diagramPromptFor("mermaid") from BuildRequest. Stays prose so the
// default BuildRequest path returns SystemPrompt() (TestBuildRequestSanity).
func (describeDiagramTool) SystemPrompt() string { return promptDescribeDiagram }

func (t describeDiagramTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	mode, err := outputMode(input.Extra, []string{"mermaid"})
	if err != nil {
		return "", "", nil, err
	}
	// mode=="prose" (absent/empty) → diagramPromptFor returns promptDescribeDiagram,
	// which equals t.SystemPrompt() — the invariant TestBuildRequestSanity checks.
	return diagramPromptFor(mode), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

// ParseOutput: mermaid mode → strip the fenced ```mermaid block and return the
// pasteable Mermaid text (raw if the model omitted the fence); prose mode →
// passthrough UNCHANGED. The mode is resolved from input.Extra (the same source
// BuildRequest uses) so a prose report that happens to contain a mermaid fence
// isn't mis-stripped.
func (describeDiagramTool) ParseOutput(input ToolInput, raw string) (any, error) {
	mode, err := outputMode(input.Extra, []string{"mermaid"})
	if err != nil {
		return nil, err
	}
	if mode == "mermaid" {
		lang, body := extractCodeBlock(raw)
		if strings.EqualFold(lang, "mermaid") {
			return strings.TrimSpace(body), nil
		}
	}
	return raw, nil
}
