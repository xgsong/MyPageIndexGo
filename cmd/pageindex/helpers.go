package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// loadConfigWithCLI loads configuration and applies CLI overrides
func loadConfigWithCLI(c *cli.Context) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if c.IsSet("model") {
		cfg.OpenAIModel = c.String("model")
	}
	if c.IsSet("max-concurrency") {
		cfg.MaxConcurrency = c.Int("max-concurrency")
	}

	return cfg, nil
}

// parseDocument parses a document from CLI arguments
func parseDocument(c *cli.Context, cfg *config.Config) (*document.Document, error) {
	pdfPath := c.String("pdf")
	mdPath := c.String("md")

	if pdfPath == "" && mdPath == "" {
		return nil, fmt.Errorf("either --pdf or --md must be specified")
	}
	if pdfPath != "" && mdPath != "" {
		return nil, fmt.Errorf("only one of --pdf or --md can be specified")
	}

	// Determine input file and parser
	var inputPath string
	var parser document.DocumentParser

	if pdfPath != "" {
		inputPath = pdfPath
		// Create OCR client if OCR is enabled
		var ocrClient document.OCRClient
		if cfg.OCREnabled {
			ocrClient = llm.NewOpenAIOCRClient(cfg)
			log.Info().Str("model", cfg.OCRModel).Msg("OCR enabled for scanned PDFs")
		}
		parser = document.NewPDFParserWithOCR(ocrClient)
		log.Info().Str("file", inputPath).Msg("Parsing PDF document")
	} else {
		inputPath = mdPath
		parser = document.NewMarkdownParser()
		log.Info().Str("file", inputPath).Msg("Parsing Markdown document")
	}

	// Open and parse document
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	doc, err := parser.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	log.Info().Int("pages", len(doc.Pages)).Msg("Document parsed successfully")
	return doc, nil
}

// createLLMClient creates an LLM client with optional caching
func createLLMClient(cfg *config.Config) (llm.LLMClient, error) {
	var llmClient llm.LLMClient = llm.NewOpenAIClient(cfg)

	// Wrap with cache if enabled
	if cfg.EnableLLMCache {
		ttl := time.Duration(cfg.LLMCacheTTL) * time.Second
		llmClient = llm.NewCachedLLMClient(llmClient, ttl, cfg.EnableSearchCache)
		log.Info().Dur("ttl", ttl).Bool("search_cache", cfg.EnableSearchCache).Msg("LLM cache enabled")
	}

	return llmClient, nil
}

// createTimeoutContext creates a context with timeout for long-running operations
func createTimeoutContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}
