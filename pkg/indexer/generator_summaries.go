package indexer

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/xgsong/mypageindexgo/internal/pool"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// generateAllSummaries generates summaries for all nodes in the tree.
func (g *IndexGenerator) generateAllSummaries(ctx context.Context, root *document.Node, progressCb ProgressCallback, startPercent, endPercent int) error { //nolint:unparam // endPercent always 100 by design
	var nodesToProcess []*document.Node
	var collect func(*document.Node)
	collect = func(node *document.Node) {
		if node == nil {
			return
		}
		// Skip root node - it represents the entire document and shouldn't have a summary
		// Root node summary would be too large and meaningless
		if node.Summary == "" && node.Title != "Document" {
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

	if g.cfg.EnableBatchCalls && len(nodesToProcess) > 1 {
		return g.generateAllSummariesBatch(ctx, nodesToProcess, progressCb, startPercent, endPercent)
	}

	eg, ctx := errgroup.WithContext(ctx)
	// Adaptive concurrency: don't exceed base concurrency or 2x CPU count
	summaryConcurrency := min(
		max(1, g.cfg.MaxConcurrency),
		runtime.NumCPU()*2,
	)
	eg.SetLimit(summaryConcurrency)

	var completed atomic.Int32

	for _, node := range nodesToProcess {
		node := node
		eg.Go(func() error {
			if err := g.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait failed: %w", err)
			}

			// Get node text and token count using cached helper
			text, _ := g.getNodeTextAndTokens(node)

			if text != "" {
				if err := g.GenerateSummariesForNode(ctx, node, text); err != nil {
					return err
				}
			}

			newCount := int(completed.Add(1))
			if progressCb != nil {
				// Use integer arithmetic for better performance
				// This provides adequate progress updates with minimal overhead
				progress := startPercent + (newCount * (endPercent - startPercent) / len(nodesToProcess))
				progressCb(progress, 100, "Generating summaries")
			}
			return nil
		})
	}

	return eg.Wait()
}

// nodeWithText represents a node with its text content and token count
type nodeWithText struct {
	node   *document.Node
	text   string
	tokens int
}

// generateAllSummariesBatch processes nodes in batches.
func (g *IndexGenerator) generateAllSummariesBatch(ctx context.Context, nodes []*document.Node, progressCb ProgressCallback, startPercent, endPercent int) error {
	batchSize := g.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	// Get safe token limit based on the model
	maxTotalTokens := llm.GetSafeBatchTokenLimit(g.cfg.OpenAIModel)
	if maxTotalTokens <= 0 {
		// Fallback to conservative limit
		maxTotalTokens = 30000
	}

	// Also respect the configured max tokens per node
	if g.cfg.MaxTokensPerNode > 0 && g.cfg.MaxTokensPerNode*3 < maxTotalTokens {
		maxTotalTokens = g.cfg.MaxTokensPerNode * 3
	}

	totalNodes := len(nodes)
	var nodesWithText []nodeWithText
	for _, node := range nodes {
		// Get node text and token count using cached helper
		text, tokens := g.getNodeTextAndTokens(node)
		
		nodesWithText = append(nodesWithText, nodeWithText{
			node:   node,
			text:   text,
			tokens: tokens,
		})
	}

	eg, ctx := errgroup.WithContext(ctx)
	// Adaptive concurrency: don't exceed base concurrency or 2x CPU count
	summaryConcurrency := min(
		max(1, g.cfg.MaxConcurrency),
		runtime.NumCPU()*2,
	)
	eg.SetLimit(summaryConcurrency)

	// Create batches using improved bin-packing algorithm
	batches := g.createBatchesWithSmartPacking(nodesWithText, batchSize, maxTotalTokens)

	var completedNodes atomic.Int32

	for _, batch := range batches {
		batch := batch
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
				} else if resp.Error != "" {
					return fmt.Errorf("failed to generate summary for node %s: %s", nwt.node.ID, resp.Error)
				} else {
					nwt.node.Summary = resp.Summary
				}

				// Update progress after each node is processed
				newCount := int(completedNodes.Add(1))
				if progressCb != nil {
					progress := startPercent + (newCount * (endPercent - startPercent) / totalNodes)
					progressCb(progress, 100, "Generating summaries")
				}
			}

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

// nodeTextCacheEntry represents a cached entry for node text and token count
type nodeTextCacheEntry struct {
	text   string
	tokens int
}

// nodeTextCache is a simple cache for node text to avoid repeated calculations
var (
	nodeTextCache     = make(map[string]*nodeTextCacheEntry)
	nodeTextCacheLock sync.RWMutex
)

// getNodeTextAndTokens returns the text content and token count for a node.
// It uses caching to avoid repeated calculations for the same node.
func (g *IndexGenerator) getNodeTextAndTokens(node *document.Node) (string, int) {
	// Generate a cache key based on node properties
	cacheKey := fmt.Sprintf("%s:%d:%d", node.ID, node.StartPage, node.EndPage)
	
	// Check cache first
	nodeTextCacheLock.RLock()
	if entry, ok := nodeTextCache[cacheKey]; ok {
		nodeTextCacheLock.RUnlock()
		return entry.text, entry.tokens
	}
	nodeTextCacheLock.RUnlock()
	
	// Not in cache, calculate
	text := g.buildNodeText(node)
	tokens := g.tokenizer.Count(text)
	
	// Store in cache
	nodeTextCacheLock.Lock()
	// Limit cache size to prevent memory leak
	if len(nodeTextCache) < 1000 {
		nodeTextCache[cacheKey] = &nodeTextCacheEntry{
			text:   text,
			tokens: tokens,
		}
	}
	nodeTextCacheLock.Unlock()
	
	return text, tokens
}

// buildNodeText builds the text content for a node by concatenating page texts.
// It performs a single pass through the pages to both calculate size and build text.
func (g *IndexGenerator) buildNodeText(node *document.Node) string {
	// Use pooled builder to reduce allocations
	builder := pool.GetBuilder()
	defer pool.PutBuilder(builder)
	
	// Calculate total length and build text in a single pass
	// First, estimate total length for efficient allocation
	totalLen := 0
	for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
		if text, ok := g.pageTextMap[pageNum]; ok {
			totalLen += len(text) + 2 // +2 for "\n\n"
		}
	}
	
	if totalLen > 0 {
		builder.Grow(totalLen)
	}
	
	// Build the text
	for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
		if text, ok := g.pageTextMap[pageNum]; ok {
			builder.WriteString(text)
			builder.WriteString("\n\n")
		}
	}
	
	return builder.String()
}

