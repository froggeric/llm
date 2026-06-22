package main

import (
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveToolID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", tools.ToolReadImage},
		{"ocr", tools.ToolExtractText},
		{"code", tools.ToolExtractCode},
		{"table", tools.ToolExtractTable},
		{"ui", tools.ToolDescribeUI},
		{"diagram", tools.ToolDescribeDiagram},
		{"chart", tools.ToolDescribeChart},
		{"error", tools.ToolDiagnoseError},
		{"compare", tools.ToolCompareImages},
		{"describe", tools.ToolReadImage},
		{"read", tools.ToolReadImage},
		{"OCR", tools.ToolExtractText}, // case-insensitive
	}
	for _, c := range cases {
		got, err := resolveToolID(c.in)
		require.NoError(t, err, "type %q", c.in)
		assert.Equal(t, c.want, got, "type %q", c.in)
	}

	if _, err := resolveToolID("bogus"); err == nil {
		t.Fatal("expected error for unknown --type")
	}
}

func TestSplitArgs(t *testing.T) {
	t.Run("positional only", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"img.png"})
		assert.Empty(t, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("flags first", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--type", "ocr", "img.png"})
		assert.Equal(t, []string{"--type", "ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("flags after positional", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"img.png", "--type", "ocr"})
		assert.Equal(t, []string{"--type", "ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("bool flag consumes no value", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--verbose", "a.png", "b.png"})
		assert.Equal(t, []string{"--verbose"}, flagArgs)
		assert.Equal(t, []string{"a.png", "b.png"}, pos)
	})
	t.Run("equals form", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--type=ocr", "img.png"})
		assert.Equal(t, []string{"--type=ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("stdin sentinel", func(t *testing.T) {
		_, pos := splitArgs([]string{"-"})
		assert.Equal(t, []string{"-"}, pos)
	})
}
