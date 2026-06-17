package models

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// loadOverlays walks dir for *.toml files in lexical (filename) order and
// deep-merges each into the existing catalog. Merge semantics: per-field,
// last-write-wins by lexical filename order.
//
// Each overlay file is itself a complete Catalog TOML (same schema as
// builtin.toml). Only the [models.<id>] entries that exist in the overlay
// are touched; entries absent from the overlay are left alone.
//
// For an [models.<id>] entry present in the overlay:
//   - Every field that is explicitly set in the overlay REPLACES the
//     corresponding field in the existing catalog entry. We treat this as
//     "field is present in TOML" semantics via a pointer-using intermediate
//     struct (see mergeModelSpec below).
//   - If <id> doesn't exist yet in the catalog, it is created.
//
// Each applied field is logged via slog at Info level so users can audit
// which overlays changed which models. F2.4.
//
// dir may be empty or non-existent; in that case loadOverlays is a no-op.
func loadOverlays(dir string, into *Catalog) error {
	if dir == "" {
		return nil
	}
	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat overlay dir %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("overlay dir %s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read overlay dir %s: %w", dir, err)
	}

	// Collect *.toml filenames in lexical order. F2.4 specifies
	// last-write-wins by lexical filename, so we apply in ascending order.
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return nil
	}

	slog.Info("applying catalog overlays", "dir", dir, "count", len(names))

	for _, name := range names {
		full := filepath.Join(dir, name)
		if err := applyOverlayFile(full, name, into); err != nil {
			return fmt.Errorf("overlay %s: %w", name, err)
		}
	}
	return nil
}

// overlayCatalog is a pointer-field mirror of Catalog so we can distinguish
// "field absent from TOML" from "field present with zero value". We use
// this to implement true per-field merge semantics rather than naive
// overwrite-everything.
type overlayModelSpec struct {
	DisplayName     *string   `toml:"display_name"`
	GGUF           *string   `toml:"gguf"`
	Mmproj         *string   `toml:"mmproj"`
	GGUFSha256     *string   `toml:"gguf_sha256"`
	MmprojSha256   *string   `toml:"mmproj_sha256"`
	Ctx            *int      `toml:"ctx"`
	GpuLayers      *int      `toml:"gpu_layers"`
	MinVramGb      *int      `toml:"min_vram_gb"`
	MinSystemRamGb *int      `toml:"min_system_ram_gb"`
	Released       *string   `toml:"released"`
	License        *string   `toml:"license"`
	HardwareTier   *string   `toml:"hardware_tier"`
	Preferred      *bool     `toml:"preferred"`
	PreferredFor   *[]string `toml:"preferred_for"`
	BenchToks      *float64  `toml:"bench_toks"`
	Notes          *string   `toml:"notes"`
}

// rawOverlay is what we decode an overlay TOML into. Models is a map of
// overlayModelSpec keyed by model ID.
type rawOverlay struct {
	SchemaVersion *int                          `toml:"schema_version"`
	Models        map[string]overlayModelSpec   `toml:"models"`
}

// applyOverlayFile reads one overlay TOML and merges its models into into.
// Each replaced field produces a slog.Info line. SchemaVersion (if set) is
// only logged; we do NOT let an overlay downgrade the catalog's
// schema_version silently — that decision belongs to the loader.
func applyOverlayFile(path, name string, into *Catalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var ov rawOverlay
	if _, err := toml.Decode(string(raw), &ov); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	if ov.SchemaVersion != nil && *ov.SchemaVersion != into.SchemaVersion {
		slog.Warn("overlay declares a different schema_version; ignoring the overlay's schema_version field",
			"overlay", name,
			"catalog_schema_version", into.SchemaVersion,
			"overlay_schema_version", *ov.SchemaVersion,
		)
	}

	for id, spec := range ov.Models {
		existing, ok := into.Models[id]
		if !ok {
			// New model introduced by overlay. Log and insert.
			existing = ModelSpec{}
			slog.Info("overlay adds new model", "overlay", name, "model_id", id)
			into.Models[id] = mergeModelSpec(existing, spec, name, id)
			continue
		}
		slog.Info("overlay modifies existing model", "overlay", name, "model_id", id)
		into.Models[id] = mergeModelSpec(existing, spec, name, id)
	}
	return nil
}

