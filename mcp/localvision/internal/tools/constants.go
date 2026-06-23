package tools

// Tool ID constants. These must match exactly the entries in the catalog's
// preferred_for lists (see internal/models/builtin.toml) so the model selector
// can match a tool to the right model. Defining them as constants avoids typos
// and lets the compiler catch a mismatch at the call site.
//
// F5.4: tool IDs are unprefixed in MVP (e.g. "read_image", not
// "vision_read_image"). This is intentional but means the user is responsible
// for ensuring no other installed MCP server advertises the same names. If
// another vision-capable MCP is installed, names may collide in Claude Code's
// tool catalog and one of the two servers must be disabled. v0.2 may add a
// configurable prefix.
const (
	idReadImage       = "read_image"
	idExtractText     = "extract_text"
	idExtractCode     = "extract_code"
	idExtractTable    = "extract_table"
	idDescribeUI      = "describe_ui"
	idDescribeDiagram = "describe_diagram"
	idDescribeChart   = "describe_chart"
	idDiagnoseError   = "diagnose_error"
	idImageToPrompt   = "image_to_prompt"
	idCompareImages   = "compare_images"
)

// Exported tool-ID aliases for callers outside this package (e.g. the CLI
// --type → tool map). The lowercase consts above remain the canonical IDs
// returned by each tool's ID() method.
const (
	ToolReadImage       = idReadImage
	ToolExtractText     = idExtractText
	ToolExtractCode     = idExtractCode
	ToolExtractTable    = idExtractTable
	ToolDescribeUI      = idDescribeUI
	ToolDescribeDiagram = idDescribeDiagram
	ToolDescribeChart   = idDescribeChart
	ToolDiagnoseError   = idDiagnoseError
	ToolImageToPrompt   = idImageToPrompt
	ToolCompareImages   = idCompareImages
)

// latencyHint is appended to every tool's Description() so the calling LLM
// (and any wrapping approval pipeline with timeouts) knows up-front how long
// to wait. Per F1.11 (no streaming in MVP) and F5.3 (the user's
// smart-approval-pipeline needs to configure its timeout).
//
// Track B's mcpserver/tools.go appends an additional "Latency:" suffix to
// every Description; this hint ensures the substring the user expects is
// present even if the per-tool text changes.
const latencyHint = " (takes 30-60 seconds per call)"
