package tools

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allToolInstances is the test fixture: every tool the registry should know
// about. Pulled from allTools() so the table-driven test stays in sync with
// the registry automatically.
var allToolInstances = allTools()

// idPattern is the canonical form for tool IDs: lowercase snake_case. Used
// by TestToolIDs to enforce the convention.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestToolIDsUnique enforces that every tool has a unique ID matching the
// lowercase snake_case convention. A duplicate would be a programming bug
// (Register panics on duplicates anyway, but this surfaces it cleanly).
func TestToolIDsUnique(t *testing.T) {
	seen := map[string]int{}
	for _, tool := range allToolInstances {
		id := tool.ID()
		seen[id]++
		if !idPattern.MatchString(id) {
			t.Errorf("tool ID %q does not match lowercase snake_case pattern", id)
		}
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("tool ID %q appears %d times; expected exactly 1", id, count)
		}
	}
}

// TestToolIDsMatchCatalogPreferredFor enforces that the tool IDs are
// exactly the union of every model's preferred_for entries in the catalog.
// Hardcoded list mirrors internal/models/builtin.toml. If a tool ID changes
// in constants.go without a matching catalog change, this test fails.
func TestToolIDsMatchCatalogPreferredFor(t *testing.T) {
	// Every ID that appears in any model's preferred_for in builtin.toml.
	catalogExpected := map[string]bool{
		idReadImage:       true,
		idReadDocument:    true,
		idExtractText:     true,
		idExtractCode:     true,
		idDescribeUI:      true,
		idDiagnoseError:   true,
		idDescribeChart:   true,
		idDescribeDiagram: true,
		idExtractTable:    true,
		idImageToPrompt:   true,
		idCompareImages:   true,
	}
	got := map[string]bool{}
	for _, tool := range allToolInstances {
		got[tool.ID()] = true
	}
	for id := range catalogExpected {
		if !got[id] {
			t.Errorf("catalog expects tool %q but registry does not expose it", id)
		}
	}
	for id := range got {
		if !catalogExpected[id] {
			t.Errorf("registry exposes tool %q but no model lists it in preferred_for", id)
		}
	}
}

// TestInputSchemaIsValidJSONSchema marshals each tool's InputSchema to JSON
// and checks that it has the minimum required fields (type + properties).
// We don't validate against the JSON Schema meta-schema here (the SDK does
// that downstream); we just catch obvious typos.
func TestInputSchemaIsValidJSONSchema(t *testing.T) {
	for _, tool := range allToolInstances {
		t.Run(tool.ID(), func(t *testing.T) {
			schema := tool.InputSchema()
			require.NotNil(t, schema, "InputSchema must not be nil")

			// Must marshal cleanly.
			b, err := json.Marshal(schema)
			require.NoError(t, err, "InputSchema must be JSON-marshallable")

			// Must be a JSON object (starts with '{').
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(b, &decoded), "InputSchema must decode as a JSON object")

			// Must have type=object and a properties map.
			typ, ok := decoded["type"]
			require.True(t, ok, "InputSchema must have a 'type' field")
			assert.Equal(t, "object", typ, "InputSchema 'type' must be 'object'")

			props, ok := decoded["properties"]
			require.True(t, ok, "InputSchema must have a 'properties' field")
			_, isMap := props.(map[string]any)
			assert.True(t, isMap, "'properties' must be a JSON object")

			// Every tool needs at least one image input field.
			propsMap := props.(map[string]any)
			_, hasPath := propsMap["image_path"]
			_, hasData := propsMap["image_data"]
			_, hasURL := propsMap["image_url"]
			_, hasImages := propsMap["images"]
			assert.True(t, hasPath || hasData || hasURL || hasImages,
				"InputSchema must declare at least one image input field")
		})
	}
}

