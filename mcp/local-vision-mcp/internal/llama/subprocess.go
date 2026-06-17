package llama

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/froggeric/llm/mcp/local-vision-mcp/internal/models"
)

// subprocess.go builds the argv for llama-server and spawns it via
// exec.CommandContext. It enforces the security invariants from F3.2:
//
//   - Never sh -c. Direct argv only.
//   - All file paths are filepath.Clean'd and verified to live inside the
//     configured models cache dir.
//   - The llama-server binary path is verified to live inside the
//     configured bin cache dir.
//   - The subprocess binds 127.0.0.1 only (--host 127.0.0.1).
//   - The port is sampled by the Go parent (F1.2): bind net.Listen on
//     127.0.0.1:0, extract the port, close, pass --port N. Never --port 0.

// errShForbidden is unused but documents the never-sh-cc rule. Defined as a
// named var so reviewers can grep "sh -c" and find the rationale.
var errShForbidden = errors.New("internal bug: sh -c is forbidden; use direct argv")

// pathInside reports whether cleaned is the same as or under root after
// filepath.Clean'ing both. Both arguments must be absolute; relative paths
// are rejected as a defense-in-depth measure.
func pathInside(child, root string) bool {
	if child == "" || root == "" {
		return false
	}
	if !filepath.IsAbs(child) || !filepath.IsAbs(root) {
		return false
	}
	c := filepath.Clean(child)
	r := filepath.Clean(root)
	if c == r {
		return true
	}
	// Add a separator so "/foo/barb" is not considered inside "/foo/bar".
	return strings.HasPrefix(c, r+string(filepath.Separator))
}

// validateModelPath checks that path is absolute, cleaned, and inside the
// models cache dir. Returns the cleaned path on success.
func validateModelPath(path, modelsDir string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path %q is not absolute", path)
	}
	if !pathInside(cleaned, modelsDir) {
		return "", fmt.Errorf("path %q is outside models cache dir %q", cleaned, modelsDir)
	}
	return cleaned, nil
}

// validateBinaryPath checks that the llama-server binary is absolute, cleaned,
// and inside the bin cache dir.
func validateBinaryPath(path, binDir string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("binary path %q is not absolute", path)
	}
	if !pathInside(cleaned, binDir) {
		return "", fmt.Errorf("binary path %q is outside bin cache dir %q", cleaned, binDir)
	}
	return cleaned, nil
}

// sampleFreePort claims a free TCP port on 127.0.0.1 by binding it in the
// parent, extracting the port number, then closing the listener and handing
// the port number to the subprocess via --port. This avoids races between
// sampling and use that plague --port 0. F1.2.
//
// There is an inherent TOCTOU race: another process can grab the port in the
// window between listener.Close() and the subprocess binding it. The spawner
// catches the resulting "address already in use" stderr and retries up to 3
// times (F4.7).
func sampleFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("sample free port: %w", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	port := addr.Port
	// Close immediately so the subprocess can bind. We accept the TOCTOU
	// race; F4.7 handles the retry.
	if err := l.Close(); err != nil {
		return 0, fmt.Errorf("close sampled port listener: %w", err)
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("sampled port out of range: %d", port)
	}
	return port, nil
}

// buildArgv constructs the argv vector for llama-server from a ModelSpec and
// the resolved local file paths. All paths must already be validated.
//
// The argv looks like:
//
//	-m <gguf> --mmproj <mmproj> --port <N> --host 127.0.0.1 -ngl <N> -c <N>
//
// -ngl -1 means "all layers on GPU" (Metal on Apple Silicon).
func buildArgv(spec models.ModelSpec, ggufPath, mmprojPath string, port int) []string {
	return []string{
		"-m", ggufPath,
		"--mmproj", mmprojPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1", // never 0.0.0.0; F3.2 security
		"-ngl", fmt.Sprintf("%d", spec.GpuLayers),
		"-c", fmt.Sprintf("%d", spec.Ctx),
	}
}

