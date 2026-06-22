package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startTestServerHTTPS returns an httptest.Server with TLS enabled, so the
// URL is https://. Required because the downloader hard-rejects http://.
//
// The HTTPS URL won't match the huggingface.co/froggeric/ regex; tests use
// withPermissiveHFRegex to allow localhost temporarily.
//
// The server supports Range requests so resume tests work.
func startTestServerHTTPS(t *testing.T, payload []byte) (*httptest.Server, *int32) {
	t.Helper()
	var mu sync.Mutex
	var count int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			start, err := parseRangeStart(rangeHdr)
			if err != nil || start >= int64(len(payload)) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			startInt := int(start)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)-startInt))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[startInt:])
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// startSlowTestServerHTTPS serves payload at a configurable delay per
// byte so we can test cancellation mid-stream.
func startSlowTestServerHTTPS(t *testing.T, payload []byte, perByteDelay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for i := 0; i < len(payload); i++ {
			_, _ = w.Write(payload[i : i+1])
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(perByteDelay)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func parseRangeStart(h string) (int64, error) {
	const prefix = "bytes="
	if !strings.HasPrefix(h, prefix) {
		return 0, fmt.Errorf("bad range header")
	}
	rest := strings.TrimPrefix(h, prefix)
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad range header")
	}
	var n int64
	if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil {
		return 0, err
	}
	return n, nil
}

// withTestTLSClient swaps downloadClient to one that trusts the
// httptest TLS cert. Restored on cleanup.
func withTestTLSClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	prev := downloadClient
	t.Cleanup(func() { downloadClient = prev })

	c := srv.Client()
	c.Timeout = 0
	downloadClient = c
}

// withPermissiveHFRegex replaces the namespace regex with one that matches
// any https:// URL, so tests can point at httptest. Restored on cleanup.
func withPermissiveHFRegex(t *testing.T) {
	t.Helper()
	prev := hfUserNamespaceRegexp
	t.Cleanup(func() { hfUserNamespaceRegexp = prev })
	hfUserNamespaceRegexp = regexp.MustCompile(`^https://`)
}

func TestDownload_CacheHitSkipsRedownload(t *testing.T) {
	payload := []byte("this is the model file content")
	want := hashHex(payload)

	srv, reqCount := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	require.NoError(t, os.WriteFile(dest, payload, 0o644))

	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, nil)
	require.NoError(t, err)

	if *reqCount != 0 {
		t.Errorf("server was hit %d times; want 0 (cache hit)", *reqCount)
	}
}

func TestDownload_FullDownloadFromScratch(t *testing.T) {
	payload := bytesRepeat("data-", 1024) // ~5KB
	want := hashHex(payload)

	srv, _ := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, nil)
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

// TestDownload_FreeSpacePrecheckRefuses: a download is refused when the
// destination volume reports less free space than the payload (simulated by
// overriding the freeDiskBytes seam). Guards against filling the system drive.
func TestDownload_FreeSpacePrecheckRefuses(t *testing.T) {
	payload := bytesRepeat("data-", 1024) // ~5KB
	want := hashHex(payload)

	srv, _ := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	prev := freeDiskBytes
	t.Cleanup(func() { freeDiskBytes = prev })
	freeDiskBytes = func(string) (int64, error) { return 100, nil } // < ~5KB payload

	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient free space")

	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "nothing should be written on refusal")
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
		{17 * 1024 * 1024 * 1024, "17.0 GiB"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, humanBytes(c.in), "input %d", c.in)
	}
}

func TestDownload_HTTPSchemeRejected(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "x.gguf")
	err := downloadImpl(context.Background(),
		"http://huggingface.co/froggeric/x/y.gguf", dest, strings.Repeat("0", 64), nil)
	require.Error(t, err)
	if !errors.Is(err, ErrHTTPSRequired) {
		t.Errorf("err = %v; want ErrHTTPSRequired", err)
	}
}

