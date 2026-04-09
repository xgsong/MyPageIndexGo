package indexer

import (
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// GeneratorOptions contains optional dependencies for IndexGenerator.
// This allows for dependency injection and easier testing.
type GeneratorOptions struct {
	// Tokenizer is the tokenizer to use. If nil, a default tokenizer will be created.
	Tokenizer *tokenizer.Tokenizer

	// RateLimiter is the rate limiter to use. If nil, a default rate limiter will be created.
	RateLimiter *DynamicRateLimiter

	// EnableNodeTextCache enables caching of node text and token counts.
	// Default: true
	EnableNodeTextCache bool

	// MaxNodeTextCacheEntries is the maximum number of entries in the node text cache.
	// Default: 1000
	MaxNodeTextCacheEntries int
}

// DefaultGeneratorOptions returns default generator options.
func DefaultGeneratorOptions() GeneratorOptions {
	return GeneratorOptions{
		EnableNodeTextCache:     true,
		MaxNodeTextCacheEntries: 1000,
	}
}

// ApplyDefaults applies default values to options where zero values are present.
func (o *GeneratorOptions) ApplyDefaults(cfg *config.Config, llmClient llm.LLMClient) error {
	if o.Tokenizer == nil {
		tok, err := tokenizer.NewTokenizer(cfg.OpenAIModel)
		if err != nil {
			return err
		}
		o.Tokenizer = tok
	}

	if o.RateLimiter == nil {
		initialConcurrency := max(1, cfg.MaxConcurrency)
		minConcurrency := max(1, initialConcurrency/2)
		maxConcurrency := max(initialConcurrency, initialConcurrency*4)
		o.RateLimiter = NewDynamicRateLimiter(initialConcurrency, minConcurrency, maxConcurrency)

		// Set up rate limit feedback if using OpenAI client
		if openaiClient, ok := llmClient.(*llm.OpenAIClient); ok {
			openaiClient.OnRateLimitInfo = func(info llm.RateLimitInfo) {
				o.RateLimiter.AdjustRate(info.Remaining, info.Reset)
			}
		}
	}

	if o.MaxNodeTextCacheEntries <= 0 {
		o.MaxNodeTextCacheEntries = 1000
	}

	return nil
}
