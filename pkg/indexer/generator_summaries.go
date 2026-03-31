package indexer

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

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

	if g.cfg.EnableBatchCalls && len(nodesToProcess) > 1 {
		return g.generateAllSummariesBatch(ctx, nodesToProcess, progressCb, startPercent, endPercent)
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

			// Pre-calculate approximate size for efficiency
			totalLen := 0
			for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
				if text, ok := g.pageTextMap[pageNum]; ok {
					totalLen += len(text) + 2
				}
			}

			var nodeText strings.Builder
			if totalLen > 0 {
				nodeText.Grow(totalLen)
			}
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

			newCount := int(completed.Add(1))
			if progressCb != nil {
				// Scale progress to the defined range
				progress := startPercent + (newCount * (endPercent - startPercent) / len(nodesToProcess))
				progressCb(progress, 100, "Generating summaries")
			}
			return nil
		})
	}

	return eg.Wait()
}

// generateAllSummariesBatch processes nodes in batches.
func (g *IndexGenerator) generateAllSummariesBatch(ctx context.Context, nodes []*document.Node, progressCb ProgressCallback, startPercent, endPercent int) error {
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
		// Pre-calculate approximate size for efficiency
		totalLen := 0
		for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
			if text, ok := g.pageTextMap[pageNum]; ok {
				totalLen += len(text) + 2
			}
		}

		var nodeText strings.Builder
		if totalLen > 0 {
			nodeText.Grow(totalLen)
		}
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

	var completedBatches atomic.Int32

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
					continue
				}
				if resp.Error != "" {
					return fmt.Errorf("failed to generate summary for node %s: %s", nwt.node.ID, resp.Error)
				}
				nwt.node.Summary = resp.Summary
			}

			completedBatches.Add(1)
			if progressCb != nil {
				progress := startPercent + (int(completedBatches.Load()) * (endPercent - startPercent) / len(batches))
				progressCb(progress, 100, "Generating summaries")
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
