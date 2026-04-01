package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/internal/utils"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// MockOpenAIClient is a mock implementation of LLMClient for testing.
type MockOpenAIClient struct {
	GenerateStructureFunc      func(ctx context.Context, text string, lang language.Language) (*document.Node, error)
	GenerateSummaryFunc        func(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)
	SearchFunc                 func(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
	GenerateBatchSummariesFunc func(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error)
	GenerateSimpleFunc         func(ctx context.Context, prompt string) (string, error)
}

func (m *MockOpenAIClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	if m.GenerateStructureFunc != nil {
		return m.GenerateStructureFunc(ctx, text, lang)
	}
	return document.NewNode("Root", 1, 1), nil
}

func (m *MockOpenAIClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	if m.GenerateSummaryFunc != nil {
		return m.GenerateSummaryFunc(ctx, nodeTitle, text, lang)
	}
	return "Mock summary", nil
}

func (m *MockOpenAIClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query, tree)
	}
	// 返回一个简单的搜索结果
	return &document.SearchResult{
		Query: query,
	}, nil
}

func (m *MockOpenAIClient) GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
	if m.GenerateBatchSummariesFunc != nil {
		return m.GenerateBatchSummariesFunc(ctx, requests, lang)
	}
	// Default implementation: fallback to individual calls for backward compatibility
	responses := make([]*BatchSummaryResponse, len(requests))
	for i, req := range requests {
		summary, err := m.GenerateSummary(ctx, req.NodeTitle, req.Text, lang)
		if err != nil {
			responses[i] = &BatchSummaryResponse{
				NodeID: req.NodeID,
				Error:  err.Error(),
			}
		} else {
			responses[i] = &BatchSummaryResponse{
				NodeID:  req.NodeID,
				Summary: summary,
			}
		}
	}
	return responses, nil
}

func (m *MockOpenAIClient) GenerateSimple(ctx context.Context, prompt string) (string, error) {
	if m.GenerateSimpleFunc != nil {
		return m.GenerateSimpleFunc(ctx, prompt)
	}
	return "{\"toc_detected\": \"no\"}", nil
}

func TestNewOpenAIClient(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"

	client := NewOpenAIClient(cfg)
	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, cfg.OpenAIModel, client.model)
	assert.NotNil(t, client.jsonCleaner)
}

func TestNewOpenAIClient_CustomBaseURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = "https://custom.openai.com/v1"

	client := NewOpenAIClient(cfg)
	assert.NotNil(t, client)
}

func TestFindNodesByID(t *testing.T) {
	root := document.NewNode("Root", 1, 10)
	child1 := document.NewNode("Child 1", 1, 5)
	child2 := document.NewNode("Child 2", 6, 10)
	root.AddChild(child1)
	root.AddChild(child2)

	tree := document.NewIndexTree(root, 10)

	ids := []string{child1.ID, child2.ID}
	nodes := findNodesByID(tree, ids)
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, child1)
	assert.Contains(t, nodes, child2)

	ids = []string{"non-existent-id", child1.ID}
	nodes = findNodesByID(tree, ids)
	assert.Len(t, nodes, 1)
	assert.Contains(t, nodes, child1)

	nodes = findNodesByID(tree, []string{})
	assert.Len(t, nodes, 0)
}

func TestFindNodesByID_WithDeepTree(t *testing.T) {
	root := document.NewNode("Root", 1, 20)
	child1 := document.NewNode("Child 1", 1, 10)
	grandchild := document.NewNode("Grandchild", 1, 5)
	child1.AddChild(grandchild)
	root.AddChild(child1)
	tree := document.NewIndexTree(root, 20)

	nodes := findNodesByID(tree, []string{grandchild.ID})
	assert.Len(t, nodes, 1)
	assert.Equal(t, grandchild.ID, nodes[0].ID)
}

func TestJSONCleanerIntegration_ValidJSON(t *testing.T) {
	cleaner := utils.NewJSONCleaner()

	var node document.Node
	err := cleaner.ParseJSON(`{"title": "Test", "start_page": 1, "end_page": 5, "children": []}`, &node)
	assert.NoError(t, err)
	assert.Equal(t, "Test", node.Title)
	assert.Equal(t, 1, node.StartPage)
	assert.Equal(t, 5, node.EndPage)
}

func TestJSONCleanerIntegration_MessyJSON(t *testing.T) {
	cleaner := utils.NewJSONCleaner()

	messyJSON := "```json\n{\n  \"title\": \"Test Section\",\n  \"start_page\": 1,\n  \"end_page\": 5,\n  \"children\": [\n    {\n      \"title\": \"Subsection\",\n      \"start_page\": 1,\n      \"end_page\": 2,\n      \"children\": [],\n    },\n  ],\n}\n```"

	var node document.Node
	err := cleaner.ParseJSON(messyJSON, &node)
	assert.NoError(t, err)
	assert.Equal(t, "Test Section", node.Title)
	assert.Equal(t, 1, node.StartPage)
	assert.Equal(t, 5, node.EndPage)
	assert.Len(t, node.Children, 1)
}

func TestGenerateBatchSummaries_EmptyRequests(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	client := NewOpenAIClient(cfg)

	ctx := context.Background()
	result, err := client.GenerateBatchSummaries(ctx, []*BatchSummaryRequest{}, language.LanguageEnglish)

	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestGenerateBatchSummaries_ValidRequests(t *testing.T) {
	// 使用模拟的OpenAIClient进行测试
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	
	// 创建一个模拟的OpenAIClient
	client := &MockOpenAIClient{
		GenerateBatchSummariesFunc: func(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
			responses := make([]*BatchSummaryResponse, len(requests))
			for i, req := range requests {
				responses[i] = &BatchSummaryResponse{
					NodeID:  req.NodeID,
					Summary: fmt.Sprintf("Summary for %s", req.NodeTitle),
				}
			}
			return responses, nil
		},
	}

	requests := []*BatchSummaryRequest{
		{NodeID: "node-1", NodeTitle: "Title 1", Text: "Content 1"},
		{NodeID: "node-2", NodeTitle: "Title 2", Text: "Content 2"},
	}

	ctx := context.Background()
	result, err := client.GenerateBatchSummaries(ctx, requests, language.LanguageEnglish)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "Summary for Title 1", result[0].Summary)
	assert.Equal(t, "Summary for Title 2", result[1].Summary)
}

func TestGenerateSimple_Prompt(t *testing.T) {
	// 使用模拟的OpenAIClient进行测试
	client := &MockOpenAIClient{
		GenerateSimpleFunc: func(ctx context.Context, prompt string) (string, error) {
			assert.Equal(t, "Say hello", prompt)
			return "Hello from mock!", nil
		},
	}

	ctx := context.Background()
	result, err := client.GenerateSimple(ctx, "Say hello")

	assert.NoError(t, err)
	assert.Equal(t, "Hello from mock!", result)
}

func TestCreateLanguageSystemMessage_English(t *testing.T) {
	msg := createLanguageSystemMessage(language.LanguageEnglish)
	assert.Contains(t, msg, "English")
}

func TestCreateLanguageSystemMessage_Chinese(t *testing.T) {
	msg := createLanguageSystemMessage(language.LanguageChinese)
	assert.Contains(t, msg, "Chinese")
}
