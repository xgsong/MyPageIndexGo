package tokenizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizer_Models(t *testing.T) {
	models := []string{
		"gpt-4",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-3.5-turbo",
		"unknown-model", // Should fallback to cl100k_base
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			tok, err := NewTokenizer(model)
			require.NoError(t, err)
			assert.NotNil(t, tok)
		})
	}
}

func TestTokenizer_LargeText(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	// Create a large text
	largeText := strings.Repeat("This is a sample sentence with multiple words. ", 200)

	count := tok.Count(largeText)
	assert.Greater(t, count, 1000)
}

func TestTokenizer_EdgeCases(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	tests := []struct {
		name        string
		text        string
		expectedMin int
		expectedMax int
	}{
		{"empty", "", 0, 0},
		{"whitespace", "   \n\t  ", 0, 3},
		{"single char", "a", 1, 1},
		{"chinese", "你好世界", 2, 6},
		{"mixed", "Hello 世界 123", 3, 7},
		{"special chars", "!@#$%^&*()", 5, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := tok.Count(tt.text)
			assert.GreaterOrEqual(t, count, tt.expectedMin)
			assert.LessOrEqual(t, count, tt.expectedMax)
		})
	}
}

func TestTokenizer_Truncate(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	text := strings.Repeat("word ", 100) // ~100 tokens

	// Test with limit larger than text
	count, truncated := tok.CountWithTruncate(text, 200)
	assert.Greater(t, count, 0)
	assert.Equal(t, text, truncated)

	// Test with limit smaller than text
	count, truncated = tok.CountWithTruncate(text, 10)
	assert.Equal(t, 10, count)
	assert.Less(t, len(truncated), len(text))

	// Verify truncation boundary
	count2 := tok.Count(truncated)
	assert.LessOrEqual(t, count2, 10)
}

func TestTokenizer_TruncateToMaxTokens_Integration(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	text := strings.Repeat("a b c d e f g h i j ", 50)

	truncated := tok.TruncateToMaxTokens(text, 20)
	count := tok.Count(truncated)
	assert.LessOrEqual(t, count, 20)
}
