package output

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestSaveAndLoadIndexTree(t *testing.T) {
	// Create a test index tree
	root := document.NewNode("Root", 1, 10)
	root.AddChild(document.NewNode("Chapter 1", 1, 5))
	root.AddChild(document.NewNode("Chapter 2", 6, 10))

	tree := document.NewIndexTree(root, 10)
	tree.DocumentInfo = "Test Document"

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "*.json")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	require.NoError(t, os.Remove(tmpPath))
	require.NoError(t, tmpFile.Close())

	// Save the index
	err = SaveIndexTree(tree, tmpPath)
	require.NoError(t, err)

	// Load it back
	loadedTree, err := LoadIndexTree(tmpPath)
	require.NoError(t, err)

	// Verify the content
	assert.Equal(t, tree.TotalPages, loadedTree.TotalPages)
	assert.Equal(t, tree.DocumentInfo, loadedTree.DocumentInfo)
	assert.Equal(t, tree.Root.Title, loadedTree.Root.Title)
	assert.Len(t, loadedTree.Root.Children, 2)
	assert.Equal(t, "Chapter 1", loadedTree.Root.Children[0].Title)
	assert.Equal(t, "Chapter 2", loadedTree.Root.Children[1].Title)
	assert.WithinDuration(t, tree.GeneratedAt, loadedTree.GeneratedAt, time.Second)
}

func TestSaveIndexTree_CreatesDirectory(t *testing.T) {
	// Create a test index tree
	root := document.NewNode("Root", 1, 10)
	tree := document.NewIndexTree(root, 10)

	// Create a path with non-existent subdirectory
	tmpDir := os.TempDir()
	nestedPath := tmpDir + "/pageindex-test/subdir/index.json"
	require.NoError(t, os.RemoveAll(tmpDir+"/pageindex-test"))

	// Save should create the directory
	err := SaveIndexTree(tree, nestedPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(nestedPath)
	assert.NoError(t, err)
}

func TestLoadIndexTree_FileNotFound(t *testing.T) {
	_, err := LoadIndexTree("nonexistent/path/file.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read index file")
}

func TestLoadIndexTree_InvalidJSON(t *testing.T) {
	// Create temp file with invalid JSON
	tmpFile, err := os.CreateTemp("", "*.json")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	_, _ = tmpFile.WriteString("not valid json")
	require.NoError(t, tmpFile.Close())

	_, err = LoadIndexTree(tmpPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse index JSON")
}

func TestSaveSearchResult(t *testing.T) {
	// Create a test search result
	node1 := document.NewNode("Relevant Section", 5, 6)
	node2 := document.NewNode("Another Section", 10, 12)

	result := &document.SearchResult{
		Query:  "What is the test?",
		Answer: "This is a test answer.",
		Nodes:  []*document.Node{node1, node2},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "*.json")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	require.NoError(t, tmpFile.Close())

	// Save the result
	err = SaveSearchResult(result, tmpPath)
	require.NoError(t, err)

	// Verify file exists and has content
	info, err := os.Stat(tmpPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}
