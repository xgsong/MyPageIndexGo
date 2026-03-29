package indexer

import (
	"context"
	"fmt"

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
	// Store reference to original document for summary generation
	g.doc = doc

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		doc.Language = language.Detect(doc.GetFullText())
	}

	// Precompute page text map for summary generation (1-based)
	g.pageTextMap = make(map[int]string, len(doc.Pages))
	for i, p := range doc.Pages {
		pageNum := i + 1 // Pages are 1-based
		g.pageTextMap[pageNum] = p.Text
	}

	// Step 1: Group pages
	groups, err := g.pageGrouper.GroupPages(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to group pages: %w", err)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no content found in document")
	}

	// Step 2: Generate structure for each group in parallel
	nodes, err := g.generateStructures(ctx, groups)
	if err != nil {
		return nil, fmt.Errorf("failed to generate structures: %w", err)
	}

	// Step 3: Merge all nodes into a single tree
	root := MergeNodes(nodes)
	if root == nil {
		return nil, fmt.Errorf("failed to merge nodes")
	}

	// Count total nodes
	root.CountNodes()

	// Step 4: Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Step 5: Generate summaries if enabled
	if g.cfg.GenerateSummaries {
		if err := g.generateAllSummaries(ctx, root); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
	}

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
