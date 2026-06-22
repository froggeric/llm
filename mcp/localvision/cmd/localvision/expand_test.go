package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile creates a file under dir with the given name and content.
func writeFile(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
}

func TestExpandLiteralFile(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	got, err := expandInputs([]string{filepath.Join(d, "a.png")}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(d, "a.png")}, got)
}

func TestExpandGlob(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	writeFile(t, d, "b.png")
	writeFile(t, d, "c.txt")
	got, err := expandInputs([]string{filepath.Join(d, "*.png")}, false)
	require.NoError(t, err)
	sort.Strings(got)
	assert.Equal(t, []string{filepath.Join(d, "a.png"), filepath.Join(d, "b.png")}, got)
}

func TestExpandGlobNoMatch(t *testing.T) {
	d := t.TempDir()
	_, err := expandInputs([]string{filepath.Join(d, "*.png")}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matched no files")
}

func TestExpandDirectoryNonRecursive(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	writeFile(t, d, "b.jpg")
	writeFile(t, d, "ignore.txt")
	sub := filepath.Join(d, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	writeFile(t, sub, "nested.png") // must be skipped (non-recursive)

	got, err := expandInputs([]string{d}, false)
	require.NoError(t, err)
	sort.Strings(got)
	assert.Equal(t, []string{filepath.Join(d, "a.png"), filepath.Join(d, "b.jpg")}, got)
}

func TestExpandDirectoryRecursive(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	sub := filepath.Join(d, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	writeFile(t, sub, "nested.png")

	got, err := expandInputs([]string{d}, true)
	require.NoError(t, err)
	sort.Strings(got)
	assert.Equal(t, []string{
		filepath.Join(d, "a.png"),
		filepath.Join(sub, "nested.png"),
	}, got)
}

func TestExpandDirectoryNoImages(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "readme.txt")
	_, err := expandInputs([]string{d}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image files")
}

func TestExpandMissingLiteral(t *testing.T) {
	_, err := expandInputs([]string{filepath.Join(t.TempDir(), "nope.png")}, false)
	require.Error(t, err)
}

func TestExpandRejectsHTTP(t *testing.T) {
	_, err := expandInputs([]string{"https://example.com/a.png"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "localhost-only")
}

func TestExpandURIPassthrough(t *testing.T) {
	got, err := expandInputs([]string{"data:image/png;base64,AAAA", "file:///tmp/x.png"}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"data:image/png;base64,AAAA", "file:///tmp/x.png"}, got)
}

func TestReadStdinPaths(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	writeFile(t, d, "b.png")
	in := strings.NewReader(filepath.Join(d, "a.png") + "\n# comment\n\n" + filepath.Join(d, "b.png") + "\n")
	got, err := readStdinPaths(in)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(d, "a.png"), filepath.Join(d, "b.png")}, got)
}

func TestReadStdinPathsEmpty(t *testing.T) {
	_, err := readStdinPaths(strings.NewReader("\n# only comments\n"))
	require.Error(t, err)
}

func TestExpandDedup(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	p := filepath.Join(d, "a.png")
	got, err := expandInputs([]string{p, p}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{p}, got)
}

func TestExpandMixedArgsOrderPreserved(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.png")
	writeFile(t, d, "b.png")
	writeFile(t, d, "c.png")
	// Explicit order must be preserved (no sort across args).
	got, err := expandInputs([]string{
		filepath.Join(d, "c.png"),
		filepath.Join(d, "a.png"),
		filepath.Join(d, "b.png"),
	}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join(d, "c.png"),
		filepath.Join(d, "a.png"),
		filepath.Join(d, "b.png"),
	}, got)
}

func TestIsImageExt(t *testing.T) {
	assert.True(t, isImageExt("a.png"))
	assert.True(t, isImageExt("a.JPEG"))
	assert.True(t, isImageExt("a.heic"))
	assert.False(t, isImageExt("a.txt"))
	assert.False(t, isImageExt("noext"))
}

func TestHasGlobChars(t *testing.T) {
	assert.True(t, hasGlobChars("*.png"))
	assert.True(t, hasGlobChars("a[0-9].png"))
	assert.True(t, hasGlobChars("img?.png"))
	assert.False(t, hasGlobChars("img.png"))
}