// TestMaxTokensWithinExpectedBands checks that each tool's MaxTokens matches
// the value documented in the plan (F4.10). A drift here is a behavior
// change worth flagging in review.
func TestMaxTokensWithinExpectedBands(t *testing.T) {
	expected := map[string]int{
		idReadImage:       1500,
		idReadDocument:    2048,
		idExtractText:     2048,
		idExtractCode:     4096,
		idExtractTable:    2048,
		idDescribeUI:      2000,
		idDescribeDiagram: 2000,
		idDescribeChart:   1024,
		idDiagnoseError:   800,
		idImageToPrompt:   1024,
		idCompareImages:   1500,
	}
	for _, tool := range allToolInstances {
		t.Run(tool.ID(), func(t *testing.T) {
			got := tool.MaxTokens()
			want, ok := expected[tool.ID()]
			require.True(t, ok, "no expected max_tokens for tool %q; update this test", tool.ID())
			assert.Equal(t, want, got, "MaxTokens drifted from plan")
		})
	}
}

// TestDescriptionIncludesLatencyHint enforces F1.11/F5.3: every tool's
// Description MUST contain the substring "(takes 30-60 seconds per call)" so
// the smart-approval-pipeline and the calling LLM know up-front how long to
// wait. Track B's mcpserver appends an additional "Latency:" line, but the
// substring must be present at the tools layer too.
func TestDescriptionIncludesLatencyHint(t *testing.T) {
	for _, tool := range allToolInstances {
		t.Run(tool.ID(), func(t *testing.T) {
			desc := tool.Description()
			assert.Contains(t, desc, "takes 30-60 seconds per call",
				"Description must include the latency hint substring verbatim")
		})
	}
}

// TestSystemPromptNonEmpty is a smoke check: an empty system prompt means
// the model will produce generic output, which defeats the entire point of
// having 11 specialized tools.
func TestSystemPromptNonEmpty(t *testing.T) {
	for _, tool := range allToolInstances {
		t.Run(tool.ID(), func(t *testing.T) {
			p := tool.SystemPrompt()
			assert.NotEmpty(t, p, "SystemPrompt must not be empty")
			assert.Greater(t, len(p), 30, "SystemPrompt suspiciously short")
		})
	}
}

// TestBuildRequestSanity is the per-tool smoke test for BuildRequest.
// Given a minimal valid ToolInput, the tool must return its own system
// prompt, a non-empty user prompt, and at least one image path. Tools must
// not call out to the executor or do I/O here.
func TestBuildRequestSanity(t *testing.T) {
	singleImageInput := ToolInput{
		Images: []ImageRef{{LocalPath: "/tmp/fake.png", Source: "/tmp/fake.png"}},
		Extra:  map[string]any{"question": "what is this?"},
	}
	twoImageInput := ToolInput{
		Images: []ImageRef{
			{LocalPath: "/tmp/a.png", Source: "/tmp/a.png"},
			{LocalPath: "/tmp/b.png", Source: "/tmp/b.png"},
		},
	}
	for _, tool := range allToolInstances {
		t.Run(tool.ID(), func(t *testing.T) {
			var input ToolInput
			if tool.ID() == idCompareImages {
				input = twoImageInput
			} else {
				input = singleImageInput
			}

			sys, user, paths, err := tool.BuildRequest(input)
			require.NoError(t, err, "BuildRequest with valid input must succeed")

			assert.Equal(t, tool.SystemPrompt(), sys,
				"BuildRequest must return the tool's own SystemPrompt")
			assert.NotEmpty(t, user, "user prompt must not be empty")
			assert.NotEmpty(t, paths, "image paths must not be empty")
			for _, p := range paths {
				assert.True(t, strings.HasPrefix(p, "/"),
					"image paths should be absolute; got %q", p)
			}
		})
	}
}

// TestDescribeChartOutputModes verifies G4: describe_chart's BuildRequest
// returns a mode-specific system prompt for output=csv/json, and the prose
// default returns SystemPrompt() (preserving the TestBuildRequestSanity
// invariant that the default path returns SystemPrompt()).
func TestDescribeChartOutputModes(t *testing.T) {
	tool := describeChartTool{}
	img := []ImageRef{{LocalPath: "/tmp/c.png", Source: "/tmp/c.png"}}

	t.Run("prose default equals SystemPrompt", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{}})
		require.NoError(t, err)
		assert.Equal(t, tool.SystemPrompt(), sys)
	})
	t.Run("csv mode", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "csv"}})
		require.NoError(t, err)
		assert.Equal(t, promptDescribeChartCSV, sys)
		assert.NotEqual(t, tool.SystemPrompt(), sys)
	})
	t.Run("json mode", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "json"}})
		require.NoError(t, err)
		assert.Equal(t, promptDescribeChartJSON, sys)
	})
	t.Run("uppercase/whitespace tolerated", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "  CSV "}})
		require.NoError(t, err)
		assert.Equal(t, promptDescribeChartCSV, sys)
	})
	t.Run("invalid mode errors clearly", func(t *testing.T) {
		_, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "xml"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported output")
	})
}

