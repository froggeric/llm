package models

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// This file holds pure parsers for the stdout of GPU-probing CLIs (nvidia-smi,
// rocm-smi). They carry no build tag so they compile on every platform and can
// be unit-tested on the dev machine (darwin); the OS-specific detectHardware
// implementations (hardware_linux.go, hardware_windows.go) call them.

// parseNvidiaSmiVRAM parses the output of
//
//	nvidia-smi --query-gpu=memory.total --format=csv,noheader,nounits
//
// which is one line per GPU, each the VRAM size in MiB (e.g. "24576"). It
// returns the FIRST GPU's VRAM in GB (llama-server uses one primary GPU) and
// ok=false if nothing parses. nvidia-smi emits "N/A" on some driver/GPU combos,
// which we treat as not-parseable.
func parseNvidiaSmiVRAM(out string) (vramGB float64, ok bool) {
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, false
	}
	first := strings.Split(out, "\n")[0]
	mib, err := strconv.ParseFloat(strings.TrimSpace(first), 64)
	if err != nil || mib <= 0 {
		return 0, false
	}
	return mib / 1024, true
}

// parseRocmSMIVRAM parses the output of
//
//	rocm-smi --showmeminfo vram --json
//
// which looks like:
//
//	{"card0": {"VRAM Total Memory (B)": "17163091968"}, "card1": {...}}
//
// It returns the first card's total VRAM in GB, ok=false if nothing parses.
func parseRocmSMIVRAM(out string) (vramGB float64, ok bool) {
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, false
	}
	var cards map[string]map[string]string
	if err := json.Unmarshal([]byte(out), &cards); err != nil {
		return 0, false
	}
	// Deterministic order: sort card keys so "first" is stable across runs.
	keys := make([]string, 0, len(cards))
	for k := range cards {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		card := cards[k]
		for field, val := range card {
			f := strings.ToLower(field)
			if !(strings.Contains(f, "vram") && strings.Contains(f, "total")) {
				continue
			}
			b, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
			if err == nil && b > 0 {
				return b / 1024 / 1024 / 1024, true
			}
		}
	}
	return 0, false
}
