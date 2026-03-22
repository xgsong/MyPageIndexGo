package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency) // Optimized from 5
	assert.Equal(t, 10, cfg.MaxPagesPerNode)
	assert.Equal(t, 24000, cfg.MaxTokensPerNode) // Optimized from 16000
	assert.Equal(t, false, cfg.GenerateSummaries)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Empty(t, cfg.OpenAIAPIKey)
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env
	origKey := os.Getenv("PAGEINDEX_OPENAI_API_KEY")
	origModel := os.Getenv("PAGEINDEX_OPENAI_MODEL")
	origConcurrency := os.Getenv("PAGEINDEX_MAX_CONCURRENCY")

	// Set test env
	os.Setenv("PAGEINDEX_OPENAI_API_KEY", "test-key-123")
	os.Setenv("PAGEINDEX_OPENAI_MODEL", "gpt-4o-mini")
	os.Setenv("PAGEINDEX_MAX_CONCURRENCY", "10")

	defer func() {
		// Restore original
		os.Setenv("PAGEINDEX_OPENAI_API_KEY", origKey)
		os.Setenv("PAGEINDEX_OPENAI_MODEL", origModel)
		os.Setenv("PAGEINDEX_MAX_CONCURRENCY", origConcurrency)
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-123", cfg.OpenAIAPIKey)
	assert.Equal(t, "gpt-4o-mini", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency)
}

func TestLoadFromEnv_RequiresAPIKey(t *testing.T) {
	// Ensure no API key in env
	origKey1 := os.Getenv("OPENAI_API_KEY")
	origKey2 := os.Getenv("PAGEINDEX_OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("PAGEINDEX_OPENAI_API_KEY")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey1)
		os.Setenv("PAGEINDEX_OPENAI_API_KEY", origKey2)
	}()

	cfg, err := LoadFromEnv()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestLoadFromEnv_NonPrefixed(t *testing.T) {
	// Test loading from non-prefixed environment variable (OPENAI_API_KEY)
	origKey := os.Getenv("OPENAI_API_KEY")
	origModel := os.Getenv("OPENAI_MODEL")
	origConcurrency := os.Getenv("MAX_CONCURRENCY")

	os.Setenv("OPENAI_API_KEY", "test-key-noprefix")
	os.Setenv("OPENAI_MODEL", "gpt-4o")
	os.Setenv("MAX_CONCURRENCY", "8")

	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
		os.Setenv("OPENAI_MODEL", origModel)
		os.Setenv("MAX_CONCURRENCY", origConcurrency)
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-noprefix", cfg.OpenAIAPIKey)
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 8, cfg.MaxConcurrency)
}

func TestLoadFromEnv_InvalidInteger(t *testing.T) {
	// Save original
	origKey := os.Getenv("PAGEINDEX_OPENAI_API_KEY")
	origConcurrency := os.Getenv("PAGEINDEX_MAX_CONCURRENCY")

	os.Setenv("PAGEINDEX_OPENAI_API_KEY", "test-key")
	os.Setenv("PAGEINDEX_MAX_CONCURRENCY", "not-a-number")

	defer func() {
		os.Setenv("PAGEINDEX_OPENAI_API_KEY", origKey)
		os.Setenv("PAGEINDEX_MAX_CONCURRENCY", origConcurrency)
	}()

	_, err := LoadFromEnv()
	// Viper returns error on invalid integer - fail fast is safer
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse value as 'int'")
}
