package document

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocument_GetFullText(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{Number: 1, Text: "Page 1 content"},
			{Number: 2, Text: "Page 2 content"},
		},
	}

	fullText := doc.GetFullText()
	assert.Contains(t, fullText, "Page 1 content")
	assert.Contains(t, fullText, "Page 2 content")
}

func TestDocument_TotalPages(t *testing.T) {
	doc := &Document{
		Pages: []Page{
			{Number: 1, Text: "Page 1"},
			{Number: 2, Text: "Page 2"},
			{Number: 3, Text: "Page 3"},
		},
	}

	assert.Equal(t, 3, doc.TotalPages())
}

func TestNewNode(t *testing.T) {
	node := NewNode("Test Node", 1, 10)

	assert.NotEmpty(t, node.ID)
	assert.Equal(t, "Test Node", node.Title)
	assert.Equal(t, 1, node.StartPage)
	assert.Equal(t, 10, node.EndPage)
	assert.Empty(t, node.Children)
}

func TestNode_AddChild(t *testing.T) {
	parent := NewNode("Parent", 1, 10)
	child := NewNode("Child", 1, 5)

	parent.AddChild(child)
	assert.Len(t, parent.Children, 1)
	assert.Equal(t, child, parent.Children[0])
}

func TestNode_CountNodes(t *testing.T) {
	root := NewNode("Root", 1, 10)
	root.AddChild(NewNode("Child 1", 1, 5))
	root.AddChild(NewNode("Child 2", 6, 10))

	count := root.CountNodes()
	assert.Equal(t, 3, count)
}

func TestNewIndexTree(t *testing.T) {
	root := NewNode("Root", 1, 10)
	tree := NewIndexTree(root, 10)

	assert.Equal(t, root, tree.Root)
	assert.Equal(t, 10, tree.TotalPages)
	assert.Equal(t, 1, tree.CountAllNodes())
	assert.False(t, tree.GeneratedAt.IsZero())
}

func TestMarkdownParser_Parse(t *testing.T) {
	parser := NewMarkdownParser()
	input := strings.NewReader(`# Hello World

This is a test paragraph.

## Second Section

Another paragraph here.
`)

	doc, err := parser.Parse(input)
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Len(t, doc.Pages, 1)
	assert.Contains(t, doc.Pages[0].Text, "Hello World")
	assert.Contains(t, doc.Pages[0].Text, "Second Section")
}

func TestMarkdownParser_SupportedExtensions(t *testing.T) {
	parser := NewMarkdownParser()
	exts := parser.SupportedExtensions()
	assert.ElementsMatch(t, []string{"md", "markdown", "txt"}, exts)
}

func TestMarkdownParser_Name(t *testing.T) {
	parser := NewMarkdownParser()
	assert.Equal(t, "Markdown", parser.Name())
}

func TestPDFParser_SupportedExtensions(t *testing.T) {
	parser := NewPDFParser()
	exts := parser.SupportedExtensions()
	assert.ElementsMatch(t, []string{"pdf"}, exts)
}

func TestPDFParser_Name(t *testing.T) {
	parser := NewPDFParser()
	assert.Equal(t, "PDF", parser.Name())
}

func TestParserRegistry_Get_NonExistent(t *testing.T) {
	registry := NewParserRegistry()
	parser, ok := registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, parser)
}

func TestParserRegistry_RegisterAndGet(t *testing.T) {
	registry := NewParserRegistry()
	parser := NewMarkdownParser()

	registry.Register("md", parser)

	// Test with lowercase extension
	foundParser, ok := registry.Get("md")
	assert.True(t, ok)
	assert.Equal(t, parser, foundParser)

	// Test with uppercase extension
	foundParser, ok = registry.Get("MD")
	assert.True(t, ok)
	assert.Equal(t, parser, foundParser)

	// Test with extension with dot
	foundParser, ok = registry.Get(".md")
	assert.True(t, ok)
	assert.Equal(t, parser, foundParser)
}

func TestDefaultRegistry(t *testing.T) {
	registry := DefaultRegistry()
	assert.NotNil(t, registry)

	// Should have PDF and Markdown parsers registered
	pdfParser, ok := registry.Get("pdf")
	assert.True(t, ok)
	assert.Equal(t, "PDF", pdfParser.Name())

	mdParser, ok := registry.Get("md")
	assert.True(t, ok)
	assert.Equal(t, "Markdown", mdParser.Name())
}
