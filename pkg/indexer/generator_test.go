package indexer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// MockLLMClient is a mock implementation of LLMClient for testing.
var _ llm.LLMClient = (*MockLLMClient)(nil)

type MockLLMClient struct {
	GenerateStructureFunc      func(ctx context.Context, text string, lang language.Language) (*document.Node, error)
	GenerateSummaryFunc        func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)
	SearchFunc                 func(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
	GenerateBatchSummariesFunc func(ctx context.Context, requests []*llm.BatchSummaryRequest, lang language.Language) ([]*llm.BatchSummaryResponse, error)
}

func (m *MockLLMClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	if m.GenerateStructureFunc != nil {
		return m.GenerateStructureFunc(ctx, text, lang)
	}
	return document.NewNode("Root", 1, 1), nil
}

func (m *MockLLMClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	return m.GenerateSummaryFunc(ctx, nodeTitle, text, lang)
}

func (m *MockLLMClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	return m.SearchFunc(ctx, query, tree)
}

func (m *MockLLMClient) GenerateBatchSummaries(ctx context.Context, requests []*llm.BatchSummaryRequest, lang language.Language) ([]*llm.BatchSummaryResponse, error) {
	if m.GenerateBatchSummariesFunc != nil {
		return m.GenerateBatchSummariesFunc(ctx, requests, lang)
	}
	// Default implementation: fallback to individual calls for backward compatibility
	responses := make([]*llm.BatchSummaryResponse, len(requests))
	for i, req := range requests {
		summary, err := m.GenerateSummary(ctx, req.NodeTitle, req.Text, lang)
		if err != nil {
			responses[i] = &llm.BatchSummaryResponse{
				NodeID: req.NodeID,
				Error:  err.Error(),
			}
		} else {
			responses[i] = &llm.BatchSummaryResponse{
				NodeID:  req.NodeID,
				Summary: summary,
			}
		}
	}
	return responses, nil
}

func TestNewIndexGenerator(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)
	assert.NotNil(t, gen)
	assert.NotNil(t, gen.tokenizer)
	assert.NotNil(t, gen.pageGrouper)
}

func TestGenerate_SingleGroup(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{
		GenerateStructureFunc: func(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
			root := document.NewNode("Root", 1, 2)
			root.AddChild(document.NewNode("Section 1", 1, 1))
			root.AddChild(document.NewNode("Section 2", 2, 2))
			return root, nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 2, Text: "Page 2 content"},
		},
	}

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.Root)
	assert.Equal(t, 2, tree.TotalPages)
	// Should have the mock root's children
	assert.Len(t, tree.Root.Children, 2)
}

func TestGenerate_MultipleGroups(t *testing.T) {
	cfg := config.DefaultConfig()
	// Lower the max tokens to force multiple groups
	cfg.MaxTokensPerNode = 20

	var mu sync.Mutex
	callCount := 0
	mockLLM := &MockLLMClient{
		GenerateStructureFunc: func(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			// Return a root with children for each group
			root := document.NewNode(fmt.Sprintf("Group %d", callCount), 1, 10)
			root.AddChild(document.NewNode(fmt.Sprintf("Section %d", callCount), 1, 5))
			return root, nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Each page has ~10 tokens, so two pages fit in 20 tokens max
	// Using 6 pages to ensure multiple groups (more than overlap*2=4)
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "This is page one with several words here"},
			{Number: 2, Text: "This is page two with several words here"},
			{Number: 3, Text: "This is page three with several words here too"},
			{Number: 4, Text: "This is page four with several words here also"},
			{Number: 5, Text: "This is page five with several words here"},
			{Number: 6, Text: "This is page six with several words here"},
		},
	}

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, 6, tree.TotalPages)
	// Should have multiple groups merged - at least 2 children at root level
	assert.GreaterOrEqual(t, len(tree.Root.Children), 2)
	// Verify that the structure generation was called multiple times
	assert.GreaterOrEqual(t, callCount, 2)
}

func TestGenerate_EmptyDocument(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	doc := &document.Document{
		Pages: []document.Page{},
	}

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.Error(t, err)
	assert.Nil(t, tree)
}

func TestGenerateSummariesForNode(t *testing.T) {
	cfg := config.DefaultConfig()
	expectedSummary := "This is a summary of the node content."

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			return expectedSummary, nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	node := document.NewNode("Test", 1, 10)
	text := "This is the node content that needs a summary."

	ctx := context.Background()
	err = gen.GenerateSummariesForNode(ctx, node, text)
	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, node.Summary)
}

func TestGenerateSummariesForNode_EmptyText(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	node := document.NewNode("Test", 1, 10)

	ctx := context.Background()
	err = gen.GenerateSummariesForNode(ctx, node, "")
	assert.NoError(t, err)
	// No error, just leaves summary empty
	assert.Equal(t, "", node.Summary)
}

