package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
openai_api_key: test-key-from-file
openai_model: gpt-4
max_concurrency: 10
log_level: debug
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(originalDir)
		require.NoError(t, err)
	}()

	// Also set env var to avoid validation error
	os.Setenv("OPENAI_API_KEY", "test-key-from-env")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	require.NoError(t, err)

	// Config file value should be used
	assert.Equal(t, "gpt-4", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_ConfigPriority(t *testing.T) {
	// Set environment variable
	os.Setenv("OPENAI_MODEL", "gpt-4o")
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_MODEL")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	require.NoError(t, err)

	// Environment variable should override default
	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
}

func TestLoad_InvalidInteger(t *testing.T) {
	os.Setenv("MAX_CONCURRENCY", "not-a-number")
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("MAX_CONCURRENCY")
	defer os.Unsetenv("OPENAI_API_KEY")

	_, err := Load()
	// Viper returns error for invalid integer values
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse value")
}

func TestLoadFromEnv_BackwardCompatibility(t *testing.T) {
	// Test backward compatibility alias
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := LoadFromEnv()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test-key", cfg.OpenAIAPIKey)
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency) // Optimized from 5
	assert.Equal(t, 10, cfg.MaxPagesPerNode)
	assert.Equal(t, 24000, cfg.MaxTokensPerNode) // Optimized from 16000
	assert.False(t, cfg.GenerateSummaries)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_CustomBaseURL(t *testing.T) {
	os.Setenv("OPENAI_BASE_URL", "https://custom.openai.api.com/v1")
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_BASE_URL")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "https://custom.openai.api.com/v1", cfg.OpenAIBaseURL)
}
