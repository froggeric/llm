// Package setup holds the first-run configuration logic shared between the
// interactive `setup` command and its tests.
//
// It is deliberately framework-free (no bubbletea/lipgloss): the interactive
// layer (prompts, ANSI) lives in cmd/localvision and is a thin reader over the
// pure functions here. That keeps the recommendation/serialization logic fully
// unit-testable and avoids pulling a TUI dependency for a one-time wizard.
package setup

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
)

// ModelOption is one selectable model in the setup wizard.
type ModelOption struct {
	ID          string
	DisplayName string
	Tier        models.HardwareTier
	Fits        bool // fits the detected hardware per the catalog
	Recommended bool // the catalog's default model for this hardware
}

// lookLLAMA is the llama-server discovery seam, overridable in tests so
// DetectLLAMAServer can be exercised without a real binary on $PATH.
var lookLLAMA = exec.LookPath

// DetectLLAMAServer reports whether a llama-server binary is on $PATH and
// returns its resolved path. Used by the wizard to show install guidance.
func DetectLLAMAServer() (path string, found bool) {
	p, err := lookLLAMA("llama-server")
	if err != nil {
		return "", false
	}
	return p, true
}

// ModelOptions returns every catalog model annotated with fit + recommended
// status, ordered so the recommended model is first, then other fitting
// models, then non-fitting ones. The recommended model is the catalog's
// DefaultModel(hw); if that selection fails (e.g. unsupported backend),
// nothing is marked recommended and the caller decides a default.
func ModelOptions(catalog *models.Catalog, hw models.HardwareInfo) []ModelOption {
	if catalog == nil {
		return nil
	}
	recommended := ""
	if id, err := catalog.DefaultModel(hw); err == nil {
		recommended = id
	}
	opts := make([]ModelOption, 0, len(catalog.Models))
	for id, m := range catalog.Models {
		opts = append(opts, ModelOption{
			ID:          id,
			DisplayName: m.DisplayName,
			Tier:        m.HardwareTier,
			Fits:        catalog.Fits(id, hw),
			Recommended: id == recommended,
		})
	}
	sort.Slice(opts, func(i, j int) bool {
		// Recommended model first.
		if opts[i].Recommended != opts[j].Recommended {
			return opts[i].Recommended
		}
		// Then fitting before non-fitting.
		if opts[i].Fits != opts[j].Fits {
			return opts[i].Fits
		}
		// Then by tier, then display name for stable ordering.
		if opts[i].Tier != opts[j].Tier {
			return opts[i].Tier < opts[j].Tier
		}
		return opts[i].DisplayName < opts[j].DisplayName
	})
	return opts
}

// Choices holds the user's setup selections.
type Choices struct {
	Model         string // catalog ID
	DefaultFormat string // optional default --format; empty = presentational
	// PerToolRouting, when true, writes explicit [tools.<id>].model tables for
	// the benchmark's recommended per-tool routing (derived from the catalog's
	// preferred_for partition for the detected hardware). v0.7.
	PerToolRouting bool
}

// BuildConfig applies Choices to a base config (typically freshly loaded with
// defaults) and returns the config to persist. It validates that the chosen
// model exists in the catalog. The base is shallow-copied, not mutated.
func BuildConfig(base *config.Config, catalog *models.Catalog, hw models.HardwareInfo, ch Choices) (*config.Config, error) {
	if base == nil {
		return nil, fmt.Errorf("setup: base config is nil")
	}
	if catalog == nil {
		return nil, fmt.Errorf("setup: catalog is nil")
	}
	if ch.Model == "" {
		return nil, fmt.Errorf("setup: no model selected")
	}
	if _, ok := catalog.Models[ch.Model]; !ok {
		return nil, fmt.Errorf("setup: model %q is not in the catalog", ch.Model)
	}
	out := *base // shallow copy is safe: every field is a value type (string/duration)
	out.DefaultModel = ch.Model
	out.DefaultFormat = ch.DefaultFormat
	if ch.PerToolRouting {
		// Write explicit [tools.<id>].model tables for the benchmark's
		// recommended per-tool routing, derived from the catalog's preferred_for
		// partition resolved against the detected hardware (catalog.ModelFor).
		// This makes the routing visible + editable in the TOML; it matches what
		// the executor would pick anyway, but freezing it lets the user tweak
		// individual tools. method (sampling) is left unset (off).
		tools := routedToolIDs(catalog)
		routing := make(map[string]config.ToolConfig, len(tools))
		for _, t := range tools {
			m, err := catalog.ModelFor(t, hw)
			if err != nil {
				continue // skip tools with no fitting model (e.g. unsupported backend)
			}
			routing[t] = config.ToolConfig{Model: m}
		}
		if len(routing) > 0 {
			out.Tools = routing
		}
	}
	return &out, nil
}

// routedToolIDs returns the set of tool IDs the catalog routes (the union of
// every model's preferred_for). Used by BuildConfig to write the recommended
// per-tool routing. Deterministic order.
func routedToolIDs(catalog *models.Catalog) []string {
	seen := map[string]bool{}
	for _, m := range catalog.Models {
		for _, t := range m.PreferredFor {
			seen[t] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
