package models

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
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ErrSchemaIncompatible is returned by Load when the catalog file declares
// a schema_version the loader cannot handle.
var ErrSchemaIncompatible = errors.New("catalog schema version incompatible")

// ErrHTTPSRequired is returned by Download when url is not https://.
// F3.2/F3.3.
var ErrHTTPSRequired = errors.New("only HTTPS URLs are supported")

// ErrURLNotInNamespace is returned when an https:// URL does not match the
// configured HuggingFace namespace regex. F3.3.
var ErrURLNotInNamespace = errors.New("URL is not in the allowed HuggingFace namespace")

// CurrentSchemaVersion is the catalog schema this build understands.
// Load rejects any catalog whose schema_version is greater than this with
// a clear "upgrade localvision" message.
const CurrentSchemaVersion = 1

// DefaultHFUser is the HuggingFace username all model URLs must live under.
// Overridable via Config.HFUser. F3.3 / F5.7.
const DefaultHFUser = "froggeric"

// progressInterval is the minimum gap between progress() callbacks. Either
// 1 MB of bytes transferred OR this interval, whichever comes first, will
// flush a progress update. Keeps the callback from being hammered.
const (
	progressInterval = 1 * time.Second
	progressByteStep = 1024 * 1024 // 1 MB
)

// downloadClient is the HTTP client used by Download. Indirected at the
// package level so tests can replace it with one wired to an httptest.Server.
//
// Default: no overall timeout; ctx controls cancellation.
var downloadClient = &http.Client{
	Timeout: 0,
}

// SetDownloadClient replaces the package HTTP client. Used by tests; not
// exported.
func SetDownloadClient(c *http.Client) { downloadClient = c }

// downloadImpl is the package-level implementation of Downloader.Download.
// Hoisted out of the method so tests can call it directly without
// constructing a Downloader value (the type is empty anyway).
//
// Algorithm:
//  1. Hard-fail if url is not https://. F3.2.
//  2. If destPath exists and its SHA256 matches expected: cache hit, return.
//  3. If destPath.partial exists, seed destPath.partial.tmp with its
//     contents, then send Range: bytes=<size>- to resume.
//  4. Stream the response to destPath.partial.tmp (appending if seeded),
//     invoking progress every 1 MB or 1s.
//  5. Verify SHA256 of destPath.partial.tmp; on mismatch delete it and
//     return ErrIntegrityFail.
//  6. Atomically rename destPath.partial.tmp -> destPath.
//  7. On ctx cancel: delete destPath.partial.tmp. Never leave a torn file.
//
// expectedSha256 is normalized (lowercased, whitespace stripped) before
// comparison. An empty expectedSha256 is treated as a programming error:
// callers must populate it from the catalog (which Validate enforces).
//
// Note on the three-file dance (destPath / .partial / .partial.tmp):
//   - destPath: the final, hash-verified file. Exists only post-success.
//   - .partial: a previously-stopped download's bytes (left intact across
//     runs so resume can pick up after a crash).
//   - .partial.tmp: the in-flight file. Deleted on any failure (including
//     ctx cancel) so we never leave a half-written file.
//
// This means a successful resume reads .partial, copies it to .partial.tmp,
// appends new bytes to .partial.tmp, verifies the full hash on .partial.tmp,
// and finally renames .partial.tmp -> destPath. The .partial file is left
// in place if we crashed mid-resume; once the rename to destPath succeeds
// it's removed implicitly by the rename target overwrite (we explicitly
// remove it).
func downloadImpl(ctx context.Context, url, destPath, expectedSha256 string, progress func(Progress)) error {
	want := normalizeHex(expectedSha256)
	if want == "" {
		return fmt.Errorf("%w: expected_sha256 is empty (caller bug)", ErrIntegrityFail)
	}
	if err := validateDownloadURL(url); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", destPath, err)
	}

	// Step 2: cache hit?
	if existingHash, ok := existingFileHash(ctx, destPath); ok {
		if existingHash == want {
			slog.Debug("download cache hit", "path", destPath, "sha256", existingHash)
			return nil
		}
		slog.Warn("existing file failed SHA256; re-downloading",
			"path", destPath, "expected", want, "got", existingHash)
		_ = os.Remove(destPath)
	}

	partialPath := destPath + ".partial"
	tmpPath := destPath + ".partial.tmp"

	// Step 3: resume?
	resumeFrom, err := existingPartialSize(partialPath)
	if err != nil {
		return err
	}

	// If we have a partial to resume from, seed tmpPath with its bytes so
	// the appended response produces a complete file. We do this by
	// copying rather than renaming: a crash mid-stream leaves .partial
	// intact for the next attempt.
	if resumeFrom > 0 {
		if err := copyFile(partialPath, tmpPath); err != nil {
			return fmt.Errorf("seed tmp from partial: %w", err)
		}
	} else {
		// Fresh download: ensure no stale tmp from a prior failed run.
		_ = os.Remove(tmpPath)
	}

	slog.Info("downloading",
		"url", url, "dest", destPath,
		"resume_from", resumeFrom, "expected_sha256", want,
	)

	// Step 4: stream to tmpPath (appending if resumeFrom > 0).
	totalWritten, totalSize, err := streamDownload(ctx, url, tmpPath, resumeFrom, progress)
	if err != nil {
		// Cancellation or HTTP error: clean up tmpPath. partialPath is
		// preserved so a future Download call can resume from it (the
		// streamed bytes are lost, but the pre-resume prefix is safe).
		_ = os.Remove(tmpPath)
		return err
	}

	// Step 5: verify SHA256 of the in-flight tmp file.
	got, err := computeSHA256(ctx, tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: hash %s: %v", ErrIntegrityFail, tmpPath, err)
	}
	if got != want {
		_ = os.Remove(tmpPath)
		// Also remove the .partial file: if our final hash doesn't match,
		// the .partial prefix is suspect too (it might be the source of
		// corruption). Force a clean re-download next time.
		_ = os.Remove(partialPath)
		return fmt.Errorf("%w: %s: expected %s, got %s", ErrIntegrityFail, tmpPath, want, got)
	}

	// Step 6: atomic rename to final destination. Remove the stale
	// .partial file first so the only artifact is destPath.
	_ = os.Remove(partialPath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename tmp to dest: %w", err)
	}

	slog.Info("download complete",
		"path", destPath,
		"bytes", totalWritten,
		"total_bytes", totalSize,
		"sha256", got,
	)
	return nil
}

