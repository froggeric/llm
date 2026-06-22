package tools

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseImageRef_FilePath exercises the primary input format: a bare
// filesystem path. Both relative and absolute paths must resolve to an
// absolute LocalPath.
func TestParseImageRef_FilePath(t *testing.T) {
	t.Run("absolute path", func(t *testing.T) {
		ref, err := ParseImageRef("/tmp/some-image.png")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/some-image.png", ref.LocalPath)
		assert.Equal(t, "/tmp/some-image.png", ref.Source)
	})

	t.Run("relative path is made absolute", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)

		ref, err := ParseImageRef("some-image.png")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(wd, "some-image.png"), ref.LocalPath)
		assert.Equal(t, "some-image.png", ref.Source, "Source preserves the original input")
	})

	t.Run("dotted relative path is cleaned", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)

		ref, err := ParseImageRef("./foo/../bar.png")
		require.NoError(t, err)
		// filepath.Abs already cleans, so the result has no ./ or ../ segments.
		expected := filepath.Join(wd, "bar.png")
		assert.Equal(t, expected, ref.LocalPath)
	})
}

// TestParseImageRef_DataURI exercises the fallback format. The base64
// payload must be decoded and written to a temp file under os.TempDir();
// the temp file must be registered for cleanup.
func TestParseImageRef_DataURI(t *testing.T) {
	t.Run("png base64 roundtrip", func(t *testing.T) {
		// 8x8 transparent PNG. Tiny so the test is fast.
		png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x08" +
			"\x00\x00\x00\x08\x08\x06\x00\x00\x00\x31\x1f\x96\xff" +
			"\x00\x00\x00\nIDATx\x9cc`\x00\x00\x00\x02\x00\x01\xe5" +
			"\x27\xde\xfc\x00\x00\x00\x00IEND\xaeB`\x82"
		encoded := base64.StdEncoding.EncodeToString([]byte(png))
		uri := "data:image/png;base64," + encoded

		ref, err := ParseImageRef(uri)
		require.NoError(t, err)
		assert.NotEqual(t, uri, ref.LocalPath, "LocalPath should be a temp file path, not the URI")
		assert.True(t, strings.HasPrefix(filepath.Base(ref.LocalPath), "lvm-image-"),
			"temp file name should have the lvm-image- prefix; got %q", filepath.Base(ref.LocalPath))
		assert.True(t, filepath.IsAbs(ref.LocalPath), "LocalPath must be absolute")
		assert.True(t, strings.Contains(ref.Source, "data:image/png;base64,"),
			"Source should preserve the data URI header")
		assert.False(t, strings.Contains(ref.Source, encoded),
			"Source should NOT carry the base64 payload (PII / log-pollution risk)")

		// Verify the temp file actually exists and has the right contents.
		got, err := os.ReadFile(ref.LocalPath)
		require.NoError(t, err)
		assert.Equal(t, []byte(png), got, "decoded bytes match original")

		// Cleanup must remove the temp file.
		CleanupImageRef(ref)
		_, err = os.Stat(ref.LocalPath)
		assert.True(t, os.IsNotExist(err), "temp file should be gone after Cleanup")
	})

	t.Run("Cleanup is idempotent", func(t *testing.T) {
		uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("hi"))
		ref, err := ParseImageRef(uri)
		require.NoError(t, err)

		CleanupImageRef(ref)
		CleanupImageRef(ref) // must not panic
		CleanupImageRef(ref) // must not panic
	})

	t.Run("CleanupImageRefs handles slice", func(t *testing.T) {
		uri1 := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("one"))
		uri2 := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("two"))
		ref1, err := ParseImageRef(uri1)
		require.NoError(t, err)
		ref2, err := ParseImageRef(uri2)
		require.NoError(t, err)

		CleanupImageRefs([]ImageRef{ref1, ref2})
		_, err = os.Stat(ref1.LocalPath)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(ref2.LocalPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("missing comma is rejected", func(t *testing.T) {
		_, err := ParseImageRef("data:image/png;base64")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidImageInput), "got %v", err)
	})

	t.Run("invalid base64 is rejected", func(t *testing.T) {
		_, err := ParseImageRef("data:image/png;base64,@@@@not-valid-b64@@@@")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidImageInput), "got %v", err)
	})
}

// TestParseImageRef_FileURI exercises the file:// URI form.
func TestParseImageRef_FileURI(t *testing.T) {
	t.Run("absolute file URI", func(t *testing.T) {
		ref, err := ParseImageRef("file:///tmp/some-image.png")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/some-image.png", ref.LocalPath)
	})

	t.Run("file URI with empty path is rejected", func(t *testing.T) {
		// file://something.png parses with Host=something.png and Path="".
		// That is a malformed URI; the path component is what we need.
		_, err := ParseImageRef("file://something.png")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidImageInput), "got %v", err)
	})

	t.Run("malformed file URI is rejected", func(t *testing.T) {
		// url.Parse rejects control characters in the URI.
		_, err := ParseImageRef("file://\x7f\x00")
		require.Error(t, err)
	})
}

// TestParseImageRef_RemoteURLRejected exercises F1.10: http(s):// URLs must
// be rejected with a clear, actionable error. The error message must guide
// the caller toward the supported alternatives.
func TestParseImageRef_RemoteURLRejected(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/image.png",
		"https://example.com/image.png",
		"https://huggingface.co/froggeric/test/resolve/main/img.png",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseImageRef(raw)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedImageSource),
				"want ErrUnsupportedImageSource, got %v", err)
			// The message must mention a supported alternative.
			msg := err.Error()
			assert.True(t,
				strings.Contains(msg, "image_path") ||
					strings.Contains(msg, "image_data") ||
					strings.Contains(msg, "data:"),
				"error should suggest an alternative; got %q", msg)
		})
	}
}

// TestParseImageRef_EmptyInput ensures empty strings are rejected, not
// silently accepted as the current working directory.
func TestParseImageRef_EmptyInput(t *testing.T) {
	_, err := ParseImageRef("")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidImageInput))
}

// TestCleanupImageRef_NonTempFile is the safety check: Cleanup on a ref
// pointing at a user file (not a temp file we created) must be a no-op.
// We don't want to delete the user's actual image.
func TestCleanupImageRef_NonTempFile(t *testing.T) {
	// Create a real file under /tmp (NOT via ParseImageRef).
	f, err := os.CreateTemp("", "user-file-*.png")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.Close()

	ref := ImageRef{LocalPath: f.Name(), Source: f.Name()}
	CleanupImageRef(ref)

	_, err = os.Stat(f.Name())
	assert.NoError(t, err, "user file must NOT be deleted by Cleanup")
}