// TestDescribeChartParseOutput verifies G4 ParseOutput: a fenced csv block is
// stripped to clean CSV, json (bare or fenced) parses to a real object, and
// prose passes through unchanged.
func TestDescribeChartParseOutput(t *testing.T) {
	tool := describeChartTool{}
	csvIn := ToolInput{Extra: map[string]any{"output": "csv"}}
	jsonIn := ToolInput{Extra: map[string]any{"output": "json"}}
	proseIn := ToolInput{Extra: map[string]any{"output": "prose"}}
	t.Run("csv fence stripped", func(t *testing.T) {
		raw := "```csv\nlabel,revenue\nQ1,4.2\nQ2,5.1\n```"
		out, err := tool.ParseOutput(csvIn, raw)
		require.NoError(t, err)
		assert.Equal(t, "label,revenue\nQ1,4.2\nQ2,5.1", out)
	})
	t.Run("bare json parsed", func(t *testing.T) {
		raw := `{"chart_type":"bar","series":[{"name":"a","points":[["Q1",4.2]]}]}`
		out, err := tool.ParseOutput(jsonIn, raw)
		require.NoError(t, err)
		m, ok := out.(map[string]any)
		require.True(t, ok, "json mode must return a parsed object")
		assert.Equal(t, "bar", m["chart_type"])
	})
	t.Run("fenced json parsed", func(t *testing.T) {
		raw := "```json\n{\"chart_type\":\"line\"}\n```"
		out, err := tool.ParseOutput(jsonIn, raw)
		require.NoError(t, err)
		m, ok := out.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "line", m["chart_type"])
	})
	t.Run("prose passthrough", func(t *testing.T) {
		raw := "## Chart type\nThis is a bar chart.\n"
		out, err := tool.ParseOutput(proseIn, raw)
		require.NoError(t, err)
		assert.Equal(t, raw, out)
	})
	t.Run("prose with json-looking body stays a string", func(t *testing.T) {
		// Regression (Tier-1 D): a prose-mode report that happens to be valid
		// JSON must NOT be parsed into structured output. The old code
		// content-sniffed and would return a float64 here.
		raw := "42.0"
		out, err := tool.ParseOutput(proseIn, raw)
		require.NoError(t, err)
		assert.Equal(t, raw, out, "prose mode must return the raw string even when it's valid JSON")
	})
}

// TestDescribeDiagramOutputModes verifies G5: mermaid mode returns the mermaid
// prompt; the prose default returns SystemPrompt().
func TestDescribeDiagramOutputModes(t *testing.T) {
	tool := describeDiagramTool{}
	img := []ImageRef{{LocalPath: "/tmp/d.png", Source: "/tmp/d.png"}}
	t.Run("prose default equals SystemPrompt", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{}})
		require.NoError(t, err)
		assert.Equal(t, tool.SystemPrompt(), sys)
	})
	t.Run("mermaid mode", func(t *testing.T) {
		sys, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "mermaid"}})
		require.NoError(t, err)
		assert.Equal(t, promptDescribeDiagramMermaid, sys)
		assert.NotEqual(t, tool.SystemPrompt(), sys)
	})
	t.Run("diagram rejects csv mode", func(t *testing.T) {
		_, _, _, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"output": "csv"}})
		require.Error(t, err)
	})
}

