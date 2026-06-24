package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSamplingFor verifies the per-tool sampling recipe matches the category
// report: coverage tools + OCR are union at a raised temp; deterministic tools
// stay single.
func TestSamplingFor(t *testing.T) {
	union := map[string]float64{
		idReadImage:     0.7,
		idDescribeUI:    0.7,
		idDescribeChart: 0.7,
		idExtractText:   0.4, // OCR peaks at mid-temp
	}
	for id, wantTemp := range union {
		s := SamplingFor(id)
		assert.Equal(t, SamplingUnion, s.Mode, "%s should be union", id)
		assert.Equal(t, wantTemp, s.Temp, "%s temp", id)
	}
	for _, id := range []string{
		idExtractCode, idExtractTable, idDescribeDiagram,
		idDiagnoseError, idImageToPrompt, idCompareImages, idReadDocument,
	} {
		s := SamplingFor(id)
		assert.Equal(t, SamplingSingle, s.Mode, "%s should be single (systematic errors)", id)
	}

	// Unknown tool → single default.
	assert.Equal(t, SamplingSingle, SamplingFor("nonexistent").Mode)
}
