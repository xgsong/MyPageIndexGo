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
	assert.Equal(t, 20, cfg.MaxConcurrency) // Optimized from 10
	assert.Equal(t, 10, cfg.MaxPagesPerNode)
	assert.Equal(t, 24000, cfg.MaxTokensPerNode) // Optimized from 16000
	assert.Equal(t, false, cfg.GenerateSummaries)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Empty(t, cfg.OpenAIAPIKey)
}

func TestLoadFromEnv(t *testing.T) {
	origKey := os.Getenv("OPENAI_API_KEY")

	if err := os.Setenv("OPENAI_API_KEY", "test-key-123"); err != nil {
		t.Fatalf("Failed to set OPENAI_API_KEY: %v", err)
	}

	defer func() {
		if origKey != "" {
			if err := os.Setenv("OPENAI_API_KEY", origKey); err != nil {
				t.Logf("Warning: Failed to restore OPENAI_API_KEY: %v", err)
			}
		} else {
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Logf("Warning: Failed to unset OPENAI_API_KEY: %v", err)
			}
		}
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-123", cfg.OpenAIAPIKey)
	// Non-sensitive config must come from config.yaml, not environment variables
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 20, cfg.MaxConcurrency)
}

func TestLoadFromEnv_RequiresAPIKey(t *testing.T) {
	origKey1 := os.Getenv("OPENAI_API_KEY")
	origKey2 := os.Getenv("OCR_API_KEY")
	
	if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
		t.Logf("Warning: Failed to unset OPENAI_API_KEY: %v", err)
	}
	if err := os.Unsetenv("OCR_API_KEY"); err != nil {
		t.Logf("Warning: Failed to unset OCR_API_KEY: %v", err)
	}
	
	defer func() {
		if origKey1 != "" {
			if err := os.Setenv("OPENAI_API_KEY", origKey1); err != nil {
				t.Logf("Warning: Failed to restore OPENAI_API_KEY: %v", err)
			}
		}
		if origKey2 != "" {
			if err := os.Setenv("OCR_API_KEY", origKey2); err != nil {
				t.Logf("Warning: Failed to restore OCR_API_KEY: %v", err)
			}
		}
	}()

	cfg, err := LoadFromEnv()
	assert.Nil(t, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestLoadFromEnv_NonPrefixed(t *testing.T) {
	origKey := os.Getenv("OPENAI_API_KEY")
	origOcrKey := os.Getenv("OCR_API_KEY")

	if err := os.Setenv("OPENAI_API_KEY", "test-key-noprefix"); err != nil {
		t.Fatalf("Failed to set OPENAI_API_KEY: %v", err)
	}
	if err := os.Setenv("OCR_API_KEY", "test-ocr-key"); err != nil {
		t.Fatalf("Failed to set OCR_API_KEY: %v", err)
	}

	defer func() {
		if origKey != "" {
			if err := os.Setenv("OPENAI_API_KEY", origKey); err != nil {
				t.Logf("Warning: Failed to restore OPENAI_API_KEY: %v", err)
			}
		} else {
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Logf("Warning: Failed to unset OPENAI_API_KEY: %v", err)
			}
		}
		if origOcrKey != "" {
			if err := os.Setenv("OCR_API_KEY", origOcrKey); err != nil {
				t.Logf("Warning: Failed to restore OCR_API_KEY: %v", err)
			}
		} else {
			if err := os.Unsetenv("OCR_API_KEY"); err != nil {
				t.Logf("Warning: Failed to unset OCR_API_KEY: %v", err)
			}
		}
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-noprefix", cfg.OpenAIAPIKey)
	assert.Equal(t, "test-ocr-key", cfg.OCRAPIKey)
	// Non-sensitive config must come from config.yaml
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 20, cfg.MaxConcurrency)
}

func TestLoadFromEnv_SensitiveOnly(t *testing.T) {
	origKey := os.Getenv("OPENAI_API_KEY")
	
	if err := os.Setenv("OPENAI_API_KEY", "test-key-sensitive"); err != nil {
		t.Fatalf("Failed to set OPENAI_API_KEY: %v", err)
	}

	defer func() {
		if origKey != "" {
			if err := os.Setenv("OPENAI_API_KEY", origKey); err != nil {
				t.Logf("Warning: Failed to restore OPENAI_API_KEY: %v", err)
			}
		} else {
			if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
				t.Logf("Warning: Failed to unset OPENAI_API_KEY: %v", err)
			}
		}
	}()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "test-key-sensitive", cfg.OpenAIAPIKey)
	// These should have default values from config.yaml, not environment
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 20, cfg.MaxConcurrency)
	assert.Equal(t, "info", cfg.LogLevel)
}