// copyFile copies src to dst, overwriting dst if it exists. Used by the
// resume path to seed .partial.tmp from .partial.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// validateDownloadURL enforces HTTPS-only (F3.2) and the HF namespace
// regex (F3.3). The namespace is configurable via Config.HFUser but the
// default is "froggeric".
//
// Tests that need a custom namespace use ValidateHFURL (which takes an
// explicit namespace argument).
var hfUserNamespaceRegexp = regexp.MustCompile(
	`^https://huggingface\.co/` + regexp.QuoteMeta(DefaultHFUser) + `/`,
)

func validateDownloadURL(url string) error {
	if !strings.HasPrefix(url, "https://") {
		// http:// or any other scheme is a hard error. F3.2.
		return fmt.Errorf("%w: url=%q (must start with https://)", ErrHTTPSRequired, url)
	}
	if !hfUserNamespaceRegexp.MatchString(url) {
		return fmt.Errorf(
			"%w: url=%q (must match %s)",
			ErrURLNotInNamespace, url, hfUserNamespaceRegexp.String(),
		)
	}
	return nil
}

// ValidateHFURL is the exported helper for callers (e.g. the Validate
// function) to check whether a catalog URL is allowed. namespace may be
// empty to use the default.
func ValidateHFURL(url, namespace string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("%w: %q", ErrHTTPSRequired, url)
	}
	ns := namespace
	if ns == "" {
		ns = DefaultHFUser
	}
	re := regexp.MustCompile(`^https://huggingface\.co/` + regexp.QuoteMeta(ns) + `/`)
	if !re.MatchString(url) {
		return fmt.Errorf("%w: %q (allowed namespace: %s)", ErrURLNotInNamespace, url, ns)
	}
	return nil
}

// existingFileHash returns the hex SHA256 of path if it exists; returns
// ok=false if the file is missing or unreadable. Used for the cache-hit
// short-circuit.
func existingFileHash(ctx context.Context, path string) (string, bool) {
	got, err := computeSHA256(ctx, path)
	if err != nil {
		return "", false
	}
	return got, true
}

// existingPartialSize returns the size of partialPath if it exists; 0
// otherwise. Any error other than not-exist is propagated.
func existingPartialSize(partialPath string) (int64, error) {
	fi, err := os.Stat(partialPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat partial: %w", err)
	}
	return fi.Size(), nil
}

// streamDownload performs the HTTP GET (with optional Range header) and
// streams the response body to tmpPath, appending to any existing file
// (used for resume). Returns total bytes written (i.e. file size on disk
// at the end, equal to resumeFrom + new bytes), the Content-Length from
// the server (-1 if unknown), and any error.
//
// If resumeFrom > 0 we send "Range: bytes=<resumeFrom>-" and open tmpPath
// for append. If the server returns 200 instead of 206 (range not
// supported), we truncate and start from 0.
func streamDownload(ctx context.Context, url, tmpPath string, resumeFrom int64, progress func(Progress)) (int64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("new request: %w", err)
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	// Figure out whether the server honored the Range. If we asked for a
	// resume but got 200, restart from scratch.
	appending := false
	contentLength := resp.ContentLength
	switch resp.StatusCode {
	case http.StatusOK:
		// Full content. If we were trying to resume, we can't; truncate.
		resumeFrom = 0
	case http.StatusPartialContent:
		appending = true
		if contentLength > 0 {
			contentLength += resumeFrom
		}
	default:
		return 0, 0, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	// Open tmpPath for write (truncate) or append.
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if appending && resumeFrom > 0 {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(tmpPath, flag, 0o644)
	if err != nil {
		return 0, 0, fmt.Errorf("open tmp: %w", err)
	}
	defer f.Close()

	var written int64
	lastTick := time.Now()
	lastBytes := int64(0)

	flush := func() {
		if progress == nil {
			return
		}
		progress(Progress{
			Downloaded: resumeFrom + written,
			Total:      contentLength,
		})
	}

	buf := make([]byte, 64*1024) // 64 KB, same as integrity streaming
	for {
		select {
		case <-ctx.Done():
			return written, contentLength, ctx.Err()
		default:
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return written, contentLength, fmt.Errorf("write tmp: %w", werr)
			}
			written += int64(n)
			if progress != nil {
				now := time.Now()
				if written-lastBytes >= progressByteStep || now.Sub(lastTick) >= progressInterval {
					lastBytes = written
					lastTick = now
					flush()
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, contentLength, fmt.Errorf("read body: %w", readErr)
		}
	}
	flush()
	return written, contentLength, nil
}

// hashBytes returns the hex SHA256 of b. Used by tests and the
// cache-hit short-circuit for tiny files.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
