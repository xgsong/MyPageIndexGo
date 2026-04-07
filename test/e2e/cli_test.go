package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const binaryPath = "../../pageindex"

// TestCLI_Help tests that CLI help command works correctly
func TestCLI_Help(t *testing.T) {
	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping CLI test: pageindex binary not found, run 'make build' first")
	}

	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr bool
	}{
		{
			name: "default help",
			args: []string{"--help"},
			want: []string{"pageindex", "Vectorless, reasoning-based RAG system", "generate", "search", "update"},
		},
		{
			name: "generate help",
			args: []string{"generate", "--help"},
			want: []string{"Generate index from a document", "--pdf", "--md", "--output", "--model"},
		},
		{
			name: "search help",
			args: []string{"search", "--help"},
			want: []string{"Search the generated index with a query", "--index", "--query", "--output"},
		},
		{
			name: "version command",
			args: []string{"--version"},
			want: []string{"1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "Command failed: %s", outputStr)
			}

			for _, wantStr := range tt.want {
				assert.Contains(t, outputStr, wantStr, "Expected output to contain '%s', got:\n%s", wantStr, outputStr)
			}
		})
	}
}

// TestCLI_Generate_NoArgs tests generate command without required arguments
func TestCLI_Generate_NoArgs(t *testing.T) {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping CLI test: pageindex binary not found")
	}

	cmd := exec.Command(binaryPath, "generate")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	assert.Error(t, err)
	// Config loads before command validation, so we expect config error first
	assert.Contains(t, outputStr, "config.yaml not found")
}

// TestCLI_Search_NoArgs tests search command without required arguments
func TestCLI_Search_NoArgs(t *testing.T) {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping CLI test: pageindex binary not found")
	}

	cmd := exec.Command(binaryPath, "search")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	assert.Error(t, err)
	assert.Contains(t, outputStr, "Required flags")
	assert.Contains(t, outputStr, "index, query")
}

// TestCLI_InvalidCommand tests invalid command handling
func TestCLI_InvalidCommand(t *testing.T) {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping CLI test: pageindex binary not found")
	}

	cmd := exec.Command(binaryPath, "nonexistent")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	assert.Error(t, err)
	assert.Contains(t, outputStr, "No help topic for 'nonexistent'")
}

// TestCLI_Generate_Markdown tests generate command with markdown input
// This test is skipped when OPENAI_API_KEY is not available
func TestCLI_Generate_Markdown(t *testing.T) {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Skipping CLI test: pageindex binary not found")
	}
	if !hasAPIKey() {
		t.Skip("Skipping CLI generate test: OPENAI_API_KEY not set")
	}
	if testing.Short() {
		t.Skip("Skipping slow CLI generate test in short mode")
	}

	inputPath := "../fixtures/test.md"
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-index.json")

	cmd := exec.Command(binaryPath,
		"generate",
		"--md", inputPath,
		"--output", outputPath,
		"--model", "gpt-4o-mini",
	)

	// Set environment variables
	cmd.Env = append(os.Environ(), "OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"))

	t.Log("Running generate command... this may take a minute")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	require.NoError(t, err, "Generate command failed: %s", outputStr)

	// Check output file exists
	_, err = os.Stat(outputPath)
	require.NoError(t, err, "Output file not created: %s", outputPath)

	// Check output file is not empty
	fileInfo, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, fileInfo.Size(), int64(0), "Output file is empty")

	t.Logf("✓ Generate command succeeded, index saved to: %s", outputPath)

	// Test search command with the generated index
	t.Run("search generated index", func(t *testing.T) {
		cmd := exec.Command(binaryPath,
			"search",
			"--index", outputPath,
			"--query", "What is the total revenue in 2023?",
		)
		cmd.Env = append(os.Environ(), "OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"))

		t.Log("Running search command...")
		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		require.NoError(t, err, "Search command failed: %s", outputStr)
		assert.NotEmpty(t, outputStr)
		assert.Contains(t, outputStr, "Answer:")
		assert.Contains(t, outputStr, "Referenced sections:")

		t.Log("✓ Search command succeeded")
	})
}