// mergeModelSpec applies every non-nil pointer field from ov onto dst,
// logging each applied field. Returns the merged result.
//
// Slices (PreferredFor) are REPLACED, not concatenated, when present in the
// overlay. This is consistent with "per-field last-write-wins" semantics:
// the overlay's value is the source of truth.
func mergeModelSpec(dst ModelSpec, ov overlayModelSpec, overlayName, modelID string) ModelSpec {
	tag := func(field, val string) {
		slog.Info("overlay applied field",
			"overlay", overlayName,
			"model_id", modelID,
			"field", field,
			"value", val,
		)
	}
	if ov.DisplayName != nil {
		dst.DisplayName = *ov.DisplayName
		tag("display_name", *ov.DisplayName)
	}
	if ov.GGUF != nil {
		dst.GGUF = *ov.GGUF
		tag("gguf", *ov.GGUF)
	}
	if ov.Mmproj != nil {
		dst.Mmproj = *ov.Mmproj
		tag("mmproj", *ov.Mmproj)
	}
	if ov.GGUFSha256 != nil {
		dst.GGUFSha256 = *ov.GGUFSha256
		tag("gguf_sha256", *ov.GGUFSha256)
	}
	if ov.MmprojSha256 != nil {
		dst.MmprojSha256 = *ov.MmprojSha256
		tag("mmproj_sha256", *ov.MmprojSha256)
	}
	if ov.Ctx != nil {
		dst.Ctx = *ov.Ctx
		tag("ctx", strconvItoa(*ov.Ctx))
	}
	if ov.GpuLayers != nil {
		dst.GpuLayers = *ov.GpuLayers
		tag("gpu_layers", strconvItoa(*ov.GpuLayers))
	}
	if ov.MinVramGb != nil {
		dst.MinVramGb = *ov.MinVramGb
		tag("min_vram_gb", strconvItoa(*ov.MinVramGb))
	}
	if ov.MinSystemRamGb != nil {
		dst.MinSystemRamGb = *ov.MinSystemRamGb
		tag("min_system_ram_gb", strconvItoa(*ov.MinSystemRamGb))
	}
	if ov.Released != nil {
		dst.Released = *ov.Released
		tag("released", *ov.Released)
	}
	if ov.License != nil {
		dst.License = *ov.License
		tag("license", *ov.License)
	}
	if ov.HardwareTier != nil {
		// We store the raw string; Validate will catch an invalid tier.
		dst.HardwareTier = HardwareTier(*ov.HardwareTier)
		tag("hardware_tier", *ov.HardwareTier)
	}
	if ov.Preferred != nil {
		dst.Preferred = *ov.Preferred
		tag("preferred", strconvBool(*ov.Preferred))
	}
	if ov.PreferredFor != nil {
		dst.PreferredFor = append([]string(nil), *ov.PreferredFor...)
		tag("preferred_for", strings.Join(*ov.PreferredFor, ","))
	}
	if ov.BenchToks != nil {
		dst.BenchToks = *ov.BenchToks
		tag("bench_toks", strconvFtoa(*ov.BenchToks))
	}
	if ov.Notes != nil {
		dst.Notes = *ov.Notes
		tag("notes", *ov.Notes)
	}
	return dst
}

// strconvItoa avoids importing strconv at the top of the file just for one
// helper. (It's still imported indirectly via fmt.)
func strconvItoa(i int) string { return fmt.Sprintf("%d", i) }
func strconvBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func strconvFtoa(f float64) string { return fmt.Sprintf("%v", f) }
