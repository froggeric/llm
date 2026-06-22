package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDirOverrides(t *testing.T) {
	base := Config{
		CacheDir:  "/home/u/.localvision",
		ModelsDir: "/home/u/.localvision/models",
		BinDir:    "/home/u/.localvision/bin",
	}

	t.Run("models-dir override only", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides("", "/Volumes/ext/models", "")
		assert.Equal(t, "/home/u/.localvision", c.CacheDir)   // unchanged
		assert.Equal(t, "/Volumes/ext/models", c.ModelsDir)   // overridden
		assert.Equal(t, "/home/u/.localvision/bin", c.BinDir) // unchanged
	})

	t.Run("cache-dir re-derives models and bin", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides("/Volumes/ext/cache", "", "")
		assert.Equal(t, "/Volumes/ext/cache", c.CacheDir)
		assert.Equal(t, "/Volumes/ext/cache/models", c.ModelsDir) // re-derived
		assert.Equal(t, "/Volumes/ext/cache/bin", c.BinDir)       // re-derived
	})

	t.Run("explicit models-dir wins over cache-dir re-derive", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides("/Volumes/ext/cache", "/custom/models", "")
		assert.Equal(t, "/Volumes/ext/cache", c.CacheDir)
		assert.Equal(t, "/custom/models", c.ModelsDir)
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")
	in := &Config{
		DefaultModel:   "qwen3.5-4b",
		DefaultFormat:  "json",
		CacheDir:       "/tmp/cache",
		ModelsDir:      "/tmp/models",
		BinDir:         "/tmp/bin",
		LogLevel:       "warn",
		LogFile:        "/tmp/lv.log",
		IdleTimeout:    90 * time.Second,
		StartupTimeout: 3 * time.Minute,
		SafetyMarginGB: 6.0,
		HFUser:         "someoneelse",
	}
	require.NoError(t, Save(path, in))

	out, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "qwen3.5-4b", out.DefaultModel)
	assert.Equal(t, "json", out.DefaultFormat)
	assert.Equal(t, "/tmp/cache", out.CacheDir)
	assert.Equal(t, "/tmp/bin", out.BinDir)
	assert.Equal(t, "warn", out.LogLevel)
	assert.Equal(t, "/tmp/lv.log", out.LogFile)
	assert.Equal(t, 90*time.Second, out.IdleTimeout)
	assert.Equal(t, 3*time.Minute, out.StartupTimeout)
	assert.Equal(t, 6.0, out.SafetyMarginGB)
	assert.Equal(t, "someoneelse", out.HFUser)
}

func TestSaveOmitsEmptyFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, Save(path, &Config{DefaultModel: "m"}))
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `default_model = "m"`)
	assert.NotContains(t, s, "default_format", "empty fields omitted")
	assert.NotContains(t, s, "cache_dir", "empty fields omitted")
}

func TestSaveEmptyPathErrors(t *testing.T) {
	require.Error(t, Save("", &Config{}))
}
