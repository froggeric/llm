package tools

import (
	"errors"
	"fmt"
)

// compare_images is the only multi-image tool (F4.9). It takes exactly two
// images and reports the differences. The model is instructed to focus on
// changes rather than describing each image independently.
//
// Preferred on gemma4-26b-a4b when available.
type compareImagesTool struct{}

func (compareImagesTool) ID() string { return idCompareImages }

func (compareImagesTool) Description() string {
	return "Compare two images and describe the differences: added or removed text, layout shifts, color or style changes, new or missing components. Returns Markdown bullets, one bullet per change. Use for diffing screenshots before and after a change." + latencyHint
}

// InputSchema accepts images as an array of exactly two refs (F4.9). Each
// element is an object with the standard image_path / image_data /
// image_url fields. The schema rejects a single image explicitly via
// minItems=2 maxItems=2; BuildRequest returns a clear error for any other
// count.
func (compareImagesTool) InputSchema() map[string]any {
	itemProps := commonSchemaProperties()
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"images": map[string]any{
				"type":        "array",
				"description": "Exactly two images to compare.",
				"items": map[string]any{
					"type":       "object",
					"properties": itemProps,
					"oneOf":      commonOneOf(),
				},
				"minItems": 2,
				"maxItems": 2,
			},
			// image_a / image_b aliases for clients that prefer named
			// arguments over positional arrays. Track B's mcpserver
			// normalizer does NOT currently understand these; the alias is
			// here for forward compatibility. Both forms collapse into
			// input.Images at the ToolInput layer.
			"image_a": map[string]any{
				"type":        "string",
				"description": "First image (alias; prefer the images array).",
			},
			"image_b": map[string]any{
				"type":        "string",
				"description": "Second image (alias; prefer the images array).",
			},
		},
		"oneOf": []map[string]any{
			{"required": []string{"images"}},
			{"required": []string{"image_a", "image_b"}},
		},
	}
}

// MaxTokens is 1500 — a typical diff is a handful of bullets, but a large
// diff (e.g. two pages of UI with many small changes) needs headroom.
// Per F4.10.
func (compareImagesTool) MaxTokens() int { return 1500 }

func (compareImagesTool) SystemPrompt() string { return promptCompareImages }

// BuildRequest requires exactly two images. It returns them in the order
// supplied so the model can refer to "image 1" and "image 2" if useful
// (the prompt tells it not to describe them separately, but order can
// still matter for sequences like before/after).
func (t compareImagesTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	if len(input.Images) != 2 {
		return "", "", nil, fmt.Errorf(
			"compare_images requires exactly 2 images, got %d "+
				"(supply an images array of length 2, or both image_a and image_b)",
			len(input.Images),
		)
	}
	for i, ref := range input.Images {
		if ref.LocalPath == "" {
			return "", "", nil, fmt.Errorf("images[%d]: missing LocalPath", i)
		}
	}
	prompt := "You are given two images. Image 1 is the first; image 2 is the second. Describe what is DIFFERENT between them per the system instructions."
	return t.SystemPrompt(), prompt, []string{input.Images[0].LocalPath, input.Images[1].LocalPath}, nil
}

func (compareImagesTool) ParseOutput(raw string) (any, error) {
	if raw == "" {
		return "", errors.New("model returned empty output")
	}
	return passthroughOutput(raw)
}
