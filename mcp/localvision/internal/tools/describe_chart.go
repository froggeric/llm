package tools

import (
	"encoding/json"
	"strings"
)

// describe_chart analyzes data visualizations. By default it emits a terse,
// structured prose report; pass output="csv" to get the underlying numbers as
// CSV (paste into a spreadsheet) or output="json" for a JSON object of the
// data (G4). Served by the default qwen3-vl-8b.
type describeChartTool struct{}

func (describeChartTool) ID() string { return idDescribeChart }

func (describeChartTool) Description() string {
	return "Describe a chart or data visualization: chart type, axes with units, data series, notable values and outliers, and overall trend. Use for bar, line, pie, scatter, heatmap, and other quantitative charts. Set output=\"csv\" to get the underlying numbers as CSV (paste into a spreadsheet) or output=\"json\" for a JSON object of the data." + latencyHint
}

func (describeChartTool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["output"] = map[string]any{
		"type":        "string",
		"enum":        []string{"prose", "csv", "json"},
		"description": "Output representation: prose (default — a structured description), csv (underlying numbers as CSV, ready for a spreadsheet), or json (a JSON object of the data).",
		"default":     "prose",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 1024 — the prompt explicitly asks for terse output. A chart
// with 10 series doesn't need 4000 tokens. Per F4.10.
func (describeChartTool) MaxTokens() int { return 1024 }

// SystemPrompt returns the prose prompt (the default mode). csv/json use
// chartPromptFor(mode) from BuildRequest; SystemPrompt stays prose so the
// default BuildRequest path returns SystemPrompt() (asserted by
// TestBuildRequestSanity).
func (describeChartTool) SystemPrompt() string { return promptDescribeChart }

func (t describeChartTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	mode, err := outputMode(input.Extra, []string{"csv", "json"})
	if err != nil {
		return "", "", nil, err
	}
	// mode=="prose" (absent/empty) → chartPromptFor returns promptDescribeChart,
	// which equals t.SystemPrompt() — the invariant TestBuildRequestSanity checks.
	return chartPromptFor(mode), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

// ParseOutput post-processes per the requested output mode (resolved from
// input.Extra via the same outputMode helper BuildRequest uses):
//   - csv: strip a fenced ```csv block and return clean CSV text (raw if the
//     model omitted the fence).
//   - json: parse a bare or fenced ```json object to any so MCP surfaces it as
//     StructuredContent; on parse failure return the text.
//   - prose (default): return the raw Markdown report UNCHANGED. Previously the
//     parser content-sniffed, so a prose report that happened to be valid JSON
//     (e.g. a bare-number trend summary) was mis-returned as structured output.
func (describeChartTool) ParseOutput(input ToolInput, raw string) (any, error) {
	mode, err := outputMode(input.Extra, []string{"csv", "json"})
	if err != nil {
		return nil, err
	}
	switch mode {
	case "csv":
		lang, body := extractCodeBlock(raw)
		if strings.EqualFold(lang, "csv") {
			return strings.TrimSpace(body), nil
		}
		return raw, nil
	case "json":
		lang, body := extractCodeBlock(raw)
		candidate := strings.TrimSpace(raw)
		if strings.EqualFold(lang, "json") {
			candidate = strings.TrimSpace(body)
		}
		if json.Valid([]byte(candidate)) {
			var v any
			if err := json.Unmarshal([]byte(candidate), &v); err == nil {
				return v, nil
			}
		}
		return raw, nil
	default: // prose
		return raw, nil
	}
}
