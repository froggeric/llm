package models

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validHash64 returns a 64-char hex string suitable for use as a SHA256 in
// tests (it's NOT a real hash of anything; we just need a syntactically
// valid placeholder that passes validHash()).
const validHash64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

const (
	ggufURL   = "https://huggingface.co/froggeric/Qwen3-VL-8B-GGUF/resolve/main/qwen3-vl-8b-q4_k_m.gguf"
	mmprojURL = "https://huggingface.co/froggeric/Qwen3-VL-8B-GGUF/resolve/main/mmproj-qwen3-vl-8b-f16.gguf"
)

// validSpec returns a ModelSpec that satisfies all Validate invariants.
func validSpec(displayName, tier string, preferred bool, preferredFor []string) ModelSpec {
	return ModelSpec{
		DisplayName:   displayName,
		GGUF:          ggufURL,
		Mmproj:        mmprojURL,
		GGUFSha256:    validHash64,
		MmprojSha256:  validHash64,
		Ctx:           32768,
		GpuLayers:     -1,
		MinVramGb:     8,
		MinSystemRamGb: 16,
		Released:      "2025-10",
		License:       "Apache-2.0",
		HardwareTier:  HardwareTier(tier),
		Preferred:     preferred,
		PreferredFor:  preferredFor,
		BenchToks:     43.0,
		Notes:         "test model",
	}
}

// validCatalog returns a Catalog with one preferred model per tier. Pass
// mutate to tweak entries (e.g. introduce a validation problem).
func validCatalog() *Catalog {
	return &Catalog{
		SchemaVersion: 1,
		Models: map[string]ModelSpec{
			"constrained-model":  validSpec("Constrained", "constrained", true, []string{"read_image"}),
			"mainstream-model":   validSpec("Mainstream", "mainstream", true, []string{"read_image"}),
			"high-end-model":     validSpec("HighEnd", "high_end", true, []string{"describe_chart"}),
		},
	}
}

func TestValidate_OK(t *testing.T) {
	c := validCatalog()
	require.NoError(t, c.Validate())
}

func TestValidate_RejectsWrongSchemaVersion(t *testing.T) {
	c := validCatalog()
	c.SchemaVersion = 99
	err := c.Validate()
	require.Error(t, err)
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Errorf("err not ErrInvalidCatalog: %v", err)
	}
	if !strings.Contains(err.Error(), "upgrade") {
		t.Errorf("error does not mention upgrade: %v", err)
	}
}

func TestValidate_MissingSchemaVersion(t *testing.T) {
	c := validCatalog()
	c.SchemaVersion = 0
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "schema_version is missing") {
		t.Errorf("missing-schema error not surfaced: %v", err)
	}
}

func TestValidate_TwoPreferredInOneTier(t *testing.T) {
	c := validCatalog()
	// Add a second preferred model in mainstream.
	c.Models["second-mainstream"] = validSpec("Second Mainstream", "mainstream", true, nil)
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "mainstream") {
		t.Errorf("error does not identify mainstream tier: %v", err)
	}
	if !strings.Contains(err.Error(), "mainstream-model") || !strings.Contains(err.Error(), "second-mainstream") {
		t.Errorf("error does not list both offending IDs: %v", err)
	}
}

func TestValidate_HTTPURLRejected(t *testing.T) {
	c := validCatalog()
	m := c.Models["mainstream-model"]
	m.GGUF = "http://huggingface.co/froggeric/x/resolve/main/y.gguf"
	c.Models["mainstream-model"] = m
	err := c.Validate()
	require.Error(t, err)
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Errorf("err not ErrInvalidCatalog: %v", err)
	}
	// Must mention HTTPS requirement.
	if !strings.Contains(err.Error(), "HTTPS") && !strings.Contains(strings.ToLower(err.Error()), "https") {
		t.Errorf("error does not mention HTTPS: %v", err)
	}
}

func TestValidate_HTTPSButWrongNamespaceRejected(t *testing.T) {
	c := validCatalog()
	m := c.Models["mainstream-model"]
	m.GGUF = "https://huggingface.co/attacker/x/resolve/main/y.gguf"
	c.Models["mainstream-model"] = m
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "namespace") && !strings.Contains(err.Error(), "froggeric") {
		t.Errorf("error does not mention namespace: %v", err)
	}
}

func TestValidate_SHA256Mandatory(t *testing.T) {
	c := validCatalog()
	m := c.Models["mainstream-model"]
	m.GGUFSha256 = ""
	c.Models["mainstream-model"] = m
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "gguf_sha256") {
		t.Errorf("error does not mention gguf_sha256: %v", err)
	}
}

func TestValidate_SHA256PlaceholderRejected(t *testing.T) {
	c := validCatalog()
	m := c.Models["mainstream-model"]
	m.MmprojSha256 = "PLACEHOLDER-PHASE3"
	c.Models["mainstream-model"] = m
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "doctor --compute-hashes") {
		t.Errorf("error does not mention doctor --compute-hashes: %v", err)
	}
}

