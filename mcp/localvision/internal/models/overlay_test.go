package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlay_test.go pins the overlay-merge behavior for chat_template_kwargs
// (Tier-1 #7): an overlay setting it must reach the merged ModelSpec, and an
// overlay omitting it must not clobber an existing value.

func TestMergeModelSpec_ChatTemplateKwargs_Set(t *testing.T) {
	dst := ModelSpec{
		DisplayName:        "base",
		ChatTemplateKwargs: map[string]any{"enable_thinking": true},
	}
	overlay := overlayModelSpec{
		ChatTemplateKwargs: &map[string]any{"enable_thinking": false},
	}
	got := mergeModelSpec(dst, overlay, "test", "m")
	require.Contains(t, got.ChatTemplateKwargs, "enable_thinking")
	assert.Equal(t, false, got.ChatTemplateKwargs["enable_thinking"],
		"overlay kwargs must replace the existing value")
}

func TestMergeModelSpec_ChatTemplateKwargs_AbsentDoesNotClobber(t *testing.T) {
	dst := ModelSpec{
		DisplayName:        "base",
		ChatTemplateKwargs: map[string]any{"enable_thinking": true},
	}
	// Overlay sets an unrelated field but NOT chat_template_kwargs.
	overlay := overlayModelSpec{Notes: strPtr("updated")}
	got := mergeModelSpec(dst, overlay, "test", "m")
	assert.Equal(t, true, got.ChatTemplateKwargs["enable_thinking"],
		"an overlay omitting chat_template_kwargs must not clobber the existing map")
}

func TestApplyOverlayFile_ChatTemplateKwargs_DecodesAndMerges(t *testing.T) {
	// End-to-end: an overlay TOML with chat_template_kwargs decodes into the
	// pointer field and merges onto an existing model.
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "override.toml")
	tomlContent := `schema_version = 1
[models."qwen3-vl-8b"]
chat_template_kwargs = {enable_thinking = false}
`
	require.NoError(t, os.WriteFile(overlayPath, []byte(tomlContent), 0o644))

	into := &Catalog{
		SchemaVersion: 1,
		Models: map[string]ModelSpec{
			"qwen3-vl-8b": {
				DisplayName:        "Qwen3-VL 8B",
				ChatTemplateKwargs: map[string]any{"enable_thinking": true},
			},
		},
	}
	require.NoError(t, applyOverlayFile(overlayPath, "override.toml", into))

	m := into.Models["qwen3-vl-8b"]
	require.Contains(t, m.ChatTemplateKwargs, "enable_thinking")
	assert.Equal(t, false, m.ChatTemplateKwargs["enable_thinking"],
		"overlay chat_template_kwargs must reach the merged model spec")
}

// strPtr is a small helper for building overlay pointer fields in tests.
func strPtr(s string) *string { return &s }
