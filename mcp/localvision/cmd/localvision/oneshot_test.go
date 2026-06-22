package main

import (
	"flag"
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

// testFlagSet builds a FlagSet mirroring the one-shot flags so splitArgs can
// detect bool flags. Kept in sync with runOneShot's registrations.
func testFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var b bool
	var s string
	fs.BoolVar(&b, "verbose", false, "")
	fs.BoolVar(&b, "recursive", false, "")
	fs.BoolVar(&b, "meta", false, "")
	fs.StringVar(&s, "type", "", "")
	fs.StringVar(&s, "model", "", "")
	return fs
}

func TestSplitArgs(t *testing.T) {
	fs := testFlagSet()
	t.Run("positional only", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"img.png"}, fs)
		assert.Empty(t, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("flags first", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--type", "ocr", "img.png"}, fs)
		assert.Equal(t, []string{"--type", "ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("flags after positional", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"img.png", "--type", "ocr"}, fs)
		assert.Equal(t, []string{"--type", "ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("bool flag consumes no value", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--verbose", "a.png", "b.png"}, fs)
		assert.Equal(t, []string{"--verbose"}, flagArgs)
		assert.Equal(t, []string{"a.png", "b.png"}, pos)
	})
	t.Run("bool flags interspersed with positionals", func(t *testing.T) {
		// Regression (L1): a bool flag right before a positional must not
		// swallow that positional as its "value".
		flagArgs, pos := splitArgs([]string{"a.png", "--meta", "b.png", "--recursive", "c.png"}, fs)
		assert.Equal(t, []string{"--meta", "--recursive"}, flagArgs)
		assert.Equal(t, []string{"a.png", "b.png", "c.png"}, pos)
	})
	t.Run("equals form", func(t *testing.T) {
		flagArgs, pos := splitArgs([]string{"--type=ocr", "img.png"}, fs)
		assert.Equal(t, []string{"--type=ocr"}, flagArgs)
		assert.Equal(t, []string{"img.png"}, pos)
	})
	t.Run("stdin sentinel", func(t *testing.T) {
		_, pos := splitArgs([]string{"-"}, fs)
		assert.Equal(t, []string{"-"}, pos)
	})
}

// TestSplitArgsDetectsAllBoolFlags is the L1 regression guard: splitArgs's bool
// detection must agree with the FlagSet's own IsBoolFlag for every registered
// flag, in both dash forms. Fails if a future bool flag isn't detected.
func TestSplitArgsDetectsAllBoolFlags(t *testing.T) {
	fs := testFlagSet()
	fs.VisitAll(func(f *flag.Flag) {
		bf, isBool := f.Value.(boolFlag)
		want := isBool && bf.IsBoolFlag()
		assert.Equal(t, want, isBoolFlagArg("--"+f.Name, fs), "flag --%s", f.Name)
		assert.Equal(t, want, isBoolFlagArg("-"+f.Name, fs), "flag -%s", f.Name)
	})
}
