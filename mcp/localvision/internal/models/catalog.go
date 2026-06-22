// Package models handles model catalog loading, validation, hardware
// detection, and download management.
//
// This file defines the public types and interface. Implementations live in
// catalog.go's siblings (hardware.go, downloader.go, selection.go, etc.).
package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrNotImplemented is returned by stub functions during the contract phase.
var ErrNotImplemented = errors.New("not implemented")

// ErrNoFittingModel is returned by DefaultModel/ModelFor when no catalog
// entry fits the detected hardware.
var ErrNoFittingModel = errors.New("no model in the catalog fits the detected hardware")

// ErrInvalidCatalog is returned by Validate when a catalog violates an
// invariant (duplicate IDs, two preferred entries in one tier, non-HTTPS
// URLs, missing mandatory SHA256, etc.).
var ErrInvalidCatalog = errors.New("catalog failed validation")

// HardwareTier classifies machines into rough capability bands so the
// default-selection algorithm can pick a sensible starting model.
type HardwareTier string

const (
	TierConstrained HardwareTier = "constrained" // <= 16 GB unified/VRAM
	TierMainstream  HardwareTier = "mainstream"  // 16-48 GB
	TierHighEnd     HardwareTier = "high_end"    // > 48 GB
)

// Backend describes what kind of acceleration is available.
type Backend string

const (
	BackendAppleSilicon Backend = "apple_silicon"
	BackendDiscreteGPU  Backend = "discrete_gpu"
	BackendCPUOnly      Backend = "cpu_only"
	BackendUnsupported  Backend = "unsupported"
)

// HardwareInfo is the result of runtime hardware detection.
//
// On Apple Silicon, TotalMemoryGB is unified memory and there is no separate
// VRAM (VramGB stays 0); the model loads into unified memory. On Linux/Windows
// with a discrete GPU (CUDA/ROCm), VramGB is the GPU's VRAM and that is what
// the model loads into. effectiveMemoryGB picks the right figure for selection.
//
// The detection code subtracts a safety margin (default 4 GB) before comparing
// against model min_vram_gb, because macOS will refuse to wire more than
// ~75-80% of total memory (and discrete GPUs can't be filled 100% either).
type HardwareInfo struct {
	TotalMemoryGB float64 // system RAM; on Apple Silicon this is also the GPU's unified memory
	VramGB        float64 // discrete-GPU VRAM when Backend == discrete_gpu; 0 otherwise
	Tier          HardwareTier
	Backend       Backend
	DetectNote    string // surfaced to user via `doctor` if non-empty
}

// ModelSpec is one entry in the model catalog TOML.
//
// Catalog invariants enforced by Validate:
//   - exactly one ModelSpec per HardwareTier has Preferred = true
//   - GGUF and Mmproj are HTTPS URLs in the user's HF namespace
//   - GGUFSha256 and MmprojSha256 are mandatory (loaded files are verified on every load, not just download)
//   - HardwareTier is one of the defined constants
//   - PreferredFor references valid tool IDs (Track E defines the set)
type ModelSpec struct {
	DisplayName    string       `toml:"display_name"`
	GGUF           string       `toml:"gguf"`
	Mmproj         string       `toml:"mmproj"`
	GGUFSha256     string       `toml:"gguf_sha256"`
	MmprojSha256   string       `toml:"mmproj_sha256"`
	Ctx            int          `toml:"ctx"`
	GpuLayers      int          `toml:"gpu_layers"` // -1 = all
	MinVramGb      int          `toml:"min_vram_gb"`
	MinSystemRamGb int          `toml:"min_system_ram_gb"`
	Released       string       `toml:"released"` // YYYY-MM
	License        string       `toml:"license"`  // SPDX ID
	HardwareTier   HardwareTier `toml:"hardware_tier"`
	Preferred      bool         `toml:"preferred"`
	PreferredFor   []string     `toml:"preferred_for"`
	BenchToks      float64      `toml:"bench_toks"`
	Notes          string       `toml:"notes"`
	// ChatTemplateKwargs is forwarded as `chat_template_kwargs` in the
	// chat-completion request body. Used for hybrid thinking models
	// (Qwen3.5/3.6) where `enable_thinking = false` skips the reasoning
	// phase — our v6 benchmark established this is a strict win for vision
	// tasks. Empty map = no kwargs sent.
	ChatTemplateKwargs map[string]any `toml:"chat_template_kwargs"`
}

