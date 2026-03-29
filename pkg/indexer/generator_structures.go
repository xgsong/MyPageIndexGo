package indexer

import (
	"context"
	"fmt"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// generateStructures generates the tree structure for each page group in parallel.
func (g *IndexGenerator) generateStructures(ctx context.Context, groups []*PageGroup) ([]*document.Node, error) {
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

			completed.Add(1)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return nodes, nil
}
