// Package models handles model catalog loading, validation, hardware
// detection, and download management.
//
// This file defines the public types and interface. Implementations live in
// catalog.go's siblings (hardware.go, downloader.go, selection.go, etc.).
package models

import (
	"context"
	"errors"
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
// On Apple Silicon, TotalMemoryGB is unified memory; there is no separate
// VRAM. The detection code subtracts a safety margin (default 4 GB) before
// comparing against model min_vram_gb, because macOS will refuse to wire
// more than ~75-80% of total memory.
type HardwareInfo struct {
	TotalMemoryGB float64
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
	DisplayName     string   `toml:"display_name"`
	GGUF           string   `toml:"gguf"`
	Mmproj         string   `toml:"mmproj"`
	GGUFSha256     string   `toml:"gguf_sha256"`
	MmprojSha256   string   `toml:"mmproj_sha256"`
	Ctx            int      `toml:"ctx"`
	GpuLayers      int      `toml:"gpu_layers"` // -1 = all
	MinVramGb      int      `toml:"min_vram_gb"`
	MinSystemRamGb int      `toml:"min_system_ram_gb"`
	Released       string   `toml:"released"`    // YYYY-MM
	License        string   `toml:"license"`     // SPDX ID
	HardwareTier   HardwareTier `toml:"hardware_tier"`
	Preferred      bool     `toml:"preferred"`
	PreferredFor   []string `toml:"preferred_for"`
	BenchToks      float64  `toml:"bench_toks"`
	Notes          string   `toml:"notes"`
}

// Catalog is the parsed catalog after merging the built-in builtin.toml with
// user overlays from ~/.local-vision-mcp/catalog.d/*.toml.
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
	return nil, ErrNotImplemented
}

// Validate enforces catalog invariants. See ModelSpec docs for the list.
//
// Returns a wrapped ErrInvalidCatalog with a list of all violations (not
// just the first), so users can fix multiple issues at once.
func (c *Catalog) Validate() error {
	return ErrNotImplemented
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
	return "", ErrNotImplemented
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
	return "", ErrNotImplemented
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
	return ErrNotImplemented
}

// DetectHardware inspects the running machine and returns its capabilities.
// On darwin (Apple Silicon) uses sysctl hw.memsize. On linux/windows in MVP
// returns Backend=BackendUnsupported; v0.2 adds real detection.
func DetectHardware() (HardwareInfo, error) {
	return HardwareInfo{Backend: BackendUnsupported}, ErrNotImplemented
}
