package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNvidiaSmiVRAM(t *testing.T) {
	cases := []struct {
		name   string
		out    string
		wantGB float64
		wantOK bool
	}{
		{"single gpu", "24576\n", 24, true},
		{"multi gpu uses first", "24576\n16384\n", 24, true},
		{"with whitespace", "  8192  \n", 8, true},
		{"empty", "", 0, false},
		{"NA", "N/A\n", 0, false},
		{"garbage", "not a number", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gb, ok := parseNvidiaSmiVRAM(c.out)
			assert.Equal(t, c.wantOK, ok, "ok")
			if ok {
				assert.InDelta(t, c.wantGB, gb, 0.01, "vram GB")
			}
		})
	}
}

func TestParseRocmSMIVRAM(t *testing.T) {
	// Realistic rocm-smi --showmeminfo vram --json output.
	out := `{
  "card0": { "VRAM Total Memory (B)": "17163091968" },
  "card1": { "VRAM Total Memory (B)": "17163091968" }
}`
	gb, ok := parseRocmSMIVRAM(out)
	require.True(t, ok)
	assert.InDelta(t, 16.0, gb, 0.1, "16 GB card")

	// Deterministic: picks the lexically-first card.
	_, ok = parseRocmSMIVRAM(`{"card0": {"VRAM Total Memory (B)": "0"}}`)
	assert.False(t, ok, "zero VRAM is not a valid detection")

	_, ok = parseRocmSMIVRAM("")
	assert.False(t, ok)
	_, ok = parseRocmSMIVRAM("{not json")
	assert.False(t, ok)
	// Field without "total" is ignored.
	_, ok = parseRocmSMIVRAM(`{"card0": {"VRAM Used Memory (B)": "1000"}}`)
	assert.False(t, ok)
}

func TestEffectiveMemoryGB(t *testing.T) {
	cases := []struct {
		name string
		hw   HardwareInfo
		want float64
	}{
		{"discrete gpu uses vram", HardwareInfo{Backend: BackendDiscreteGPU, TotalMemoryGB: 64, VramGB: 24}, 24},
		{"apple uses unified system ram", HardwareInfo{Backend: BackendAppleSilicon, TotalMemoryGB: 18}, 18},
		{"cpu only uses system ram", HardwareInfo{Backend: BackendCPUOnly, TotalMemoryGB: 32}, 32},
		{"discrete but vram unknown falls back to system", HardwareInfo{Backend: BackendDiscreteGPU, TotalMemoryGB: 32, VramGB: 0}, 32},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.InDelta(t, c.want, effectiveMemoryGB(c.hw), 0.001)
		})
	}
}

// TestFitsModelUsesVRAMForDiscreteGPU proves a model that fits in system RAM but
// not in a small discrete GPU's VRAM is correctly rejected — the core
// cross-platform correctness property of v0.4.
func TestFitsModelUsesVRAMForDiscreteGPU(t *testing.T) {
	model := ModelSpec{MinVramGb: 20}
	const safety = 4.0 // +1 resident → available = effective - 5

	// 64 GB system RAM, 16 GB VRAM: effective is 16, available 11 → does NOT fit.
	discrete := HardwareInfo{Backend: BackendDiscreteGPU, TotalMemoryGB: 64, VramGB: 16}
	assert.False(t, fitsModel(model, discrete, safety), "must use VRAM (16), not system RAM (64)")

	// Same machine reported as CPU-only: effective is 64 → fits.
	cpu := HardwareInfo{Backend: BackendCPUOnly, TotalMemoryGB: 64}
	assert.True(t, fitsModel(model, cpu, safety))

	// Apple Silicon with 64 GB unified: effective is 64 → fits (no regression).
	apple := HardwareInfo{Backend: BackendAppleSilicon, TotalMemoryGB: 64}
	assert.True(t, fitsModel(model, apple, safety))

	// Discrete GPU with ample VRAM (32 GB): effective 32, available 27 → fits.
	bigGPU := HardwareInfo{Backend: BackendDiscreteGPU, TotalMemoryGB: 64, VramGB: 32}
	assert.True(t, fitsModel(model, bigGPU, safety))
}
