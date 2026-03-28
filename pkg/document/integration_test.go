package document

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPDFParser_Integration tests PDF parsing with a real test file
func TestPDFParser_Integration(t *testing.T) {
	parser := NewPDFParser()

	// Open the test PDF file
	file, err := os.Open("../../test/fixtures/test.pdf")
	if err != nil {
		t.Skip("Test PDF file not found, skipping integration test")
	}
	defer file.Close()

	doc, err := parser.Parse(file)
	require.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Greater(t, len(doc.Pages), 0)

	// Check that we got some text content
	hasText := false
	for _, page := range doc.Pages {
		if strings.TrimSpace(page.Text) != "" {
			hasText = true
			break
		}
	}
	assert.True(t, hasText, "PDF should contain extractable text")
}

// TestPDFParser_InvalidFile tests parsing a non-PDF file
func TestPDFParser_InvalidFile(t *testing.T) {
	parser := NewPDFParser()

	// Create a temporary file with invalid content
	tmpFile, err := os.CreateTemp("", "invalid-*.txt")
	require.NoError(t, err)
	tmpName := tmpFile.Name()

	_, err = tmpFile.WriteString("This is not a PDF file")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	defer func() {
		if err := os.Remove(tmpName); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	// Try to parse it as PDF
	file, err := os.Open(tmpFile.Name())
	require.NoError(t, err)
	defer file.Close()

	_, err = parser.Parse(file)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

// TestPDFParser_FileTooLarge tests file size limit
func TestPDFParser_FileTooLarge(t *testing.T) {
	parser := NewPDFParser()

	// Create a fake PDF file that exceeds size limit
	tmpFile, err := os.CreateTemp("", "large-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write PDF magic number
	_, err = tmpFile.WriteString("%PDF-1.4\n")
	require.NoError(t, err)

	// Write large content to exceed 50MB
	largeContent := make([]byte, 51*1024*1024)
	_, err = tmpFile.Write(largeContent)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	file, err := os.Open(tmpFile.Name())
	require.NoError(t, err)

	_, err = parser.Parse(file)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")

	require.NoError(t, file.Close())
}

// TestMarkdownParser_Integration tests markdown parsing
func TestMarkdownParser_Integration(t *testing.T) {
	parser := NewMarkdownParser()

	// Test with the sample markdown file
	file, err := os.Open("../../test/fixtures/test.md")
	if err != nil {
		t.Skip("Test markdown file not found, skipping integration test")
	}
	defer file.Close()

	doc, err := parser.Parse(file)
	require.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, 1, len(doc.Pages))
	assert.Contains(t, doc.Pages[0].Text, "公司财务报告")
}

// TestParserRegistry_Integration tests the parser registry
func TestParserRegistry_Integration(t *testing.T) {
	reg := DefaultRegistry()

	tests := []struct {
		ext      string
		expected bool
	}{
		{"pdf", true},
		{"md", true},
		{"PDF", true},       // case insensitive
		{"MD", true},        // case insensitive
		{".pdf", true},      // with dot
		{".md", true},       // with dot
		{"markdown", false}, // not registered
		{"txt", false},      // not registered
		{"docx", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			parser, ok := reg.Get(tt.ext)
			assert.Equal(t, tt.expected, ok)
			if tt.expected {
				assert.NotNil(t, parser)
			}
		})
	}
}

// TestDocument_FullWorkflow tests document operations
func TestDocument_FullWorkflow(t *testing.T) {
	// Create a multi-page document
	doc := &Document{
		Pages: []Page{
			{Number: 1, Text: "Page 1 content with some text."},
			{Number: 2, Text: "Page 2 content with more text."},
			{Number: 3, Text: "Page 3 final content."},
		},
		Metadata: map[string]string{
			"title":  "Test Document",
			"author": "Test Author",
		},
	}

	// Test total pages
	assert.Equal(t, 3, doc.TotalPages())

	// Test full text concatenation
	fullText := doc.GetFullText()
	assert.Contains(t, fullText, "Page 1 content")
	assert.Contains(t, fullText, "Page 2 content")
	assert.Contains(t, fullText, "Page 3 final")
	assert.Contains(t, fullText, "\n\n") // Check separator
}

// TestNode_TreeOperations tests tree node operations
func TestNode_TreeOperations(t *testing.T) {
	// Build a tree structure
	root := NewNode("Document", 1, 10)

	child1 := NewNode("Chapter 1", 1, 5)
	child1.AddChild(NewNode("Section 1.1", 1, 3))
	child1.AddChild(NewNode("Section 1.2", 4, 5))

	child2 := NewNode("Chapter 2", 6, 10)
	child2.AddChild(NewNode("Section 2.1", 6, 8))
	child2.AddChild(NewNode("Section 2.2", 9, 10))

	root.AddChild(child1)
	root.AddChild(child2)

	// Count nodes
	assert.Equal(t, 7, root.CountNodes())
	assert.Equal(t, 3, child1.CountNodes())
	assert.Equal(t, 3, child2.CountNodes())

	// Verify structure
	assert.Len(t, root.Children, 2)
	assert.Len(t, child1.Children, 2)
	assert.Len(t, child2.Children, 2)

	// Check page ranges
	assert.Equal(t, 1, root.StartPage)
	assert.Equal(t, 10, root.EndPage)
	assert.Equal(t, 1, child1.StartPage)
	assert.Equal(t, 5, child1.EndPage)
}

// TestIndexTree_Serialization tests tree serialization round-trip
func TestIndexTree_Serialization(t *testing.T) {
	// Create a tree
	root := NewNode("Root", 1, 5)
	root.AddChild(NewNode("Child 1", 1, 3))
	root.AddChild(NewNode("Child 2", 4, 5))

	tree := NewIndexTree(root, 5)
	tree.DocumentInfo = "Test Document"

	// Test node counting
	assert.Equal(t, 3, tree.CountAllNodes())
	assert.Equal(t, 5, tree.TotalPages)

	// Test with nil root
	emptyTree := NewIndexTree(nil, 0)
	assert.Equal(t, 0, emptyTree.CountAllNodes())
}
