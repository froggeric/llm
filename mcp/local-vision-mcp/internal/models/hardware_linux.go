//go:build linux

package models

// detectHardware on linux in MVP returns Backend=BackendUnsupported with
// a note pointing at v0.2. Per F2.2, we deliberately do NOT ship a
// half-working stub that pretends to detect VRAM; the build tag makes the
// failure explicit. The plan calls for CUDA/ROCm detection in v0.2.
//
// We still report total system RAM from /proc/meminfo (best effort) so the
// `doctor` command can show *something* useful.
func detectHardware() (HardwareInfo, error) {
	totalGB := readProcMemTotalGB()
	return HardwareInfo{
		TotalMemoryGB: totalGB,
		Tier:          TierConstrained,
		Backend:       BackendUnsupported,
		DetectNote:    "Linux hardware detection not implemented in MVP (planned for v0.2: CUDA + ROCm). Configure default_model manually.",
	}, nil
}
