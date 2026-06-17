// local-vision-mcp is a Claude Code (and any-MCP-client) plugin that wraps a
// local llama.cpp subprocess to provide vision-language model tools to text-only
// coding LLMs.
//
// This is a stub entry point. The full implementation lives in
// internal/mcpserver, internal/llama, internal/models, internal/tools, and
// internal/config. Track A creates the Makefile that builds this binary;
// Phase 2 (lead) fills in subcommand dispatch (run, doctor, version).
package main

import (
	"fmt"
	"os"

	"github.com/froggeric/llm/mcp/local-vision-mcp/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("local-vision-mcp %s (commit %s, built %s)\n", version.Version, version.GitCommit, version.BuildDate)
		return
	}
	// Stub: Phase 2 wires up subcommands (run, doctor, version).
	fmt.Fprintln(os.Stderr, "local-vision-mcp: not yet implemented; this is a Phase 0 contract stub")
	os.Exit(1)
}
