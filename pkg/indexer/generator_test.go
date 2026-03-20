package indexer

import (
	"context"
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

	callCount := 0
	mockLLM := &MockLLMClient{
		GenerateStructureFunc: func(ctx context.Context, text string) (*document.Node, error) {
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
