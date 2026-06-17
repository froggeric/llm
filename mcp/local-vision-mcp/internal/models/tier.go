package models

// classifyTier maps a total unified/VRAM amount (in GB) to a coarse hardware
// tier. Used by the darwin hardware detector; pure function so it's trivially
// testable without any platform-specific syscalls.
//
// Bands (chosen to align with the model catalog's min_vram_gb spread):
//   - <= 16 GB: constrained  (8 GB Mac can only fit the 4B model; 16 GB borderline)
//   - 16-48 GB: mainstream   (16 GB Macs can fit 8B models; up to 48 GB covers M2/M3 Max)
//   - > 48 GB: high_end      (M3 Max 64GB+, Mac Studio 128GB+)
//
// The boundary at 16 GB is inclusive of "constrained" so an exact 16 GB
// machine still gets the smaller default. The 48 GB boundary is inclusive
// of "mainstream" so 48 GB machines don't get pushed to the 26B model which
// has min_vram_gb=24 and would leave no safety margin.
func classifyTier(totalMemoryGB float64) HardwareTier {
	switch {
	case totalMemoryGB <= 16:
		return TierConstrained
	case totalMemoryGB <= 48:
		return TierMainstream
	default:
		return TierHighEnd
	}
}