func TestDownload_WrongNamespaceRejected(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "x.gguf")
	// Don't call withPermissiveHFRegex; the default froggeric-only regex
	// is in effect.
	err := downloadImpl(context.Background(),
		"https://huggingface.co/attacker/x/y.gguf", dest, strings.Repeat("0", 64), nil)
	require.Error(t, err)
	if !errors.Is(err, ErrURLNotInNamespace) {
		t.Errorf("err = %v; want ErrURLNotInNamespace", err)
	}
}

func TestDownload_ResumeFromPartial(t *testing.T) {
	payload := bytesRepeat("DATA", 1280) // 5120 bytes
	want := hashHex(payload)

	srv, reqCount := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	partial := dest + ".partial"
	require.NoError(t, os.WriteFile(partial, payload[:1024], 0o644))

	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, nil)
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	if *reqCount != 1 {
		t.Errorf("server hit %d times; want 1 (single resumed GET)", *reqCount)
	}
}

func TestDownload_SHA256MismatchDeletesFile(t *testing.T) {
	payload := []byte("content that does NOT match the declared hash")
	srv, _ := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	declared := strings.Repeat("a", 64)
	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, declared, nil)
	require.Error(t, err)
	if !errors.Is(err, ErrIntegrityFail) {
		t.Errorf("err = %v; want ErrIntegrityFail", err)
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest still exists after mismatch: %v", err)
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Errorf(".partial still exists after mismatch: %v", err)
	}
}

func TestDownload_ContextCancelCleansUp(t *testing.T) {
	payload := bytesRepeat("X", 64)

	srv := startSlowTestServerHTTPS(t, payload, 20*time.Millisecond)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	err := downloadImpl(ctx, srv.URL+"/x.gguf", dest, hashHex(payload), nil)
	require.Error(t, err)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v; want DeadlineExceeded", err)
	}

	if _, err := os.Stat(dest + ".partial.tmp"); !os.IsNotExist(err) {
		t.Errorf(".partial.tmp still exists after cancel: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest exists after cancel: %v", err)
	}
}

func TestDownload_ProgressCallbackInvoked(t *testing.T) {
	payload := bytesRepeat("P", 4*1024) // 4 KB; several chunks
	want := hashHex(payload)

	srv, _ := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")

	var mu sync.Mutex
	var lastDownloaded int64
	var callbackCount int
	progress := func(p Progress) {
		mu.Lock()
		defer mu.Unlock()
		lastDownloaded = p.Downloaded
		callbackCount++
	}

	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, progress)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	if callbackCount == 0 {
		t.Error("progress callback was never invoked")
	}
	if lastDownloaded != int64(len(payload)) {
		t.Errorf("final progress = %d; want %d", lastDownloaded, len(payload))
	}
}

func TestDownload_ExistingFileWithWrongHashForcesRedownload(t *testing.T) {
	payload := []byte("the right content")
	want := hashHex(payload)

	srv, reqCount := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	dest := filepath.Join(t.TempDir(), "model.gguf")
	require.NoError(t, os.WriteFile(dest, []byte("the WRONG content"), 0o644))

	err := downloadImpl(context.Background(), srv.URL+"/x.gguf", dest, want, nil)
	require.NoError(t, err)
	if *reqCount == 0 {
		t.Error("server was not hit; bad-hash dest should force redownload")
	}

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

// helpers

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func bytesRepeat(s string, n int) []byte {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return out
}

// TestDownloader_Method exercises the (Downloader).Download method (not
// just downloadImpl) so the method-level coverage is non-zero.
func TestDownloader_Method(t *testing.T) {
	payload := []byte("via method")
	want := hashHex(payload)

	srv, _ := startTestServerHTTPS(t, payload)
	withTestTLSClient(t, srv)
	withPermissiveHFRegex(t)

	var d Downloader
	dest := filepath.Join(t.TempDir(), "model.gguf")
	require.NoError(t, d.Download(context.Background(), srv.URL+"/x.gguf", dest, want, nil))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}
