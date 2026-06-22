package llama

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binary_test.go exercises the archive-SHA verification, tar extraction, and
// discovery logic in findOrDownloadBinary. We use an httptest.Server as a
// stand-in for github.com/ggml-org/llama.cpp releases.

// setTestTag swaps llamaServerDownloadTag for the test and restores it on
// cleanup, so findOrDownloadBinary computes a predictable bundle dir name.
func setTestTag(t *testing.T, tag string) {
	t.Helper()
	prev := llamaServerDownloadTag
	llamaServerDownloadTag = tag
	t.Cleanup(func() { llamaServerDownloadTag = prev })
}

// sha256hex returns the lowercase hex SHA256 of data.
func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// makeFakeBundleArchive builds an in-memory tar.gz containing
// llama-bTEST/llama-server (an executable stub), matching the bundle layout
// downloadVerifyAndExtract extracts. Returns the archive bytes.
func makeFakeBundleArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	stub := []byte("#!/bin/sh\necho llama-server stub\n")
	entries := []struct {
		name string
		mode int64
		body []byte
	}{
		{"llama-bTEST/", 0o755, nil},
		{"llama-bTEST/llama-server", 0o755, stub},
	}
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: tar.TypeDir,
		}
		if e.body != nil {
			hdr.Typeflag = tar.TypeReg
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if e.body != nil {
			_, err := tw.Write(e.body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// TestDownloadVerifyAndExtractSuccess: a server serving a tar.gz bundle whose
// archive SHA matches. The inner llama-server is extracted to the bundle dir,
// chmod'd +x, and the sidecar records the verified SHA.
func TestDownloadVerifyAndExtractSuccess(t *testing.T) {
	skipOnWindows(t, "Unix exec-bit on extracted binary")
	archive := makeFakeBundleArchive(t)
	wantHex := sha256hex(archive)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "llama-bTEST")
	binPath := filepath.Join(bundleDir, "llama-server")
	sidecar := binPath + ".sha256"

	gotPath, err := downloadVerifyAndExtract(t.Context(), srv.URL+"/x", bundleDir, binPath, sidecar, wantHex)
	require.NoError(t, err)
	assert.Equal(t, binPath, gotPath)

	info, err := os.Stat(binPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "extracted binary should be executable")

	marked, err := os.ReadFile(sidecar)
	require.NoError(t, err)
	assert.Equal(t, wantHex, strings.TrimSpace(string(marked)))

	_, err = os.Stat(binPath + ".tar.gz.tmp")
	assert.True(t, os.IsNotExist(err), "tmp archive should be removed")
}

// TestDownloadVerifyAndExtractSHAMismatch: server returns an archive whose
// SHA does not match; the call fails with *ErrIntegrityFailStruct and no
// binary or tmp file is left behind.
func TestDownloadVerifyAndExtractSHAMismatch(t *testing.T) {
	archive := makeFakeBundleArchive(t)
	wantHex := strings.Repeat("a", 64) // will not match

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "llama-bTEST")
	binPath := filepath.Join(bundleDir, "llama-server")

	_, err := downloadVerifyAndExtract(t.Context(), srv.URL+"/x", bundleDir, binPath, binPath+".sha256", wantHex)
	require.Error(t, err)
	var ie *ErrIntegrityFailStruct
	require.True(t, asErrIntegrityFail(err, &ie), "want *ErrIntegrityFailStruct, got %T: %v", err, err)
	assert.Equal(t, wantHex, ie.Expected)

	_, err = os.Stat(binPath)
	assert.True(t, os.IsNotExist(err), "binary must not exist on SHA mismatch")
	_, err = os.Stat(binPath + ".tar.gz.tmp")
	assert.True(t, os.IsNotExist(err), "tmp archive must be removed on SHA mismatch")
}

// asErrIntegrityFail is a thin wrapper around a type assertion for the local
// *ErrIntegrityFailStruct type.
func asErrIntegrityFail(err error, target **ErrIntegrityFailStruct) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*ErrIntegrityFailStruct); ok {
		*target = e
		return true
	}
	return false
}

// TestDownloadVerifyAndExtractHTTPSError: an HTTP (not HTTPS) URL is rejected
// by the guard before any download.
func TestDownloadVerifyAndExtractHTTPSError(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "llama-server")
	_, err := downloadVerifyAndExtract(t.Context(), "http://example.com/x", dir, binPath, binPath+".sha256", strings.Repeat("a", 64))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

