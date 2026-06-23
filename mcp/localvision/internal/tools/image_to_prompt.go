package tools

// image_to_prompt reverses an image into a text-to-image (diffusion) prompt
// that could recreate it: subject, medium/style, composition, lighting, color
// palette, mood, and a paste-ready comma-separated tag line. The optional
// question argument steers the output toward a specific generator or style.
//
// Preferred on qwen3-vl-8b (constrained) and qwen3.6-27b (mainstream), like the
// other perception tools.
type imageToPromptTool struct{}

func (imageToPromptTool) ID() string { return idImageToPrompt }

func (imageToPromptTool) Description() string {
	return "Generate a text-to-image prompt that could recreate an image: subject, medium/style, composition and camera details, lighting, color palette, and mood, plus a ready-to-paste comma-separated tag line. Use to reverse-engineer an image into a reproducible generation prompt for Midjourney, SDXL, Flux, DALL·E, etc." + latencyHint
}

// InputSchema is a single image plus an optional question that steers the
// output (e.g. "Midjourney v6", "SDXL", "a watercolor reinterpretation").
func (imageToPromptTool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["question"] = map[string]any{
		"type":        "string",
		"description": "Optional: steer the output toward a target generator or style (e.g. \"Midjourney v6\", \"SDXL\", \"a watercolor reinterpretation\").",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 1024 — a rich structured prompt across five sections plus a tag
// line fits comfortably; tight enough to keep the model from rambling.
func (imageToPromptTool) MaxTokens() int { return 1024 }

func (imageToPromptTool) SystemPrompt() string { return promptImageToPrompt }

// BuildRequest requires exactly one image and honors an optional question so
// the caller can target a specific generator or style.
func (t imageToPromptTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, true), []string{ref.LocalPath}, nil
}

func (imageToPromptTool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