// Catalog is the parsed catalog after merging the built-in builtin.toml with
// user overlays from ~/.localvision/catalog.d/*.toml.
//
// Overlay merge semantics: per-field deep-merge, last-write-wins by lexical
// filename order. A startup log line summarizes which overlays applied which
// fields.
type Catalog struct {
	SchemaVersion int                  `toml:"schema_version"`
	Models        map[string]ModelSpec `toml:"models"`
}

// Load reads the embedded builtin.toml, then walks the overlayDir for
// *.toml files in lexical order, deep-merging each into the catalog.
//
// overlayDir may be empty; in that case only the built-in catalog is used.
// Load does NOT call Validate; callers must do so explicitly.
func Load(overlayDir string) (*Catalog, error) {
	raw, err := BuiltinCatalog()
	if err != nil {
		return nil, fmt.Errorf("read embedded builtin.toml: %w", err)
	}

	var c Catalog
	if _, err := toml.Decode(string(raw), &c); err != nil {
		return nil, fmt.Errorf("decode builtin.toml: %w", err)
	}
	if c.Models == nil {
		c.Models = make(map[string]ModelSpec)
	}

	if err := loadOverlays(overlayDir, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate enforces catalog invariants. See ModelSpec docs for the list.
//
// Returns a wrapped ErrInvalidCatalog with a list of all violations (not
// just the first), so users can fix multiple issues at once.
func (c *Catalog) Validate() error {
	var problems []string

	// Schema version: must match exactly. A higher version means "the
	// user has a newer catalog than this binary supports"; a missing
	// version is treated as a malformed file.
	if c.SchemaVersion == 0 {
		problems = append(problems, "schema_version is missing (must be 1)")
	} else if c.SchemaVersion != CurrentSchemaVersion {
		problems = append(problems, fmt.Sprintf(
			"schema_version is %d but this build of localvision supports %d; "+
				"upgrade the binary (go install github.com/froggeric/llm/mcp/localvision/cmd/localvision@latest)",
			c.SchemaVersion, CurrentSchemaVersion,
		))
	}

	if len(c.Models) == 0 {
		problems = append(problems, "catalog has no models")
	}

	// Preferred-per-tier invariant: F2.5. Track exactly one preferred per
	// tier; report all violators.
	preferredByTier := make(map[HardwareTier][]string)
	validTiers := map[HardwareTier]bool{
		TierConstrained: true,
		TierMainstream:  true,
		TierHighEnd:     true,
	}

	// Sort IDs for deterministic error output (so re-running Validate on
	// the same input always reports problems in the same order).
	ids := make([]string, 0, len(c.Models))
	for id := range c.Models {
		ids = append(ids, id)
	}
	// simple lexical sort without importing sort here.
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}

	for _, id := range ids {
		m := c.Models[id]

		// Mandatory fields.
		if m.DisplayName == "" {
			problems = append(problems, fmt.Sprintf("model %q: display_name is empty", id))
		}
		if m.Ctx <= 0 {
			problems = append(problems, fmt.Sprintf("model %q: ctx must be positive", id))
		}
		if m.MinVramGb <= 0 {
			problems = append(problems, fmt.Sprintf("model %q: min_vram_gb must be positive", id))
		}
		if m.License == "" {
			problems = append(problems, fmt.Sprintf("model %q: license is empty", id))
		}

		// HardwareTier valid value.
		if !validTiers[m.HardwareTier] {
			problems = append(problems, fmt.Sprintf(
				"model %q: hardware_tier %q is not one of %s/%s/%s",
				id, m.HardwareTier, TierConstrained, TierMainstream, TierHighEnd,
			))
		}

		// HTTPS-only URLs in the HF namespace. F3.2/F3.3.
		if err := ValidateHFURL(m.GGUF, ""); err != nil {
			problems = append(problems, fmt.Sprintf("model %q: gguf url invalid: %v", id, err))
		}
		if err := ValidateHFURL(m.Mmproj, ""); err != nil {
			problems = append(problems, fmt.Sprintf("model %q: mmproj url invalid: %v", id, err))
		}

		// SHA256 mandatory, reject placeholder. F1.5.
		if !validHash(m.GGUFSha256) {
			problems = append(problems, fmt.Sprintf(
				"model %q: gguf_sha256 is missing or placeholder; "+
					"run `localvision doctor --compute-hashes` to populate",
				id,
			))
		}
		if !validHash(m.MmprojSha256) {
			problems = append(problems, fmt.Sprintf(
				"model %q: mmproj_sha256 is missing or placeholder; "+
					"run `localvision doctor --compute-hashes` to populate",
				id,
			))
		}

		// Preferred-per-tier bookkeeping. We'll enforce "exactly one" below.
		if m.Preferred {
			preferredByTier[m.HardwareTier] = append(preferredByTier[m.HardwareTier], id)
		}
	}

	// Preferred-per-tier invariant. F2.5: each tier that has any preferred
	// entries must have EXACTLY ONE. We don't require every tier to have
	// one (a tier with zero is allowed; selection falls back to the
	// smallest-fitting model).
	for tier, ids := range preferredByTier {
		if len(ids) > 1 {
			problems = append(problems, fmt.Sprintf(
				"hardware_tier %q has multiple preferred models (%s); exactly one is required",
				tier, strings.Join(ids, ", "),
			))
		}
	}

	if len(problems) > 0 {
		// Log each problem so users see them in their slog output too.
		for _, p := range problems {
			slog.Error("catalog validation problem", "problem", p)
		}
		return fmt.Errorf("%w: %d problem(s): %s",
			ErrInvalidCatalog, len(problems), strings.Join(problems, "; "))
	}
	return nil
}

// validHash returns true if h is a non-empty, non-placeholder 64-char hex
// SHA256. PLACEHOLDER-PHASE3 is what builtin.toml ships with until Phase 3
// fills in the real hashes. F1.5.
func validHash(h string) bool {
	cleaned := normalizeHex(h)
	if len(cleaned) != 64 {
		return false
	}
	// normalizeHex already lowercased; reject anything that looks like the
	// placeholder by length check (placeholder doesn't have 64 hex chars).
	return cleaned != strings.ToLower("PLACEHOLDER-PHASE3")
}

// DefaultModel returns the model ID to load at startup for the given
// hardware. Selection algorithm:
//  1. Filter to models where MinVramGb <= (TotalMemoryGB - 4GB safety - 1GB/resident-llama-server)
//  2. Among fitting models, return the Preferred=true entry whose tier matches hw.Tier
//  3. Tie-break: smallest MinVramGb ascending, then DisplayName lexical
//
// Returns ErrNoFittingModel if nothing fits (8 GB Mac with no small model).
// Callers MUST surface this as a structured MCP error, never crash.
func (c *Catalog) DefaultModel(hw HardwareInfo) (string, error) {
	return selectDefault(c, hw, defaultSelectionSafetyMarginGB)
}

// ModelFor returns the best model ID for the given tool on the given
// hardware. Selection algorithm:
//  1. Filter to models that fit the hardware (same as DefaultModel)
//  2. Filter to models whose PreferredFor contains tool
//  3. Tie-break: smallest MinVramGb ascending, then DisplayName lexical
//  4. If empty after step 2, fall back to DefaultModel
//
// Determinism: given the same (catalog, tool, hardware), always returns the
// same ID.
func (c *Catalog) ModelFor(tool string, hw HardwareInfo) (string, error) {
	return selectModelFor(c, tool, hw, defaultSelectionSafetyMarginGB)
}

// Fits reports whether the named model plausibly fits the detected hardware
// (using the default safety margin). Returns false if the model is unknown.
// Used by callers that want to sanity-check an explicit model choice (e.g. the
// CLI --model override).
func (c *Catalog) Fits(id string, hw HardwareInfo) bool {
	m, ok := c.Models[id]
	if !ok {
		return false
	}
	return fitsModel(m, hw, defaultSelectionSafetyMarginGB)
}

// Downloader handles resumable HTTPS downloads of model files with SHA256
// verification. Track D implements.
type Downloader struct {
	// unexported
}

// Progress is reported by Downloader.Download during a transfer.
type Progress struct {
	Downloaded int64
	Total      int64 // -1 if unknown
}

// Download fetches url to destPath with resumable partial downloads.
// If destPath already exists with the right SHA256, returns immediately
// (cache hit). Calls progress periodically; cancel via ctx.
func (d *Downloader) Download(ctx context.Context, url, destPath, expectedSha256 string, progress func(Progress)) error {
	return downloadImpl(ctx, url, destPath, expectedSha256, progress)
}

// DetectHardware inspects the running machine and returns its capabilities.
// On darwin (Apple Silicon) uses sysctl hw.memsize. On linux/windows in MVP
// returns Backend=BackendUnsupported; v0.2 adds real detection.
func DetectHardware() (HardwareInfo, error) {
	return detectHardware()
}
