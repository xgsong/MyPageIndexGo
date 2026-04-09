// Package workflow provides shared document processing workflows used by CLI and MCP layers.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// DocumentService encapsulates the document processing workflow.
// It provides a unified interface for parsing documents and generating indices,
// eliminating code duplication between CLI and MCP layers.
type DocumentService struct {
	cfg       *config.Config
	llmClient llm.LLMClient
}

// NewDocumentService creates a new DocumentService with the given configuration.
// It initializes the LLM client and applies cache decorator if enabled.
func NewDocumentService(cfg *config.Config) (*DocumentService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	// Create base LLM client (returns llm.LLMClient interface)
	var llmClient llm.LLMClient = llm.NewOpenAIClient(cfg)

	// Wrap with cache if enabled
	if cfg.EnableLLMCache {
		ttl := time.Duration(cfg.LLMCacheTTL) * time.Second
		llmClient = llm.NewCachedLLMClient(llmClient, ttl, cfg.EnableSearchCache)
		log.Info().
			Dur("ttl", ttl).
			Bool("search_cache", cfg.EnableSearchCache).
			Msg("LLM cache enabled")
	}

	return &DocumentService{
		cfg:       cfg,
		llmClient: llmClient,
	}, nil
}

// DocumentServiceOptions contains options for document processing.
type DocumentServiceOptions struct {
	// ProgressCallback is called during index generation to report progress.
	// If nil, no progress reporting is done.
	ProgressCallback func(done, total int, description string)
}

// ParseDocument parses a document from the given file path.
// It automatically detects the file type (PDF or Markdown) and applies appropriate parsing.
// If OCR is enabled in config and the file is a PDF, OCR will be used for scanned pages.
func (s *DocumentService) ParseDocument(ctx context.Context, filePath string) (*document.Document, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not accessible: %w", err)
	}

	// Detect file type
	ext := filepath.Ext(filePath)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("file", filePath).Msg("Failed to close file")
		}
	}()

	// Create appropriate parser
	var parser document.DocumentParser
	switch ext {
	case ".pdf", ".PDF":
		parser = s.createPDFParser()
		log.Info().Str("file", filePath).Msg("Parsing PDF document")
	case ".md", ".markdown", ".MD", ".MARKDOWN":
		parser = document.NewMarkdownParser()
		log.Info().Str("file", filePath).Msg("Parsing Markdown document")
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	// Parse document
	doc, err := parser.Parse(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	log.Info().Int("pages", len(doc.Pages)).Msg("Document parsed successfully")
	return doc, nil
}

// createPDFParser creates a PDF parser with OCR support if enabled.
func (s *DocumentService) createPDFParser() document.DocumentParser {
	// Create OCR client if OCR is enabled
	var ocrClient document.OCRClient
	if s.cfg.OCREnabled {
		ocrClient = llm.NewOpenAIOCRClient(s.cfg)
		log.Info().Msg("OCR client enabled for PDF parsing")
	}

	// Create parser with options from config
	parserOptions := document.PDFParserOptions{
		OCRRenderDPI:   s.cfg.OCRRenderDPI,
		OCRConcurrency: s.cfg.OCRConcurrency,
	}

	return document.NewPDFParserWithOptions(ocrClient, parserOptions)
}

// GenerateIndex generates an index tree from the given document.
// The progress callback in options is called during generation to report progress.
func (s *DocumentService) GenerateIndex(ctx context.Context, doc *document.Document, opts DocumentServiceOptions) (*document.IndexTree, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is required")
	}

	// Create index generator
	generator, err := indexer.NewIndexGenerator(s.cfg, s.llmClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create index generator: %w", err)
	}

	// Generate index with progress callback
	// Only log if no progress callback (CLI uses progress bar instead)
	if opts.ProgressCallback == nil {
		log.Info().Msg("Generating index...")
	}
	startTime := time.Now()

	tree, err := generator.GenerateWithTOC(ctx, doc, opts.ProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to generate index: %w", err)
	}

	duration := time.Since(startTime)
	log.Info().
		Dur("duration", duration).
		Int("nodes", len(tree.Root.Children)).
		Msg("Index generation completed")

	return tree, nil
}

// ProcessDocument is a convenience method that parses a document and generates an index in one call.
// This is the most common use case for both CLI and MCP layers.
func (s *DocumentService) ProcessDocument(ctx context.Context, filePath string, opts DocumentServiceOptions) (*document.IndexTree, error) {
	// Parse document
	doc, err := s.ParseDocument(ctx, filePath)
	if err != nil {
		return nil, err
	}

	// Generate index
	return s.GenerateIndex(ctx, doc, opts)
}

// LLMClient returns the LLM client used by this service.
// This can be used for advanced use cases that need direct LLM access.
func (s *DocumentService) LLMClient() llm.LLMClient {
	return s.llmClient
}

// Config returns the configuration used by this service.
func (s *DocumentService) Config() *config.Config {
	return s.cfg
}
