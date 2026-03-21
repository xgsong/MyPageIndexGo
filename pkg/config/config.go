package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	OpenAIAPIKey      string `mapstructure:"openai_api_key"`
	OpenAIBaseURL     string `mapstructure:"openai_base_url"`
	OpenAIModel       string `mapstructure:"openai_model"`
	MaxConcurrency    int    `mapstructure:"max_concurrency"`
	MaxPagesPerNode   int    `mapstructure:"max_pages_per_node"`
	MaxTokensPerNode  int    `mapstructure:"max_tokens_per_node"`
	GenerateSummaries bool   `mapstructure:"generate_summaries"`
	LogLevel          string `mapstructure:"log_level"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		OpenAIModel:       "gpt-4o",
		MaxConcurrency:    5,
		MaxPagesPerNode:   10,
		MaxTokensPerNode:  16000,
		GenerateSummaries: false,
		LogLevel:          "info",
	}
}

// Load loads configuration from .env file, environment variables and config file.
// It tries to load .env first, then falls back to environment variables.
func Load() (*Config, error) {
	// Try to load .env file if it exists
	_ = godotenv.Load()

	v := viper.New()

	// Set defaults
	cfg := DefaultConfig()
	v.SetDefault("openai_api_key", "")
	v.SetDefault("openai_base_url", "")
	v.SetDefault("openai_model", cfg.OpenAIModel)
	v.SetDefault("max_concurrency", cfg.MaxConcurrency)
	v.SetDefault("max_pages_per_node", cfg.MaxPagesPerNode)
	v.SetDefault("max_tokens_per_node", cfg.MaxTokensPerNode)
	v.SetDefault("generate_summaries", cfg.GenerateSummaries)
	v.SetDefault("log_level", cfg.LogLevel)

	// Read from environment variables with prefix
	v.AutomaticEnv()
	v.SetEnvPrefix("PAGEINDEX")

	// Also bind non-prefixed versions for compatibility with .env
	_ = v.BindEnv("openai_api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("openai_base_url", "OPENAI_BASE_URL")
	_ = v.BindEnv("openai_model", "OPENAI_MODEL")
	_ = v.BindEnv("max_concurrency", "MAX_CONCURRENCY")
	_ = v.BindEnv("max_pages_per_node", "MAX_PAGES_PER_NODE")
	_ = v.BindEnv("max_tokens_per_node", "MAX_TOKENS_PER_NODE")
	_ = v.BindEnv("generate_summaries", "GENERATE_SUMMARIES")
	_ = v.BindEnv("log_level", "LOG_LEVEL")

	// Try to read from config file if exists
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.pageindex")

	// Read config file if present
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is okay - fall back to env defaults
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required OpenAI API key
	if cfg.OpenAIAPIKey == "" {
		// Try all possible locations
		cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
		if cfg.OpenAIAPIKey == "" {
			cfg.OpenAIAPIKey = os.Getenv("PAGEINDEX_OPENAI_API_KEY")
		}
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required (check .env file)")
		}
	}

	// Handle OpenAI base URL if set
	if cfg.OpenAIBaseURL == "" {
		cfg.OpenAIBaseURL = os.Getenv("OPENAI_BASE_URL")
		if cfg.OpenAIBaseURL == "" {
			cfg.OpenAIBaseURL = os.Getenv("PAGEINDEX_OPENAI_BASE_URL")
		}
	}

	return cfg, nil
}

// LoadFromEnv loads configuration directly from environment variables.
// This is an alias for Load — kept for backward compatibility.
func LoadFromEnv() (*Config, error) {
	return Load()
}
