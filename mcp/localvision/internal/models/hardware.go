package models

// detectHardware is the platform-specific detection entry point.
// It is defined in:
//   - hardware_darwin.go  (//go:build darwin)
//   - hardware_linux.go   (//go:build linux)
//   - hardware_windows.go (//go:build windows)
//
// On any other platform (freebsd, etc.) hardware_fallback.go provides a
// stub that returns BackendUnsupported.
//
// The function is unexported; the public surface is DetectHardware in
// catalog.go.
//
// This file is intentionally just a comment holder — the function
// signature lives in the platform-specific files, and DetectHardware
// (in catalog.go) calls into it.
