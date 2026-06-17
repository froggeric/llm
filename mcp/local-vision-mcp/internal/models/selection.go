package models

import (
	"errors"
	"fmt"
	"sort"
)

// safetyMarginGB is subtracted from hw.TotalMemoryGB when computing
// "available" memory for model loading. F1.6. Default 4 GB matches the
// plan; user can override via config.safety_margin_gb (Track A).
//
// We do NOT import config here to avoid a circular dependency; selection
// is called by callers that already have the user-configured margin.
// defaultSelectionSafetyMarginGB is a fallback used when the caller does
// not provide one (e.g. tests).
const defaultSelectionSafetyMarginGB = 4.0

// residentLlamaServerMarginGB is the additional memory budget we reserve
// for the llama-server subprocess itself (~1 GB resident overhead). The
// plan's pseudocode uses "minus 1 (per running llama-server)".
const residentLlamaServerMarginGB = 1.0

// fitsModel returns true if the model can plausibly fit in the detected
// hardware, per F1.6:
//
//	available = totalMemoryGB - safetyMarginGB - residentMarginGB
//	fits      = available >= model.MinVramGb
//
// safetyMarginGB defaults to 4 if 0 or negative. We use >= rather than >
// because the catalog's min_vram_gb is itself a floor.
func fitsModel(model ModelSpec, hw HardwareInfo, safetyMarginGB float64) bool {
	margin := safetyMarginGB
	if margin <= 0 {
		margin = defaultSelectionSafetyMarginGB
	}
	available := hw.TotalMemoryGB - margin - residentLlamaServerMarginGB
	return float64(model.MinVramGb) <= available
}

// fittingModels returns catalog entries (id, spec) that fit the hardware,
// sorted deterministically: smallest MinVramGb ascending, then DisplayName
// lexically. F1.8 determinism.
func fittingModels(c *Catalog, hw HardwareInfo, safetyMarginGB float64) []modelEntry {
	out := make([]modelEntry, 0, len(c.Models))
	for id, m := range c.Models {
		if fitsModel(m, hw, safetyMarginGB) {
			out = append(out, modelEntry{id: id, spec: m})
		}
	}
	sortDeterministic(out)
	return out
}

// modelEntry pairs an ID with its spec for sorting. Used internally.
type modelEntry struct {
	id   string
	spec ModelSpec
}

// sortDeterministic orders entries by:
//  1. MinVramGb ascending
//  2. DisplayName ascending (lexical)
//
// The ID is NOT a tiebreaker: DisplayName is the user-facing label and is
// guaranteed unique by Validate (which rejects two models with the same
// DisplayName in the same tier to avoid this ambiguity).
func sortDeterministic(in []modelEntry) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].spec.MinVramGb != in[j].spec.MinVramGb {
			return in[i].spec.MinVramGb < in[j].spec.MinVramGb
		}
		return in[i].spec.DisplayName < in[j].spec.DisplayName
	})
}

// selectDefault implements the DefaultModel algorithm from PLAN-v2.md:
//
//  1. Filter to fitting models.
//  2. If empty: return ErrNoFittingModel.
//  3. If the Preferred=true entry whose HardwareTier matches hw.Tier is in
//     the fitting set, return its ID.
//  4. Otherwise, return the first entry in deterministic order.
//
// safetyMarginGB is configurable so callers can pass through the user's
// config. Tests pass 0 to get the default.
func selectDefault(c *Catalog, hw HardwareInfo, safetyMarginGB float64) (string, error) {
	candidates := fittingModels(c, hw, safetyMarginGB)
	if len(candidates) == 0 {
		return "", fmt.Errorf(
			"%w: total %.1f GB, no model fits with %.0f GB safety margin",
			ErrNoFittingModel, hw.TotalMemoryGB, safetyMarginGB,
		)
	}
	// Step 3: look for the tier's preferred entry among fitting candidates.
	// Validate has already enforced "exactly one preferred per tier", so
	// finding one matching (tier, preferred=true) is unambiguous.
	for _, e := range candidates {
		if e.spec.Preferred && e.spec.HardwareTier == hw.Tier {
			return e.id, nil
		}
	}
	// Step 4: deterministic fallback to smallest, then lexically-first.
	return candidates[0].id, nil
}

// selectModelFor implements the ModelFor algorithm from PLAN-v2.md:
//
//  1. Filter to fitting models.
//  2. If empty: fall through to DefaultModel (which itself may return
//     ErrNoFittingModel).
//  3. Filter the fitting set to those whose PreferredFor contains tool.
//  4. If non-empty: return the first by deterministic order.
//  5. If empty: fall back to DefaultModel.
//
// Determinism: same (catalog, tool, hardware) always returns same ID. F1.8.
func selectModelFor(c *Catalog, tool string, hw HardwareInfo, safetyMarginGB float64) (string, error) {
	fitting := fittingModels(c, hw, safetyMarginGB)
	if len(fitting) == 0 {
		// Nothing fits at all -> surface the no-fitting-model error
		// directly; don't pretend DefaultModel can rescue us.
		return selectDefault(c, hw, safetyMarginGB)
	}

	// Build the "explicitly listed for this tool" set.
	var listed []modelEntry
	for _, e := range fitting {
		for _, t := range e.spec.PreferredFor {
			if t == tool {
				listed = append(listed, e)
				break
			}
		}
	}
	if len(listed) == 0 {
		// No model explicitly lists this tool. Fall back to default.
		return selectDefault(c, hw, safetyMarginGB)
	}
	// The list was built by iterating over already-sorted fitting, so
	// listed is also in deterministic order. Return the first.
	return listed[0].id, nil
}

// errNothingFits is a sentinel for tests; not exported.
var errNothingFits = errors.New("nothing fits")
