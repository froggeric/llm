//go:build windows

package models

// detectHardware on windows in MVP returns Backend=BackendUnsupported
// with a note pointing at v0.2. Per F2.2, we deliberately do NOT ship a
// half-working stub. CUDA / DirectML detection is v0.2 work.
func detectHardware() (HardwareInfo, error) {
	return HardwareInfo{
		TotalMemoryGB: 0,
		Tier:          TierConstrained,
		Backend:       BackendUnsupported,
		DetectNote:    "Windows hardware detection not implemented in MVP (planned for v0.2: CUDA + DirectML). Configure default_model manually.",
	}, nil
}
