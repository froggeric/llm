//go:build darwin

package models

import (
	"log/slog"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

// detectHardware on darwin:
//   - On arm64 (Apple Silicon): reads hw.memsize via sysctl for total
//     unified memory. There is no separate VRAM. The Backend is
//     apple_silicon. Tier is derived from classifyTier.
//   - On amd64 (Intel Macs): we don't support discrete-GPU detection in
//     MVP. Returns Backend=cpu_only with a note pointing at v0.2.
//
// Detection is documented as an estimate. Real "available" VRAM depends on
// how much wired memory other apps have grabbed; we approximate with total
// minus a safety margin applied later in selection. The user can override
// via config.safety_margin_gb or config.default_model.
func detectHardware() (HardwareInfo, error) {
	// Total physical memory in bytes via sysctl.
	memBytes, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		slog.Warn("hw.memsize sysctl failed; falling back to conservative defaults",
			"err", err)
		// Return something safe rather than fail the whole server.
		return HardwareInfo{
			TotalMemoryGB: 0,
			Tier:          TierConstrained,
			Backend:       BackendUnsupported,
			DetectNote:    "hw.memsize sysctl failed: " + err.Error(),
		}, nil
	}

	totalGB := float64(memBytes) / 1024 / 1024 / 1024

	switch runtime.GOARCH {
	case "arm64":
		tier := classifyTier(totalGB)
		// Build a human-readable note explaining the estimate + how to
		// override. Surfaced via `doctor` output.
		var sb strings.Builder
		sb.WriteString("estimated; manual override via config if wrong")
		sb.WriteString(" (Apple Silicon: unified memory, no separate VRAM)")
		return HardwareInfo{
			TotalMemoryGB: totalGB,
			Tier:          tier,
			Backend:       BackendAppleSilicon,
			DetectNote:    sb.String(),
		}, nil
	default:
		// amd64 / Intel Mac. We don't have Metal support wired up in MVP
		// for discrete GPUs; treat as CPU-only which is too slow for VLMs
		// but lets the binary at least run.
		return HardwareInfo{
			TotalMemoryGB: totalGB,
			Tier:          classifyTier(totalGB),
			Backend:       BackendCPUOnly,
			DetectNote:    "Intel Mac: CPU-only mode. Apple Silicon recommended. v0.2 may add discrete GPU support.",
		}, nil
	}
}
