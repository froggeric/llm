package main

import (
	"os"
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunSetup_NonInteractive (Tier-2 F): --non-interactive resolves Choices
// from env vars and writes the config without prompting (so setup works from
// CI/scripts with no TTY). Config is redirected to a temp XDG root so the real
// config is never touched.
func TestRunSetup_NonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LOCALVISION_SETUP_MODEL", "qwen3-vl-8b")
	t.Setenv("LOCALVISION_SETUP_ROUTING", "true")

	code := runSetup([]string{"--non-interactive"})
	require.Equal(t, exitOK, code, "non-interactive setup should succeed (exit 0)")

	data, err := os.ReadFile(config.DefaultPath())
	require.NoError(t, err, "config should be written to the default path")
	s := string(data)
	assert.Contains(t, s, "qwen3-vl-8b", "default model should be the env-specified one")
	assert.Contains(t, s, "[tools.", "routing=true should write per-tool [tools.<id>] tables")
}

// TestRunSetup_NonInteractiveAliasYes: --yes is an alias for --non-interactive.
func TestRunSetup_NonInteractiveAliasYes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LOCALVISION_SETUP_MODEL", "qwen3-vl-8b")

	require.Equal(t, exitOK, runSetup([]string{"--yes"}))
}

// TestRunSetup_NonInteractiveBadModel: an explicit LOCALVISION_SETUP_MODEL that
// isn't a catalog ID errors instead of silently picking a different model.
func TestRunSetup_NonInteractiveBadModel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LOCALVISION_SETUP_MODEL", "no-such-model")

	code := runSetup([]string{"--non-interactive"})
	assert.Equal(t, exitGeneric, code, "an unknown model ID should fail setup")
}
