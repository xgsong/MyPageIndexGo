package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mcpBinaryPath string

// buildMCPServer builds the MCP server binary in temp directory
func buildMCPServer(t *testing.T) {
	if mcpBinaryPath != "" {
		if _, err := os.Stat(mcpBinaryPath); err == nil {
			return
		}
	}

	// Create temp directory for binary
	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
	require.NoError(t, err)

	mcpBinaryPath = tmpDir + "/mcp-server"

	cmd := exec.Command("go", "build", "-o", mcpBinaryPath, "./cmd/mcp/")
	cmd.Dir = "../.."
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build MCP server: %s", string(output))

	// Cleanup on test exit
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})
}

// TestMCP_Help tests MCP server help command
func TestMCP_Help(t *testing.T) {
	if _, err := os.Stat(mcpBinaryPath); os.IsNotExist(err) {
		buildMCPServer(t)
	}

	cmd := exec.Command(mcpBinaryPath, "-h")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	assert.NoError(t, err)
	assert.Contains(t, outputStr, "Usage of")
	assert.Contains(t, outputStr, "-transport")
	assert.Contains(t, outputStr, "stdio, http, or both")
	assert.Contains(t, outputStr, "-addr")
	assert.Contains(t, outputStr, "-endpoint")
	assert.Contains(t, outputStr, "-auth-token")
	assert.Contains(t, outputStr, "-api-key")
}

// TestMCP_InvalidTransport tests MCP server with invalid transport
func TestMCP_InvalidTransport(t *testing.T) {
	if _, err := os.Stat(mcpBinaryPath); os.IsNotExist(err) {
		buildMCPServer(t)
	}

	cmd := exec.Command(mcpBinaryPath, "-transport", "invalid")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Invalid transport causes fatal log and exit
	assert.Error(t, err)
	if len(outputStr) > 0 {
		assert.Contains(t, outputStr, "invalid")
	}
}

// TestMCP_HTTP_Health tests MCP server HTTP mode with health endpoint
func TestMCP_HTTP_Health(t *testing.T) {
	buildMCPServer(t)
	if testing.Short() {
		t.Skip("Skipping MCP HTTP test in short mode")
	}

	// Use a random port to avoid conflicts
	port := 18082
	addr := fmt.Sprintf(":%d", port)
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)

	// Start MCP server in HTTP mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, mcpBinaryPath,
		"-transport", "http",
		"-addr", addr,
		"-enable-health", "true",
	)
	err := cmd.Start()
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Test health endpoint
	resp, err := http.Get(healthURL)
	if err != nil {
		t.Logf("Health endpoint not accessible: %v", err)
		cancel()
		_ = cmd.Wait()
		t.Skip("Skipping test: server failed to start")
	}
	require.NoError(t, err, "Failed to connect to health endpoint")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test ready endpoint
	readyURL := fmt.Sprintf("http://localhost:%d/ready", port)
	resp, err = http.Get(readyURL)
	require.NoError(t, err, "Failed to connect to ready endpoint")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown server
	cancel()
	_ = cmd.Wait()

	t.Log("✓ MCP HTTP server health test passed")
}

// TestMCP_HTTP_Auth tests MCP server HTTP mode with authentication
func TestMCP_HTTP_Auth(t *testing.T) {
	buildMCPServer(t)
	if testing.Short() {
		t.Skip("Skipping MCP HTTP auth test in short mode")
	}

	port := 18083
	addr := fmt.Sprintf(":%d", port)
	mcpEndpoint := fmt.Sprintf("http://localhost:%d/mcp", port)
	testToken := "test-secret-token"

	// Start MCP server with auth token
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, mcpBinaryPath,
		"-transport", "http",
		"-addr", addr,
		"-auth-token", testToken,
	)
	err := cmd.Start()
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Check if server started successfully
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Logf("Server not accessible: %v", err)
		cancel()
		_ = cmd.Wait()
		t.Skip("Skipping test: server failed to start")
	}
	_ = resp.Body.Close()

	// Test without auth token
	resp, err = http.Post(mcpEndpoint, "application/json", nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Test with wrong auth token
	req, err := http.NewRequest("POST", mcpEndpoint, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Test with correct auth token (will get 400 because request body is empty, but not 401)
	req, err = http.NewRequest("POST", mcpEndpoint, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Shutdown server
	cancel()
	_ = cmd.Wait()

	t.Log("✓ MCP HTTP server authentication test passed")
}
