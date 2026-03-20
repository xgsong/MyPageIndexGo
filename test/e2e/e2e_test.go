package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/output"
)

// hasAPIKey checks if OPENAI_API_KEY is set in environment
func hasAPIKey() bool {
	key := os.Getenv("OPENAI_API_KEY")
	return key != ""
}

// TestE2E_GenerateAndSearch tests the complete end-to-end flow:
// 1. Load configuration
// 2. Parse input document
// 3. Generate index
// 4. Save index to JSON
// 5. Load index from JSON
// 6. Execute search
// 7. Verify search result
//
// This test is skipped when OPENAI_API_KEY is not available in environment.
func TestE2E_GenerateAndSearch(t *testing.T) {
	if !hasAPIKey() {
		t.Skip("Skipping E2E test: OPENAI_API_KEY not set in environment")
	}

	// Step 1: Load configuration
	cfg, err := config.LoadFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, cfg.OpenAIAPIKey)

	// Use a cheaper model for testing
	if cfg.OpenAIModel == "gpt-4o" {
		// Override to cheaper model for test if not explicitly set
		if os.Getenv("OPENAI_MODEL") == "" {
			cfg.OpenAIModel = "gpt-4o-mini"
		}
	}

	// Step 2: Open and parse input document
	inputPath := "../fixtures/test.md"
	file, err := os.Open(inputPath)
	require.NoError(t, err)
	defer file.Close()

	// Get parser for markdown
	reg := document.DefaultRegistry()
	parser, ok := reg.Get("md")
	require.True(t, ok)

	doc, err := parser.Parse(file)
	require.NoError(t, err)
	assert.Equal(t, 1, doc.TotalPages())
	assert.Contains(t, doc.Pages[0].Text, "Acme公司")

	// Step 3: Create LLM client and generate index
	llmClient := llm.NewOpenAIClient(cfg)
	generator, err := indexer.NewIndexGenerator(cfg, llmClient)
	require.NoError(t, err)

	ctx := context.Background()
	t.Log("Generating index... this may take a minute")
	tree, err := generator.Generate(ctx, doc)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.Root)
	assert.Greater(t, tree.CountAllNodes(), 0)
	t.Logf("Generated index with %d nodes", tree.CountAllNodes())

	// Step 4: Save index to temporary JSON file
	tmpFile, err := os.CreateTemp("", "pageindex-e2e-*.json")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Close()

	err = output.SaveIndexTree(tree, tmpPath)
	require.NoError(t, err)
	t.Logf("Index saved to: %s", tmpPath)

	// Step 5: Load index back from JSON
	loadedTree, err := output.LoadIndexTree(tmpPath)
	require.NoError(t, err)
	assert.Equal(t, tree.TotalPages, loadedTree.TotalPages)
	assert.Equal(t, tree.CountAllNodes(), loadedTree.CountAllNodes())

	// Step 6: Execute search
	searcher := indexer.NewSearcher(llmClient)
	query := "What was the total revenue in 2023 and what percentage growth did the company achieve?"

	t.Logf("Searching for: %s", query)
	result, err := searcher.Search(ctx, query, loadedTree)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, query, result.Query)
	assert.NotEmpty(t, result.Answer)

	// Step 7: Verify we got a non-empty answer
	answer := result.Answer
	t.Logf("Search answer received (%d characters)", len(answer))

	// Verify that at least one node was referenced
	assert.NotEmpty(t, result.Nodes)
	t.Logf("Number of referenced nodes: %d", len(result.Nodes))

	// If summaries were generated, check that answer contains expected information
	if cfg.GenerateSummaries {
		// When summaries are generated, LLM has access to the actual data
		assert.Contains(t, answer, "10", "answer should contain 10 billion revenue")
		assert.Contains(t, answer, "15", "answer should contain 15% growth")
	} else {
		// Without summaries, LLM can only point to where to find the information
		// This is expected behavior
		t.Log("Note: GENERATE_SUMMARIES is false, answer won't contain specific numbers (expected)")
	}

	t.Log("✓ E2E test completed successfully!")
}

