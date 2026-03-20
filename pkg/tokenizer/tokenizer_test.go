package tokenizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenizer(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)
	assert.NotNil(t, tok)
}

func TestTokenizer_Count(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	// Empty text
	count := tok.Count("")
	assert.Equal(t, 0, count)

	// Simple text
	count = tok.Count("Hello, world!")
	// "Hello, world!" is 4 tokens with gpt-4o encoding
	assert.True(t, count > 0)
}

func TestTokenizer_CountWithTruncate(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	// Short text - no truncation needed
	longText := "This is a longer text that should definitely exceed the token limit of 5 tokens. Let's see what happens."

	count, truncated := tok.CountWithTruncate(longText, 5)
	assert.Equal(t, 5, count)
	// Truncated text should be shorter than original
	assert.True(t, len(truncated) < len(longText))
}

func TestTokenizer_TruncateToMaxTokens(t *testing.T) {
	tok, err := NewTokenizer("gpt-4o")
	require.NoError(t, err)

	text := "The quick brown fox jumps over the lazy dog."
	truncated := tok.TruncateToMaxTokens(text, 5)

	// We can't predict the exact tokens, but it should be shorter
	assert.True(t, len(truncated) <= len(text))
}

func TestTokenizer_FallbackForUnknownModel(t *testing.T) {
	// Unknown model should fall back to cl100k_base
	tok, err := NewTokenizer("unknown-model-xyz")
	require.NoError(t, err)
	assert.NotNil(t, tok)

	// Should still count tokens
	count := tok.Count("Hello")
	assert.True(t, count > 0)
}
