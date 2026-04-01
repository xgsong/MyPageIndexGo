package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// DefaultConfig returns a default configuration for TESTING PURPOSES ONLY.
// Production deployments MUST use config.yaml for all non-sensitive configuration.
// This function exists solely to support existing tests and should not be used in production code.
func DefaultConfig() *Config {
	return &Config{
		OpenAIAPIKey:        "", // Empty by default for tests
		OCRAPIKey:           "", // Empty by default for tests
		OpenAIBaseURL:       "https://api.openai.com/v1",
		OpenAIModel:         "gpt-4o",
		OCRModel:            "GLM-OCR-Q8_0",
		OCREnabled:          false,
		OpenAIOCRBaseURL:    "http://localhost:8080",
		OCRRenderDPI:        150,
		OCRConcurrency:      5,
		OCRTimeout:          60,
		MaxConcurrency:      20,
		MaxPagesPerNode:     5,
		MaxTokensPerNode:    16000,
		GenerateSummaries:   false, // Disable summary generation temporarily for debugging structure extraction
		LogLevel:            "info",
		EnableLLMCache:      true,
		LLMCacheTTL:         3600,
		EnableSearchCache:   false,
		EnableBatchCalls:    false,
		BatchSize:           10,
		TOCheckPageNum:      20,
		MaxTokenNumEachNode: 2000,
		SkipTOCFix:          false,
		SkipAppearanceCheck: false,
	}
}

// Config holds the application configuration.
// All non-sensitive fields must be explicitly set in config.yaml for production use
// Sensitive fields (API keys) must be set in .env or environment variables
type Config struct {
	// Sensitive configuration (from environment variables/.env only)
	OpenAIAPIKey string `mapstructure:"openai_api_key"` // Required, from environment only
	OCRAPIKey    string `mapstructure:"ocr_api_key"`    // Optional, for cloud OCR providers

	// LLM Configuration (from config.yaml)
	OpenAIBaseURL string `mapstructure:"openai_base_url"`
	OpenAIModel   string `mapstructure:"openai_model"`

	// OCR Configuration (from config.yaml)
	OCRModel         string `mapstructure:"ocr_model"`           // Model name for OCR (e.g., GLM-OCR-Q8_0)
	OCREnabled       bool   `mapstructure:"ocr_enabled"`         // Enable OCR for scanned PDFs
	OpenAIOCRBaseURL string `mapstructure:"openai_ocr_base_url"` // OpenAI-compatible OCR API base URL
	OCRRenderDPI     int    `mapstructure:"ocr_render_dpi"`      // DPI for PDF rendering to images
	OCRConcurrency   int    `mapstructure:"ocr_concurrency"`     // Maximum concurrent OCR requests
	OCRTimeout       int    `mapstructure:"ocr_timeout"`         // OCR request timeout in seconds

	// Indexer Configuration (from config.yaml)
	MaxConcurrency    int  `mapstructure:"max_concurrency"`
	MaxPagesPerNode   int  `mapstructure:"max_pages_per_node"`
	MaxTokensPerNode  int  `mapstructure:"max_tokens_per_node"`
	GenerateSummaries bool `mapstructure:"generate_summaries"`
	EnableBatchCalls  bool `mapstructure:"enable_batch_calls"` // Enable batch LLM calls for summary generation
	BatchSize         int  `mapstructure:"batch_size"`         // Number of summaries per batch call
	TOCheckPageNum    int  `mapstructure:"toc_check_page_num"` // Max pages to scan for TOC detection

	// Cache Configuration (from config.yaml)
	EnableLLMCache    bool `mapstructure:"enable_llm_cache"`
	LLMCacheTTL       int  `mapstructure:"llm_cache_ttl"`       // TTL in seconds, 0 means no expiration
	EnableSearchCache bool `mapstructure:"enable_search_cache"` // Enable caching for search results

	// Performance Optimization (from config.yaml)
	SkipTOCFix          bool `mapstructure:"skip_toc_fix"`          // Skip TOC fix retry to improve performance
	SkipAppearanceCheck bool `mapstructure:"skip_appearance_check"` // Skip appearance check to improve performance

	// TOC and Content Processing (from config.yaml)
	MaxTokenNumEachNode int `mapstructure:"max_token_num_each_node"` // Max tokens per node for large node recursion

	// Logging Configuration (from config.yaml)
	LogLevel string `mapstructure:"log_level"`
}