func TestValidate_InvalidHardwareTier(t *testing.T) {
	c := validCatalog()
	m := c.Models["mainstream-model"]
	m.HardwareTier = "ludicrous"
	c.Models["mainstream-model"] = m
	err := c.Validate()
	require.Error(t, err)
	if !strings.Contains(err.Error(), "hardware_tier") {
		t.Errorf("error does not mention hardware_tier: %v", err)
	}
}

// TestLoad_BuiltinPlusOverlays verifies that Load merges the builtin
// catalog with two overlay files, applying last-write-wins in lexical
// order.
func TestLoad_BuiltinPlusOverlays(t *testing.T) {
	tmp := t.TempDir()

	// Overlay A overrides display_name on builtin.qwen3-vl-8b.
	overlayA := `
schema_version = 1

[models.qwen3-vl-8b]
display_name = "Overridden in A"
`
	if err := os.WriteFile(filepath.Join(tmp, "00-a.toml"), []byte(overlayA), 0o644); err != nil {
		t.Fatal(err)
	}

	// Overlay B overrides display_name again. B sorts after A so it wins.
	overlayB := `
schema_version = 1

[models.qwen3-vl-8b]
display_name = "Overridden in B"
min_vram_gb = 99
`
	if err := os.WriteFile(filepath.Join(tmp, "01-b.toml"), []byte(overlayB), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(tmp)
	require.NoError(t, err)

	m, ok := c.Models["qwen3-vl-8b"]
	require.True(t, ok, "qwen3-vl-8b missing after overlay")
	if m.DisplayName != "Overridden in B" {
		t.Errorf("DisplayName = %q; want %q (B should win by lexical order)", m.DisplayName, "Overridden in B")
	}
	if m.MinVramGb != 99 {
		t.Errorf("MinVramGb = %d; want 99", m.MinVramGb)
	}

	// Other builtin fields untouched.
	if m.License != "Apache-2.0" {
		t.Errorf("License changed by overlay: %q", m.License)
	}
	if m.GGUF == "" {
		t.Errorf("GGUF URL wiped by overlay")
	}
}

func TestLoad_NoOverlayDir(t *testing.T) {
	c, err := Load("")
	require.NoError(t, err)
	if c.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d; want %d", c.SchemaVersion, CurrentSchemaVersion)
	}
	if len(c.Models) == 0 {
		t.Error("builtin catalog has no models")
	}
	// Spot-check: qwen3-vl-4b must exist and be preferred in constrained.
	m, ok := c.Models["qwen3-vl-4b"]
	require.True(t, ok)
	if !m.Preferred {
		t.Error("qwen3-vl-4b should be preferred")
	}
	if m.HardwareTier != TierConstrained {
		t.Errorf("qwen3-vl-4b tier = %q; want constrained", m.HardwareTier)
	}
}

func TestLoad_NonexistentOverlayDirIgnored(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	if len(c.Models) == 0 {
		t.Error("models missing")
	}
}

func TestLoad_OverlayAddsNewModel(t *testing.T) {
	tmp := t.TempDir()
	overlay := `
schema_version = 1

[models.custom-3b]
display_name = "Custom 3B"
gguf = "https://huggingface.co/froggeric/Custom-3B-GGUF/resolve/main/custom-3b.gguf"
mmproj = "https://huggingface.co/froggeric/Custom-3B-GGUF/resolve/main/mmproj.gguf"
gguf_sha256 = "` + validHash64 + `"
mmproj_sha256 = "` + validHash64 + `"
ctx = 16384
gpu_layers = -1
min_vram_gb = 4
min_system_ram_gb = 8
released = "2025-01"
license = "MIT"
preferred = false
preferred_for = []
hardware_tier = "constrained"
bench_toks = 50.0
notes = "user-supplied"
`
	if err := os.WriteFile(filepath.Join(tmp, "custom.toml"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(tmp)
	require.NoError(t, err)
	m, ok := c.Models["custom-3b"]
	require.True(t, ok, "custom-3b missing")
	assert.Equal(t, "Custom 3B", m.DisplayName)
}

// TestDefaultModel_Determinism checks that calling DefaultModel twice on
// the same inputs returns the same answer (F1.8).
func TestDefaultModel_Determinism(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 32, Tier: TierMainstream, Backend: BackendAppleSilicon}

	a, errA := c.DefaultModel(hw)
	b, errB := c.DefaultModel(hw)
	require.NoError(t, errA)
	require.NoError(t, errB)
	if a != b {
		t.Errorf("DefaultModel non-deterministic: %q vs %q", a, b)
	}
}

func TestDefaultModel_PrefersTierPreferred(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 32, Tier: TierMainstream, Backend: BackendAppleSilicon}

	id, err := c.DefaultModel(hw)
	require.NoError(t, err)
	if id != "mainstream-model" {
		t.Errorf("DefaultModel = %q; want mainstream-model (tier-preferred)", id)
	}
}

