package models

import (
	"testing"
)

func TestClassifyTier(t *testing.T) {
	cases := []struct {
		name string
		gb   float64
		want HardwareTier
	}{
		{"zero", 0, TierConstrained},
		{"4GB (under-spec)", 4, TierConstrained},
		{"8GB M1", 8, TierConstrained},
		{"16GB M2 (inclusive lower boundary)", 16, TierConstrained},
		{"17GB (just above 16)", 17, TierMainstream},
		{"24GB M2 Max", 24, TierMainstream},
		{"32GB M3 Max", 32, TierMainstream},
		{"48GB (inclusive upper boundary)", 48, TierMainstream},
		{"49GB", 49, TierHighEnd},
		{"64GB", 64, TierHighEnd},
		{"128GB Mac Studio", 128, TierHighEnd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTier(tc.gb)
			if got != tc.want {
				t.Errorf("classifyTier(%.1f) = %q; want %q", tc.gb, got, tc.want)
			}
		})
	}
}

func TestClassifyTier_NegativeAndHuge(t *testing.T) {
	// Defensive: negative doesn't panic, classifies to constrained.
	if got := classifyTier(-1); got != TierConstrained {
		t.Errorf("classifyTier(-1) = %q; want %q", got, TierConstrained)
	}
	// Pathological large value goes to high_end.
	if got := classifyTier(1e6); got != TierHighEnd {
		t.Errorf("classifyTier(1e6) = %q; want %q", got, TierHighEnd)
	}
}

func TestDetectHardware_RunsOnThisMachine(t *testing.T) {
	// This is a smoke test: detectHardware must at least return a valid
	// Backend and not panic. We don't assert specific values because the
	// test runs on whichever platform CI is on.
	hw, err := detectHardware()
	if err != nil {
		t.Fatalf("detectHardware returned error: %v", err)
	}
	if hw.Backend == "" {
		t.Error("Backend is empty")
	}
	if hw.TotalMemoryGB < 0 {
		t.Errorf("TotalMemoryGB is negative: %f", hw.TotalMemoryGB)
	}
}