// Load loads configuration:
// 1. First tries to read config.yaml file (required for production)
// 2. If config.yaml not found and running in test mode, uses DefaultConfig
// 3. Then loads .env file for sensitive credentials
// 4. Sensitive fields are only taken from environment variables, never from config.yaml
func Load() (*Config, error) {
	v := viper.New()

	// Check if running in test mode
	isTest := os.Getenv("PAGEINDEX_TEST") == "1" || (len(os.Args) > 0 && strings.HasSuffix(os.Args[0], ".test"))

	// --------------------------
	// Step 1: Load config.yaml (required for production)
	// --------------------------
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.pageindex")

	var cfg Config

	// Set default values from DefaultConfig for viper
	defaultCfg := DefaultConfig()
	v.SetDefault("openai_api_key", defaultCfg.OpenAIAPIKey)
	v.SetDefault("openai_base_url", defaultCfg.OpenAIBaseURL)
	v.SetDefault("openai_model", defaultCfg.OpenAIModel)
	v.SetDefault("ocr_model", defaultCfg.OCRModel)
	v.SetDefault("ocr_enabled", defaultCfg.OCREnabled)
	v.SetDefault("openai_ocr_base_url", defaultCfg.OpenAIOCRBaseURL)
	v.SetDefault("ocr_render_dpi", defaultCfg.OCRRenderDPI)
	v.SetDefault("ocr_concurrency", defaultCfg.OCRConcurrency)
	v.SetDefault("ocr_timeout", defaultCfg.OCRTimeout)
	v.SetDefault("max_concurrency", defaultCfg.MaxConcurrency)
	v.SetDefault("max_pages_per_node", defaultCfg.MaxPagesPerNode)
	v.SetDefault("max_tokens_per_node", defaultCfg.MaxTokensPerNode)
	v.SetDefault("generate_summaries", defaultCfg.GenerateSummaries)
	v.SetDefault("log_level", defaultCfg.LogLevel)
	v.SetDefault("enable_llm_cache", defaultCfg.EnableLLMCache)
	v.SetDefault("llm_cache_ttl", defaultCfg.LLMCacheTTL)
	v.SetDefault("enable_search_cache", defaultCfg.EnableSearchCache)
	v.SetDefault("enable_batch_calls", defaultCfg.EnableBatchCalls)
	v.SetDefault("batch_size", defaultCfg.BatchSize)
	v.SetDefault("toc_check_page_num", defaultCfg.TOCheckPageNum)
	v.SetDefault("max_token_num_each_node", defaultCfg.MaxTokenNumEachNode)
	v.SetDefault("skip_toc_fix", defaultCfg.SkipTOCFix)
	v.SetDefault("skip_appearance_check", defaultCfg.SkipAppearanceCheck)

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if !isTest {
				return nil, fmt.Errorf("config.yaml not found. Please create one in current directory or ~/.pageindex/")
			}
			// In test mode, use defaults + environment variables
		} else {
			return nil, fmt.Errorf("failed to read config.yaml: %w", err)
		}
	}

	// Unmarshal configuration (defaults + config file + env vars for test)
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// --------------------------
	// Step 2: Load environment variables
	// --------------------------
	_ = godotenv.Load()

	// Only load sensitive credentials from environment variables/.env
	// All other configuration must come from config.yaml
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cfg.OpenAIAPIKey = apiKey
	}
	// OCR API key for cloud OCR providers
	if apiKey := os.Getenv("OCR_API_KEY"); apiKey != "" {
		cfg.OCRAPIKey = apiKey
	}

	// --------------------------
	// Step 3: Validate configuration
	// --------------------------
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// validateConfig checks that all required configuration fields are present
func validateConfig(cfg *Config) error {
	// Validate sensitive required fields
	if cfg.OpenAIAPIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required. Please set it in .env file or environment variable")
	}

	// Validate required LLM fields
	if cfg.OpenAIBaseURL == "" {
		return fmt.Errorf("openai_base_url is required in config.yaml")
	}
	if cfg.OpenAIModel == "" {
		return fmt.Errorf("openai_model is required in config.yaml")
	}

	// Validate required indexer fields
	if cfg.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be greater than 0 in config.yaml")
	}
	if cfg.MaxPagesPerNode <= 0 {
		return fmt.Errorf("max_pages_per_node must be greater than 0 in config.yaml")
	}
	if cfg.MaxTokensPerNode <= 0 {
		return fmt.Errorf("max_tokens_per_node must be greater than 0 in config.yaml")
	}

	// Validate OCR fields if OCR is enabled
	if cfg.OCREnabled {
		if cfg.OCRModel == "" {
			return fmt.Errorf("ocr_model is required in config.yaml when ocr_enabled is true")
		}
		if cfg.OpenAIOCRBaseURL == "" {
			return fmt.Errorf("openai_ocr_base_url is required in config.yaml when ocr_enabled is true")
		}
		if cfg.OCRRenderDPI <= 0 {
			return fmt.Errorf("ocr_render_dpi must be greater than 0 in config.yaml")
		}
		if cfg.OCRConcurrency <= 0 {
			return fmt.Errorf("ocr_concurrency must be greater than 0 in config.yaml")
		}
		if cfg.OCRTimeout <= 0 {
			return fmt.Errorf("ocr_timeout must be greater than 0 in config.yaml")
		}
	}

	// Validate batch config if batch calls are enabled
	if cfg.EnableBatchCalls {
		if cfg.BatchSize <= 0 {
			return fmt.Errorf("batch_size must be greater than 0 in config.yaml when enable_batch_calls is true")
		}
	}

	// Validate cache config if cache is enabled
	if cfg.EnableLLMCache && cfg.LLMCacheTTL < 0 {
		return fmt.Errorf("llm_cache_ttl cannot be negative in config.yaml")
	}

	// Validate TOC config
	if cfg.TOCheckPageNum <= 0 {
		return fmt.Errorf("toc_check_page_num must be greater than 0 in config.yaml")
	}
	if cfg.MaxTokenNumEachNode <= 0 {
		return fmt.Errorf("max_token_num_each_node must be greater than 0 in config.yaml")
	}

	// Validate logging config
	if cfg.LogLevel == "" {
		return fmt.Errorf("log_level is required in config.yaml")
	}

	return nil
}

// LoadFromEnv loads configuration directly from environment variables.
// This is an alias for Load — kept for backward compatibility.
func LoadFromEnv() (*Config, error) {
	return Load()
}
