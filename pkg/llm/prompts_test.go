package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestGenerateStructurePrompt(t *testing.T) {
	prompt := GenerateStructurePrompt()
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "hierarchical table of contents")
	assert.Contains(t, prompt, "JSON")
}

func TestRenderSummaryPrompt(t *testing.T) {
	prompt, err := RenderSummaryPrompt("Introduction", "Some sample text content.")
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Introduction")
	assert.Contains(t, prompt, "Some sample text content.")
}

func TestSearchPrompt(t *testing.T) {
	root := document.NewNode("Test Document", 1, 10)
	root.AddChild(document.NewNode("Chapter 1", 1, 5))
	root.AddChild(document.NewNode("Chapter 2", 6, 10))
	tree := document.NewIndexTree(root, 10)

	query := "What is this document about?"

	prompt, err := SearchPrompt(query, tree)
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, query)
	assert.Contains(t, prompt, "Test Document")
	assert.Contains(t, prompt, "Chapter 1")
}

func TestBatchSummaryPrompt(t *testing.T) {
	prompt := batchSummaryPrompt()
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "summarizer")
	assert.Contains(t, prompt, "summaries")
}

func TestRenderBatchSummaryPrompt(t *testing.T) {
	requests := []*BatchSummaryRequest{
		{
			NodeID:    "node-1",
			NodeTitle: "Chapter 1",
			Text:      "This is the content of chapter 1",
		},
		{
			NodeID:    "node-2",
			NodeTitle: "Chapter 2",
			Text:      "This is the content of chapter 2",
		},
	}

	prompt, err := RenderBatchSummaryPrompt(requests)
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "node_id")
	assert.Contains(t, prompt, "Chapter 1")
	assert.Contains(t, prompt, "Chapter 2")
	assert.Contains(t, prompt, "node-1")
	assert.Contains(t, prompt, "node-2")
}

func TestRenderBatchSummaryPrompt_EmptyRequests(t *testing.T) {
	prompt, err := RenderBatchSummaryPrompt([]*BatchSummaryRequest{})
	assert.NoError(t, err)
	assert.NotEmpty(t, prompt)
}

func TestSearchPrompt_WithNilTree(t *testing.T) {
	query := "test query"
	tree := document.NewIndexTree(nil, 0)

	prompt, err := SearchPrompt(query, tree)
	assert.NoError(t, err)
	assert.Contains(t, prompt, query)
}

func TestSearchPrompt_WithDeepTree(t *testing.T) {
	root := document.NewNode("Root", 1, 20)
	child1 := document.NewNode("Chapter 1", 1, 10)
	child1.AddChild(document.NewNode("Section 1.1", 1, 5))
	child1.AddChild(document.NewNode("Section 1.2", 6, 10))
	child2 := document.NewNode("Chapter 2", 11, 20)
	root.AddChild(child1)
	root.AddChild(child2)
	tree := document.NewIndexTree(root, 20)

	prompt, err := SearchPrompt("Where is Section 1.2?", tree)
	assert.NoError(t, err)
	assert.Contains(t, prompt, "Section 1.2")
	assert.Contains(t, prompt, "Chapter 1")
}