// createBatchesWithSmartPacking creates batches using an improved bin-packing algorithm
// that handles large nodes and respects token limits more accurately.
func (g *IndexGenerator) createBatchesWithSmartPacking(nodesWithText []nodeWithText, batchSize, maxTotalTokens int) [][]nodeWithText {
	var batches [][]nodeWithText
	
	// Separate nodes into categories based on size
	var hugeNodes []nodeWithText    // > 80% of limit
	var largeNodes []nodeWithText   // 50-80% of limit
	var mediumNodes []nodeWithText  // 20-50% of limit
	var smallNodes []nodeWithText   // < 20% of limit
	
	for _, nwt := range nodesWithText {
		if nwt.tokens > maxTotalTokens*80/100 {
			// Huge node that exceeds 80% of limit - needs special handling
			hugeNodes = append(hugeNodes, nwt)
		} else if nwt.tokens > maxTotalTokens/2 {
			// Large node that needs its own batch
			largeNodes = append(largeNodes, nwt)
		} else if nwt.tokens > maxTotalTokens/5 {
			// Medium node
			mediumNodes = append(mediumNodes, nwt)
		} else {
			// Small node
			smallNodes = append(smallNodes, nwt)
		}
	}
	
	// Process huge nodes - they need to be split or handled individually
	for _, nwt := range hugeNodes {
		// For now, put each huge node in its own batch
		// In the future, we could split the text into chunks
		batches = append(batches, []nodeWithText{nwt})
	}
	
	// Process large nodes - each gets its own batch
	for _, nwt := range largeNodes {
		batches = append(batches, []nodeWithText{nwt})
	}
	
	// Process medium and small nodes using improved bin-packing
	allNormalNodes := append(mediumNodes, smallNodes...)
	if len(allNormalNodes) > 0 {
		// Sort nodes by token count (descending) for better packing
		sortedNodes := make([]nodeWithText, len(allNormalNodes))
		copy(sortedNodes, allNormalNodes)
		// Simple bubble sort since n is small
		for i := 0; i < len(sortedNodes); i++ {
			for j := i + 1; j < len(sortedNodes); j++ {
				if sortedNodes[i].tokens < sortedNodes[j].tokens {
					sortedNodes[i], sortedNodes[j] = sortedNodes[j], sortedNodes[i]
				}
			}
		}
		
		// First Fit Decreasing packing with dynamic batch limits
		for _, nwt := range sortedNodes {
			placed := false
			for i := range batches {
				// Skip batches that already have a huge or large node
				if len(batches[i]) == 1 {
					firstNodeTokens := batches[i][0].tokens
					if firstNodeTokens > maxTotalTokens/2 {
						// This batch already has a large or huge node
						continue
					}
				}
				
				// Calculate current batch tokens with overhead
				batchTokens := 0
				for _, batchItem := range batches[i] {
					batchTokens += batchItem.tokens
				}
				// Account for JSON overhead per request (~200 tokens)
				jsonOverhead := len(batches[i]) * 200
				totalBatchTokens := batchTokens + jsonOverhead
				
				// Calculate if we can add this node
				// Use 90% of limit for safety margin
				safeLimit := maxTotalTokens * 90 / 100
				if len(batches[i]) < batchSize && totalBatchTokens+nwt.tokens+200 <= safeLimit {
					batches[i] = append(batches[i], nwt)
					placed = true
					break
				}
			}
			if !placed {
				batches = append(batches, []nodeWithText{nwt})
			}
		}
	}
	
	// Optimize batches by trying to merge small batches
	g.optimizeBatches(&batches, maxTotalTokens)
	
	return batches
}

// optimizeBatches tries to merge small batches to reduce API calls
func (g *IndexGenerator) optimizeBatches(batches *[][]nodeWithText, maxTotalTokens int) {
	// Try to merge small batches
	for i := 0; i < len(*batches); i++ {
		for j := i + 1; j < len(*batches); j++ {
			// Check if both batches are small and can be merged
			if len((*batches)[i]) + len((*batches)[j]) <= 5 { // Only merge very small batches
				// Calculate total tokens if merged
				totalTokens := 0
				for _, nwt := range (*batches)[i] {
					totalTokens += nwt.tokens
				}
				for _, nwt := range (*batches)[j] {
					totalTokens += nwt.tokens
				}
				// Account for JSON overhead
				totalNodes := len((*batches)[i]) + len((*batches)[j])
				jsonOverhead := totalNodes * 200
				totalWithOverhead := totalTokens + jsonOverhead
				
				// Check if merged batch would fit within limit
				if totalWithOverhead <= maxTotalTokens*80/100 { // Use 80% for safety
					// Merge batch j into batch i
					(*batches)[i] = append((*batches)[i], (*batches)[j]...)
					// Remove batch j
					*batches = append((*batches)[:j], (*batches)[j+1:]...)
					j-- // Adjust index since we removed an element
				}
			}
		}
	}
}
