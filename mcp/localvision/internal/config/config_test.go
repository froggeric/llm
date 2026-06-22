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
	// Use OS-absolute paths (t.TempDir) for overrides so expandPath is an
	// identity and the test holds on Windows as well as Unix.
	base := Config{
		CacheDir:  "/home/u/.localvision",
		ModelsDir: "/home/u/.localvision/models",
		BinDir:    "/home/u/.localvision/bin",
	}
	tmp := t.TempDir()
	cacheOverride := filepath.Join(tmp, "cache")
	modelsOverride := filepath.Join(tmp, "custom-models")

	t.Run("models-dir override only", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides("", modelsOverride, "")
		assert.Equal(t, "/home/u/.localvision", c.CacheDir)   // unchanged
		assert.Equal(t, modelsOverride, c.ModelsDir)          // overridden
		assert.Equal(t, "/home/u/.localvision/bin", c.BinDir) // unchanged
	})

	t.Run("cache-dir re-derives models and bin", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides(cacheOverride, "", "")
		assert.Equal(t, cacheOverride, c.CacheDir)
		assert.Equal(t, filepath.Join(cacheOverride, "models"), c.ModelsDir) // re-derived
		assert.Equal(t, filepath.Join(cacheOverride, "bin"), c.BinDir)       // re-derived
	})

	t.Run("explicit models-dir wins over cache-dir re-derive", func(t *testing.T) {
		c := base
		c.ApplyDirOverrides(cacheOverride, modelsOverride, "")
		assert.Equal(t, cacheOverride, c.CacheDir)
		assert.Equal(t, modelsOverride, c.ModelsDir)
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// OS-absolute paths so Load's expandPath is an identity on every platform.
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "cfgdir")
	path := filepath.Join(dir, "nested", "config.toml")
	in := &Config{
		DefaultModel:   "qwen3.5-4b",
		DefaultFormat:  "json",
		CacheDir:       filepath.Join(tmp, "cache"),
		ModelsDir:      filepath.Join(tmp, "models"),
		BinDir:         filepath.Join(tmp, "bin"),
		LogLevel:       "warn",
		LogFile:        filepath.Join(tmp, "lv.log"),
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
	assert.Equal(t, in.CacheDir, out.CacheDir)
	assert.Equal(t, in.BinDir, out.BinDir)
	assert.Equal(t, "warn", out.LogLevel)
	assert.Equal(t, in.LogFile, out.LogFile)
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