func TestGenerateSummariesForNode_NilNode(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	ctx := context.Background()
	err = gen.GenerateSummariesForNode(ctx, nil, "text")
	assert.Error(t, err)
}

func TestGenerateSummariesForNode_LLMError(t *testing.T) {
	cfg := config.DefaultConfig()
	expectedErr := fmt.Errorf("LLM service unavailable")

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			return "", expectedErr
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	node := document.NewNode("Test", 1, 10)
	text := "This is the node content that needs a summary."

	ctx := context.Background()
	err = gen.GenerateSummariesForNode(ctx, node, text)
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "failed to generate summary")
	// Summary should remain empty
	assert.Equal(t, "", node.Summary)
}

func TestGenerateAllSummaries_NoSummariesNeeded(t *testing.T) {
	cfg := config.DefaultConfig()
	mockLLM := &MockLLMClient{}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Create a tree where all nodes already have summaries
	root := document.NewNode("Root", 1, 10)
	root.Summary = "Root summary"
	child1 := document.NewNode("Child 1", 1, 5)
	child1.Summary = "Child 1 summary"
	child2 := document.NewNode("Child 2", 6, 10)
	child2.Summary = "Child 2 summary"
	root.AddChild(child1)
	root.AddChild(child2)

	// Set the document and precompute pageTextMap in the generator
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 2, Text: "Page 2 content"},
			{Number: 3, Text: "Page 3 content"},
			{Number: 4, Text: "Page 4 content"},
			{Number: 5, Text: "Page 5 content"},
			{Number: 6, Text: "Page 6 content"},
			{Number: 7, Text: "Page 7 content"},
			{Number: 8, Text: "Page 8 content"},
			{Number: 9, Text: "Page 9 content"},
			{Number: 10, Text: "Page 10 content"},
		},
	}
	gen.doc = doc
	// Precompute pageTextMap like Generate method does
	gen.pageTextMap = make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		gen.pageTextMap[p.Number] = p.Text
	}

	ctx := context.Background()
	err = gen.generateAllSummaries(ctx, root)
	assert.NoError(t, err)

	// Summaries should remain unchanged
	assert.Equal(t, "Root summary", root.Summary)
	assert.Equal(t, "Child 1 summary", child1.Summary)
	assert.Equal(t, "Child 2 summary", child2.Summary)
}

func TestGenerateAllSummaries_GenerateAll(t *testing.T) {
	cfg := config.DefaultConfig()
	summaryMap := map[string]string{
		"Root":    "Root document summary",
		"Child 1": "Child 1 section summary",
		"Child 2": "Child 2 section summary",
		"Section": "Section summary",
	}

	var mu sync.Mutex
	callCount := 0

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			return summaryMap[nodeTitle], nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Create a tree with no summaries
	root := document.NewNode("Root", 1, 10)
	child1 := document.NewNode("Child 1", 1, 5)
	child1.AddChild(document.NewNode("Section", 1, 2))
	child2 := document.NewNode("Child 2", 6, 10)
	root.AddChild(child1)
	root.AddChild(child2)

	// Set the document and precompute pageTextMap in the generator
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 2, Text: "Page 2 content"},
			{Number: 3, Text: "Page 3 content"},
			{Number: 4, Text: "Page 4 content"},
			{Number: 5, Text: "Page 5 content"},
			{Number: 6, Text: "Page 6 content"},
			{Number: 7, Text: "Page 7 content"},
			{Number: 8, Text: "Page 8 content"},
			{Number: 9, Text: "Page 9 content"},
			{Number: 10, Text: "Page 10 content"},
		},
	}
	gen.doc = doc
	// Precompute pageTextMap like Generate method does
	gen.pageTextMap = make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		gen.pageTextMap[p.Number] = p.Text
	}

	ctx := context.Background()
	err = gen.generateAllSummaries(ctx, root)
	assert.NoError(t, err)

	// Should have called GenerateSummary 4 times for all 4 nodes
	assert.Equal(t, 4, callCount)

	// All nodes should have summaries
	assert.Equal(t, "Root document summary", root.Summary)
	assert.Equal(t, "Child 1 section summary", child1.Summary)
	assert.Equal(t, "Child 2 section summary", child2.Summary)
	assert.Equal(t, "Section summary", child1.Children[0].Summary)
}

func TestGenerateAllSummaries_MissingPages(t *testing.T) {
	cfg := config.DefaultConfig()
	expectedSummary := "Node with missing pages summary"

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			// Should only get text for existing pages
			assert.Contains(t, text, "Page 1 content")
			assert.Contains(t, text, "Page 3 content")
			assert.NotContains(t, text, "Page 2") // Page 2 is missing
			return expectedSummary, nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Create node that spans pages 1-3, but page 2 is missing
	node := document.NewNode("Test Node", 1, 3)

	// Set the document and precompute pageTextMap with missing page 2
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 3, Text: "Page 3 content"},
		},
	}
	gen.doc = doc
	// Precompute pageTextMap like Generate method does
	gen.pageTextMap = make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		gen.pageTextMap[p.Number] = p.Text
	}

	ctx := context.Background()
	err = gen.generateAllSummaries(ctx, node)
	assert.NoError(t, err)

	assert.Equal(t, expectedSummary, node.Summary)
}

