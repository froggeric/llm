//go:build linux

package models

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// readProcMemTotalGB reads /proc/meminfo and returns MemTotal in GB as a
// float. Returns 0 on any error (we don't fail hardware detection just
// because meminfo was unreadable on a weird container setup).
//
// /proc/meminfo line format:
//
//	MemTotal:       16384000 kB
func readProcMemTotalGB() float64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		// Expect: ["MemTotal:", "16384000", "kB"]
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return float64(kb) / 1024 / 1024
	}
	return 0
}
