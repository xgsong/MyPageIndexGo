package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/logging"
	"github.com/xgsong/mypageindexgo/pkg/output"
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
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

func generateAction(c *cli.Context) error {
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

	// Setup logging
	logger := logging.Setup(cfg.LogLevel)
	log.Logger = logger

	// Validate input
	pdfPath := c.String("pdf")
	mdPath := c.String("md")

	if pdfPath == "" && mdPath == "" {
		return fmt.Errorf("either --pdf or --md must be specified")
	}
	if pdfPath != "" && mdPath != "" {
		return fmt.Errorf("only one of --pdf or --md can be specified")
	}

	// Determine input file and parser
	var inputPath string
	var parser document.DocumentParser

	if pdfPath != "" {
		inputPath = pdfPath
		parser = document.NewPDFParser()
		log.Info().Str("file", inputPath).Msg("Parsing PDF document")
	} else {
		inputPath = mdPath
		parser = document.NewMarkdownParser()
		log.Info().Str("file", inputPath).Msg("Parsing Markdown document")
	}

	// Open and parse document
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	doc, err := parser.Parse(file)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	log.Info().Int("pages", len(doc.Pages)).Msg("Document parsed successfully")

	// Create LLM client
	llmClient := llm.NewOpenAIClient(cfg)

	// Create index generator
	generator, err := indexer.NewIndexGenerator(cfg, llmClient)
	if err != nil {
		return fmt.Errorf("failed to create index generator: %w", err)
	}

	// Generate index
	log.Info().Msg("Generating index...")
	startTime := time.Now()

	ctx := context.Background()
	tree, err := generator.Generate(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to generate index: %w", err)
	}

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
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logging
	logger := logging.Setup(cfg.LogLevel)
	log.Logger = logger

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
	llmClient := llm.NewOpenAIClient(cfg)

	// Create searcher
	searcher := indexer.NewSearcher(llmClient)

	// Perform search
	log.Info().Msg("Searching...")
	startTime := time.Now()

	ctx := context.Background()
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

// init sets up zerolog for console output
func init() {
	// Use console writer for nicer output
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: false},
	).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339
}

// printError prints a user-friendly error message
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

// printErrorAndExit prints an error and exits with code 1
func printErrorAndExit(msg string) {
	printError(msg)
	os.Exit(1)
}

// ensureSuffix ensures a string ends with the given suffix
func ensureSuffix(s, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s
	}
	return s + suffix
}
