package tools

// describe_chart analyzes data visualizations. The terse, structured output
// is designed for downstream numerical reasoning. Preferred on
// gemma4-26b-a4b when available.
type describeChartTool struct{}

func (describeChartTool) ID() string { return idDescribeChart }

func (describeChartTool) Description() string {
	return "Describe a chart or data visualization: chart type, axes with units, data series, notable values and outliers, and overall trend. Use for bar, line, pie, scatter, heatmap, and other quantitative charts." + latencyHint
}

func (describeChartTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": commonSchemaProperties(),
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 1024 — the prompt explicitly asks for terse output. A chart
// with 10 series doesn't need 4000 tokens. Per F4.10.
func (describeChartTool) MaxTokens() int { return 1024 }

func (describeChartTool) SystemPrompt() string { return promptDescribeChart }

func (t describeChartTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	ref, err := requireSingleImage(input)
	if err != nil {
		return "", "", nil, err
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, false), []string{ref.LocalPath}, nil
}

func (describeChartTool) ParseOutput(raw string) (any, error) { return passthroughOutput(raw) }
