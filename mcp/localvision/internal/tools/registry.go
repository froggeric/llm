package tools

// registry.go provides the tool factory and ordering logic for the Registry.
// The Registry type itself and its method signatures are defined in tool.go
// (locked contract); this file supplies the implementation of NewRegistry's
// helper (allTools) and documents the registry invariants.
//
// Determinism: allTools returns tools sorted alphabetically by ID. The
// Register method enforces uniqueness. Together these guarantee that
// NewRegistry().All() returns the same 9 tools in the same order on every
// call, which keeps tools/list output stable for clients that diff snapshots.
//
// F5.4 (tool name collisions): tool IDs in this MCP are unprefixed (e.g.
// "read_image" rather than "vision_read_image"). This is intentional for
// MVP — the Claude Code skill layer is expected to namespace the MCP — but
// it means that if the user has another vision-capable MCP installed (e.g.
// a cloud describe-image MCP), tool names may collide and one of the two
// servers must be disabled. v0.2 may add a configurable prefix to mitigate
// this; the per-tool ID constants in constants.go are the single source of
// truth so a prefix can be added in one place when that ships.

// allTools returns instances of all 9 tools, ordered alphabetically by ID.
// NewRegistry iterates this slice and Register()s each.
//
// The slice is constructed in alphabetical order (not via sort) so the
// returned order is deterministic and inspectable by reading this function
// top-to-bottom. Adding a new tool requires both adding it here AND to the
// catalog's preferred_for lists; the registry_test.go enforces the count.
func allTools() []Tool {
	return []Tool{
		compareImagesTool{},   // compare_images
		describeChartTool{},   // describe_chart
		describeDiagramTool{}, // describe_diagram
		describeUITool{},      // describe_ui
		diagnoseErrorTool{},   // diagnose_error
		extractCodeTool{},     // extract_code
		extractTableTool{},    // extract_table
		extractTextTool{},     // extract_text
		readImageTool{},       // read_image
	}
}

// Compile-time assertion: every allTools entry implements Tool.
//
// This catches accidental signature drift early (e.g. someone renames a
// method on the interface in tool.go but forgets to update a tool impl).
// The assertion is a no-op at runtime; the compiler does the work.
var _ = []Tool{
	compareImagesTool{},
	describeChartTool{},
	describeDiagramTool{},
	describeUITool{},
	diagnoseErrorTool{},
	extractCodeTool{},
	extractTableTool{},
	extractTextTool{},
	readImageTool{},
}

// ExpectedToolCount is the number of tools the registry must expose for v0.1.
// Tests assert that NewRegistry().All() returns exactly this many. Bumping
// this constant requires a catalog update (preferred_for lists) and a
// SKILL.md refresh.
const ExpectedToolCount = 9
