//go:build windows

package models

import (
	"context"
	"fmt"
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// detectHardware on windows reports total system RAM via GlobalMemoryStatusEx
// (kernel32; x/sys/windows v0.46 doesn't wrap it, so it's called directly) and
// probes for an NVIDIA CUDA GPU via nvidia-smi (the driver places it on %PATH%).
// If found, VramGB is set and selection uses VRAM.
//
// DirectML (DirectX 12) GPU detection is not implemented: enumerating DXGI
// adapters without cgo is involved, and llama.cpp's DirectML backend is less
// common than CUDA. Users on DirectML-only hardware set default_model manually.
//
// The nvidia-smi probe is wrapped in a 3s timeout (exec.CommandContext) so a
// hung driver can't stall boot. The probes are package-level vars so tests can
// exercise the branching.
const gpuProbeTimeout = 3 * time.Second

var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemStat = kernel32.NewProc("GlobalMemoryStatusEx")

	winTotalMemoryGB = func() (float64, error) {
		// MEMORYSTATUSEX layout (Win32): two DWORDs then DWORDLONGs.
		var m struct {
			length       uint32
			memoryLoad   uint32
			totalPhys    uint64
			availPhys    uint64
			totalPF      uint64
			availPF      uint64
			totalVirt    uint64
			availVirt    uint64
			availExtVirt uint64
		}
		m.length = uint32(unsafe.Sizeof(m))
		r, _, _ := procGlobalMemStat.Call(uintptr(unsafe.Pointer(&m)))
		if r == 0 {
			return 0, fmt.Errorf("GlobalMemoryStatusEx failed")
		}
		return float64(m.totalPhys) / 1024 / 1024 / 1024, nil
	}
	winRunNvidiaSmi = func() ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gpuProbeTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output()
	}
)

func detectHardware() (HardwareInfo, error) {
	memGB, memErr := winTotalMemoryGB()
	hw := HardwareInfo{TotalMemoryGB: memGB, Backend: BackendCPUOnly}

	if out, err := winRunNvidiaSmi(); err == nil {
		if vram, ok := parseNvidiaSmiVRAM(string(out)); ok {
			hw.VramGB = vram
			hw.Backend = BackendDiscreteGPU
			hw.Tier = classifyTier(effectiveMemoryGB(hw))
			hw.DetectNote = fmt.Sprintf("Windows: NVIDIA CUDA GPU, %.1f GB VRAM (selection uses VRAM).", vram)
			return hw, nil
		}
	}

	hw.Tier = classifyTier(memGB)
	if memErr != nil {
		// A 0 GB Windows box is impossible — surface that detection failed so
		// the user isn't mystified by a misleading "constrained" tier.
		hw.Backend = BackendUnsupported
		hw.DetectNote = "Windows: memory detection failed (" + memErr.Error() + "); set default_model manually."
		return hw, nil
	}
	hw.DetectNote = "Windows: no NVIDIA GPU detected (CPU-only). DirectML detection is planned; set default_model for now."
	return hw, nil
}
