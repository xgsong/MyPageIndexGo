package indexer

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// MockLLMClient is a mock implementation of LLMClient for testing.
var _ llm.LLMClient = (*MockLLMClient)(nil)

type MockLLMClient struct {
	GenerateStructureFunc func(ctx context.Context, text string) (*document.Node, error)
	GenerateSummaryFunc   func(ctx context.Context, nodeTitle string, text string) (string, error)
	SearchFunc            func(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
}

func (m *MockLLMClient) GenerateStructure(ctx context.Context, text string) (*document.Node, error) {
	return m.GenerateStructureFunc(ctx, text)
}

func (m *MockLLMClient) GenerateSummary(ctx context.Context, nodeTitle string, text string) (string, error) {
	return m.GenerateSummaryFunc(ctx, nodeTitle, text)
}

func (m *MockLLMClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	return m.SearchFunc(ctx, query, tree)
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
		GenerateStructureFunc: func(ctx context.Context, text string) (*document.Node, error) {
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
		GenerateStructureFunc: func(ctx context.Context, text string) (*document.Node, error) {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			if callCount == 1 {
				root := document.NewNode("Group 1", 1, 2)
				root.AddChild(document.NewNode("Section 1", 1, 1))
				return root, nil
			}
			root := document.NewNode("Group 2", 3, 4)
			root.AddChild(document.NewNode("Section 2", 3, 3))
			return root, nil
		},
	}

	gen, err := NewIndexGenerator(cfg, mockLLM)
	assert.NoError(t, err)

	// Each page has ~10 tokens, so two pages fit in 20 tokens max
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "This is page one with several words here"},
			{Number: 2, Text: "This is page two with several words here"},
			{Number: 3, Text: "This is page three with several words here too"},
			{Number: 4, Text: "This is page four with several words here also"},
		},
	}

	ctx := context.Background()
	tree, err := gen.Generate(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, tree)
	assert.Equal(t, 4, tree.TotalPages)
	// Should have two groups merged → should have two children at root level
	assert.Len(t, tree.Root.Children, 2)
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
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string) (string, error) {
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
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string) (string, error) {
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

	// Set the document in the generator
	gen.doc = &document.Document{
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
		"Root":     "Root document summary",
		"Child 1":  "Child 1 section summary",
		"Child 2":  "Child 2 section summary",
		"Section":  "Section summary",
	}

	var mu sync.Mutex
	callCount := 0

	mockLLM := &MockLLMClient{
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string) (string, error) {
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

	// Set the document in the generator
	gen.doc = &document.Document{
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
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string) (string, error) {
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

	// Set the document with missing page 2
	gen.doc = &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 3, Text: "Page 3 content"},
		},
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
		GenerateSummaryFunc: func(ctx context.Context, nodeTitle string, text string) (string, error) {
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

	// Set the document with empty pages
	gen.doc = &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: ""},
			{Number: 2, Text: ""},
		},
	}

	ctx := context.Background()
	err = gen.generateAllSummaries(ctx, node)
	assert.NoError(t, err)

	// Should call GenerateSummary even with empty page text because of added newlines
	assert.Equal(t, 1, summaryCallCount)
	assert.Equal(t, "summary", node.Summary)
}
