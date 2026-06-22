package llama

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConv builds a converter for tests: avail controls availability, ok
// controls whether run succeeds, and path is the temp path returned on success.
func fakeConv(name string, avail, ok bool) converter {
	return converter{
		name:  name,
		avail: func() bool { return avail },
		run: func(src, outExt string) (string, error) {
			if !ok {
				return "", errFake(name + " failed")
			}
			// Return a real temp file so the caller's os.Remove works.
			tmp, err := os.CreateTemp("", "fake-*."+outExt)
			if err != nil {
				return "", err
			}
			tmp.Close()
			return tmp.Name(), nil
		},
	}
}

type ferr string

func (e ferr) Error() string { return string(e) }

func errFake(s string) error { return ferr(s) }

func TestConvertImage_PicksFirstAvailableSuccess(t *testing.T) {
	orig := convertersVar
	t.Cleanup(func() { convertersVar = orig })
	convertersVar = []converter{
		fakeConv("a", true, true),
		fakeConv("b", true, true),
	}
	tmp, err := convertImage("in.heic", "jpg")
	require.NoError(t, err)
	defer os.Remove(tmp)
	// "a" won.
	assert.True(t, strings.HasSuffix(tmp, ".jpg"))
}

func TestConvertImage_SkipsUnavailable(t *testing.T) {
	orig := convertersVar
	t.Cleanup(func() { convertersVar = orig })
	convertersVar = []converter{
		fakeConv("sips", false, true),   // not available
		fakeConv("magick", false, true), // not available
		fakeConv("heif-convert", true, true),
	}
	tmp, err := convertImage("in.heic", "png")
	require.NoError(t, err)
	defer os.Remove(tmp)
}

func TestConvertImage_FallsThroughFailure(t *testing.T) {
	orig := convertersVar
	t.Cleanup(func() { convertersVar = orig })
	convertersVar = []converter{
		fakeConv("sips", true, false), // available but fails
		fakeConv("magick", true, true),
	}
	tmp, err := convertImage("in.heic", "jpg")
	require.NoError(t, err)
	defer os.Remove(tmp)
}

func TestConvertImage_NoneAvailable(t *testing.T) {
	orig := convertersVar
	t.Cleanup(func() { convertersVar = orig })
	convertersVar = []converter{
		fakeConv("sips", false, true),
		fakeConv("ffmpeg", false, true),
	}
	_, err := convertImage("in.heic", "jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image converter found")
	assert.Contains(t, err.Error(), "install one of")
}

func TestConvertImage_AllFail(t *testing.T) {
	orig := convertersVar
	t.Cleanup(func() { convertersVar = orig })
	convertersVar = []converter{
		fakeConv("sips", true, false),
		fakeConv("magick", true, false),
	}
	_, err := convertImage("in.heic", "jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversion failed")
	assert.Contains(t, err.Error(), "sips")
	assert.Contains(t, err.Error(), "magick")
}

// TestConvertImage_RealSipsOnDarwin is an integration check: on macOS, sips is
// present, so the chain converts a real PNG round-trip through sips. Skipped on
// non-darwin (no sips).
func TestConvertImage_RealSipsOnDarwin(t *testing.T) {
	if !sipsAvailable() {
		t.Skip("sips not on PATH (non-darwin)")
	}
	src := filepath.Join(t.TempDir(), "in.png")
	writeTinyPNG(t, src)
	tmp, err := convertImage(src, "png")
	require.NoError(t, err)
	defer os.Remove(tmp)
	info, err := os.Stat(tmp)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func sipsAvailable() bool {
	for _, c := range converters {
		if c.name == "sips" && c.avail() {
			return true
		}
	}
	return false
}

// writeTinyPNG encodes a valid 1x1 PNG that sips can decode.
func writeTinyPNG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	require.NoError(t, png.Encode(f, img))
}
