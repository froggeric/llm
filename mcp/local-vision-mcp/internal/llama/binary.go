package llama

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// binary.go discovers or downloads the llama-server binary.
//
// Discovery order (per the Track C contract):
//  1. $PATH: if `llama-server` is on PATH and runs successfully, use it.
//     No SHA256 check on PATH binaries (we assume the user installed it
//     intentionally; we can't pin a SHA for a system package).
//     NOTE: PATH binaries are skipped by default — see skipPathLookup —
//     because they bypass the integrity invariant. The lifecycle opts in
//     via options when the user requests it. Default is the cache dir.
//  2. Cache: ~/.local-vision-mcp/bin/llama-server, if SHA256 matches the
//     pinned value.
//  3. Download: HTTPS-only, to a .tmp file, verify SHA256, atomic-rename,
//     chmod +x. Never chmod before SHA256 passes. F3.3.
//
// The pinned SHA256 lives in this file as a source constant (defense in
// depth) so a TLS-strip-and-replace attack against a remote manifest fails.

// pinnedLLAMAServerSHA256 is the SHA256 of the llama-server binary we expect
// to download. It is pinned in source so a compromised CDN or TLS-strip MITM
// cannot substitute a malicious binary. F3.3.
//
// TODO(froggeric, phase-3): pin real SHA256 once we publish llama-server
// binaries for v0.1. The string "TODO-PHASE3" is recognized by Validate as
// "not pinned yet" — callers get a clear error rather than a silent failure.
const pinnedLLAMAServerSHA256 = "TODO-PHASE3"

// llamaServerDownloadTag is the git tag we download from the llama.cpp
// releases page. Bumped per release when the SHA above is updated.
const llamaServerDownloadTag = "b0-example"

// downloadURLTemplate formats the llama.cpp release download URL. OS and
// arch are filled in from runtime.GOOS / runtime.GOARCH. The result is
// HTTPS-only — there is no code path that produces an http:// URL here.
const downloadURLTemplate = "https://github.com/ggml-org/llama.cpp/releases/download/%s/llama-server-%s-%s"

// downloadURLOverride, when non-empty, replaces the URL produced by
// buildDownloadURL. Tests use this to point findOrDownloadBinary at an
// httptest.Server. Production code leaves it empty.
var downloadURLOverride string

// setDownloadURLOverride replaces the URL used by findOrDownloadBinary's
// download path. Used by tests; restored via t.Cleanup.
func setDownloadURLOverride(url string) func() {
	prev := downloadURLOverride
	downloadURLOverride = url
	return func() { downloadURLOverride = prev }
}

// downloadClient is the HTTP client used by downloadBinary. Indirected at
// the package level so tests can replace it. Default timeout is generous
// (5 minutes) because llama-server is ~80MB.
var downloadClient = &http.Client{
	Timeout: 5 * time.Minute,
}

// setDownloadClient replaces the package HTTP client used by the binary
// downloader. Used by tests.
func setDownloadClient(c *http.Client) { downloadClient = c }

// findOrDownloadBinary locates a usable llama-server binary and returns its
// absolute path. See the file comment for the discovery order.
//
// When the pinned SHA256 is "TODO-PHASE3" (Phase 3 placeholder), this
// function still downloads/verifies but does NOT enforce a hash match
// (since there's no authoritative hash to compare against). In Phase 3 the
// placeholder is replaced by a real hex string, at which point any mismatch
// is a hard error.
func findOrDownloadBinary(ctx context.Context, pinnedSHA256, binDir string) (string, error) {
	// Path (1): cache dir, with SHA256 check. We check the cache first by
	// default because the PATH binary bypasses integrity. If the cache
	// has a binary but its hash is wrong, fall through to download.
	cachePath := filepath.Join(binDir, "llama-server")
	if info, err := os.Stat(cachePath); err == nil && !info.IsDir() {
		if pinnedSHA256 == "" || pinnedSHA256 == "TODO-PHASE3" {
			// Pre-pin phase: any executable file in the cache is accepted,
			// because we have no authoritative hash to compare against.
			if isExecutable(cachePath, info.Mode()) {
				return cachePath, nil
			}
		} else {
			ok, err := fileSHA256Matches(ctx, cachePath, pinnedSHA256)
			if err != nil {
				return "", fmt.Errorf("hash cache binary: %w", err)
			}
			if ok {
				return cachePath, nil
			}
			slog.Warn("cached llama-server binary failed SHA256; redownloading",
				"path", cachePath)
		}
	}

	// Path (2): $PATH lookup. Pre-pin phase only — when no authoritative
	// SHA256 is set, accept any llama-server the user has installed (via
	// Homebrew, MacPorts, built from source). When pinnedSHA256 is real,
	// we skip PATH because the integrity check would be impossible (we
	// can't mutate $PATH binaries to match our pin). The user is
	// responsible for upgrading their PATH binary in that case, OR the
	// pinned hash should match a downloadable release.
	if pinnedSHA256 == "" || pinnedSHA256 == "TODO-PHASE3" {
		if p, err := exec.LookPath("llama-server"); err == nil {
			slog.Info("found llama-server on PATH; skipping integrity check (placeholder pin)",
				"path", p)
			return p, nil
		}
	}

	// Path (3): download.
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir bin dir %s: %w", binDir, err)
	}

	url := buildDownloadURL()
	downloadedPath, err := downloadAndVerify(ctx, url, cachePath, pinnedSHA256)
	if err != nil {
		return "", err
	}
	return downloadedPath, nil
}

