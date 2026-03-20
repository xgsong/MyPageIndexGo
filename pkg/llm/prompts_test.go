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
	// Create a simple test tree
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
