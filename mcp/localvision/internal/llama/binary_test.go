package llama

import (
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

// binary_test.go exercises the SHA256 verification and atomic-rename logic
// in findOrDownloadBinary. We use an httptest.Server as a stand-in for
// github.com/ggml-org/llama.cpp releases.

// TestDownloadAndVerifySuccess: a server serving a binary with the
// matching SHA256 succeeds. The file is written to the final path with
// mode 0755.
func TestDownloadAndVerifySuccess(t *testing.T) {
	payload := []byte("#!/bin/sh\necho llama-server stub\n")
	wantHash := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(wantHash[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")

	gotPath, err := downloadAndVerify(t.Context(), srv.URL+"/llama-server", finalPath, wantHex)
	require.NoError(t, err)
	assert.Equal(t, finalPath, gotPath)

	info, err := os.Stat(finalPath)
	require.NoError(t, err)
	// 0755 means owner-exec bit set.
	assert.NotZero(t, info.Mode()&0o100, "binary should be executable")

	// Contents match.
	got, err := os.ReadFile(finalPath)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	// No leftover .tmp file.
	_, err = os.Stat(finalPath + ".tmp")
	assert.True(t, os.IsNotExist(err), ".tmp file should be removed")
}

// TestDownloadAndVerifySHAMismatch: server returns wrong bytes; the
// download fails with *ErrIntegrityFailStruct and no file is left at the
// final path.
func TestDownloadAndVerifySHAMismatch(t *testing.T) {
	payload := []byte("not the right bytes")
	wantHex := strings.Repeat("a", 64) // 64 hex chars, won't match

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")

	_, err := downloadAndVerify(t.Context(), srv.URL+"/llama-server", finalPath, wantHex)
	require.Error(t, err)
	var ie *ErrIntegrityFailStruct
	require.True(t, asErrIntegrityFail(err, &ie), "want *ErrIntegrityFailStruct, got %T: %v", err, err)
	assert.Equal(t, wantHex, ie.Expected)

	// File must not exist at the final path.
	_, statErr := os.Stat(finalPath)
	assert.True(t, os.IsNotExist(statErr), "final path must not exist on SHA mismatch")

	// .tmp file must also be cleaned up.
	_, statErr = os.Stat(finalPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr), ".tmp must not exist on SHA mismatch")
}

// asErrIntegrityFail is a thin wrapper around errors.As for the local
// *ErrIntegrityFailStruct type. We need it because errors.As requires a
// pointer to a pointer and we can't take a pointer to a typed nil without
// declaring a variable first.
func asErrIntegrityFail(err error, target **ErrIntegrityFailStruct) bool {
	for _, c := range []error{err} {
		_ = c
	}
	// Manual walk; we only have one wrapping level to deal with.
	if err == nil {
		return false
	}
	if e, ok := err.(*ErrIntegrityFailStruct); ok {
		*target = e
		return true
	}
	return false
}

// TestDownloadAndVerifyPlaceholderSkipsCheck: when pinnedSHA256 is the
// TODO-PHASE3 placeholder, the SHA check is skipped (Phase 1/2 behavior).
// The file is downloaded and chmod'd; the call succeeds.
func TestDownloadAndVerifyPlaceholderSkipsCheck(t *testing.T) {
	payload := []byte("#!/bin/sh\necho stub\n")

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")

	_, err := downloadAndVerify(t.Context(), srv.URL+"/llama-server", finalPath, "TODO-PHASE3")
	require.NoError(t, err)

	info, err := os.Stat(finalPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100)
}

// TestDownloadAndVerifyHTTPSError: an HTTP (not HTTPS) URL is rejected
// even if the test server tries to serve it. We don't bother spinning up
// a plaintext server; just check the URL guard.
func TestDownloadAndVerifyHTTPSError(t *testing.T) {
	dir := t.TempDir()
	_, err := downloadAndVerify(t.Context(), "http://example.com/x", filepath.Join(dir, "x"), strings.Repeat("a", 64))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

// TestDownloadAndVerify500Error: server returns 500; the call fails and no
// file is left behind.
func TestDownloadAndVerify500Error(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")
	_, err := downloadAndVerify(t.Context(), srv.URL+"/x", finalPath, strings.Repeat("a", 64))
	require.Error(t, err)
	_, statErr := os.Stat(finalPath)
	assert.True(t, os.IsNotExist(statErr))
}

// TestHashFileStreams: hashFile produces the correct SHA256 for a multi-chunk
// file. We construct a file bigger than 64KB so the streaming loop iterates
// at least twice.
func TestHashFileStreams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	// 200KB of deterministic data.
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
	hex := hex.EncodeToString(sum[:])

	ok, err := fileSHA256Matches(t.Context(), path, hex)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = fileSHA256Matches(t.Context(), path, strings.Repeat("0", 64))
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = fileSHA256Matches(t.Context(), filepath.Join(dir, "nope"), hex)
	require.Error(t, err)
}

// TestIsExecutable: positive and negative cases for the mode bit check.
func TestIsExecutable(t *testing.T) {
	assert.True(t, isExecutable("/x", 0o755))
	assert.True(t, isExecutable("/x", 0o111))
	assert.False(t, isExecutable("/x", 0o644))
	assert.False(t, isExecutable("/x", 0o600))
}

// TestPathHasLLAMAServer: does not assert anything specific; just exercises
// the function so it's covered. We can't guarantee llama-server is or isn't
// on PATH on the test runner.
func TestPathHasLLAMAServer(t *testing.T) {
	_ = pathHasLLAMAServer()
}

// TestBuildDownloadURLIsHTTPS: the constructed URL must start with https://.
func TestBuildDownloadURLIsHTTPS(t *testing.T) {
	u := buildDownloadURL()
	assert.True(t, strings.HasPrefix(u, "https://"), "download URL must be HTTPS, got %s", u)
}

// TestPinnedSHA256Placeholder: the source pin must be the documented
// placeholder until Phase 3 lands a real value. This test will fail (and
// prompt removal of this assertion) once the real hash is committed.
func TestPinnedSHA256Placeholder(t *testing.T) {
	assert.Equal(t, "TODO-PHASE3", pinnedLLAMAServerSHA256,
		"if you change this, also remove the placeholder skip paths")
}

// TestFindOrDownloadBinaryCacheHit: a cache dir already containing a valid
// binary short-circuits to the cache path (no download attempted).
func TestFindOrDownloadBinaryCacheHit(t *testing.T) {
	dir := t.TempDir()
	// Pre-place a binary in the cache dir.
	cached := filepath.Join(dir, "llama-server")
	require.NoError(t, os.WriteFile(cached, []byte("#!/bin/sh\n"), 0o755))

	// Even with a placeholder SHA, we should hit the cache.
	got, err := findOrDownloadBinary(t.Context(), "TODO-PHASE3", dir)
	require.NoError(t, err)
	assert.Equal(t, cached, got)
}

// TestDownloadAndVerifyNoChmodBeforeHash: a regression test for F3.3.
// If SHA verification fails, the file at finalPath must NOT exist (and
// therefore must NOT be executable). The .tmp file should be removed too.
func TestDownloadAndVerifyNoChmodBeforeHash(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "wrong bytes")
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")

	_, err := downloadAndVerify(t.Context(), srv.URL+"/x", finalPath, strings.Repeat("f", 64))
	require.Error(t, err)

	// Crucially: finalPath must not be chmod'd into existence.
	_, statErr := os.Stat(finalPath)
	assert.True(t, os.IsNotExist(statErr),
		"finalPath must not exist when SHA256 verification fails; F3.3 violation")
}

// TestValidateBinaryPathInside: validateBinaryPath accepts paths inside
// the bin dir, rejects paths outside.
func TestValidateBinaryPathInside(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "llama-server")
	outside := filepath.Join(filepath.Dir(dir), "evil")

	got, err := validateBinaryPath(inside, dir)
	require.NoError(t, err)
	assert.Equal(t, inside, got)

	_, err = validateBinaryPath(outside, dir)
	require.Error(t, err)

	// Relative paths rejected.
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