// TestDownloadVerifyAndExtract500Error: server returns 500; the call fails and
// no binary is left behind.
func TestDownloadVerifyAndExtract500Error(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	binPath := filepath.Join(dir, "llama-server")
	_, err := downloadVerifyAndExtract(t.Context(), srv.URL+"/x", dir, binPath, binPath+".sha256", strings.Repeat("a", 64))
	require.Error(t, err)
	_, err = os.Stat(binPath)
	assert.True(t, os.IsNotExist(err))
}

// TestHashFileStreams: hashFile produces the correct SHA256 for a multi-chunk
// file. We construct a file bigger than 64KB so the streaming loop iterates
// at least twice.
func TestHashFileStreams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	payload := make([]byte, 200*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	require.NoError(t, os.WriteFile(path, payload, 0o600))

	h := sha256.New()
	h.Write(payload)
	want := hex.EncodeToString(h.Sum(nil))

	got, err := hashFile(t.Context(), path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestFileSHA256Matches: matches returns true for matching hash, false for
// mismatching, and propagates file-not-found errors.
func TestFileSHA256Matches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	payload := []byte("hello")
	require.NoError(t, os.WriteFile(path, payload, 0o600))
	sum := sha256.Sum256(payload)
	hexStr := hex.EncodeToString(sum[:])

	ok, err := fileSHA256Matches(t.Context(), path, hexStr)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = fileSHA256Matches(t.Context(), path, strings.Repeat("0", 64))
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = fileSHA256Matches(t.Context(), filepath.Join(dir, "nope"), hexStr)
	require.Error(t, err)
}

// TestIsExecutable: positive and negative cases for the mode bit check.
func TestIsExecutable(t *testing.T) {
	assert.True(t, isExecutable("/x", 0o755))
	assert.True(t, isExecutable("/x", 0o111))
	assert.False(t, isExecutable("/x", 0o644))
	assert.False(t, isExecutable("/x", 0o600))
}

// TestPathHasLLAMAServer: exercises the function so it's covered. We can't
// guarantee llama-server is or isn't on PATH on the test runner.
func TestPathHasLLAMAServer(t *testing.T) {
	_ = pathHasLLAMAServer()
}

// TestBuildDownloadURLIsHTTPS: the constructed URL must start with https://.
func TestBuildDownloadURLIsHTTPS(t *testing.T) {
	u := buildDownloadURL()
	assert.True(t, strings.HasPrefix(u, "https://"), "download URL must be HTTPS, got %s", u)
}

// TestFindOrDownloadBinaryCacheHit: a cache dir already containing a verified
// bundle (sidecar matches) short-circuits to the cache path. PATH is isolated
// so the $PATH branch isn't taken on machines with llama-server installed.
func TestFindOrDownloadBinaryCacheHit(t *testing.T) {
	skipOnWindows(t, "Unix #!/bin/sh fake binary + exec bit")
	t.Setenv("PATH", t.TempDir())
	setTestTag(t, "bTEST")

	wantHex := strings.Repeat("a", 64)
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "llama-bTEST")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	cached := filepath.Join(bundleDir, "llama-server")
	require.NoError(t, os.WriteFile(cached, []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(cached+".sha256", []byte(wantHex), 0o600))

	got, err := findOrDownloadBinary(t.Context(), wantHex, dir)
	require.NoError(t, err)
	assert.Equal(t, cached, got)
}

// TestFindOrDownloadBinaryPATHPreferredOverCache: when a llama-server is on
// $PATH, it is used even if a verified cache bundle exists (locked decision:
// PATH is the robust, user-controlled default).
func TestFindOrDownloadBinaryPATHPreferredOverCache(t *testing.T) {
	skipOnWindows(t, "Unix PATH separator + #!/bin/sh")
	pathDir := t.TempDir()
	fakeBin := filepath.Join(pathDir, "llama-server")
	require.NoError(t, os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", pathDir)

	setTestTag(t, "bTEST")
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "llama-bTEST")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	cached := filepath.Join(bundleDir, "llama-server")
	require.NoError(t, os.WriteFile(cached, []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(cached+".sha256", []byte(strings.Repeat("a", 64)), 0o600))

	got, err := findOrDownloadBinary(t.Context(), strings.Repeat("a", 64), dir)
	require.NoError(t, err)
	assert.Equal(t, fakeBin, got, "PATH binary must be preferred over the cache")
}

// TestDownloadVerifyAndExtractNoChmodBeforeHash: a regression test for F3.3.
// If SHA verification fails, the extracted binary must NOT exist (and therefore
// must NOT be executable). The .tmp archive must be removed too.
func TestDownloadVerifyAndExtractNoChmodBeforeHash(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not a valid archive")
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "llama-bTEST")
	binPath := filepath.Join(bundleDir, "llama-server")

	_, err := downloadVerifyAndExtract(t.Context(), srv.URL+"/x", bundleDir, binPath, binPath+".sha256", strings.Repeat("f", 64))
	require.Error(t, err)

	_, statErr := os.Stat(binPath)
	assert.True(t, os.IsNotExist(statErr),
		"binary must not exist when SHA256 verification fails; F3.3 violation")
}

// TestExtractTarGzRejectsPathTraversal: an entry escaping destDir is refused.
func TestExtractTarGzRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "../evil.txt", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg,
	}))
	_, _ = tw.Write([]byte("x"))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archivePath := filepath.Join(t.TempDir(), "evil.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, buf.Bytes(), 0o600))

	dest := t.TempDir()
	err := extractTarGz(archivePath, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes dest dir")
}

// TestExtractTarGzPreservesExecBit: the executable bit from the tar header is
// preserved on extraction (a naive 0o644 write would strip it).
func TestExtractTarGzPreservesExecBit(t *testing.T) {
	skipOnWindows(t, "Unix exec-bit preservation")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "bin/runme", Mode: 0o755, Size: 4, Typeflag: tar.TypeReg,
	}))
	_, _ = tw.Write([]byte("#!sh"))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archivePath := filepath.Join(t.TempDir(), "a.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, buf.Bytes(), 0o600))

	dest := t.TempDir()
	require.NoError(t, extractTarGz(archivePath, dest))

	info, err := os.Stat(filepath.Join(dest, "bin", "runme"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "executable bit must be preserved")
}

// TestExtractTarGzRejectsSymlink: a symlink (or hardlink) entry is refused.
func TestExtractTarGzRejectsSymlink(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "link", Linkname: "/etc/passwd", Mode: 0o777, Typeflag: tar.TypeSymlink,
	}))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	archivePath := filepath.Join(t.TempDir(), "a.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, buf.Bytes(), 0o600))

	err := extractTarGz(archivePath, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link entry")
}

// TestValidateBinaryPathAbsolute: validateBinaryPath accepts any absolute
// path (since v0.2, a $PATH-discovered binary like /opt/homebrew/bin/llama-server
// lives outside ~/.localvision/bin and must be allowed) and rejects relative paths.
func TestValidateBinaryPathAbsolute(t *testing.T) {
	skipOnWindows(t, "Unix-absolute path literal")
	dir := t.TempDir()

	// Absolute path outside the cache dir — the $PATH-binary case.
	got, err := validateBinaryPath("/opt/homebrew/bin/llama-server", dir)
	require.NoError(t, err)
	assert.Equal(t, "/opt/homebrew/bin/llama-server", got)

	// A cache-dir path still works.
	inside := filepath.Join(dir, "llama-server")
	got, err = validateBinaryPath(inside, dir)
	require.NoError(t, err)
	assert.Equal(t, inside, got)

	// Paths are cleaned.
	got, err = validateBinaryPath("/opt//homebrew/./bin/llama-server", dir)
	require.NoError(t, err)
	assert.Equal(t, "/opt/homebrew/bin/llama-server", got)

	// Relative paths are rejected.
	_, err = validateBinaryPath("llama-server", dir)
	require.Error(t, err)
}

// TestValidateModelPathInside: same for model paths.
func TestValidateModelPathInside(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "model.gguf")
	got, err := validateModelPath(inside, dir)
	require.NoError(t, err)
	assert.Equal(t, inside, got)

	_, err = validateModelPath(filepath.Join(filepath.Dir(dir), "model.gguf"), dir)
	require.Error(t, err)
}

// TestStderrIndicatesPortConflict: the pattern matcher recognizes the
// "address already in use" string from llama-server.
func TestStderrIndicatesPortConflict(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"listen: bind: address already in use", true},
		{"error: address in use", true},
		{"EADDRINUSE", true},
		{"bind() failed", true},
		{"totally unrelated", false},
		{"", false},
	}
	for i, c := range cases {
		name := fmt.Sprintf("case-%d", i)
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, stderrIndicatesPortConflict(c.s))
		})
	}
}
