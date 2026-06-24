package tools

// SamplingMode says how a tool's inference should be repeated and aggregated
// when multi-sampling is opted into (F5). Source: benchmark/vlm/CATEGORY-REPORT.md
// "Implications for the localvision MCP".
type SamplingMode string

const (
	// SamplingSingle — one call. Default; also the right choice for tools whose
	// errors are systematic (extract_code, describe_diagram, diagnose_error).
	SamplingSingle SamplingMode = "single"
	// SamplingUnion — N calls at a raised temperature, then fuse the N outputs
	// into one comprehensive result (a merge pass). Helps coverage tools, where
	// each run notices different details.
	SamplingUnion SamplingMode = "union"
)

// Sampling is a tool's multi-sampling recipe: the mode and the temperature to
// use WHEN sampling. Single-call inference always uses 0.1 (deterministic)
// regardless of this — Temp only applies when the caller opts into sampling
// (reps > 1), because raising temperature is the prerequisite ("the gate") for
// correlation to help at all (at 0.1 the N runs come out ~identical).
type Sampling struct {
	Mode SamplingMode
	// Temp is the sampling temperature (used only when reps > 1).
	Temp float64
}

// samplingByTool is the per-tool recipe. Coverage tools (read_image,
// describe_ui, describe_chart) and noisy OCR (extract_text) benefit from union
// sampling at a raised temperature; deterministic/perception tools do not
// (their errors are systematic, so sampling can't help) and stay single.
// extract_table would use "majority" but that needs per-tool structured voting,
// so it stays single for now. read_document/image_to_prompt/compare_images are
// untested for sampling → single.
var samplingByTool = map[string]Sampling{
	idReadImage:     {Mode: SamplingUnion, Temp: 0.7},
	idDescribeUI:    {Mode: SamplingUnion, Temp: 0.7},
	idDescribeChart: {Mode: SamplingUnion, Temp: 0.7},
	idExtractText:   {Mode: SamplingUnion, Temp: 0.4}, // OCR peaks at mid-temp, eases at 0.7
}

// SamplingFor returns the sampling recipe for a tool. Tools not in the map get
// single/0.1 (today's behavior).
func SamplingFor(toolID string) Sampling {
	if s, ok := samplingByTool[toolID]; ok {
		return s
	}
	return Sampling{Mode: SamplingSingle, Temp: 0.1}
}
