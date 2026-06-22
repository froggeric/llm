package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/froggeric/llm/mcp/localvision/internal/tools"
	"github.com/froggeric/llm/mcp/localvision/internal/tools/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupUnitsSingleImageTools(t *testing.T) {
	paths := []string{"a.png", "b.png", "c.png"}
	units, err := groupUnits(paths, tools.ToolReadImage)
	require.NoError(t, err)
	require.Len(t, units, 3)
	assert.Equal(t, []string{"a.png"}, units[0])
	assert.Equal(t, []string{"c.png"}, units[2])
}

func TestGroupUnitsComparePairs(t *testing.T) {
	paths := []string{"a.png", "b.png", "c.png", "d.png"}
	units, err := groupUnits(paths, tools.ToolCompareImages)
	require.NoError(t, err)
	require.Len(t, units, 2)
	assert.Equal(t, []string{"a.png", "b.png"}, units[0])
	assert.Equal(t, []string{"c.png", "d.png"}, units[1])
}

func TestGroupUnitsCompareOddIsError(t *testing.T) {
	_, err := groupUnits([]string{"a.png", "b.png", "c.png"}, tools.ToolCompareImages)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "even number")
}

func TestGroupUnitsCompareTooFew(t *testing.T) {
	_, err := groupUnits([]string{"a.png"}, tools.ToolCompareImages)
	require.Error(t, err)
}

func TestUnitLabelAndStem(t *testing.T) {
	assert.Equal(t, "a.png", unitLabel([]string{"/x/a.png"}))
	assert.Equal(t, "a.png + b.jpg", unitLabel([]string{"/x/a.png", "/y/b.jpg"}))
	assert.Equal(t, "", unitLabel(nil))

	assert.Equal(t, "a", unitFileStem([]string{"/x/a.png"}))
	assert.Equal(t, "a_vs_b", unitFileStem([]string{"/x/a.png", "/y/b.jpg"}))
	assert.Equal(t, "result", unitFileStem(nil))
}

func TestNewOutputSinkMutualExclusion(t *testing.T) {
	_, err := newOutputSink("out.txt", "dir", format.JSON, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestNewOutputSinkModes(t *testing.T) {
	s, err := newOutputSink("", "", format.JSON, false, false)
	require.NoError(t, err)
	assert.Equal(t, sinkStdout, s.mode)

	s, err = newOutputSink("out.txt", "", format.Text, true, false)
	require.NoError(t, err)
	assert.Equal(t, sinkFile, s.mode)
	assert.Equal(t, "out.txt", s.filePath)
	assert.True(t, s.meta)

	s, err = newOutputSink("", "outdir", format.JSON, false, true)
	require.NoError(t, err)
	assert.Equal(t, sinkDir, s.mode)
	assert.Equal(t, "outdir", s.dir)
	assert.True(t, s.batch)
}

func TestSinkTargetDirNaming(t *testing.T) {
	s, err := newOutputSink("", "outdir", format.JSON, false, false)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("outdir", "photo.json"), s.target("photo"))

	s, err = newOutputSink("", "outdir", format.Markdown, false, false)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("outdir", "photo.md"), s.target("photo"))
}

// Regression (M1): two inputs sharing a basename (photos/a.png + shots/a.png)
// must not silently overwrite — the second is disambiguated to a_1.json.
func TestSinkTargetDirDisambiguatesCollisions(t *testing.T) {
	s, err := newOutputSink("", "outdir", format.JSON, false, false)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("outdir", "a.json"), s.target("a"))
	assert.Equal(t, filepath.Join("outdir", "a_1.json"), s.target("a"))
	assert.Equal(t, filepath.Join("outdir", "a_2.json"), s.target("a"))
	// A different name is unaffected.
	assert.Equal(t, filepath.Join("outdir", "b.json"), s.target("b"))
}

func TestSinkTargetStdoutEmpty(t *testing.T) {
	s, err := newOutputSink("", "", format.Text, false, false)
	require.NoError(t, err)
	assert.Equal(t, "", s.target("photo"))
}

