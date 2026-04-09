package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/logging"
	"github.com/xgsong/mypageindexgo/pkg/output"
	"github.com/xgsong/mypageindexgo/pkg/workflow"
)

func main() {
	app := &cli.App{
		Name:     "pageindex",
		Usage:    "Vectorless, reasoning-based RAG system",
		Version:  "1.0.0",
		Compiled: time.Now(),
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate index from a document (PDF or Markdown)",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "pdf",
						Usage: "Path to PDF file",
					},
					&cli.StringFlag{
						Name:  "md",
						Usage: "Path to Markdown file",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output JSON file path",
						Value:   "index.json",
					},
					&cli.StringFlag{
						Name:  "model",
						Usage: "OpenAI model to use",
						Value: "gpt-4o",
					},
					&cli.IntFlag{
						Name:  "max-concurrency",
						Usage: "Maximum concurrent LLM calls",
						Value: 5,
					},
				},
				Action: generateAction,
			},
			{
				Name:  "search",
				Usage: "Search the generated index with a query",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "index",
						Usage:    "Path to index JSON file",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "query",
						Aliases:  []string{"q"},
						Usage:    "Search query",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output JSON file for results",
					},
				},
				Action: searchAction,
			},
			{
				Name:  "update",
				Usage: "Update an existing index with new document content",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "existing",
						Usage:    "Path to existing index JSON file",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "pdf",
						Usage: "Path to new PDF file to add",
					},
					&cli.StringFlag{
						Name:  "md",
						Usage: "Path to new Markdown file to add",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output JSON file path for the merged index",
						Value:   "merged_index.json",
					},
					&cli.StringFlag{
						Name:  "model",
						Usage: "OpenAI model to use",
						Value: "gpt-4o",
					},
					&cli.IntFlag{
						Name:  "max-concurrency",
						Usage: "Maximum concurrent LLM calls",
						Value: 5,
					},
				},
				Action: updateAction,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

func generateAction(c *cli.Context) error {
	// Setup logging with default level first
	setupLogging("info")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if c.IsSet("model") {
		cfg.OpenAIModel = c.String("model")
	}
	if c.IsSet("max-concurrency") {
		cfg.MaxConcurrency = c.Int("max-concurrency")
	}

	// Validate model after CLI flag overrides
	if cfg.OpenAIModel == "" {
		return fmt.Errorf("openai_model is required: set it in config.yaml or use --model flag")
	}

	// Reconfigure logging with actual level from config
	setupLogging(cfg.LogLevel)

	// Validate input
	pdfPath := c.String("pdf")
	mdPath := c.String("md")

	if pdfPath == "" && mdPath == "" {
		return fmt.Errorf("either --pdf or --md must be specified")
	}
	if pdfPath != "" && mdPath != "" {
		return fmt.Errorf("only one of --pdf or --md can be specified")
	}

	// Determine input file
	var inputPath string
	if pdfPath != "" {
		inputPath = pdfPath
	} else {
		inputPath = mdPath
	}

	// Create document service
	svc, err := workflow.NewDocumentService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create document service: %w", err)
	}

	// Generate index
	log.Info().Msg("Generating index...")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// Progress: 5 stages, each 20%
	bar := progressbar.Default(100, "Generating index...")
	progressCallback := func(done, total int, desc string) {
		if total > 0 {
			percent := int(float64(done) / float64(total) * 100)
			bar.Set(percent)   // nolint:errcheck // Progress bar error non-critical
			bar.Describe(desc) // nolint:errcheck // Progress bar error non-critical
		}
	}
	bar.Set(5)                   // nolint:errcheck // Progress bar error non-critical
	bar.Describe("Initializing") // nolint:errcheck // Progress bar error non-critical

	// Process document using the service
	opts := workflow.DocumentServiceOptions{
		ProgressCallback: progressCallback,
	}
	tree, err := svc.ProcessDocument(ctx, inputPath, opts)
	if err != nil {
		return err
	}

	bar.Finish() // nolint:errcheck // Progress bar error non-critical

	elapsed := time.Since(startTime)

	// Save index
	outputPath := c.String("output")
	if err := output.SaveIndexTree(tree, outputPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	// Print summary
	fmt.Printf("\n✓ Index generation complete!\n")
	fmt.Printf("  • Total pages: %d\n", tree.TotalPages)
	fmt.Printf("  • Total nodes: %d\n", tree.CountAllNodes())
	fmt.Printf("  • Time elapsed: %.1fs\n", elapsed.Seconds())
	fmt.Printf("  • Output saved to: %s\n", outputPath)

	return nil
}

