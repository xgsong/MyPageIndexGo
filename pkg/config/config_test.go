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
	origKey := os.Getenv("OPENAI_API_KEY")

	// Set test env - only sensitive credentials can be set via environment
	os.Setenv("OPENAI_API_KEY", "test-key-123")

	defer func() {
		// Restore original
		os.Setenv("OPENAI_API_KEY", origKey)
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-123", cfg.OpenAIAPIKey)
	// Non-sensitive config must come from config.yaml, not environment variables
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency)
}

func TestLoadFromEnv_RequiresAPIKey(t *testing.T) {
	// Ensure no API key in env
	origKey1 := os.Getenv("OPENAI_API_KEY")
	origKey2 := os.Getenv("OCR_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OCR_API_KEY")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey1)
		os.Setenv("OCR_API_KEY", origKey2)
	}()

	cfg, err := LoadFromEnv()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestLoadFromEnv_NonPrefixed(t *testing.T) {
	// Test loading from non-prefixed environment variable (OPENAI_API_KEY)
	origKey := os.Getenv("OPENAI_API_KEY")
	origOcrKey := os.Getenv("OCR_API_KEY")

	os.Setenv("OPENAI_API_KEY", "test-key-noprefix")
	os.Setenv("OCR_API_KEY", "test-ocr-key")

	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
		os.Setenv("OCR_API_KEY", origOcrKey)
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-noprefix", cfg.OpenAIAPIKey)
	assert.Equal(t, "test-ocr-key", cfg.OCRAPIKey)
	// Non-sensitive config must come from config.yaml
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency)
}

func TestLoadFromEnv_SensitiveOnly(t *testing.T) {
	// Test that only sensitive credentials are loaded from environment
	// Non-sensitive config (model, concurrency, etc.) must come from config.yaml
	origKey := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "test-key-sensitive")

	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-sensitive", cfg.OpenAIAPIKey)
	// These should have default values from config.yaml, not environment
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, "info", cfg.LogLevel)
}