// TestDescribeDiagramParseOutput verifies G5: a fenced mermaid block is stripped
// to pasteable markup; prose passes through unchanged.
func TestDescribeDiagramParseOutput(t *testing.T) {
	tool := describeDiagramTool{}
	mermaidIn := ToolInput{Extra: map[string]any{"output": "mermaid"}}
	proseIn := ToolInput{Extra: map[string]any{"output": "prose"}}
	t.Run("mermaid fence stripped", func(t *testing.T) {
		raw := "```mermaid\nflowchart TD\n  A --> B\n```"
		out, err := tool.ParseOutput(mermaidIn, raw)
		require.NoError(t, err)
		assert.Equal(t, "flowchart TD\n  A --> B", out)
	})
	t.Run("prose passthrough", func(t *testing.T) {
		raw := "## Diagram type\nThis is an architecture diagram.\n"
		out, err := tool.ParseOutput(proseIn, raw)
		require.NoError(t, err)
		assert.Equal(t, raw, out)
	})
}

// TestBuildRequestRejectsMissingImage ensures single-image tools surface a
// clear error when the caller forgot to supply an image. compare_images is
// excluded (it needs exactly 2, not at-least-1).
func TestBuildRequestRejectsMissingImage(t *testing.T) {
	emptyInput := ToolInput{Extra: map[string]any{}}
	for _, tool := range allToolInstances {
		if tool.ID() == idCompareImages {
			continue
		}
		t.Run(tool.ID(), func(t *testing.T) {
			_, _, _, err := tool.BuildRequest(emptyInput)
			require.Error(t, err, "must reject zero-image input")
			assert.Contains(t, err.Error(), "image",
				"error should mention the missing image")
		})
	}
}