func TestDefaultModel_FallsBackToSmallest(t *testing.T) {
	// No preferred entry in the matching tier; smallest MinVramGb wins.
	c := &Catalog{
		SchemaVersion: 1,
		Models: map[string]ModelSpec{
			"big":   {DisplayName: "Big", MinVramGb: 20, HardwareTier: TierHighEnd, GGUF: ggufURL, Mmproj: mmprojURL, GGUFSha256: validHash64, MmprojSha256: validHash64, Ctx: 4096, License: "MIT"},
			"small": {DisplayName: "Small", MinVramGb: 4, HardwareTier: TierHighEnd, GGUF: ggufURL, Mmproj: mmprojURL, GGUFSha256: validHash64, MmprojSha256: validHash64, Ctx: 4096, License: "MIT"},
		},
	}
	hw := HardwareInfo{TotalMemoryGB: 64, Tier: TierHighEnd}
	id, err := c.DefaultModel(hw)
	require.NoError(t, err)
	if id != "small" {
		t.Errorf("DefaultModel = %q; want small (smallest MinVramGb)", id)
	}
}

func TestDefaultModel_TieBreakByDisplayName(t *testing.T) {
	// Two models with same MinVramGb: alphabetical by DisplayName.
	c := &Catalog{
		SchemaVersion: 1,
		Models: map[string]ModelSpec{
			"zeta":  {DisplayName: "Zeta",  MinVramGb: 8, HardwareTier: TierMainstream, GGUF: ggufURL, Mmproj: mmprojURL, GGUFSha256: validHash64, MmprojSha256: validHash64, Ctx: 4096, License: "MIT"},
			"alpha": {DisplayName: "Alpha", MinVramGb: 8, HardwareTier: TierMainstream, GGUF: ggufURL, Mmproj: mmprojURL, GGUFSha256: validHash64, MmprojSha256: validHash64, Ctx: 4096, License: "MIT"},
		},
	}
	hw := HardwareInfo{TotalMemoryGB: 32, Tier: TierMainstream}
	id, err := c.DefaultModel(hw)
	require.NoError(t, err)
	if id != "alpha" {
		t.Errorf("DefaultModel = %q; want alpha (lexical)", id)
	}
}

func TestDefaultModel_NoFittingModel(t *testing.T) {
	c := &Catalog{
		SchemaVersion: 1,
		Models: map[string]ModelSpec{
			"big": {DisplayName: "Big", MinVramGb: 64, HardwareTier: TierHighEnd, GGUF: ggufURL, Mmproj: mmprojURL, GGUFSha256: validHash64, MmprojSha256: validHash64, Ctx: 4096, License: "MIT"},
		},
	}
	hw := HardwareInfo{TotalMemoryGB: 8, Tier: TierConstrained}
	_, err := c.DefaultModel(hw)
	if !errors.Is(err, ErrNoFittingModel) {
		t.Errorf("DefaultModel err = %v; want ErrNoFittingModel", err)
	}
}

func TestModelFor_PrefersListed(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 64, Tier: TierHighEnd}

	id, err := c.ModelFor("describe_chart", hw)
	require.NoError(t, err)
	// "describe_chart" is in high-end-model.PreferredFor.
	if id != "high-end-model" {
		t.Errorf("ModelFor(describe_chart) = %q; want high-end-model", id)
	}
}

func TestModelFor_FallsBackToDefaultWhenToolNotListed(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 32, Tier: TierMainstream}

	id, err := c.ModelFor("unknown_tool", hw)
	require.NoError(t, err)
	if id != "mainstream-model" {
		t.Errorf("ModelFor(unknown) = %q; want default mainstream-model", id)
	}
}

func TestModelFor_NoFittingModel(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 4, Tier: TierConstrained}
	_, err := c.ModelFor("read_image", hw)
	if !errors.Is(err, ErrNoFittingModel) {
		t.Errorf("ModelFor err = %v; want ErrNoFittingModel", err)
	}
}

func TestModelFor_Determinism(t *testing.T) {
	c := validCatalog()
	hw := HardwareInfo{TotalMemoryGB: 32, Tier: TierMainstream}
	a, errA := c.ModelFor("read_image", hw)
	b, errB := c.ModelFor("read_image", hw)
	require.NoError(t, errA)
	require.NoError(t, errB)
	if a != b {
		t.Errorf("ModelFor non-deterministic: %q vs %q", a, b)
	}
}

// Sanity check: DetectHardware doesn't panic and returns a non-empty Backend.
func TestDetectHardware_Smoke(t *testing.T) {
	hw, err := DetectHardware()
	require.NoError(t, err)
	assert.NotEqual(t, Backend(""), hw.Backend)
}

// TestBuiltinCatalogEmbedded verifies the //go:embed is wired correctly.
func TestBuiltinCatalogEmbedded(t *testing.T) {
	raw, err := BuiltinCatalog()
	require.NoError(t, err)
	if !strings.Contains(string(raw), "schema_version") {
		t.Error("BuiltinCatalog missing schema_version")
	}
}

// TestContextImported is a tiny sanity check that the context import is
// still used; the downloader test file actually exercises it.
func TestContextImported(_ *testing.T) {
	_ = context.Background()
}
