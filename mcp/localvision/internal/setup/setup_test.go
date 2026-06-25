package setup

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCatalog builds a 3-model catalog spanning tiers and fit margins.
func testCatalog() *models.Catalog {
	return &models.Catalog{
		SchemaVersion: 1,
		Models: map[string]models.ModelSpec{
			"small": {DisplayName: "Small 4B", HardwareTier: models.TierConstrained, MinVramGb: 6, Preferred: true, PreferredFor: []string{"read_image"}},
			"big":   {DisplayName: "Big 27B", HardwareTier: models.TierMainstream, MinVramGb: 20, Preferred: true, PreferredFor: []string{"describe_chart"}},
			"huge":  {DisplayName: "Huge 70B", HardwareTier: models.TierHighEnd, MinVramGb: 60, Preferred: false},
		},
	}
}

func mainstreamHW() models.HardwareInfo {
	return models.HardwareInfo{TotalMemoryGB: 32, Tier: models.TierMainstream, Backend: models.BackendAppleSilicon}
}

func TestModelOptionsOrdering(t *testing.T) {
	opts := ModelOptions(testCatalog(), mainstreamHW(), 0)
	require.Len(t, opts, 3)

	// Recommended (big, mainstream) first.
	assert.Equal(t, "big", opts[0].ID)
	assert.True(t, opts[0].Recommended)
	assert.True(t, opts[0].Fits)

	// Then the other fitting model (small).
	assert.Equal(t, "small", opts[1].ID)
	assert.True(t, opts[1].Fits)
	assert.False(t, opts[1].Recommended)

	// Then the non-fitting model (huge) last.
	assert.Equal(t, "huge", opts[2].ID)
	assert.False(t, opts[2].Fits)
}

func TestModelOptionsNilCatalog(t *testing.T) {
	assert.Nil(t, ModelOptions(nil, mainstreamHW(), 0))
}

func TestModelOptionsRecommendedMarkedOnce(t *testing.T) {
	opts := ModelOptions(testCatalog(), mainstreamHW(), 0)
	n := 0
	for _, o := range opts {
		if o.Recommended {
			n++
		}
	}
	assert.Equal(t, 1, n, "exactly one recommended model")
}

func TestModelOptionsUnsupportedBackendNoRecommended(t *testing.T) {
	// An unsupported backend (e.g. Linux in v0.2) yields no DefaultModel, so
	// nothing is marked recommended but all models still appear.
	hw := models.HardwareInfo{Backend: models.BackendUnsupported}
	opts := ModelOptions(testCatalog(), hw, 0)
	require.NotEmpty(t, opts)
	for _, o := range opts {
		assert.False(t, o.Recommended)
	}
}

func TestBuildConfigAppliesChoices(t *testing.T) {
	base := &config.Config{CacheDir: "/tmp/lv"}
	out, err := BuildConfig(base, testCatalog(), mainstreamHW(), Choices{Model: "small", DefaultFormat: "json"})
	require.NoError(t, err)
	assert.Equal(t, "small", out.DefaultModel)
	assert.Equal(t, "json", out.DefaultFormat)
	// Base preserved (copy, not mutation).
	assert.Equal(t, "", base.DefaultModel)
	// Untouched fields carried over.
	assert.Equal(t, "/tmp/lv", out.CacheDir)
}

func TestBuildConfigRejectsMissingModel(t *testing.T) {
	_, err := BuildConfig(&config.Config{}, testCatalog(), mainstreamHW(), Choices{Model: "ghost"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the catalog")
}

func TestBuildConfigRejectsEmptyModel(t *testing.T) {
	_, err := BuildConfig(&config.Config{}, testCatalog(), mainstreamHW(), Choices{})
	require.Error(t, err)
}

func TestBuildConfigNilGuards(t *testing.T) {
	_, err := BuildConfig(nil, testCatalog(), mainstreamHW(), Choices{Model: "small"})
	require.Error(t, err)
	_, err = BuildConfig(&config.Config{}, nil, mainstreamHW(), Choices{Model: "small"})
	require.Error(t, err)
}

func TestDetectLLAMAServer(t *testing.T) {
	orig := lookLLAMA
	t.Cleanup(func() { lookLLAMA = orig })

	lookLLAMA = func(string) (string, error) { return "/usr/local/bin/llama-server", nil }
	p, ok := DetectLLAMAServer()
	assert.True(t, ok)
	assert.Equal(t, "/usr/local/bin/llama-server", p)

	lookLLAMA = func(string) (string, error) { return "", exec.ErrNotFound }
	_, ok = DetectLLAMAServer()
	assert.False(t, ok)

	lookLLAMA = func(string) (string, error) { return "", errors.New("boom") }
	_, ok = DetectLLAMAServer()
	assert.False(t, ok)
}