// TestCompareImagesRequiresExactlyTwo enforces F4.9: the compare_images
// tool accepts an array of exactly two image refs. One or three must be
// rejected with a clear error.
func TestCompareImagesRequiresExactlyTwo(t *testing.T) {
	tool := compareImagesTool{}

	t.Run("zero images rejected", func(t *testing.T) {
		_, _, _, err := tool.BuildRequest(ToolInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2")
	})

	t.Run("one image rejected", func(t *testing.T) {
		input := ToolInput{
			Images: []ImageRef{{LocalPath: "/tmp/only.png"}},
		}
		_, _, _, err := tool.BuildRequest(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2")
	})

	t.Run("three images rejected", func(t *testing.T) {
		input := ToolInput{
			Images: []ImageRef{
				{LocalPath: "/tmp/a.png"},
				{LocalPath: "/tmp/b.png"},
				{LocalPath: "/tmp/c.png"},
			},
		}
		_, _, _, err := tool.BuildRequest(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "2")
	})

	t.Run("two images accepted", func(t *testing.T) {
		input := ToolInput{
			Images: []ImageRef{
				{LocalPath: "/tmp/a.png"},
				{LocalPath: "/tmp/b.png"},
			},
		}
		_, user, paths, err := tool.BuildRequest(input)
		require.NoError(t, err)
		assert.Len(t, paths, 2)
		assert.NotEmpty(t, user)
	})
}

// TestParseOutputSanity is the per-tool smoke test for ParseOutput. Each
// tool is fed a representative model output (good and bad cases) and must
// not panic, returning either a valid result or a structured error.
func TestParseOutputSanity(t *testing.T) {
	cases := []struct {
		toolID string
		inputs []string
	}{
		{idReadImage, []string{
			"## Layout\nA photo of a cat sitting on a windowsill.",
			"",
			"   ",
		}},
		{idExtractText, []string{
			"hello world\nthis is text",
			"",
		}},
		{idExtractCode, []string{
			// Well-formed fenced block with prose around it.
			"Here's the code:\n```go\npackage main\n\nfunc main() {}\n```\nThat's it.",
			// Bare code, no fence — must still produce non-empty code.
			"package main\n\nfunc main() {}",
			// Fence without language tag.
			"```\necho hi\n```",
			// Empty input.
			"",
		}},
		{idExtractTable, []string{
			// Proper Markdown table.
			"| Col1 | Col2 |\n| --- | --- |\n| a | b |\n| c | d |",
			// Two tables separated by a rule.
			"| A | B |\n| --- | --- |\n| 1 | 2 |\n\n---\n\n| C | D |\n| --- | --- |\n| 3 | 4 |",
			// Prose-only — should fall back to verbatim.
			"No table here.",
			"",
		}},
		{idDescribeUI, []string{"## Layout\nA login form.", ""}},
		{idDescribeDiagram, []string{"## Type\nER diagram", ""}},
		{idDescribeChart, []string{"## Type\nBar chart", ""}},
		{idImageToPrompt, []string{"## Subject\nA cat on a windowsill.\n## Tags\ncat, windowsill, sunlight, cozy", ""}},
		{idDiagnoseError, []string{"Error: nil pointer", ""}},
		{idCompareImages, []string{
			"- Text label changed\n- Button moved",
			"The images are identical.",
		}},
		{idReadDocument, []string{
			"## Document summary\nA 3-page quarterly sales report.\n### Page 1\nCover and headline figures.",
			"",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.toolID, func(t *testing.T) {
			tool := findTool(t, tc.toolID)
			require.NotNil(t, tool)
			for _, raw := range tc.inputs {
				t.Run("input_len_"+itoa(len(raw)), func(t *testing.T) {
					// Must not panic. We allow either a result or an error;
					// tools that special-case empty input (compare_images)
					// may legitimately return an error.
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("ParseOutput panicked on input %q: %v", raw, r)
						}
					}()
					_, _ = tool.ParseOutput(ToolInput{}, raw)
				})
			}
		})
	}
}

// TestExtractCodeParseOutput is a focused test for extract_code's
// ParseOutput, since it does real work (stripping prose outside the fence).
func TestExtractCodeParseOutput(t *testing.T) {
	tool := extractCodeTool{}

	t.Run("strips leading and trailing prose", func(t *testing.T) {
		raw := "Here is the code:\n\n```python\nprint('hi')\n```\n\nHope this helps."
		out, err := tool.ParseOutput(ToolInput{}, raw)
		require.NoError(t, err)

		m, ok := out.(map[string]any)
		require.True(t, ok, "ParseOutput should return a map")
		assert.Equal(t, "python", m["language"])
		assert.Equal(t, "print('hi')", m["code"])
	})

	t.Run("preserves indentation inside fence", func(t *testing.T) {
		raw := "```go\nfunc main() {\n\tx := 1\n\t_ = x\n}\n```"
		out, err := tool.ParseOutput(ToolInput{}, raw)
		require.NoError(t, err)
		m := out.(map[string]any)
		assert.Equal(t, "go", m["language"])
		assert.Contains(t, m["code"], "\n\tx := 1")
	})

	t.Run("no fence returns verbatim", func(t *testing.T) {
		raw := "package main\nfunc main() {}"
		out, err := tool.ParseOutput(ToolInput{}, raw)
		require.NoError(t, err)
		m := out.(map[string]any)
		assert.Equal(t, "", m["language"])
		assert.Equal(t, raw, m["code"])
	})
}

// TestExtractTableParseOutput is a focused test for extract_table's
// ParseOutput, since it does real work (stripping prose).
func TestExtractTableParseOutput(t *testing.T) {
	tool := extractTableTool{}

	t.Run("strips prose, keeps tables", func(t *testing.T) {
		raw := "Here is the table:\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\nDone."
		out, err := tool.ParseOutput(ToolInput{}, raw)
		require.NoError(t, err)
		s := out.(string)
		assert.Contains(t, s, "| A | B |")
		assert.Contains(t, s, "| 1 | 2 |")
		assert.NotContains(t, s, "Here is the table:")
		assert.NotContains(t, s, "Done.")
	})

	t.Run("no tables returns verbatim", func(t *testing.T) {
		raw := "There were no tables in the image."
		out, err := tool.ParseOutput(ToolInput{}, raw)
		require.NoError(t, err)
		assert.Equal(t, raw, out)
	})
}

// findTool looks up a tool by ID in the registry, failing the test if missing.
func findTool(t *testing.T, id string) Tool {
	t.Helper()
	for _, tool := range allToolInstances {
		if tool.ID() == id {
			return tool
		}
	}
	return nil
}

// itoa is a tiny local helper to avoid importing strconv just for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