func TestGenerateAllSummaries_EmptyText(t *testing.T) {
	cfg := config.DefaultConfig()
	summaryCallCount := 0

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			summaryCallCount++
			// Text should contain the newlines even if page content is empty
			assert.Equal(t, "\n\n\n\n", text)
			return "summary", nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Create node that spans pages with no text
	node := document.NewNode("Empty Node", 1, 2)

	// Set the document and precompute pageTextMap with empty pages
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: ""},
			{Number: 2, Text: ""},
		},
	}
	gen.doc = doc
	// Precompute pageTextMap like Generate method does
	gen.pageTextMap = make(map[int]string, len(doc.Pages))
	for _, p := range doc.Pages {
		gen.pageTextMap[p.Number] = p.Text
	}

	ctx := context.Background()
	err = gen.generateAllSummaries(ctx, node)
	assert.NoError(t, err)

	// Should call GenerateSummary even with empty page text because of added newlines
	assert.Equal(t, 1, summaryCallCount)
	assert.Equal(t, "summary", node.Summary)
}

func TestPageTextMapReuse(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GenerateSummaries = true

	var generateStructureCalls int
	var generateSummaryCalls int
	var mu sync.Mutex

	mockLLM := &MockLLMClient{
		GenerateStructureFunc: func(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
			mu.Lock()
			defer mu.Unlock()
			generateStructureCalls++
			root := document.NewNode("Root", 1, 2)
			root.AddChild(document.NewNode("Section 1", 1, 1))
			root.AddChild(document.NewNode("Section 2", 2, 2))
			return root, nil
		},
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			generateSummaryCalls++
			return fmt.Sprintf("Summary for %s", nodeTitle), nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 2, Text: "Page 2 content"},
		},
	}

	// Before Generate, pageTextMap should be nil
	assert.Nil(t, gen.pageTextMap)

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, tree)

	// After Generate, pageTextMap should be populated
	assert.NotNil(t, gen.pageTextMap)
	assert.Equal(t, 2, len(gen.pageTextMap))
	assert.Equal(t, "Page 1 content", gen.pageTextMap[1])
	assert.Equal(t, "Page 2 content", gen.pageTextMap[2])

	// Should have called both structure generation and summary generation
	assert.Equal(t, 1, generateStructureCalls)
	// 3 nodes total: Root + 2 sections
	assert.Equal(t, 3, generateSummaryCalls)

	// All nodes should have summaries
	assert.NotEmpty(t, tree.Root.Summary)
	assert.NotEmpty(t, tree.Root.Children[0].Summary)
	assert.NotEmpty(t, tree.Root.Children[1].Summary)
}

func TestSummaryGeneration_ConcurrentLimit(t *testing.T) {
	// Test that summary generation uses 2x the configured concurrency
	cfg := config.DefaultConfig()
	cfg.MaxConcurrency = 2 // Base concurrency is 2, so summary should use 4
	cfg.GenerateSummaries = true

	var mu sync.Mutex
	activeGoroutines := 0
	maxActiveGoroutines := 0
	// We need enough nodes to test concurrency limits
	numNodes := 10

	mockLLM := &MockLLMClient{
		GenerateStructureFunc: func(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
			// Create a root with many child nodes to test summary generation concurrency
			root := document.NewNode("Root", 1, numNodes)
			for i := 1; i <= numNodes; i++ {
				child := document.NewNode(fmt.Sprintf("Section %d", i), i, i)
				root.AddChild(child)
			}
			return root, nil
		},
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
			mu.Lock()
			activeGoroutines++
			if activeGoroutines > maxActiveGoroutines {
				maxActiveGoroutines = activeGoroutines
			}
			mu.Unlock()

			// Simulate some processing time
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}

			mu.Lock()
			activeGoroutines--
			mu.Unlock()

			return fmt.Sprintf("Summary for %s", nodeTitle), nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Create a document with enough pages
	var pages []document.Page
	for i := 1; i <= numNodes; i++ {
		pages = append(pages, document.Page{Number: i, Text: fmt.Sprintf("Page %d content", i)})
	}
	doc := &document.Document{Pages: pages}

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, tree)

	// Max active goroutines should be <= 4 (2x base concurrency of 2)
	// It might be slightly less due to scheduling, but should not exceed 4
	assert.LessOrEqual(t, maxActiveGoroutines, 4)
	// With dynamic rate limiting enabled, concurrency is controlled by rate limiter
	// It should be at least 1, showing that concurrency is working
	assert.GreaterOrEqual(t, maxActiveGoroutines, 1)
}
