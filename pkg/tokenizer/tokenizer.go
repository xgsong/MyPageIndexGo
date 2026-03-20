package tokenizer

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer counts tokens for different models using tiktoken.
type Tokenizer struct {
	encoding *tiktoken.Tiktoken
	model    string
}

// NewTokenizer creates a new Tokenizer for the given model.
func NewTokenizer(model string) (*Tokenizer, error) {
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fallback to cl100k_base if model not found
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, fmt.Errorf("failed to get encoding: %w", err)
		}
	}

	return &Tokenizer{
		encoding: enc,
		model:    model,
	}, nil
}

// Count returns the number of tokens in the given text.
func (t *Tokenizer) Count(text string) int {
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountWithTruncate returns the token count and indicates if text exceeds max tokens.
// If it exceeds, it returns the count and the truncated text.
func (t *Tokenizer) CountWithTruncate(text string, maxTokens int) (int, string) {
	tokens := t.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return len(tokens), text
	}

	truncated := t.encoding.Decode(tokens[:maxTokens])
	return maxTokens, truncated
}

// TruncateToMaxTokens truncates text to fit within max tokens.
func (t *Tokenizer) TruncateToMaxTokens(text string, maxTokens int) string {
	_, truncated := t.CountWithTruncate(text, maxTokens)
	return truncated
}