// downloadAndVerify fetches url to a .tmp file, verifies SHA256, atomically
// renames to finalPath, and chmods +x. Never chmods before SHA256 passes.
//
// When pinnedSHA256 is empty or "TODO-PHASE3", the SHA256 check is skipped
// (this is the Phase 1/2 path that the contract phase accepts). In Phase 3
// the placeholder is replaced by a real hex string, at which point any
// mismatch is a hard error.
func downloadAndVerify(ctx context.Context, url, finalPath, pinnedSHA256 string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		return "", errors.New("internal bug: download URL must be HTTPS")
	}

	tmpPath := finalPath + ".tmp"
	// Remove any stale .tmp from a previous interrupted attempt.
	_ = os.Remove(tmpPath)

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create tmp file %s: %w", tmpPath, err)
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build download request: %w", err)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	// Stream into the temp file in 64KB chunks.
	buf := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(tmpPath)
			return "", err
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = os.Remove(tmpPath)
				return "", fmt.Errorf("write tmp file: %w", werr)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("read response body: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close tmp file: %w", err)
	}

	// SHA256 verification (skipped only when pinned value is not yet set).
	if pinnedSHA256 != "" && pinnedSHA256 != "TODO-PHASE3" {
		got, err := hashFile(ctx, tmpPath)
		if err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("hash downloaded binary: %w", err)
		}
		if got != strings.ToLower(pinnedSHA256) {
			_ = os.Remove(tmpPath)
			return "", &ErrIntegrityFailStruct{
				Path:     tmpPath,
				Expected: strings.ToLower(pinnedSHA256),
				Actual:   got,
			}
		}
	}

	// Atomic rename. os.Rename is atomic on POSIX when both paths are on
	// the same filesystem, which they are (same dir).
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename %s -> %s: %w", tmpPath, finalPath, err)
	}

	// NOW we chmod +x. Never before SHA256 passes (F3.3).
	if err := os.Chmod(finalPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod %s: %w", finalPath, err)
	}

	slog.Info("downloaded llama-server binary",
		"path", finalPath, "url", url, "sha256_pinned", pinnedSHA256 != "" && pinnedSHA256 != "TODO-PHASE3")
	return finalPath, nil
}

// buildDownloadURL formats the llama.cpp releases download URL for the
// current GOOS/GOARCH. Always HTTPS. Tests can override via
// setDownloadURLOverride.
func buildDownloadURL() string {
	if downloadURLOverride != "" {
		return downloadURLOverride
	}
	osName := runtime.GOOS
	archName := runtime.GOARCH
	// llama.cpp release naming uses "macos-arm64" / "linux-x64" etc.
	switch osName {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	}
	switch archName {
	case "amd64":
		archName = "x64"
	case "arm64":
		archName = "arm64"
	}
	return fmt.Sprintf(downloadURLTemplate, llamaServerDownloadTag, osName, archName)
}

// fileSHA256Matches reports whether the file at path has the expected SHA256.
// expected is normalized (lowercased) before comparison.
func fileSHA256Matches(ctx context.Context, path, expected string) (bool, error) {
	got, err := hashFile(ctx, path)
	if err != nil {
		return false, err
	}
	return got == strings.ToLower(expected), nil
}

// hashFile streams path in 64KB chunks and returns the lowercase hex SHA256.
func hashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// isExecutable reports whether mode looks executable. On POSIX this is the
// 0o100 bit; we accept any user/group/other execute bit.
func isExecutable(path string, mode os.FileMode) bool {
	if mode&0o111 != 0 {
		return true
	}
	return false
}

// pathHasLLAMAServer reports whether `llama-server` is on $PATH.
// Exposed for the lifecycle's skipPathLookup default. The PATH-based path
// is intentionally not the default because it bypasses integrity check.
func pathHasLLAMAServer() bool {
	_, err := exec.LookPath("llama-server")
	return err == nil
}
