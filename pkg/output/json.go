package output

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

// SaveIndexTree saves an IndexTree to a JSON file.
// Creates parent directories if they don't exist.
func SaveIndexTree(tree *document.IndexTree, path string) error {
	return saveJSON(tree, path)
}

// SaveSearchResult saves a SearchResult to a JSON file.
// Creates parent directories if they don't exist.
func SaveSearchResult(result *document.SearchResult, path string) error {
	return saveJSON(result, path)
}

// LoadIndexTree loads an IndexTree from a JSON file.
func LoadIndexTree(path string) (*document.IndexTree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	var tree document.IndexTree
	if err := sonic.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("failed to parse index JSON: %w", err)
	}

	// Rebuild the node map after loading from JSON
	tree.BuildNodeMap()

	return &tree, nil
}

// saveJSON marshals data to JSON and writes it to a file.
func saveJSON(data interface{}, path string) error {
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal with indentation for readability
	jsonData, err := sonic.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}
