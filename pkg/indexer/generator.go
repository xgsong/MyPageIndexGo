package indexer

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

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
	maxTokens := cfg.MaxTokensPerNode
	if maxTokens <= 0 {
		maxTokens = 24000
	}

	pageGrouper := NewPageGrouper(tok, maxTokens)

	// Initialize dynamic rate limiter
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

	// Set rate limit callback if this is an OpenAI client
	if openaiClient, ok := llmClient.(*llm.OpenAIClient); ok {
		openaiClient.OnRateLimitInfo = func(info llm.RateLimitInfo) {
			rateLimiter.AdjustRate(info.Remaining, info.Reset)
		}
	}

	return gen, nil
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

// generateStructures generates the tree structure for each page group in parallel.
func (g *IndexGenerator) generateStructures(ctx context.Context, groups []*PageGroup) ([]*document.Node, error) {
	startTime := time.Now()
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(g.cfg.MaxConcurrency)

	nodes := make([]*document.Node, len(groups))
	var completed atomic.Int32

	for i, group := range groups {
		i := i
		group := group
		eg.Go(func() error {
			if err := g.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait failed: %w", err)
			}

			node, err := g.llmClient.GenerateStructure(ctx, group.Text, g.doc.Language)
			if err != nil {
				return fmt.Errorf("group %d (%d-%d): failed to generate structure: %w", i+1, group.StartPage, group.EndPage, err)
			}
			nodes[i] = node

			newCount := completed.Add(1)
			if newCount%5 == 0 || int(newCount) == len(groups) {
				log.Info().
					Int32("completed", newCount).
					Int("total", len(groups)).
					Dur("elapsed", time.Since(startTime)).
					Msg("Structure generation progress")
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// generateAllSummaries generates summaries for all nodes in the tree.
func (g *IndexGenerator) generateAllSummaries(ctx context.Context, root *document.Node) error {
	startTime := time.Now()

	var nodesToProcess []*document.Node
	var collect func(*document.Node)
	collect = func(node *document.Node) {
		if node == nil {
			return
		}
		if node.Summary == "" {
			nodesToProcess = append(nodesToProcess, node)
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(root)

	if len(nodesToProcess) == 0 {
		return nil
	}

	log.Info().Int("nodes_to_summarize", len(nodesToProcess)).Msg("Starting summary generation")

	if g.cfg.EnableBatchCalls && len(nodesToProcess) > 1 {
		return g.generateAllSummariesBatch(ctx, nodesToProcess)
	}

	eg, ctx := errgroup.WithContext(ctx)
	summaryConcurrency := max(1, g.cfg.MaxConcurrency*2)
	eg.SetLimit(summaryConcurrency)

	var completed atomic.Int32

	for _, node := range nodesToProcess {
		node := node
		eg.Go(func() error {
			if err := g.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait failed: %w", err)
			}

			var nodeText strings.Builder
			for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
				if text, ok := g.pageTextMap[pageNum]; ok {
					nodeText.WriteString(text)
					nodeText.WriteString("\n\n")
				}
			}

			if nodeText.Len() > 0 {
				if err := g.GenerateSummariesForNode(ctx, node, nodeText.String()); err != nil {
					return err
				}
			}

			newCount := completed.Add(1)
			if newCount%10 == 0 || int(newCount) == len(nodesToProcess) {
				log.Info().
					Int32("completed", newCount).
					Int("total", len(nodesToProcess)).
					Dur("elapsed", time.Since(startTime)).
					Msg("Summary generation progress")
			}
			return nil
		})
	}

	return eg.Wait()
}

// generateAllSummariesBatch processes nodes in batches.
func (g *IndexGenerator) generateAllSummariesBatch(ctx context.Context, nodes []*document.Node) error {
	startTime := time.Now()
	batchSize := g.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	maxTotalTokens := 100000
	if g.cfg.MaxTokensPerNode > 0 {
		maxTotalTokens = g.cfg.MaxTokensPerNode * 6
	}

	type nodeWithText struct {
		node   *document.Node
		text   string
		tokens int
	}
	var nodesWithText []nodeWithText
	for _, node := range nodes {
		var nodeText strings.Builder
		for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
			if text, ok := g.pageTextMap[pageNum]; ok {
				nodeText.WriteString(text)
				nodeText.WriteString("\n\n")
			}
		}
		text := nodeText.String()
		tokens := g.tokenizer.Count(text)
		nodesWithText = append(nodesWithText, nodeWithText{
			node:   node,
			text:   text,
			tokens: tokens,
		})
	}

	eg, ctx := errgroup.WithContext(ctx)
	summaryConcurrency := max(1, g.cfg.MaxConcurrency*2)
	eg.SetLimit(summaryConcurrency)

	var batches [][]nodeWithText
	var currentBatch []nodeWithText
	currentTotalTokens := 0

	for _, nwt := range nodesWithText {
		if len(currentBatch) >= batchSize || (currentTotalTokens+nwt.tokens) > maxTotalTokens {
			if len(currentBatch) > 0 {
				batches = append(batches, currentBatch)
				currentBatch = nil
				currentTotalTokens = 0
			}
		}
		currentBatch = append(currentBatch, nwt)
		currentTotalTokens += nwt.tokens
	}
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	log.Info().
		Int("batches", len(batches)).
		Int("batch_size", batchSize).
		Msg("Summary batching configuration")

	completedBatches := 0

	for _, batch := range batches {
		eg.Go(func() error {
			if err := g.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait failed: %w", err)
			}

			var requests []*llm.BatchSummaryRequest
			for _, nwt := range batch {
				if nwt.text == "" {
					continue
				}
				requests = append(requests, &llm.BatchSummaryRequest{
					NodeID:    nwt.node.ID,
					NodeTitle: nwt.node.Title,
					Text:      nwt.text,
				})
			}

			if len(requests) == 0 {
				return nil
			}

			responses, err := g.llmClient.GenerateBatchSummaries(ctx, requests, g.doc.Language)
			if err != nil {
				return fmt.Errorf("batch summary generation failed: %w", err)
			}

			responseMap := make(map[string]*llm.BatchSummaryResponse)
			for _, resp := range responses {
				responseMap[resp.NodeID] = resp
			}

			for _, nwt := range batch {
				if nwt.text == "" {
					continue
				}
				resp, ok := responseMap[nwt.node.ID]
				if !ok {
					summary, err := g.llmClient.GenerateSummary(ctx, nwt.node.Title, nwt.text, g.doc.Language)
					if err != nil {
						return fmt.Errorf("failed to generate summary for node %s: %w", nwt.node.ID, err)
					}
					nwt.node.Summary = summary
					continue
				}
				if resp.Error != "" {
					return fmt.Errorf("failed to generate summary for node %s: %s", nwt.node.ID, resp.Error)
				}
				nwt.node.Summary = resp.Summary
			}

			completedBatches++
			log.Info().
				Int("completed", completedBatches).
				Int("total", len(batches)).
				Dur("elapsed", time.Since(startTime)).
				Msg("Batch summary progress")

			return nil
		})
	}

	return eg.Wait()
}

// GenerateSummariesForNode generates a summary for a specific node.
func (g *IndexGenerator) GenerateSummariesForNode(ctx context.Context, node *document.Node, text string) error {
	if node == nil {
		return fmt.Errorf("nil node")
	}
	if text == "" {
		return nil
	}

	// Use document language if available, otherwise detect from text
	lang := language.LanguageEnglish
	if g.doc != nil && g.doc.Language.Code != "" {
		lang = g.doc.Language
	}

	summary, err := g.llmClient.GenerateSummary(ctx, node.Title, text, lang)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	node.Summary = summary
	return nil
}