// TestE2E_GenerateAndSearch_PDF tests the complete end-to-end flow with PDF input:
// 1. Load configuration
// 2. Parse PDF document
// 3. Generate index
// 4. Save index to JSON
// 5. Load index from JSON
// 6. Execute search
// 7. Verify search result
//
// This test is skipped when:
// - OPENAI_API_KEY is not available
// - testing.Short() is enabled (for quick test runs)
func TestE2E_GenerateAndSearch_PDF(t *testing.T) {
	if !hasAPIKey() {
		t.Skip("Skipping E2E test: OPENAI_API_KEY not set in environment")
	}
	if testing.Short() {
		t.Skip("Skipping slow PDF E2E test in short mode")
	}

	// Step 1: Load configuration
	cfg, err := config.LoadFromEnv()
	require.NoError(t, err)
	require.NotEmpty(t, cfg.OpenAIAPIKey)

	// Use a cheaper model for testing
	if cfg.OpenAIModel == "gpt-4o" {
		// Override to cheaper model for test if not explicitly set
		if os.Getenv("OPENAI_MODEL") == "" {
			cfg.OpenAIModel = "gpt-4o-mini"
		}
	}

	// Step 2: Open and parse input PDF
	inputPath := "../fixtures/test.pdf"
	file, err := os.Open(inputPath)
	require.NoError(t, err)
	defer file.Close()

	// Get parser for PDF
	reg := document.DefaultRegistry()
	parser, ok := reg.Get("pdf")
	require.True(t, ok)

	t.Log("Parsing PDF document... this may take a moment")
	doc, err := parser.Parse(file)
	require.NoError(t, err)
	assert.Greater(t, doc.TotalPages(), 0)
	t.Logf("Parsed %d pages from PDF", doc.TotalPages())

	// Step 3: Create LLM client and generate index
	llmClient := llm.NewOpenAIClient(cfg)
	generator, err := indexer.NewIndexGenerator(cfg, llmClient)
	require.NoError(t, err)

	ctx := context.Background()
	t.Log("Generating index... this may take several minutes depending on page count")
	tree, err := generator.Generate(ctx, doc)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.NotNil(t, tree.Root)
	assert.Greater(t, tree.CountAllNodes(), 0)
	t.Logf("Generated index with %d nodes", tree.CountAllNodes())

	// Step 4: Save index to temporary JSON file
	tmpFile, err := os.CreateTemp("", "pageindex-e2e-pdf-*.json")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.Close()

	err = output.SaveIndexTree(tree, tmpPath)
	require.NoError(t, err)
	t.Logf("Index saved to: %s", tmpPath)

	// Step 5: Load index back from JSON
	loadedTree, err := output.LoadIndexTree(tmpPath)
	require.NoError(t, err)
	assert.Equal(t, tree.TotalPages, loadedTree.TotalPages)
	assert.Equal(t, tree.CountAllNodes(), loadedTree.CountAllNodes())

	// Step 6: Execute a sample search
	searcher := indexer.NewSearcher(llmClient)
	query := "What is the main topic of this document?"

	t.Logf("Searching for: %s", query)
	result, err := searcher.Search(ctx, query, loadedTree)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, query, result.Query)
	assert.NotEmpty(t, result.Answer)
	t.Logf("Search answer received (%d characters)", len(result.Answer))

	// Verify that at least one node was referenced
	assert.NotEmpty(t, result.Nodes)
	t.Logf("Number of referenced nodes: %d", len(result.Nodes))

	t.Log("✓ PDF E2E test completed successfully!")
}

// TestE2E_ConfigLoading tests that configuration loads correctly from environment
func TestE2E_ConfigLoading(t *testing.T) {
	// This test doesn't require API key, just checks that config loading works
	// We expect it to fail because no API key in CI environment
	_, err := config.LoadFromEnv()
	// It's okay if it fails, we just expect it to complete without panic
	// If it succeeds, that's fine too (when running locally with API key)
	if err != nil {
		assert.Contains(t, err.Error(), "OPENAI_API_KEY")
	}
}