func searchAction(c *cli.Context) error {
	// Setup logging with default level first
	setupLogging("info")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Reconfigure logging with actual level from config
	setupLogging(cfg.LogLevel)

	// Get arguments
	indexPath := c.String("index")
	query := c.String("query")
	outputPath := c.String("output")

	log.Info().Str("index", indexPath).Str("query", query).Msg("Loading index")

	// Load index
	tree, err := output.LoadIndexTree(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	log.Info().Int("nodes", tree.CountAllNodes()).Msg("Index loaded successfully")

	// Create LLM client
	var llmClient llm.LLMClient = llm.NewOpenAIClient(cfg)

	// Wrap with cache if enabled
	if cfg.EnableLLMCache {
		ttl := time.Duration(cfg.LLMCacheTTL) * time.Second
		llmClient = llm.NewCachedLLMClient(llmClient, ttl, cfg.EnableSearchCache)
		log.Info().Dur("ttl", ttl).Bool("search_cache", cfg.EnableSearchCache).Msg("LLM cache enabled")
	}

	// Create searcher
	searcher := indexer.NewSearcher(llmClient)

	// Perform search
	log.Info().Msg("Searching...")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := searcher.Search(ctx, query, tree)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Save result if output path specified
	if outputPath != "" {
		if err := output.SaveSearchResult(result, outputPath); err != nil {
			return fmt.Errorf("failed to save search result: %w", err)
		}
		log.Info().Str("file", outputPath).Msg("Search result saved")
	}

	// Print result
	fmt.Printf("\n✓ Search complete! (elapsed: %.1fs)\n\n", elapsed.Seconds())
	fmt.Printf("Query: %s\n\n", result.Query)
	fmt.Printf("Answer:\n%s\n\n", result.Answer)

	if len(result.Nodes) > 0 {
		fmt.Printf("Referenced nodes:\n")
		for i, node := range result.Nodes {
			pageRange := formatPageRange(node.StartPage, node.EndPage)
			fmt.Printf("  %d. %s (%s)\n", i+1, node.Title, pageRange)
		}
		fmt.Println()
	}

	return nil
}

func formatPageRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("page %d", start)
	}
	return fmt.Sprintf("pages %d-%d", start, end)
}

// updateAction handles the update command for incremental index updates
func updateAction(c *cli.Context) error {
	// Setup logging with default level first
	setupLogging("info")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if c.IsSet("model") {
		cfg.OpenAIModel = c.String("model")
	}
	if c.IsSet("max-concurrency") {
		cfg.MaxConcurrency = c.Int("max-concurrency")
	}

	// Validate model after CLI flag overrides
	if cfg.OpenAIModel == "" {
		return fmt.Errorf("openai_model is required: set it in config.yaml or use --model flag")
	}

	// Reconfigure logging with actual level from config
	setupLogging(cfg.LogLevel)

	// Validate input
	existingIndexPath := c.String("existing")
	pdfPath := c.String("pdf")
	mdPath := c.String("md")

	if pdfPath == "" && mdPath == "" {
		return fmt.Errorf("either --pdf or --md must be specified for the new document")
	}
	if pdfPath != "" && mdPath != "" {
		return fmt.Errorf("only one of --pdf or --md can be specified")
	}

	// Load existing index
	log.Info().Str("index", existingIndexPath).Msg("Loading existing index")
	existingTree, err := output.LoadIndexTree(existingIndexPath)
	if err != nil {
		return fmt.Errorf("failed to load existing index: %w", err)
	}
	log.Info().
		Int("pages", existingTree.TotalPages).
		Int("nodes", existingTree.CountAllNodes()).
		Msg("Existing index loaded successfully")

	// Determine input file for the new document
	var inputPath string
	if pdfPath != "" {
		inputPath = pdfPath
	} else {
		inputPath = mdPath
	}

	// Create document service
	svc, err := workflow.NewDocumentService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create document service: %w", err)
	}

	// Parse new document
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	newDoc, err := svc.ParseDocument(ctx, inputPath)
	if err != nil {
		return err
	}

	// Generate merged index
	log.Info().Msg("Generating merged index...")
	startTime := time.Now()

	// Create index generator using the service's LLM client
	generator, err := indexer.NewIndexGenerator(cfg, svc.LLMClient())
	if err != nil {
		return fmt.Errorf("failed to create index generator: %w", err)
	}

	mergedTree, err := generator.Update(ctx, existingTree, newDoc)
	if err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	elapsed := time.Since(startTime)

	// Save merged index
	outputPath := c.String("output")
	if err := output.SaveIndexTree(mergedTree, outputPath); err != nil {
		return fmt.Errorf("failed to save merged index: %w", err)
	}

	// Print summary
	fmt.Printf("\n✓ Index update complete!\n")
	fmt.Printf("  • Original pages: %d\n", existingTree.TotalPages)
	fmt.Printf("  • New pages added: %d\n", len(newDoc.Pages))
	fmt.Printf("  • Total pages: %d\n", mergedTree.TotalPages)
	fmt.Printf("  • Total nodes: %d\n", mergedTree.CountAllNodes())
	fmt.Printf("  • Time elapsed: %.1fs\n", elapsed.Seconds())
	fmt.Printf("  • Output saved to: %s\n", outputPath)

	return nil
}

// setupLogging sets up logging with the specified level
func setupLogging(level string) {
	logger := logging.Setup(level)
	log.Logger = logger
}
