package output

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestSaveAndLoadIndexTree_RoundTrip(t *testing.T) {
	// Create a complex tree
	root := document.NewNode("Document Root", 1, 20)
	root.Summary = "This is the document summary"

	child1 := document.NewNode("Chapter 1", 1, 10)
	child1.Summary = "Chapter 1 summary"
	child1.AddChild(document.NewNode("Section 1.1", 1, 5))
	child1.AddChild(document.NewNode("Section 1.2", 6, 10))

	child2 := document.NewNode("Chapter 2", 11, 20)
	child2.Summary = "Chapter 2 summary"
	child2.AddChild(document.NewNode("Section 2.1", 11, 15))
	child2.AddChild(document.NewNode("Section 2.2", 16, 20))

	root.AddChild(child1)
	root.AddChild(child2)

	tree := document.NewIndexTree(root, 20)
	tree.DocumentInfo = "Test Document Info"

	// Save to temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-index.json")

	err := SaveIndexTree(tree, tmpFile)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(tmpFile)
	require.NoError(t, err)

	// Load back
	loadedTree, err := LoadIndexTree(tmpFile)
	require.NoError(t, err)

	// Verify tree structure
	assert.Equal(t, tree.TotalPages, loadedTree.TotalPages)
	assert.Equal(t, tree.CountAllNodes(), loadedTree.CountAllNodes())
	assert.Equal(t, tree.DocumentInfo, loadedTree.DocumentInfo)
	assert.Equal(t, tree.Root.Title, loadedTree.Root.Title)
	assert.Equal(t, tree.Root.Summary, loadedTree.Root.Summary)
	assert.Len(t, loadedTree.Root.Children, 2)
}

func TestSaveIndexTree_NestedDirectory(t *testing.T) {
	root := document.NewNode("Root", 1, 1)
	tree := document.NewIndexTree(root, 1)

	// Create nested directory path
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "level1", "level2", "level3", "index.json")

	err := SaveIndexTree(tree, nestedPath)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(filepath.Join(tmpDir, "level1", "level2", "level3"))
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(nestedPath)
	require.NoError(t, err)
}

func TestLoadIndexTree_InvalidJSON_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(tmpFile, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = LoadIndexTree(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse index JSON")
}

func TestLoadIndexTree_NonExistent(t *testing.T) {
	nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist.json")

	_, err := LoadIndexTree(nonExistentPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestSaveAndLoadSearchResult(t *testing.T) {
	// Create a search result with nodes
	node1 := document.NewNode("Section 1", 1, 5)
	node2 := document.NewNode("Section 2", 6, 10)

	result := &document.SearchResult{
		Query:  "What is the revenue?",
		Answer: "The revenue was $10 billion with 15% growth.",
		Nodes:  []*document.Node{node1, node2},
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "search-result.json")

	err := SaveSearchResult(result, tmpFile)
	require.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	assert.Contains(t, string(content), "What is the revenue?")
	assert.Contains(t, string(content), "$10 billion")
	assert.Contains(t, string(content), "Section 1")
}

func TestSaveJSON_PermissionDenied(t *testing.T) {
	// Try to save to a read-only directory
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission denied test on Windows")
	}

	root := document.NewNode("Root", 1, 1)
	tree := document.NewIndexTree(root, 1)

	// Create a read-only directory
	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readonlyDir, 0555)
	require.NoError(t, err)

	// Try to save to it
	path := filepath.Join(readonlyDir, "test.json")
	err = SaveIndexTree(tree, path)
	assert.Error(t, err)
}

func TestSaveIndexTree_UUIDPreservation(t *testing.T) {
	// Create tree with known UUIDs
	root := document.NewNode("Root", 1, 10)
	originalID := root.ID

	child := document.NewNode("Child", 1, 5)
	childOriginalID := child.ID
	root.AddChild(child)

	tree := document.NewIndexTree(root, 10)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "uuid-test.json")

	err := SaveIndexTree(tree, tmpFile)
	require.NoError(t, err)

	loadedTree, err := LoadIndexTree(tmpFile)
	require.NoError(t, err)

	// Verify UUIDs are preserved
	assert.Equal(t, originalID, loadedTree.Root.ID)
	assert.Equal(t, childOriginalID, loadedTree.Root.Children[0].ID)
}
