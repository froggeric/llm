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

// effectiveMemoryGB returns the memory a model loads into: VRAM when a discrete
// GPU is present, else system (unified) RAM. On Apple Silicon the backend is
// apple_silicon (not discrete) and VramGB is 0, so this returns TotalMemoryGB —
// preserving the pre-v0.4 Apple behaviour exactly. On Linux/Windows + NVIDIA/ROCm
// it returns VRAM, which is correct since llama-server offloads the model there.
func effectiveMemoryGB(hw HardwareInfo) float64 {
	if hw.Backend == BackendDiscreteGPU && hw.VramGB > 0 {
		return hw.VramGB
	}
	return hw.TotalMemoryGB
}

// fitsModel returns true if the model can plausibly fit in the detected
// hardware, per F1.6:
//
//	available = effectiveMemoryGB(hw) - safetyMarginGB - residentMarginGB
//	fits      = available >= model.MinVramGb
//
// safetyMarginGB defaults to 4 if 0 or negative. We use >= rather than >
// because the catalog's min_vram_gb is itself a floor.
func fitsModel(model ModelSpec, hw HardwareInfo, safetyMarginGB float64) bool {
	margin := safetyMarginGB
	if margin <= 0 {
		margin = defaultSelectionSafetyMarginGB
	}
	available := effectiveMemoryGB(hw) - margin - residentLlamaServerMarginGB
	return float64(model.MinVramGb) <= available
}

// fittingModels returns catalog entries (id, spec) that fit the hardware,
// sorted deterministically: LARGEST MinVramGb first (most capable model that
// fits wins), then DisplayName lexically as a tiebreaker. F1.8 determinism.
//
// We pick largest-fitting rather than smallest-fitting because the catalog
// is ordered by capability — bigger models are generally better when they
// fit. A 96 GB Mac should get Qwen3-VL 8B, not 4B, when both fit.
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
//  1. MinVramGb DESCENDING (most capable first)
//  2. DisplayName ascending (lexical)
//
// The "largest fitting wins" rule means: when a user has plenty of VRAM,
// they get the best model that fits, not the smallest one. The
// tier-preferred lookup in selectDefault still takes priority — this sort
// only applies when no preferred model matches the user's tier.
func sortDeterministic(in []modelEntry) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].spec.MinVramGb != in[j].spec.MinVramGb {
			return in[i].spec.MinVramGb > in[j].spec.MinVramGb
		}
		return in[i].spec.DisplayName < in[j].spec.DisplayName
	})
}

// selectDefault implements the DefaultModel algorithm:
//
//  1. Filter to fitting models.
//  2. If empty: return ErrNoFittingModel.
//  3. If the Preferred=true entry whose HardwareTier matches hw.Tier is in
//     the fitting set, return its ID.
//  4. Otherwise, prefer the largest fitting model that lists at least one
//     tool in preferred_for (so the generic default matches what tool calls
//     actually use); if none lists a tool, fall back to the largest fitting
//     model overall.
//
// safetyMarginGB is configurable so callers can pass through the user's
// config. Tests pass 0 to get the default.
func selectDefault(c *Catalog, hw HardwareInfo, safetyMarginGB float64) (string, error) {
	// Normalize once so the no-fitting-model error reports the margin actually
	// applied (callers/tests pass <=0 to mean "use the default"; fitsModel would
	// otherwise substitute 4 GB silently and the message would read "0 GB").
	if safetyMarginGB <= 0 {
		safetyMarginGB = defaultSelectionSafetyMarginGB
	}
	candidates := fittingModels(c, hw, safetyMarginGB)
	if len(candidates) == 0 {
		return "", fmt.Errorf(
			"%w: total %.1f GB, no model fits with %.0f GB safety margin",
			ErrNoFittingModel, hw.TotalMemoryGB, safetyMarginGB,
		)
	}
	// Step 3: look for the tier's preferred entry among fitting candidates.
	// Validate has already enforced "at most one preferred per tier", so
	// finding one matching (tier, preferred=true) is unambiguous.
	for _, e := range candidates {
		if e.spec.Preferred && e.spec.HardwareTier == hw.Tier {
			return e.id, nil
		}
	}
	// Step 4: deterministic fallback. fittingModels sorts by MinVramGb
	// DESCENDING, so iteration is largest-first. Prefer models that list at
	// least one tool (preferred_for non-empty): a model that lists no tool is
	// opt-in (--model only) and should not become the default just because it
	// is the largest. If none lists a tool, take the largest fitting model.
	var listed []modelEntry
	for _, e := range candidates {
		if len(e.spec.PreferredFor) > 0 {
			listed = append(listed, e)
		}
	}
	if len(listed) > 0 {
		return listed[0].id, nil
	}
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
	if safetyMarginGB <= 0 {
		safetyMarginGB = defaultSelectionSafetyMarginGB
	}
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
