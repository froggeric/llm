package format

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Format
	}{
		{"", Auto},
		{"auto", Auto},
		{"AUTO", Auto},
		{"text", Text},
		{"txt", Text},
		{"markdown", Markdown},
		{"md", Markdown},
		{"json", JSON},
		{"yaml", YAML},
		{"yml", YAML},
		{"xml", XML},
		{"  JSON  ", JSON},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		require.NoError(t, err, "input %q", c.in)
		assert.Equal(t, c.want, got, "input %q", c.in)
	}
	_, err := Parse("bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown --format")
}

func TestSuffix(t *testing.T) {
	assert.Equal(t, "json", Suffix(JSON))
	assert.Equal(t, "yaml", Suffix(YAML))
	assert.Equal(t, "xml", Suffix(XML))
	assert.Equal(t, "md", Suffix(Markdown))
	assert.Equal(t, "txt", Suffix(Text))
	assert.Equal(t, "txt", Suffix(Auto), "Auto falls back to txt")
}

// stringOut is the parsed shape for 8 of 9 tools.
const stringOut = "# Heading\n\nSome text about the image."

// codeOut mirrors tools.extractCodeTool.ParseOutput's return value.
var codeOut = map[string]any{"language": "python", "code": "print('hi')"}

func TestConvertText(t *testing.T) {
	// String tools: text == the parsed string verbatim.
	b, err := Convert("read_image", stringOut, Text)
	require.NoError(t, err)
	assert.Equal(t, stringOut, string(b))

	// Code tool, Text: bare code body, no fence.
	b, err = Convert("extract_code", codeOut, Text)
	require.NoError(t, err)
	assert.Equal(t, "print('hi')", string(b))
}

func TestConvertMarkdown(t *testing.T) {
	// String tools: markdown == the parsed string verbatim.
	b, err := Convert("read_image", stringOut, Markdown)
	require.NoError(t, err)
	assert.Equal(t, stringOut, string(b))

	// Code tool, Markdown: fenced block with language tag.
	b, err = Convert("extract_code", codeOut, Markdown)
	require.NoError(t, err)
	assert.Equal(t, "```python\nprint('hi')\n```", string(b))

	// Code tool without a language tag still fences.
	b, err = Convert("extract_code", map[string]any{"language": "", "code": "x = 1"}, Markdown)
	require.NoError(t, err)
	assert.Equal(t, "```\nx = 1\n```", string(b))
}

func TestConvertJSON(t *testing.T) {
	// String result wraps in the envelope.
	b, err := Convert("read_image", stringOut, JSON)
	require.NoError(t, err)
	assertValidJSON(t, b)
	var env resultEnvelope
	require.NoError(t, json.Unmarshal(b, &env))
	assert.Equal(t, "read_image", env.Tool)
	assert.Equal(t, stringOut, env.Result)

	// Code result keeps the structured map under result.
	b, err = Convert("extract_code", codeOut, JSON)
	require.NoError(t, err)
	assertValidJSON(t, b)
	require.NoError(t, json.Unmarshal(b, &env))
	assert.Equal(t, "extract_code", env.Tool)
	m, ok := env.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "python", m["language"])
	assert.Equal(t, "print('hi')", m["code"])
}

func TestConvertYAML(t *testing.T) {
	b, err := Convert("describe_chart", "Revenue is up.", YAML)
	require.NoError(t, err)
	var env resultEnvelope
	require.NoError(t, yaml.Unmarshal(b, &env))
	assert.Equal(t, "describe_chart", env.Tool)
	assert.Equal(t, "Revenue is up.", env.Result)

	// YAML for code keeps structure.
	b, err = Convert("extract_code", codeOut, YAML)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(b, &env))
	m, ok := env.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "python", m["language"])
}

func TestConvertXML(t *testing.T) {
	// String result.
	b, err := Convert("read_image", "hello & world", XML)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, s, `<localvision tool="read_image">`)
	assert.Contains(t, s, `<result>hello &amp; world</result>`)
	// Must be valid XML.
	assertValidXML(t, b)

	// Code result: language as attr, code as child element.
	b, err = Convert("extract_code", codeOut, XML)
	require.NoError(t, err)
	assertValidXML(t, b)
	assert.Contains(t, string(b), `language="python"`)
	// xml.Marshal escapes ' as &#39; (valid XML); assert on the escaped form.
	assert.Contains(t, string(b), "<code>print(&#39;hi&#39;)</code>")
}

func TestConvertJSONNeverEmitsNull(t *testing.T) {
	// A nil parsed value (defensive) must not produce a null result.
	b, err := Convert("read_image", nil, JSON)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"result": null`)
}

func TestConvertAutoBehavesLikeMarkdown(t *testing.T) {
	b1, err := Convert("read_image", stringOut, Auto)
	require.NoError(t, err)
	b2, err := Convert("read_image", stringOut, Markdown)
	require.NoError(t, err)
	assert.Equal(t, b2, b1)
}

func TestConvertUnsupportedFormat(t *testing.T) {
	_, err := Convert("read_image", "x", Format("csv"))
	require.Error(t, err)
}

func TestConvertEscapesStructuredChars(t *testing.T) {
	// JSON must escape quotes; XML must escape angle brackets.
	tricky := `He said "hi" <tag> & co.`
	b, err := Convert("read_image", tricky, JSON)
	require.NoError(t, err)
	assertValidJSON(t, b)
	assert.Contains(t, string(b), `He said \"hi\"`)

	b, err = Convert("read_image", tricky, XML)
	require.NoError(t, err)
	assertValidXML(t, b)
	assert.Contains(t, string(b), "&lt;tag&gt;")
}

func assertValidJSON(t *testing.T, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}
}

func assertValidXML(t *testing.T, b []byte) {
	t.Helper()
	var v any
	if err := xml.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, b)
	}
}

func TestConvertCodeMissingFields(t *testing.T) {
	// A map with only language (no code) is not a code result → treated as a
	// generic map and JSON-encoded under result.
	b, err := Convert("extract_code", map[string]any{"language": "go"}, JSON)
	require.NoError(t, err)
	assertValidJSON(t, b)
	// Not fenced as code in markdown (no code key).
	b, err = Convert("extract_code", map[string]any{"language": "go"}, Markdown)
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(string(b), "```"))
}