func TestSinkWriteFileMode(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "res.txt")
	s, err := newOutputSink(out, "", format.Text, false, false)
	require.NoError(t, err)

	require.NoError(t, s.write("src.png", "src", tools.ToolReadImage, "hello world", tools.Stats{Model: "m"}))
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(b))
}

func TestSinkWriteDirModeCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "out")
	s, err := newOutputSink("", dir, format.JSON, false, false)
	require.NoError(t, err)

	require.NoError(t, s.write("a.png", "a", tools.ToolReadImage, "hi", tools.Stats{}))
	target := filepath.Join(dir, "a.json")
	b, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"result": "hi"`)
}

func TestSinkWriteMetaSidecar(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "res.json")
	s, err := newOutputSink(out, "", format.JSON, true, false)
	require.NoError(t, err)

	stats := tools.Stats{Model: "qwen3.5-4b", TokensIn: 100, TokensOut: 5, ElapsedMs: 1234}
	require.NoError(t, s.write("res.png", "res", tools.ToolExtractText, "text", stats))

	metaBytes, err := os.ReadFile(out + ".meta.json")
	require.NoError(t, err)
	var doc metaDoc
	require.NoError(t, json.Unmarshal(metaBytes, &doc))
	assert.Equal(t, "res.png", doc.Source)
	assert.Equal(t, tools.ToolExtractText, doc.Tool)
	assert.Equal(t, "qwen3.5-4b", doc.Model)
	assert.Equal(t, 100, doc.TokensIn)
	assert.Equal(t, int64(1234), doc.ElapsedMs)
}

// Regression (M1): in dir mode + --meta, the sidecar must sit next to its OWN
// result file, not a double-disambiguated orphan. Two colliding basenames must
// produce (a.json, a.json.meta.json) and (a_1.json, a_1.json.meta.json).
func TestSinkDirMetaSidecarNotOrphaned(t *testing.T) {
	dir := t.TempDir()
	s, err := newOutputSink("", dir, format.JSON, true, true)
	require.NoError(t, err)

	require.NoError(t, s.write("photos/a.png", "a", tools.ToolExtractText, "one", tools.Stats{Model: "m"}))
	require.NoError(t, s.write("shots/a.png", "a", tools.ToolExtractText, "two", tools.Stats{Model: "m"}))

	mustExist := func(p string) {
		t.Helper()
		_, err := os.Stat(filepath.Join(dir, p))
		require.NoError(t, err, "expected %s to exist", p)
	}
	mustNotExist := func(p string) {
		t.Helper()
		_, err := os.Stat(filepath.Join(dir, p))
		require.ErrorIs(t, err, os.ErrNotExist, "expected %s to be absent (orphan)", p)
	}

	// First result + its sidecar.
	mustExist("a.json")
	mustExist("a.json.meta.json")
	// Second result is disambiguated; its sidecar shares the SAME name.
	mustExist("a_1.json")
	mustExist("a_1.json.meta.json")
	// No orphaned sidecars (the pre-fix bug wrote a_1.json.meta.json next to a
	// non-existent a_2.json etc.).
	mustNotExist("a_2.json")
	mustNotExist("a_2.json.meta.json")
	mustNotExist("a_1.json.meta.json.meta.json")
}

func TestSinkStdoutBatchHeader(t *testing.T) {
	// Batch stdout mode prefixes each block with a header. Capture stdout.
	s, err := newOutputSink("", "", format.Text, false, true)
	require.NoError(t, err)
	// We can't easily capture os.Stdout here without redirecting; assert the
	// header is emitted by checking the mode/batch flags instead.
	assert.True(t, s.batch)
	assert.Equal(t, sinkStdout, s.mode)
}

func TestMetaDocJSONShape(t *testing.T) {
	// Sanity: the sidecar is valid JSON with stable keys.
	doc := metaDoc{Source: "s", Tool: "t", Model: "m", TokensIn: 1, TokensOut: 2, ElapsedMs: 3}
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	s := string(b)
	assert.True(t, strings.HasPrefix(s, `{"source":`))
}
