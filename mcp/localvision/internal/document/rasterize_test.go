package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withChain swaps rasterizersVar for the test and restores it on cleanup.
func withChain(t *testing.T, chain []rasterizer) {
	t.Helper()
	prev := rasterizersVar
	rasterizersVar = chain
	t.Cleanup(func() { rasterizersVar = prev })
}

// fakeRasterizer returns a rasterizer whose run writes n dummy PNGs into outDir
// named page-<i>.png (i=1..n). avail controls availability.
func fakeRasterizer(name string, n int, avail bool, fail bool) rasterizer {
	return rasterizer{
		name:  name,
		avail: func() bool { return avail },
		run: func(_ context.Context, _ string, outDir string, _ int) ([]string, error) {
			if fail {
				return nil, errBoom
			}
			var paths []string
			for i := 1; i <= n; i++ {
				p := filepath.Join(outDir, "page-"+itoa(i)+".png")
				if err := os.WriteFile(p, []byte("PNG"), 0o644); err != nil {
					return nil, err
				}
				paths = append(paths, p)
			}
			return paths, nil
		},
	}
}

var errBoom = boomErr{}

type boomErr struct{}

func (boomErr) Error() string { return "boom" }

// tiny helper to avoid importing strconv in the fake (keeps the fake obvious).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func TestIsPDF(t *testing.T) {
	dir := t.TempDir()

	pdf := filepath.Join(dir, "doc.pdf")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.5\n%binary..."), 0o644))
	assert.True(t, IsPDF(pdf), "a file starting with %PDF- is a PDF")

	bin := filepath.Join(dir, "image.bin") // mimics a data:-URI PDF written to .bin
	require.NoError(t, os.WriteFile(bin, []byte("%PDF-1.4 junk"), 0o644))
	assert.True(t, IsPDF(bin), "magic-byte detection must catch a .bin that is actually a PDF")

	png := filepath.Join(dir, "pic.png")
	require.NoError(t, os.WriteFile(png, []byte("\x89PNG\r\n\x1a\n"), 0o644))
	assert.False(t, IsPDF(png), "a PNG is not a PDF")

	missing := filepath.Join(dir, "nope")
	assert.False(t, IsPDF(missing), "a missing file is not a PDF")
}

func TestRasterizeNoRasterizer(t *testing.T) {
	withChain(t, nil)
	paths, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 3)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoRasterizer)
	assert.Contains(t, err.Error(), "pdftoppm", "error should name installable options")
	assert.Nil(t, paths)
}

func TestRasterizeFirstWins(t *testing.T) {
	withChain(t, []rasterizer{
		fakeRasterizer("broken", 0, true, true),  // available but fails
		fakeRasterizer("good", 3, true, false),   // succeeds
		fakeRasterizer("unused", 9, true, false), // never reached
	})
	paths, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 3)
	require.NoError(t, err)
	require.Len(t, paths, 3, "the first SUCCESSFUL rasterizer wins")
}

func TestRasterizeAllFail(t *testing.T) {
	withChain(t, []rasterizer{
		fakeRasterizer("a", 0, true, true),
		fakeRasterizer("b", 0, true, true),
	})
	_, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a, b", "error names the tools that were tried")
}

func TestRasterizeSortedNumerically(t *testing.T) {
	// Write 12 pages so a lexical sort would mis-order (page-10 before page-2).
	withChain(t, []rasterizer{fakeRasterizer("good", 12, true, false)})
	paths, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 12)
	require.NoError(t, err)
	require.Len(t, paths, 12)
	// Numeric order: 1,2,3,...,12 (NOT 1,10,11,12,2,...).
	assert.Equal(t, "page-1.png", filepath.Base(paths[0]))
	assert.Equal(t, "page-2.png", filepath.Base(paths[1]))
	assert.Equal(t, "page-10.png", filepath.Base(paths[9]))
	assert.Equal(t, "page-12.png", filepath.Base(paths[11]))
}

func TestRasterizeMaxPagesCapPassedThrough(t *testing.T) {
	var seenMax int
	r := rasterizer{
		name:  "spy",
		avail: func() bool { return true },
		run: func(_ context.Context, _ string, outDir string, maxPages int) ([]string, error) {
			seenMax = maxPages
			p := filepath.Join(outDir, "page-1.png")
			return []string{p}, os.WriteFile(p, []byte("PNG"), 0o644)
		},
	}
	withChain(t, []rasterizer{r})

	t.Run("explicit cap", func(t *testing.T) {
		_, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 5)
		require.NoError(t, err)
		assert.Equal(t, 5, seenMax)
	})
	t.Run("default cap", func(t *testing.T) {
		_, err := Rasterize(context.Background(), "x.pdf", t.TempDir(), 0)
		require.NoError(t, err)
		assert.Equal(t, MaxPages, seenMax)
	})
}

func TestPageNum(t *testing.T) {
	cases := map[string]int{
		"page-1.png":  1,
		"page-2.png":  2,
		"page-10.png": 10,
		"3.png":       3,
		"page.png":    0,
		"cover.png":   0,
	}
	for name, want := range cases {
		assert.Equal(t, want, pageNum(name), name)
	}
}
