package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRegistryReturnsAllTools enforces the cardinality invariant: the
// registry must expose exactly ExpectedToolCount tools. Adding or removing
// a tool requires bumping the constant and updating the catalog's
// preferred_for lists.
func TestNewRegistryReturnsAllTools(t *testing.T) {
	r := NewRegistry()
	all := r.All()
	require.Len(t, all, ExpectedToolCount,
		"registry must expose exactly %d tools", ExpectedToolCount)
}

// TestRegistryGet ensures Get returns the right tool and reports existence
// correctly for both present and missing IDs.
func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	t.Run("existing tools", func(t *testing.T) {
		for _, id := range []string{
			idReadImage, idReadDocument, idExtractText, idExtractCode, idExtractTable,
			idDescribeUI, idDescribeDiagram, idDescribeChart,
			idDiagnoseError, idImageToPrompt, idCompareImages,
		} {
			t.Run(id, func(t *testing.T) {
				tool, ok := r.Get(id)
				require.True(t, ok, "tool %q should be registered", id)
				require.NotNil(t, tool)
				assert.Equal(t, id, tool.ID())
			})
		}
	})

	t.Run("missing tool", func(t *testing.T) {
		_, ok := r.Get("nonexistent_tool")
		assert.False(t, ok)
	})

	t.Run("empty ID", func(t *testing.T) {
		_, ok := r.Get("")
		assert.False(t, ok)
	})
}

// TestRegistryAllIsSortedByID ensures the order returned by All is
// deterministic alphabetical-by-ID. This matters for tools/list stability.
func TestRegistryAllIsSortedByID(t *testing.T) {
	r := NewRegistry()
	all := r.All()

	// Build the expected sorted order.
	expected := make([]string, 0, len(all))
	for _, t := range all {
		expected = append(expected, t.ID())
	}

	// Verify it's already sorted.
	for i := 1; i < len(expected); i++ {
		if expected[i-1] > expected[i] {
			t.Errorf("All() not sorted by ID: %q before %q",
				expected[i-1], expected[i])
		}
	}

	// Compare against the literal expected sequence for the v0.1 catalog.
	wantOrder := []string{
		"compare_images",
		"describe_chart",
		"describe_diagram",
		"describe_ui",
		"diagnose_error",
		"extract_code",
		"extract_table",
		"extract_text",
		"image_to_prompt",
		"read_document",
		"read_image",
	}
	assert.Equal(t, wantOrder, expected)
}

// TestRegistryAllIsDeterministic calls All twice and confirms the order is
// stable. Maps don't iterate in a defined order in Go; the implementation
// must sort, not rely on map iteration.
func TestRegistryAllIsDeterministic(t *testing.T) {
	r := NewRegistry()
	first := r.All()
	// Call several times; map iteration order can vary, so multiple calls
	// catch non-determinism faster than two.
	for trial := 0; trial < 10; trial++ {
		next := r.All()
		require.Len(t, next, len(first))
		for i := range first {
			assert.Equal(t, first[i].ID(), next[i].ID(),
				"trial %d: position %d differs", trial, i)
		}
	}
}

// TestRegistryRegisterRejectsDuplicates ensures Register panics on a
// duplicate ID. This is a programming error, not user-facing, so panic is
// the right behavior (caught in tests, never reached in production).
func TestRegistryRegisterRejectsDuplicates(t *testing.T) {
	t.Run("duplicate ID panics", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r, "Register must panic on duplicate ID")
		}()
		r := NewRegistry()
		r.Register(readImageTool{}) // already registered
	})

	t.Run("nil tool panics", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r, "Register must panic on nil Tool")
		}()
		r := NewRegistry()
		r.Register(nil)
	})
}

// TestRegistryNilSafe ensures All on a nil or empty Registry doesn't crash.
// Some callers may have a nil pointer in error paths.
func TestRegistryNilSafe(t *testing.T) {
	var r *Registry
	// All on nil returns nil rather than panicking.
	assert.NotPanics(t, func() {
		got := r.All()
		assert.Nil(t, got)
	})
}

// TestExpectedToolCountIs11 is a sanity guard: if someone bumps the constant
// without thinking, this test fails loudly and forces a conversation.
func TestExpectedToolCountIs11(t *testing.T) {
	assert.Equal(t, 11, ExpectedToolCount,
		"v0.6 ships 11 tools; if this changes, update the catalog and SKILL.md too")
}
