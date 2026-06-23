package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemp writes content to path's parent dir (created) and returns path.
func writeTemp(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, content, 0o644))
}

// TestMigrateLegacyFile_Hit is the happy path: a v0.4-era file at the flat
// path matches this model's SHA and is claimed into its per-model subdir.
func TestMigrateLegacyFile_Hit(t *testing.T) {
	dir := t.TempDir()
	content := []byte("model bytes")
	sha := hashBytes(content)
	legacy := filepath.Join(dir, "mmproj-F16.gguf")
	dest := filepath.Join(dir, "qwen3-vl-8b", "mmproj-F16.gguf")
	writeTemp(t, legacy, content)

	migrated, err := MigrateLegacyFile(context.Background(), legacy, dest, sha)
	require.NoError(t, err)
	assert.True(t, migrated)

	// File moved to the new per-model path; legacy gone.
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, content, got)
	_, statErr := os.Stat(legacy)
	assert.True(t, os.IsNotExist(statErr), "legacy file should be gone after migrate")
}

// TestMigrateLegacyFile_Idempotent: once migrated (dest exists), a second call
// is a no-op and does not touch the (now-absent) legacy path.
func TestMigrateLegacyFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("xyz")
	sha := hashBytes(content)
	legacy := filepath.Join(dir, "model.gguf")
	dest := filepath.Join(dir, "m", "model.gguf")
	writeTemp(t, legacy, content)

	_, err := MigrateLegacyFile(context.Background(), legacy, dest, sha)
	require.NoError(t, err)

	// Second call: dest exists, legacy gone -> no-op, no error.
	migrated, err := MigrateLegacyFile(context.Background(), legacy, dest, sha)
	require.NoError(t, err)
	assert.False(t, migrated)
}

// TestMigrateLegacyFile_SHA_Mismatch_Left_Alone is the colliding-mmproj case:
// the legacy file exists but belongs to a DIFFERENT model (SHA mismatch). It
// must be left in place so the rightful owner can claim it on its own load,
// and must NOT be renamed into this model's subdir.
func TestMigrateLegacyFile_SHA_Mismatch_Left_Alone(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "mmproj-F16.gguf")
	dest := filepath.Join(dir, "qwen3.6-27b", "mmproj-F16.gguf")
	writeTemp(t, legacy, []byte("this is the 8B's projector, not the 27B's"))
	wrongSHA := hashBytes([]byte("the 27B's actual projector bytes"))

	migrated, err := MigrateLegacyFile(context.Background(), legacy, dest, wrongSHA)
	require.NoError(t, err)
	assert.False(t, migrated, "a SHA-mismatched legacy file must not be claimed")

	// Legacy untouched; dest not created.
	_, statErr := os.Stat(legacy)
	require.NoError(t, statErr, "legacy file must remain for its real owner")
	_, statErr = os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "dest must not be created for a mismatched file")
}

// TestMigrateLegacyFile_NoLegacy: nothing at the legacy path -> no-op.
func TestMigrateLegacyFile_NoLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "absent.gguf")
	dest := filepath.Join(dir, "m", "absent.gguf")

	migrated, err := MigrateLegacyFile(context.Background(), legacy, dest, "deadbeef")
	require.NoError(t, err)
	assert.False(t, migrated)
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr))
}

// TestMigrateLegacyFile_DestExists: dest already present -> no-op even if a
// legacy file also exists (don't clobber a freshly-downloaded new-layout file).
func TestMigrateLegacyFile_DestExists(t *testing.T) {
	dir := t.TempDir()
	content := []byte("old v0.4 bytes")
	sha := hashBytes(content)
	legacy := filepath.Join(dir, "model.gguf")
	dest := filepath.Join(dir, "m", "model.gguf")
	writeTemp(t, legacy, content)
	writeTemp(t, dest, []byte("already-migrated new-layout bytes"))

	migrated, err := MigrateLegacyFile(context.Background(), legacy, dest, sha)
	require.NoError(t, err)
	assert.False(t, migrated)

	// Dest unchanged; legacy still present (we didn't claim it).
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "already-migrated new-layout bytes", string(got))
	_, statErr := os.Stat(legacy)
	require.NoError(t, statErr)
}

// TestMigrateLegacyFile_TwoModelsOneMmproj simulates the real bug: three models
// share the basename mmproj-F16.gguf, but only one file is on disk (the last
// loaded model's). Each model's load should claim it only if it is THAT model's
// file; the others leave it and download their own.
func TestMigrateLegacyFile_TwoModelsOneMmproj(t *testing.T) {
	dir := t.TempDir()
	mmproj8B := []byte("8B projector")
	legacy := filepath.Join(dir, "mmproj-F16.gguf")
	writeTemp(t, legacy, mmproj8B) // on disk: the 8B's projector

	sha8B := hashBytes(mmproj8B)
	sha27B := hashBytes([]byte("27B projector"))

	// 27B loads first: legacy SHA != 27B -> not claimed, left in place.
	migrated, err := MigrateLegacyFile(context.Background(),
		legacy, filepath.Join(dir, "qwen3.6-27b", "mmproj-F16.gguf"), sha27B)
	require.NoError(t, err)
	assert.False(t, migrated)

	// 8B loads: legacy SHA == 8B -> claimed into the 8B's subdir.
	migrated, err = MigrateLegacyFile(context.Background(),
		legacy, filepath.Join(dir, "qwen3-vl-8b", "mmproj-F16.gguf"), sha8B)
	require.NoError(t, err)
	assert.True(t, migrated)

	// Legacy gone (claimed by 8B); 27B's subdir never created.
	_, statErr := os.Stat(legacy)
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(filepath.Join(dir, "qwen3.6-27b", "mmproj-F16.gguf"))
	assert.True(t, os.IsNotExist(statErr), "27B must download its own; it did not claim the 8B's")
}
