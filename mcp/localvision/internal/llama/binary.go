package llama

import (
	"archive/tar"
	"compress/gzip"
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

	"github.com/froggeric/llm/mcp/localvision/internal/progress"
)

// binary.go discovers or downloads the llama-server binary.
//
// Discovery order:
//  1. $PATH: a user-installed `llama-server` (e.g. via `brew install
//     llama.cpp`) is the preferred default. We can't pin a SHA for a system
//     binary, so it is used as-is and a WARN is logged ("integrity NOT
//     verified"). This is the robust path — Homebrew keeps it current and
//     resolves its dylib dependencies correctly.
//  2. Cache: a previously-extracted bundle at <binDir>/llama-b<TAG>/, trusted
//     only when a sidecar records the verified archive SHA.
//  3. Download: a pinned official llama.cpp release tar.gz (HTTPS-only),
//     verified against the pinned ARCHIVE SHA256 BEFORE extraction, then
//     extracted preserving the bundle dir (dylib siblings), quarantine
//     stripped, and chmod +x. Never chmod before SHA passes. F3.3.
//
// The pinned archive SHA256 lives in this file as a source constant (defense
// in depth) so a TLS-strip-and-replace attack against a remote manifest fails.

// pinnedLLAMAArchiveSHA256 is the SHA256 of the llama.cpp release tar.gz ASSET
// (llama-b<TAG>-bin-macos-arm64.tar.gz), NOT of the inner llama-server binary.
// We pin the archive (not the extracted binary) so integrity is enforced BEFORE
// extraction. Pinned in source so a compromised CDN or TLS-strip MITM cannot
// substitute. Bump llamaServerDownloadTag and recompute together:
//
//	curl -fsSL <asset-url> | shasum -a 256
const pinnedLLAMAArchiveSHA256 = "021b947de63cbedcb39f7bed356be03fbef9aec5a3d77a716aa99df57454af59"

// llamaServerDownloadTag is the llama.cpp release tag we pin to (NOT "latest").
// It is a `var` (not const) so tests can swap it for a fixed value. Bump
// intentionally and record the bump date in CHANGELOG.
var llamaServerDownloadTag = "b9758"

// downloadURLTemplate formats the llama.cpp release download URL. The asset is
// a tar.gz dylib BUNDLE named llama-b<TAG>-bin-<os>-<arch>.tar.gz. HTTPS-only.
const downloadURLTemplate = "https://github.com/ggml-org/llama.cpp/releases/download/%[1]s/llama-%[1]s-bin-%[2]s-%[3]s.tar.gz"

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
// absolute path. See the file comment for the discovery order. The signature
// is fixed: lifecycle.go calls it opaquely with the pinned archive SHA.
func findOrDownloadBinary(ctx context.Context, pinnedSHA256, binDir string) (string, error) {
	// (1) $PATH — preferred. A user-installed llama-server (e.g. via
	// `brew install llama.cpp`) is the robust default. We can't pin a SHA for
	// a system binary, so it is used as-is and WARN-logged as unverified.
	if p, err := exec.LookPath("llama-server"); err == nil {
		slog.Warn("using llama-server from $PATH; integrity NOT verified (user-installed)",
			"path", p)
		return p, nil
	}

	// (2) Cache: a previously-extracted bundle, trusted only if a sidecar
	// records the verified archive SHA. Survives across manager lifetimes.
	bundleDir := filepath.Join(binDir, "llama-b"+strings.TrimPrefix(llamaServerDownloadTag, "b"))
	cachedBin := filepath.Join(bundleDir, "llama-server")
	sidecar := cachedBin + ".sha256"
	if info, err := os.Stat(cachedBin); err == nil && !info.IsDir() && isExecutable(cachedBin, info.Mode()) {
		if marked, merr := os.ReadFile(sidecar); merr == nil &&
			strings.TrimSpace(string(marked)) == strings.ToLower(pinnedSHA256) {
			return cachedBin, nil
		}
		slog.Warn("cached llama-server bundle has stale/missing SHA sidecar; re-downloading",
			"path", cachedBin)
	}

	// (3) Download the pinned tar.gz, verify ARCHIVE SHA, extract, strip quarantine.
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir bin dir %s: %w", binDir, err)
	}
	url := buildDownloadURL()
	return downloadVerifyAndExtract(ctx, url, bundleDir, cachedBin, sidecar, pinnedSHA256)
}

