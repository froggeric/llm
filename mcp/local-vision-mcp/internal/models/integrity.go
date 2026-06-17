package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ErrIntegrityFail is returned when a file's SHA256 does not match the
// expected value. Callers must treat this as a hard error: a mismatched
// model file would produce silent garbage outputs downstream (llama.cpp
// has no in-process integrity check).
var ErrIntegrityFail = errors.New("model integrity check failed")

// hashResult is one entry in the integrity cache.
type hashResult struct {
	// path/size/mtime are the cache key (effectively; the map key is
	// path|size|mtime). Stored here for clarity and for LRU eviction.
	size  int64
	mtime int64 // unix nanos
	hash  string
}

// integrityCache memoizes SHA256 results so the loader doesn't re-hash
// a 5GB model file on every tools/call. The cache key is
// (path, size, mtime); any of those changing invalidates the entry.
//
// Cap is 16 entries (the realistic maximum: 4 models x 2 files + a few
// overlay-modified variants). Eviction is FIFO (oldest insertion), which
// is good enough for this workload; no need for true LRU complexity.
type integrityCache struct {
	mu     sync.RWMutex
	entries map[string]hashResult
	order  []string // FIFO order for eviction
	cap    int
}

const defaultIntegrityCacheCap = 16

var defaultCache = newIntegrityCache(defaultIntegrityCacheCap)

func newIntegrityCache(cap int) *integrityCache {
	if cap <= 0 {
		cap = defaultIntegrityCacheCap
	}
	return &integrityCache{
		entries: make(map[string]hashResult, cap),
		cap:     cap,
	}
}

// cacheKey is the composite key in the entries map.
func cacheKey(path string, size int64, mtime int64) string {
	return fmt.Sprintf("%s|%d|%d", path, size, mtime)
}

// lookup returns the cached hash for (path, size, mtime), or "" on miss.
func (c *integrityCache) lookup(path string, size int64, mtime int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.entries[cacheKey(path, size, mtime)]
	if !ok {
		return "", false
	}
	return r.hash, true
}

// store adds an entry, evicting the oldest if at capacity.
func (c *integrityCache) store(path string, size int64, mtime int64, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(path, size, mtime)
	if _, exists := c.entries[key]; exists {
		// Already cached (race between two goroutines); update in place.
		c.entries[key] = hashResult{size: size, mtime: mtime, hash: hash}
		return
	}
	if len(c.entries) >= c.cap && len(c.order) > 0 {
		// Evict oldest insertion.
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	c.entries[key] = hashResult{size: size, mtime: mtime, hash: hash}
	c.order = append(c.order, key)
}

// invalidate removes a path from the cache (e.g. after we delete a corrupt
// file). Best-effort.
func (c *integrityCache) invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// We don't know size/mtime, so remove any entries that start with the path.
	for i := 0; i < len(c.order); i++ {
		k := c.order[i]
		if _, ok := c.entries[k]; ok {
			// Compare prefix only.
			if startsWithPath(k, path) {
				delete(c.entries, k)
				c.order = append(c.order[:i], c.order[i+1:]...)
				i--
			}
		}
	}
}

func startsWithPath(key, path string) bool {
	if len(key) <= len(path) {
		return false
	}
	return key[:len(path)] == path && key[len(path)] == '|'
}

// statKey returns (size, mtime, ok) for path; ok=false if stat fails.
func statKey(path string) (int64, int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	return fi.Size(), fi.ModTime().UnixNano(), true
}

// computeSHA256 streams the file in 64KB chunks and returns the hex-encoded
// hash. Streaming avoids loading 5GB files into memory.
//
// ctx cancellation is honored: if ctx is cancelled mid-read, returns
// ctx.Err() and aborts.
func computeSHA256(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 64*1024) // 64KB; matches OS page cache granularity
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			// Hash.Write never returns an error per the crypto.Hash interface.
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifySHA256 returns nil if path's SHA256 matches expectedHex, or a wrapped
// ErrIntegrityFail with both actual and expected hashes if it does not.
// Results are memoized in the integrity cache keyed by (path, size, mtime)
// so repeated loads of the same file don't re-hash.
//
// expectedHex is lowercased and stripped of any whitespace before comparison.
// An empty expectedHex is treated as an immediate failure (it should have
// been caught by Validate, but defense in depth).
func verifySHA256(ctx context.Context, path, expectedHex string) error {
	// Normalize the expected hex.
	want := normalizeHex(expectedHex)
	if want == "" {
		return fmt.Errorf("%w: expected hash is empty (catalog entry missing or placeholder)", ErrIntegrityFail)
	}

	size, mtime, ok := statKey(path)
	if !ok {
		return fmt.Errorf("%w: cannot stat %s", ErrIntegrityFail, path)
	}

	if got, ok := defaultCache.lookup(path, size, mtime); ok {
		// Cache hit. Compare hexes.
		if got == want {
			return nil
		}
		return fmt.Errorf("%w: %s: expected %s, got %s", ErrIntegrityFail, path, want, got)
	}

	got, err := computeSHA256(ctx, path)
	if err != nil {
		return fmt.Errorf("%w: failed to hash %s: %v", ErrIntegrityFail, path, err)
	}

	// Cache the result regardless of match. A mismatch won't change without
	// a stat change, so caching it is fine.
	defaultCache.store(path, size, mtime, got)

	if got != want {
		return fmt.Errorf("%w: %s: expected %s, got %s", ErrIntegrityFail, path, want, got)
	}
	return nil
}

// VerifySHA256 is the exported wrapper around verifySHA256 so callers outside
// the package (e.g. the lifecycle manager in track C) can verify files.
func VerifySHA256(ctx context.Context, path, expectedHex string) error {
	return verifySHA256(ctx, path, expectedHex)
}

func normalizeHex(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			out = append(out, byte(r))
		case r >= 'a' && r <= 'f':
			out = append(out, byte(r))
		case r >= 'A' && r <= 'F':
			out = append(out, byte(r)+32) // to lower
		default:
			// skip whitespace, colons, etc.
		}
	}
	return string(out)
}

// ensure mtime has a known reference for test determinism. Not actually
// used in non-test code, but ensures time import isn't unused if we trim
// features later.
var _ = time.Now
