package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	OpenAIAPIKey      string `mapstructure:"openai_api_key"`
	OpenAIBaseURL     string `mapstructure:"openai_base_url"`
	OpenAIModel       string `mapstructure:"openai_model"`
	OCRModel          string `mapstructure:"ocr_model"`   // Model name for OCR (e.g., GLM-OCR-Q8_0)
	OCREnabled        bool   `mapstructure:"ocr_enabled"` // Enable OCR for scanned PDFs
	MaxConcurrency    int    `mapstructure:"max_concurrency"`
	MaxPagesPerNode   int    `mapstructure:"max_pages_per_node"`
	MaxTokensPerNode  int    `mapstructure:"max_tokens_per_node"`
	GenerateSummaries bool   `mapstructure:"generate_summaries"`
	LogLevel          string `mapstructure:"log_level"`
	EnableLLMCache    bool   `mapstructure:"enable_llm_cache"`
	LLMCacheTTL       int    `mapstructure:"llm_cache_ttl"`       // TTL in seconds, 0 means no expiration
	EnableSearchCache bool   `mapstructure:"enable_search_cache"` // Enable caching for search results
	EnableBatchCalls  bool   `mapstructure:"enable_batch_calls"`  // Enable batch LLM calls for summary generation
	BatchSize         int    `mapstructure:"batch_size"`          // Number of summaries to generate per batch
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		OpenAIModel:       "gpt-4o",
		OCRModel:          "GLM-OCR-Q8_0",
		OCREnabled:        false,
		MaxConcurrency:    10,
		MaxPagesPerNode:   10,
		MaxTokensPerNode:  24000,
		GenerateSummaries: false,
		LogLevel:          "info",
		EnableLLMCache:    true,
		LLMCacheTTL:       3600,
		EnableSearchCache: false,
		EnableBatchCalls:  true,
		BatchSize:         20,
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
	v.SetDefault("ocr_model", cfg.OCRModel)
	v.SetDefault("ocr_enabled", cfg.OCREnabled)
	v.SetDefault("max_concurrency", cfg.MaxConcurrency)
	v.SetDefault("max_pages_per_node", cfg.MaxPagesPerNode)
	v.SetDefault("max_tokens_per_node", cfg.MaxTokensPerNode)
	v.SetDefault("generate_summaries", cfg.GenerateSummaries)
	v.SetDefault("log_level", cfg.LogLevel)
	v.SetDefault("enable_llm_cache", cfg.EnableLLMCache)
	v.SetDefault("llm_cache_ttl", cfg.LLMCacheTTL)
	v.SetDefault("enable_search_cache", cfg.EnableSearchCache)
	v.SetDefault("enable_batch_calls", cfg.EnableBatchCalls)
	v.SetDefault("batch_size", cfg.BatchSize)

	// Read from environment variables with prefix
	// SetEnvPrefix must be called BEFORE AutomaticEnv
	v.SetEnvPrefix("PAGEINDEX")
	v.AutomaticEnv()

	// Also bind non-prefixed versions for compatibility with .env
	_ = v.BindEnv("openai_api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("openai_base_url", "OPENAI_BASE_URL")
	_ = v.BindEnv("openai_model", "OPENAI_MODEL")
	_ = v.BindEnv("ocr_model", "OCR_MODEL")
	_ = v.BindEnv("ocr_enabled", "OCR_ENABLED")
	_ = v.BindEnv("max_concurrency", "MAX_CONCURRENCY")
	_ = v.BindEnv("max_pages_per_node", "MAX_PAGES_PER_NODE")
	_ = v.BindEnv("max_tokens_per_node", "MAX_TOKENS_PER_NODE")
	_ = v.BindEnv("generate_summaries", "GENERATE_SUMMARIES")
	_ = v.BindEnv("log_level", "LOG_LEVEL")
	_ = v.BindEnv("enable_llm_cache", "ENABLE_LLM_CACHE")
	_ = v.BindEnv("llm_cache_ttl", "LLM_CACHE_TTL")
	_ = v.BindEnv("enable_search_cache", "ENABLE_SEARCH_CACHE")
	_ = v.BindEnv("enable_batch_calls", "ENABLE_BATCH_CALLS")
	_ = v.BindEnv("batch_size", "BATCH_SIZE")

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
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required (check .env file)")
	}

	return cfg, nil
}

// LoadFromEnv loads configuration directly from environment variables.
// This is an alias for Load — kept for backward compatibility.
func LoadFromEnv() (*Config, error) {
	return Load()
}