// spawnResult bundles the artifacts a successful spawn produces.
type spawnResult struct {
	cmd    *exec.Cmd
	port   int
	stderr *limitedBuffer
}

// spawnOptions are the inputs to spawnSubprocess. All paths must be absolute
// and cleaned by the caller.
type spawnOptions struct {
	// BinaryPath is the validated llama-server path.
	BinaryPath string
	// BinDir is the cache root for the binary, used for validation.
	BinDir string
	// GGUFPath is the validated main model file path.
	GGUFPath string
	// MmprojPath is the validated mmproj file path (may be empty for a
	// text-only model; the lifecycle never passes that case today).
	MmprojPath string
	// ModelsDir is the cache root for model files, used for validation.
	ModelsDir string
	// Spec is the catalog entry; only GpuLayers and Ctx are read.
	Spec models.ModelSpec
}

// spawnSubprocess validates all paths, samples a free port, builds argv, and
// starts the subprocess via exec.CommandContext. It does NOT wait for the
// subprocess to become healthy — see waitForHealth in health.go.
//
// F3.6: ctx is propagated to the subprocess via exec.CommandContext. When
// ctx is cancelled, the OS sends SIGKILL to the subprocess (Go's default
// behavior for CommandContext). For graceful SIGTERM, use Shutdown.
//
// F3.7: the caller is responsible for `go cmd.Wait()` immediately after this
// returns; the lifecycle watcher goroutine does that.
func spawnSubprocess(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
	binPath, err := validateBinaryPath(opts.BinaryPath, opts.BinDir)
	if err != nil {
		return nil, fmt.Errorf("binary path validation: %w", err)
	}
	gguf, err := validateModelPath(opts.GGUFPath, opts.ModelsDir)
	if err != nil {
		return nil, fmt.Errorf("gguf path validation: %w", err)
	}
	var mmproj string
	if opts.MmprojPath != "" {
		mmproj, err = validateModelPath(opts.MmprojPath, opts.ModelsDir)
		if err != nil {
			return nil, fmt.Errorf("mmproj path validation: %w", err)
		}
	}

	port, err := sampleFreePort()
	if err != nil {
		return nil, err
	}

	argv := buildArgv(opts.Spec, gguf, mmproj, port)

	// exec.CommandContext never invokes a shell; argv is passed directly.
	// F3.2: this is the only correct way to spawn an untrusted subprocess.
	cmd := exec.CommandContext(ctx, binPath, argv...)

	// limitedBuffer keeps the last 1KB so the watcher can surface a useful
	// tail on crash without unbounded memory growth. F1.4.
	stderr := newLimitedBuffer(1024)
	cmd.Stderr = stderr
	// Discard stdout; the server prints ASCII art and build info we don't
	// need (and that may contain user paths if the user sets --verbose). If
	// we ever need it for debugging, swap to a second limitedBuffer.
	cmd.Stdout = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start llama-server: %w", err)
	}

	return &spawnResult{
		cmd:    cmd,
		port:   port,
		stderr: stderr,
	}, nil
}

// addressInUsePatterns is the set of substrings we look for in stderr to
// detect a port-binding failure that justifies a retry. All entries are
// already lowercased; stderrIndicatesPortConflict lowercases its input
// before matching. llama-server prints one of these on Linux, macOS, and
// Windows (libuv / POSIX error string).
var addressInUsePatterns = []string{
	"address already in use",
	"address in use",
	"bind() failed",
	"eaddrinuse",
}

// stderrIndicatesPortConflict reports whether stderr output suggests the
// subprocess failed to bind its port. Used by the lifecycle's retry loop
// (F4.7).
func stderrIndicatesPortConflict(stderr string) bool {
	lower := strings.ToLower(stderr)
	for _, p := range addressInUsePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// Compile-time guard: ensure errShForbidden is used so reviewers grepping
// for "sh -c" land on its rationale. The variable is never returned; it
// exists for documentation.
var _ = errShForbidden