// downloadVerifyAndExtract fetches url (a tar.gz) to a .tmp file, verifies the
// ARCHIVE SHA256, extracts the bundle preserving its dir layout, chmods the
// inner llama-server +x, strips macOS quarantine, and writes a SHA sidecar.
// Never chmods before SHA passes (F3.3). pinnedSHA256 is the archive hash.
func downloadVerifyAndExtract(ctx context.Context, url, bundleDir, finalBinPath, sidecarPath, pinnedSHA256 string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		return "", errors.New("internal bug: download URL must be HTTPS")
	}

	tmpArchive := finalBinPath + ".tar.gz.tmp"
	_ = os.Remove(tmpArchive) // remove any stale tmp from a prior interrupted run

	// The tmp archive lives inside the bundle dir; ensure it exists.
	if err := os.MkdirAll(filepath.Dir(tmpArchive), 0o755); err != nil {
		return "", fmt.Errorf("mkdir bundle dir: %w", err)
	}

	f, err := os.OpenFile(tmpArchive, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create tmp archive %s: %w", tmpArchive, err)
	}
	// Close exactly once: the explicit close below inspects the error and marks
	// closed=true; the defer only closes on paths that don't reach the explicit
	// close (the early-return error branches), so we never double-close (which
	// would mask a real write/close error with a benign "already closed").
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("build download request: %w", err)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	// Stream into the temp archive in 64KB chunks, forwarding byte progress to
	// the ctx sink (throttled to ~1 MB / 1 s, mirroring models/downloader.go) so
	// callers can show % during the ~80 MB binary fetch. total is 0 when the
	// server sends no Content-Length (unknown).
	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	var written int64
	lastTick := time.Now()
	lastBytes := int64(0)
	reportBytes := func() {
		progress.Report(ctx, progress.Update{
			Phase:   "downloading",
			Detail:  "llama-server",
			Current: float64(written),
			Total:   float64(total),
			Unit:    "bytes",
			Message: "downloading llama-server",
		})
	}
	buf := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(tmpArchive)
			return "", err
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = os.Remove(tmpArchive)
				return "", fmt.Errorf("write tmp archive: %w", werr)
			}
			written += int64(n)
			if now := time.Now(); written-lastBytes >= 1024*1024 || now.Sub(lastTick) >= time.Second {
				lastBytes = written
				lastTick = now
				reportBytes()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = os.Remove(tmpArchive)
			return "", fmt.Errorf("read response body: %w", rerr)
		}
	}
	reportBytes() // final flush
	closed = true // we close explicitly below; keep the defer from double-closing
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("close tmp archive: %w", err)
	}

	// Verify the ARCHIVE SHA256 BEFORE extracting.
	want := strings.ToLower(pinnedSHA256)
	got, err := hashFile(ctx, tmpArchive)
	if err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("hash archive: %w", err)
	}
	if got != want {
		_ = os.Remove(tmpArchive)
		return "", &ErrIntegrityFailStruct{Path: tmpArchive, Expected: want, Actual: got}
	}

	// Extract the tar.gz into bundleDir's parent so llama-b<TAG>/ lands at bundleDir.
	if err := os.MkdirAll(filepath.Dir(bundleDir), 0o755); err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("mkdir bundle parent: %w", err)
	}
	if err := extractTarGz(tmpArchive, filepath.Dir(bundleDir)); err != nil {
		_ = os.Remove(tmpArchive)
		return "", fmt.Errorf("extract archive: %w", err)
	}
	_ = os.Remove(tmpArchive)

	// NOW chmod +x (only after SHA passed — F3.3).
	if err := os.Chmod(finalBinPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod extracted binary %s: %w", finalBinPath, err)
	}

	// Strip macOS quarantine recursively over the whole bundle.
	stripQuarantine(bundleDir)

	// Record the verified archive SHA so the cache path trusts this bundle later.
	if err := os.WriteFile(sidecarPath, []byte(want), 0o600); err != nil {
		slog.Warn("could not write llama-server SHA sidecar; cache will re-download", "err", err)
	}

	slog.Info("downloaded+extracted llama-server bundle",
		"path", finalBinPath, "tag", llamaServerDownloadTag)
	return finalBinPath, nil
}

// extractTarGz extracts a gzip'd tar into destDir. It refuses entries that
// escape destDir (path traversal) and refuses link entries (symlink/hardlink
// — supply-chain hygiene). Executable bits are preserved from the tar
// headers. The llama.cpp bundle has no links and no absolute paths, so
// refusal is always safe here.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(destDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(cleanDest, hdr.Name)
		// Path-traversal defense: the cleaned target must be destDir itself
		// or live beneath it.
		if !strings.HasPrefix(filepath.Clean(target)+string(filepath.Separator), cleanDest+string(filepath.Separator)) &&
			filepath.Clean(target) != cleanDest {
			return fmt.Errorf("archive entry %q escapes dest dir", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("archive contains a link entry %q — refusing", hdr.Name)
		}
	}
	return nil
}

// stripQuarantine clears the macOS com.apple.quarantine extended attribute
// recursively over the bundle dir, so the downloaded llama-server and its
// dylib siblings aren't killed by Gatekeeper on first launch. No-op off darwin.
func stripQuarantine(bundleDir string) {
	if runtime.GOOS != "darwin" {
		return
	}
	if _, err := exec.LookPath("xattr"); err != nil {
		slog.Warn("xattr not found; cannot strip macOS quarantine from llama-server bundle", "err", err)
		return
	}
	// -c clears, -r recurses (so dylib siblings are cleared too). Quarantine
	// may be absent (benign); not fatal.
	if out, err := exec.Command("xattr", "-cr", bundleDir).CombinedOutput(); err != nil {
		slog.Warn("xattr -cr failed (may be benign if quarantine absent)", "err", err, "out", string(out))
	}
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
