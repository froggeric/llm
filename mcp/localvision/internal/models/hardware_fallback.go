//go:build !darwin && !linux && !windows

package models

// detectHardware fallback for any platform we don't explicitly support
// (e.g. freebsd, netbsd, openbsd). Returns BackendUnsupported.
func detectHardware() (HardwareInfo, error) {
	return HardwareInfo{
		TotalMemoryGB: 0,
		Tier:          TierConstrained,
		Backend:       BackendUnsupported,
		DetectNote:    "Unsupported platform. Only darwin/arm64 is supported in MVP.",
	}, nil
}
