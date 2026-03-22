package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/internal/utils"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

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
	// We can't easily inspect the base URL since it's inside the client, but construction succeeds
}

func TestFindNodesByID(t *testing.T) {
	// Create a test tree
	root := document.NewNode("Root", 1, 10)
	child1 := document.NewNode("Child 1", 1, 5)
	child2 := document.NewNode("Child 2", 6, 10)
	root.AddChild(child1)
	root.AddChild(child2)

	tree := document.NewIndexTree(root, 10)

	// Test finding existing nodes
	ids := []string{child1.ID, child2.ID}
	nodes := findNodesByID(tree, ids)
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, child1)
	assert.Contains(t, nodes, child2)

	// Test finding non-existent node
	ids = []string{"non-existent-id", child1.ID}
	nodes = findNodesByID(tree, ids)
	assert.Len(t, nodes, 1)
	assert.Contains(t, nodes, child1)

	// Test empty IDs
	nodes = findNodesByID(tree, []string{})
	assert.Len(t, nodes, 0)
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
