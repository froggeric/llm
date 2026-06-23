package tools

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpandInputNoOpForNonExpander: read_image (and the other 10 non-document
// tools) don't implement Expander, so ExpandInput must return the input
// unchanged.
func TestExpandInputNoOpForNonExpander(t *testing.T) {
	in := ToolInput{Images: []ImageRef{{LocalPath: "/tmp/a.png"}}, Extra: map[string]any{}}
	out, err := ExpandInput(context.Background(), readImageTool{}, in)
	require.NoError(t, err)
	assert.Equal(t, in, out, "a non-Expander tool must be passed through unchanged")
}

// TestReadDocumentExpandImagesNonPDF: a plain image is returned unchanged
// (lenient — read_document on an image still works, no rasterization).
func TestReadDocumentExpandImagesNonPDF(t *testing.T) {
	png := filepath.Join(t.TempDir(), "pic.png")
	require.NoError(t, os.WriteFile(png, []byte("\x89PNG\r\n"), 0o644))
	in := ToolInput{Images: []ImageRef{{LocalPath: png}}, Extra: map[string]any{}}
	out, err := readDocumentTool{}.ExpandImages(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, in.Images, out, "a non-PDF must pass through unchanged")
}

// TestReadDocumentExpandImagesPDF: a PDF-shaped file is rasterized into N page
// refs, each registered for cleanup; CleanupImageRefs reaps the page temps and
// their now-empty out dir but leaves the user's PDF untouched.
func TestReadDocumentExpandImagesPDF(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "doc.pdf")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.5\n%fake"), 0o644)) // magic bytes only

	// Swap the rasterizer seam for a fake that writes 3 dummy page PNGs (so no
	// external PDF tool is needed).
	prev := rasterize
	t.Cleanup(func() { rasterize = prev })
	rasterize = func(_ context.Context, _ string, outDir string, _ int) ([]string, error) {
		var ps []string
		for i := 1; i <= 3; i++ {
			p := filepath.Join(outDir, "page-"+strconv.Itoa(i)+".png")
			if err := os.WriteFile(p, []byte("PNG"), 0o644); err != nil {
				return nil, err
			}
			ps = append(ps, p)
		}
		return ps, nil
	}

	tool := readDocumentTool{}
	in := ToolInput{Images: []ImageRef{{LocalPath: pdf}}, Extra: map[string]any{}}
	out, err := tool.ExpandImages(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, out, 3, "one ref per rasterized page")

	// Each page temp exists on disk before cleanup.
	for _, r := range out {
		_, statErr := os.Stat(r.LocalPath)
		assert.NoError(t, statErr, "page temp must exist until cleanup")
	}

	// CleanupImageRefs reaps the page temps (and their now-empty out dir).
	CleanupImageRefs(out)
	for _, r := range out {
		_, statErr := os.Stat(r.LocalPath)
		assert.True(t, os.IsNotExist(statErr), "page temp %q should be gone after cleanup", r.LocalPath)
	}

	// The user's PDF must survive cleanup (it was never registered as a temp).
	_, err = os.Stat(pdf)
	assert.NoError(t, err, "the user's PDF must NOT be removed by cleanup")
}

// TestReadDocumentBuildRequest: a single image returns SystemPrompt() (so the
// auto-iterating TestBuildRequestSanity invariant holds), and the user prompt
// honors an optional question.
func TestReadDocumentBuildRequest(t *testing.T) {
	tool := readDocumentTool{}
	img := []ImageRef{{LocalPath: "/tmp/p1.png"}, {LocalPath: "/tmp/p2.png"}}
	sys, user, paths, err := tool.BuildRequest(ToolInput{Images: img, Extra: map[string]any{"question": "what is the conclusion?"}})
	require.NoError(t, err)
	assert.Equal(t, tool.SystemPrompt(), sys)
	assert.Contains(t, user, "conclusion")
	assert.Len(t, paths, 2)
}
