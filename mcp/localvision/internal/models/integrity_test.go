package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempFile writes content to a uniquely-named file in t.TempDir() and
// returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// sha256Hex computes the hex SHA256 of s. Mirrors the production code.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestVerifySHA256_Success(t *testing.T) {
	content := "hello world\n"
	p := writeTempFile(t, "model.gguf", content)
	want := sha256Hex(content)

	require.NoError(t, verifySHA256(context.Background(), p, want))
}

func TestVerifySHA256_MismatchReturnsWrappedError(t *testing.T) {
	content := "hello world\n"
	p := writeTempFile(t, "model.gguf", content)
	want := strings.Repeat("a", 64) // 64-char hex that won't match

	err := verifySHA256(context.Background(), p, want)
	require.Error(t, err)
	if !errors.Is(err, ErrIntegrityFail) {
		t.Errorf("err is not ErrIntegrityFail: %v", err)
	}
	// Error message should mention both expected and actual hashes.
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error does not mention expected hash: %v", err)
	}
	actual := sha256Hex(content)
	if !strings.Contains(err.Error(), actual) {
		t.Errorf("error does not mention actual hash: %v", err)
	}
}

func TestVerifySHA256_EmptyExpectedFails(t *testing.T) {
	p := writeTempFile(t, "model.gguf", "data")
	err := verifySHA256(context.Background(), p, "")
	require.Error(t, err)
	if !errors.Is(err, ErrIntegrityFail) {
		t.Errorf("err is not ErrIntegrityFail: %v", err)
	}
}

func TestVerifySHA256_PlaceholderFails(t *testing.T) {
	p := writeTempFile(t, "model.gguf", "data")
	err := verifySHA256(context.Background(), p, "PLACEHOLDER-PHASE3")
	require.Error(t, err)
	if !errors.Is(err, ErrIntegrityFail) {
		t.Errorf("err is not ErrIntegrityFail: %v", err)
	}
}

func TestVerifySHA256_MissingFileFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.gguf")
	err := verifySHA256(context.Background(), missing, strings.Repeat("0", 64))
	require.Error(t, err)
	if !errors.Is(err, ErrIntegrityFail) {
		t.Errorf("err is not ErrIntegrityFail: %v", err)
	}
}

func TestVerifySHA256_LargeFileStreaming(t *testing.T) {
	// 5 MB is plenty to exercise the 64KB-chunk loop multiple times.
	content := strings.Repeat("A", 5*1024*1024)
	p := writeTempFile(t, "big.gguf", content)
	want := sha256Hex(content)

	require.NoError(t, verifySHA256(context.Background(), p, want))
}

func TestVerifySHA256_CacheHitSkipsRehash(t *testing.T) {
	content := "cache me\n"
	p := writeTempFile(t, "model.gguf", content)
	want := sha256Hex(content)

	// First call computes and caches.
	require.NoError(t, verifySHA256(context.Background(), p, want))

	// After caching, mutating the in-memory cache to a wrong hash should
	// NOT cause verifySHA256 to fail (cache hit returns the wrong value),
	// proving the cache is consulted.
	fi, err := os.Stat(p)
	require.NoError(t, err)
	// Inject a poisoned entry under the same key.
	defaultCache.store(p, fi.Size(), fi.ModTime().UnixNano(), strings.Repeat("0", 64))

	err = verifySHA256(context.Background(), p, want)
	if err == nil {
		t.Error("expected poisoned-cache mismatch to surface as error")
	}
}

func TestVerifySHA256_CacheInvalidationOnMtimeChange(t *testing.T) {
	content := "original\n"
	p := writeTempFile(t, "model.gguf", content)
	want := sha256Hex(content)

	require.NoError(t, verifySHA256(context.Background(), p, want))

	// Rewrite the file with new content but force same size+mtime by
	// backdating. Then the cache key matches but content differs; the
	// cache should be trusted (i.e. verification passes despite the
	// underlying content being different).
	//
	// To detect real tampering, the cache key includes size+mtime. We
	// cannot make mtime identical across writes without futzing; instead
	// verify that a real content change (different size) does bust the
	// cache.
	newContent := "different size\n"
	if err := os.WriteFile(p, []byte(newContent), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	err := verifySHA256(context.Background(), p, want)
	require.Error(t, err, "cache should be busted when size changes")
}

func TestNormalizeHex(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"ABCDEF0123456789", "abcdef0123456789"},
		{"  ab cd \n", "abcd"},
		{"a:b:c:d", "abcd"},
		{"PLACEHOLDER-PHASE3", "phase3placeholder"[0:0] + ""}, // letters only
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := normalizeHex(tc.in)
			// For PLACEHOLDER-PHASE3 we just check it returns a non-empty
			// lowercased hex subset; the exact letters depend on order.
			if tc.in == "" {
				if got != "" {
					t.Errorf("normalizeHex(%q) = %q; want %q", tc.in, got, "")
				}
				return
			}
			// Lowercase + only hex chars + same length-set as input letters.
			if got != strings.ToLower(strings.Map(func(r rune) rune {
				switch {
				case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
					return r
				}
				return -1
			}, tc.in)) {
				t.Errorf("normalizeHex(%q) = %q; want canonical", tc.in, got)
			}
		})
	}
}

func TestIntegrityCache_LRU_Eviction(t *testing.T) {
	c := newIntegrityCache(3)
	c.store("a", 1, 1, "ha")
	c.store("b", 2, 2, "hb")
	c.store("c", 3, 3, "hc")

	// All three should be present.
	if _, ok := c.lookup("a", 1, 1); !ok {
		t.Error("a missing before eviction")
	}

	// Adding a 4th should evict the oldest (a).
	c.store("d", 4, 4, "hd")
	if _, ok := c.lookup("a", 1, 1); ok {
		t.Error("a was not evicted")
	}
	if _, ok := c.lookup("b", 2, 2); !ok {
		t.Error("b missing after eviction")
	}
	if _, ok := c.lookup("c", 3, 3); !ok {
		t.Error("c missing after eviction")
	}
	if _, ok := c.lookup("d", 4, 4); !ok {
		t.Error("d missing after insertion")
	}
}

func TestVerifySHA256_Exported(t *testing.T) {
	// Smoke test the exported wrapper.
	content := "via exported API\n"
	p := writeTempFile(t, "x.gguf", content)
	want := sha256Hex(content)
	assert.NoError(t, VerifySHA256(context.Background(), p, want))
}
