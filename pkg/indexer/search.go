package indexer

import (
	"context"
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// Searcher handles reasoning-based search on an existing index tree.
type Searcher struct {
	llmClient llm.LLMClient
}

// NewSearcher creates a new Searcher.
func NewSearcher(llmClient llm.LLMClient) *Searcher {
	return &Searcher{
		llmClient: llmClient,
	}
}

// Search performs a reasoning-based search on the index tree given a query.
// It uses the LLM to identify relevant nodes and generate a comprehensive answer.
func (s *Searcher) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	if tree == nil || tree.Root == nil {
		return nil, fmt.Errorf("invalid index tree")
	}

	result, err := s.llmClient.Search(ctx, query, tree)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return result, nil
}
