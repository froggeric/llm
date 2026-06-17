// Package version holds build-time metadata for the local-vision-mcp binary.
// Values are injected via -ldflags at release time; defaults apply for dev builds.
package version

// Version is the semantic version of the binary. Overridden at build time via:
//
//	-ldflags "-X github.com/froggeric/llm/mcp/local-vision-mcp/internal/version.Version=0.1.0"
var Version = "dev"

// GitCommit is the short SHA of the commit the binary was built from.
var GitCommit = "unknown"

// BuildDate is the ISO-8601 build timestamp.
var BuildDate = "unknown"
