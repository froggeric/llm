package main

import (
	"io"
	"os"
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunSetup_NonInteractive (Tier-2 F): --non-interactive writes a config
// from env vars without prompting. Config is redirected to a temp XDG root.
//
// This asserts only hardware-independent output: default_model is always
// written from LOCALVISION_SETUP_MODEL regardless of how much fits. The
// per-tool [tools.*] tables depend on what fits the DETECTED hardware (a 7 GB
// CI runner fits nothing under the 4 GB margin, so none are written — correct
// behavior), so they're asserted separately in TestSetupNonInteractive_Routing
// against a synthetic large box.
func TestRunSetup_NonInteractive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LOCALVISION_SETUP_MODEL", "qwen3-vl-8b")
	t.Setenv("LOCALVISION_SETUP_ROUTING", "true")

	require.Equal(t, exitOK, runSetup([]string{"--non-interactive"}),
		"non-interactive setup should succeed (exit 0)")

	data, err := os.ReadFile(config.DefaultPath())
	require.NoError(t, err, "config should be written to the default path")
	assert.Contains(t, string(data), `default_model = "qwen3-vl-8b"`,
		"default_model should be the env-specified model")
}

// TestSetupNonInteractive_Routing: on a box where models fit, routing=true
// writes the per-tool [tools.<id>] tables. Calls setupNonInteractive directly
// with a synthetic 64 GB HardwareInfo so the result is deterministic (the
// end-to-end runSetup path uses the real detected hardware, which varies).
func TestSetupNonInteractive_Routing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("LOCALVISION_SETUP_MODEL", "qwen3-vl-8b")
	t.Setenv("LOCALVISION_SETUP_ROUTING", "true")

	catalog, err := models.Load("")
	require.NoError(t, err)
	hw := models.HardwareInfo{
		TotalMemoryGB: 64,
		Tier:          models.TierHighEnd,
		Backend:       models.BackendAppleSilicon,
	}
	cfg, err := config.Load("")
	require.NoError(t, err)
	opts := setup.ModelOptions(catalog, hw, cfg.SafetyMarginGB)

	require.Equal(t, exitOK, setupNonInteractive(cfg, catalog, hw, opts, io.Discard))

	data, err := os.ReadFile(config.DefaultPath())
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `default_model = "qwen3-vl-8b"`)
	assert.Contains(t, s, "[tools.",
		"routing=true on a 64 GB box should write per-tool [tools.<id>] tables")
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

	assert.Equal(t, exitGeneric, runSetup([]string{"--non-interactive"}),
		"an unknown model ID should fail setup")
}
