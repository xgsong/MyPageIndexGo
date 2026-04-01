package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_UsesConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
openai_base_url: https://api.openai.com/v1
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

	// Set env var for sensitive credential
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "test-key-from-env"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cfg, err := Load()
	require.NoError(t, err)

	// Config file values should be used for non-sensitive config
	assert.Equal(t, "gpt-4", cfg.OpenAIModel)
	assert.Equal(t, 10, cfg.MaxConcurrency) // From config.yaml
	assert.Equal(t, "debug", cfg.LogLevel)
	// Sensitive config from environment
	assert.Equal(t, "test-key-from-env", cfg.OpenAIAPIKey)
}

func TestLoad_ConfigPriority(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "test-key", cfg.OpenAIAPIKey)
}

func TestLoad_CustomBaseURL(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
openai_base_url: https://custom.openai.api.com/v1
openai_model: gpt-4o
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

	// Set env var for sensitive credential
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "https://custom.openai.api.com/v1", cfg.OpenAIBaseURL)
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "gpt-4o", cfg.OpenAIModel)
	assert.Equal(t, 20, cfg.MaxConcurrency)
	assert.Equal(t, 5, cfg.MaxPagesPerNode)
	assert.Equal(t, 16000, cfg.MaxTokensPerNode)
	assert.False(t, cfg.GenerateSummaries)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Empty(t, cfg.OpenAIAPIKey)
	assert.Empty(t, cfg.OCRAPIKey)
}

func TestLoadFromEnv_BackwardCompatibility(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cfg, err := LoadFromEnv()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test-key", cfg.OpenAIAPIKey)
}
