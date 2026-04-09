package indexer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// nodeTextCacheEntry represents a cached entry for node text and token count
type nodeTextCacheEntry struct {
	text   string
	tokens int
}

// IndexGenerator coordinates the full index generation process.
type IndexGenerator struct {
	cfg           *config.Config
	llmClient     llm.LLMClient
	tokenizer     *tokenizer.Tokenizer
	pageGrouper   *PageGrouper
	rateLimiter   *DynamicRateLimiter // Dynamic rate limiter based on API feedback
	nodeTextCache map[string]*nodeTextCacheEntry
	cacheLock     sync.RWMutex
	opts          GeneratorOptions
}

// NewIndexGenerator creates a new IndexGenerator.
func NewIndexGenerator(cfg *config.Config, llmClient llm.LLMClient) (*IndexGenerator, error) {
	return NewIndexGeneratorWithOptions(cfg, llmClient, DefaultGeneratorOptions())
}

// NewIndexGeneratorWithOptions creates a new IndexGenerator with custom options.
// This allows for dependency injection and easier testing.
func NewIndexGeneratorWithOptions(cfg *config.Config, llmClient llm.LLMClient, opts GeneratorOptions) (*IndexGenerator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if llmClient == nil {
		return nil, fmt.Errorf("nil LLM client")
	}

	// Apply default options
	if err := opts.ApplyDefaults(cfg, llmClient); err != nil {
		return nil, fmt.Errorf("failed to apply generator options: %w", err)
	}

	// Get max tokens per group from config
	maxTokens := min(max(1, cfg.MaxTokensPerNode), 24000)
	pageGrouper := NewPageGrouper(opts.Tokenizer, maxTokens)

	gen := &IndexGenerator{
		cfg:           cfg,
		llmClient:     llmClient,
		tokenizer:     opts.Tokenizer,
		pageGrouper:   pageGrouper,
		rateLimiter:   opts.RateLimiter,
		nodeTextCache: make(map[string]*nodeTextCacheEntry),
		opts:          opts,
	}

	return gen, nil
}

// getNodeTextAndTokens returns the text content and token count for a node.
// It uses instance-level caching to avoid repeated calculations for the same node.
func (g *IndexGenerator) getNodeTextAndTokens(node *document.Node, pageTextMap map[int]string) (string, int) {
	// Generate a cache key based on node properties
	cacheKey := fmt.Sprintf("%s:%d:%d", node.ID, node.StartPage, node.EndPage)

	// Check cache first
	g.cacheLock.RLock()
	if entry, ok := g.nodeTextCache[cacheKey]; ok {
		g.cacheLock.RUnlock()
		return entry.text, entry.tokens
	}
	g.cacheLock.RUnlock()

	// Not in cache, calculate
	text := buildNodeText(node, pageTextMap)
	tokens := g.tokenizer.Count(text)

	// Store in cache if enabled
	if g.opts.EnableNodeTextCache {
		g.cacheLock.Lock()
		// Limit cache size to prevent memory leak
		if len(g.nodeTextCache) < g.opts.MaxNodeTextCacheEntries {
			g.nodeTextCache[cacheKey] = &nodeTextCacheEntry{
				text:   text,
				tokens: tokens,
			}
		}
		g.cacheLock.Unlock()
	}

	return text, tokens
}

// buildNodeText builds the text content for a node by concatenating page texts.
// It performs a single pass through the pages to both calculate size and build text.
func buildNodeText(node *document.Node, pageTextMap map[int]string) string {
	// Calculate total length for efficient allocation
	totalLen := 0
	for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
		if text, ok := pageTextMap[pageNum]; ok {
			totalLen += len(text) + 2 // +2 for "\n\n"
		}
	}

	var builder strings.Builder
	if totalLen > 0 {
		builder.Grow(totalLen)
	}

	// Build the text
	for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
		if text, ok := pageTextMap[pageNum]; ok {
			builder.WriteString(text)
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
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

	avgTokensPerNode := max(1, totalTokens/nodeCount)
	calculatedTokens := avgTokensPerNode * batchSize
	tokensPerBatch = max(minTokensPerBatch, min(calculatedTokens, maxTokensLimit))

	return batchSize, tokensPerBatch
}

// Generate generates a complete index tree from a parsed document.
// It performs the full process: grouping → parallel structure generation → merging → (optional) summary generation.
// NOTE: IndexGenerator is NOT safe for concurrent use. Each Generate/GenerateWithTOC call
// should be on a dedicated instance or called sequentially.
func (g *IndexGenerator) Generate(ctx context.Context, doc *document.Document) (*document.IndexTree, error) {
	// Clear instance-level cache for each new document
	g.nodeTextCache = make(map[string]*nodeTextCacheEntry)

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		detector := language.NewDetector()
		doc.Language = detector.DetectWithSampleSize(doc.GetFullText(), g.cfg.LanguageDetectSampleSize)
	}

	// Precompute page text map for summary generation (1-based)
	pageTextMap := make(map[int]string, len(doc.Pages))
	for i, p := range doc.Pages {
		pageNum := i + 1 // Pages are 1-based
		pageTextMap[pageNum] = p.Text
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
	nodes, err := g.generateStructures(ctx, groups, nil, doc)
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
		if err := g.generateAllSummaries(ctx, root, nil, 80, 100, doc, pageTextMap); err != nil {
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
