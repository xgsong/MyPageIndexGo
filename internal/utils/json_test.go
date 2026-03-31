package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONCleaner_Clean(t *testing.T) {
	cleaner := NewJSONCleaner()

	tests := []struct {
		name     string
		input    string
		wantCont string
	}{
		{
			name:     "removes markdown code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			wantCont: "{\"key\": \"value\"}",
		},
		{
			name:     "removes just backticks",
			input:    "`{\"key\": \"value\"}`",
			wantCont: "{\"key\": \"value\"}",
		},
		{
			name:     "trims whitespace",
			input:    "\n\n  {\"key\": \"value\"}  \n\n",
			wantCont: "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := cleaner.Clean(tt.input)
			assert.Contains(t, cleaned, tt.wantCont)
		})
	}
}

func TestJSONCleaner_ParseJSON(t *testing.T) {
	cleaner := NewJSONCleaner()

	input := "```json\n{\"title\": \"test\", \"pages\": 10}\n```"

	var result struct {
		Title string `json:"title"`
		Pages int    `json:"pages"`
	}

	err := cleaner.ParseJSON(input, &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Title)
	assert.Equal(t, 10, result.Pages)
}

func TestJSONCleaner_ParseJSON_IgnoresUnknownFields(t *testing.T) {
	cleaner := NewJSONCleaner()

	// LLMs often add unexpected fields; parser must tolerate them
	input := `{"title": "test", "pages": 10, "extra_field": "should be ignored"}`

	var result struct {
		Title string `json:"title"`
		Pages int    `json:"pages"`
	}

	err := cleaner.ParseJSON(input, &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Title)
	assert.Equal(t, 10, result.Pages)
}

func TestJSONCleaner_RemovesTrailingComma(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Trailing comma in object
	input := `{"items": [1, 2, 3,], "key": "value"}`
	cleaned := cleaner.Clean(input)

	var result map[string]any
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	assert.Equal(t, []any{float64(1), float64(2), float64(3)}, result["items"])
}

func TestJSONCleaner_EscapedBackslash(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Input with escaped backslash followed by quote: \\"
	input := `{"path": "C:\\Users\\test"}`
	cleaned := cleaner.Clean(input)
	assert.Equal(t, `{"path": "C:\\Users\\test"}`, cleaned)

	var result struct {
		Path string `json:"path"`
	}
	err := cleaner.ParseJSON(input, &result)
	require.NoError(t, err)
	assert.Equal(t, `C:\Users\test`, result.Path)
}

func TestJSONCleaner_EscapedQuoteInText(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Input with escaped quote in Chinese text - simulates LLM output
	input := `{"summary": "斯密认为，分工并非人类智慧的产物，而是人类天性中某种倾向的必然结果。这种倾向就是\"互通有无，物物交换，互相\"。"}`
	cleaned := cleaner.Clean(input)

	var result struct {
		Summary string `json:"summary"`
	}
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	// The escaped quotes should be properly parsed as regular quotes
	assert.Equal(t, `斯密认为，分工并非人类智慧的产物，而是人类天性中某种倾向的必然结果。这种倾向就是"互通有无，物物交换，互相"。`, result.Summary)
}

func TestJSONCleaner_UnescapedBackslashBeforeQuote(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Input with unescaped backslash before quote - problematic case
	// This simulates when LLM outputs text with backslash that shouldn't be there
	input := `{"summary": "text with backslash\\ before quote"}`
	cleaned := cleaner.Clean(input)

	var result struct {
		Summary string `json:"summary"`
	}
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	// Backslash-quote sequence should be preserved as-is in JSON
	assert.Equal(t, `text with backslash\ before quote`, result.Summary)
}

func TestJSONCleaner_InvalidEscapeSequence(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Simulates LLM output with invalid escape sequence: backslash followed by non-escape char
	// This is the problematic case from the user's report
	input := `{"summary": "为斯密后来的经济\影响"}`
	cleaned := cleaner.Clean(input)

	var result struct {
		Summary string `json:"summary"`
	}
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	// Invalid escape \影 should be converted to \\影 (escaped backslash + char)
	assert.Equal(t, `为斯密后来的经济\影响`, result.Summary)
}

func TestJSONCleaner_InvalidEscapeSequenceWithNewline(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Simulates LLM output with backslash followed by 'n' that's not a real newline
	input := `{"summary": "text\not a newline"}`
	cleaned := cleaner.Clean(input)

	var result struct {
		Summary string `json:"summary"`
	}
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	// \n should be preserved as valid escape sequence
	assert.Equal(t, "text\not a newline", result.Summary)
}
