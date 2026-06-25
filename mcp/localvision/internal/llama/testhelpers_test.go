package llama

import (
	"runtime"
	"testing"
)

// skipOnWindows skips a test on Windows. Used for pre-existing tests that
// assert Unix-specific behavior the Windows port doesn't replicate: exec-bit
// preservation (Windows has no exec bit), #!/bin/sh shebang subprocesses,
// SIGTERM/SIGKILL (unsupported on Windows), and Unix-absolute path literals
// (filepath.Abs resolves "/opt/..." drive-relative on Windows). The production
// code these cover is cross-platform; the test assertions are not. Full Windows
// test-suite coverage for these paths is tracked as follow-up.
func skipOnWindows(t *testing.T, why string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skipf("Windows: %s", why)
	}
}

// ptrFloat64 returns a pointer to v, for building *float64 ChatRequest fields
// in tests (Temperature, TopP).
func ptrFloat64(v float64) *float64 { return &v }
