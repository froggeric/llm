//go:build linux

package models

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// detectHardware on linux probes system RAM (/proc/meminfo) and GPU backends:
//   - NVIDIA CUDA via `nvidia-smi` (VRAM from --query-gpu=memory.total).
//   - AMD ROCm via `rocm-smi` (VRAM from --showmeminfo vram --json).
//
// If a discrete GPU is found, VramGB is set and model selection uses VRAM (see
// effectiveMemoryGB). Otherwise the backend is cpu_only and selection falls back
// to system RAM (vision will be slow but functional).
//
// The subprocess probes are package-level vars so tests can exercise
// detectHardware's branching without nvidia-smi/rocm-smi installed. Each is
// wrapped in a 3s timeout (exec.CommandContext) so a hung driver can't stall
// server boot indefinitely.
const gpuProbeTimeout = 3 * time.Second

var (
	linuxRunNvidiaSmi = func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output()
	}
	linuxRunRocmSmi = func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "rocm-smi", "--showmeminfo", "vram", "--json").Output()
	}
)

func detectHardware() (HardwareInfo, error) {
	hw := HardwareInfo{TotalMemoryGB: readProcMemTotalGB(), Backend: BackendCPUOnly}

	if out, err := linuxRunNvidiaSmi(); err == nil {
		if vram, ok := parseNvidiaSmiVRAM(string(out)); ok {
			hw.VramGB = vram
			hw.Backend = BackendDiscreteGPU
			hw.Tier = classifyTier(effectiveMemoryGB(hw))
			hw.DetectNote = fmt.Sprintf("Linux: NVIDIA CUDA GPU, %.1f GB VRAM (selection uses VRAM).", vram)
			return hw, nil
		}
	}

	if out, err := linuxRunRocmSmi(); err == nil {
		if vram, ok := parseRocmSMIVRAM(string(out)); ok {
			hw.VramGB = vram
			hw.Backend = BackendDiscreteGPU
			hw.Tier = classifyTier(effectiveMemoryGB(hw))
			hw.DetectNote = fmt.Sprintf("Linux: AMD ROCm GPU, %.1f GB VRAM (selection uses VRAM).", vram)
			return hw, nil
		}
	}

	hw.Tier = classifyTier(hw.TotalMemoryGB)
	hw.DetectNote = "Linux: no NVIDIA/ROCm GPU detected (CPU-only; vision will be slow). Install CUDA/ROCm drivers, or set default_model."
	return hw, nil
}
