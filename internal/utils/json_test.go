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

	var result map[string]interface{}
	err := cleaner.ParseJSON(cleaned, &result)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{float64(1), float64(2), float64(3)}, result["items"])
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
