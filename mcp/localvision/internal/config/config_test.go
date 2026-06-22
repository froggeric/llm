package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
