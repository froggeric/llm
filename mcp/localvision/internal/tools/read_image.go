package tools

// read_image is the generic "describe this image" tool. Used when the caller
// doesn't have a more specific intent. The default qwen3-vl-8b lists this tool
// in preferred_for (qwen3.5-4b is the constrained-hardware fallback).
type readImageTool struct{}

// ID returns the tool identifier surfaced to MCP clients.
func (readImageTool) ID() string { return idReadImage }

// Description carries the use case + latency hint (F1.11, F5.3). Track B's
// mcpserver appends an additional "Latency:" suffix on top of this.
func (readImageTool) Description() string {
	return "Describe an image in detail: visible text, objects, people, layout, colors, and notable features. Use this when no more specific vision tool applies." + latencyHint
}

// InputSchema is a JSON Schema object with image_path / image_data /
// image_url (F1.10) plus an optional question for steering the description.
func (readImageTool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["question"] = map[string]any{
		"type":        "string",
		"description": "Optional specific question about the image (steers the description).",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 1500 — enough for a detailed description of a busy image,
// not enough to ramble. Per F4.10.
func (readImageTool) MaxTokens() int { return 1500 }

// SystemPrompt returns the task-tuned prompt (see prompt.go).
func (readImageTool) SystemPrompt() string { return promptReadImage }

// BuildRequest turns parsed input into the prompts + image paths the model
// needs. Returns an error if the caller didn't supply exactly one image.
func (t readImageTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, true), []string{ref.LocalPath}, nil
}

// ParseOutput returns the model's text unchanged.
func (readImageTool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
