package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// IndexGenerator coordinates the full index generation process.
type IndexGenerator struct {
	cfg         *config.Config
	llmClient   llm.LLMClient
	tokenizer   *tokenizer.Tokenizer
	pageGrouper *PageGrouper
	doc         *document.Document  // Original document for summary generation
	pageTextMap map[int]string      // Precomputed page number to text map for summary generation
	rateLimiter *DynamicRateLimiter // Dynamic rate limiter based on API feedback
}

// NewIndexGenerator creates a new IndexGenerator.
func NewIndexGenerator(cfg *config.Config, llmClient llm.LLMClient) (*IndexGenerator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if llmClient == nil {
		return nil, fmt.Errorf("nil LLM client")
	}

	tok, err := tokenizer.NewTokenizer(cfg.OpenAIModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create tokenizer: %w", err)
	}

	// Get max tokens per group from config
	maxTokens := min(max(1, cfg.MaxTokensPerNode), 24000)

	pageGrouper := NewPageGrouper(tok, maxTokens)

	initialConcurrency := max(1, cfg.MaxConcurrency)
	minConcurrency := max(1, initialConcurrency/2)
	maxConcurrency := max(initialConcurrency, initialConcurrency*4)
	rateLimiter := NewDynamicRateLimiter(initialConcurrency, minConcurrency, maxConcurrency)

	gen := &IndexGenerator{
		cfg:         cfg,
		llmClient:   llmClient,
		tokenizer:   tok,
		pageGrouper: pageGrouper,
		rateLimiter: rateLimiter,
	}

	if openaiClient, ok := llmClient.(*llm.OpenAIClient); ok {
		openaiClient.OnRateLimitInfo = func(info llm.RateLimitInfo) {
			rateLimiter.AdjustRate(info.Remaining, info.Reset)
		}
	}

	return gen, nil
}

func calculateOptimalBatchSize(nodeCount int, totalTokens int) (batchSize int, tokensPerBatch int) {
	const (
		minBatchSize      = 5
		maxBatchSize      = 50
		minTokensPerBatch = 10000
		maxTokensLimit    = 100000
		targetBatches     = 10
	)

	if nodeCount <= minBatchSize {
		return nodeCount, maxTokensLimit
	}

	batchSize = max(minBatchSize, min(nodeCount/targetBatches, maxBatchSize))

	avgTokensPerNode := totalTokens / nodeCount
	calculatedTokens := avgTokensPerNode * batchSize
	tokensPerBatch = max(minTokensPerBatch, min(calculatedTokens, maxTokensLimit))

	return batchSize, tokensPerBatch
}

// Generate generates a complete index tree from a parsed document.
// It performs the full process: grouping → parallel structure generation → merging → (optional) summary generation.
func (g *IndexGenerator) Generate(ctx context.Context, doc *document.Document) (*document.IndexTree, error) {
	startTime := time.Now()
	log.Info().
		Int("pages", len(doc.Pages)).
		Str("language", doc.Language.Name).
		Msg("Starting index generation")

	// Store reference to original document for summary generation
	g.doc = doc

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		doc.Language = language.Detect(doc.GetFullText())
		log.Info().Str("detected_language", doc.Language.Name).Msg("Detected document language")
	}

	// Precompute page text map for summary generation
	g.pageTextMap = make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		g.pageTextMap[p.Number] = p.Text
	}

	// Step 1: Group pages
	stepStart := time.Now()
	groups, err := g.pageGrouper.GroupPages(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to group pages: %w", err)
	}
	log.Info().
		Int("groups", len(groups)).
		Dur("duration", time.Since(stepStart)).
		Msg("Page grouping complete")

	if len(groups) == 0 {
		return nil, fmt.Errorf("no content found in document")
	}

	// Step 2: Generate structure for each group in parallel
	stepStart = time.Now()
	nodes, err := g.generateStructures(ctx, groups)
	if err != nil {
		return nil, fmt.Errorf("failed to generate structures: %w", err)
	}
	log.Info().
		Int("nodes", len(nodes)).
		Dur("duration", time.Since(stepStart)).
		Msg("Structure generation complete")

	// Step 3: Merge all nodes into a single tree
	root := MergeNodes(nodes)
	if root == nil {
		return nil, fmt.Errorf("failed to merge nodes")
	}

	// Count total nodes
	totalNodes := root.CountNodes()
	log.Info().Int("total_nodes", totalNodes).Msg("Tree structure created")

	// Step 4: Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Step 5: Generate summaries if enabled
	if g.cfg.GenerateSummaries {
		stepStart = time.Now()
		log.Info().Int("nodes", totalNodes).Msg("Starting summary generation")
		if err := g.generateAllSummaries(ctx, root); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
		log.Info().
			Dur("duration", time.Since(stepStart)).
			Msg("Summary generation complete")
	}

	log.Info().
		Dur("total_duration", time.Since(startTime)).
		Int("total_nodes", totalNodes).
		Msg("Index generation complete")

	return tree, nil
}

// Update updates an existing index tree with new document content.
func (g *IndexGenerator) Update(ctx context.Context, existingTree *document.IndexTree, newDoc *document.Document) (*document.IndexTree, error) {
	if existingTree == nil {
		return nil, fmt.Errorf("nil existing index tree")
	}
	if newDoc == nil {
		return nil, fmt.Errorf("nil new document")
	}

	newTree, err := g.Generate(ctx, newDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate index for new document: %w", err)
	}

	mergedTree := existingTree.Clone()
	mergedTree.Merge(newTree)

	return mergedTree, nil
}
