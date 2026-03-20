package indexer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestNewSearcher(t *testing.T) {
	mockLLM := &MockLLMClient{}
	searcher := NewSearcher(mockLLM)
	assert.NotNil(t, searcher)
}

func TestSearch_ValidQuery(t *testing.T) {
	expectedResult := &document.SearchResult{
		Query:  "What is the test about?",
		Answer: "The test is about unit testing.",
		Nodes: []*document.Node{
			document.NewNode("Testing", 1, 2),
		},
	}

	mockLLM := &MockLLMClient{
		SearchFunc: func(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
			return expectedResult, nil
		},
	}

	searcher := NewSearcher(mockLLM)
	root := document.NewNode("Root", 1, 10)
	tree := document.NewIndexTree(root, 10)

	ctx := context.Background()
	result, err := searcher.Search(ctx, "What is the test about?", tree)
	assert.NoError(t, err)
	assert.Same(t, expectedResult, result)
	assert.Equal(t, "What is the test about?", result.Query)
}

func TestSearch_EmptyQuery(t *testing.T) {
	mockLLM := &MockLLMClient{}
	searcher := NewSearcher(mockLLM)
	root := document.NewNode("Root", 1, 10)
	tree := document.NewIndexTree(root, 10)

	ctx := context.Background()
	result, err := searcher.Search(ctx, "", tree)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSearch_NilTree(t *testing.T) {
	mockLLM := &MockLLMClient{}
	searcher := NewSearcher(mockLLM)

	ctx := context.Background()
	result, err := searcher.Search(ctx, "query", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSearch_NilRoot(t *testing.T) {
	mockLLM := &MockLLMClient{}
	searcher := NewSearcher(mockLLM)
	tree := &document.IndexTree{
		Root: nil,
	}

	ctx := context.Background()
	result, err := searcher.Search(ctx, "query", tree)
	assert.Error(t, err)
	assert.Nil(t, result)
}
