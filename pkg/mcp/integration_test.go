package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamableHTTPClientIntegration(t *testing.T) {
	mcpSrv := NewMCPServer()
	cfg := &Config{
		Addr:         ":18086",
		Endpoint:     "/mcp",
		EnableHealth: true,
		EnableCORS:   true,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	errChan := make(chan error, 1)
	go func() {
		err := httpSrv.Start()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		select {
		case <-errChan:
		case <-time.After(3 * time.Second):
		}
	})

	t.Run("health endpoint returns healthy status", func(t *testing.T) {
		resp, err := http.Get("http://localhost:18086/health")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var body map[string]string
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "healthy", body["status"])
	})

	t.Run("ready endpoint returns ready status", func(t *testing.T) {
		resp, err := http.Get("http://localhost:18086/ready")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]string
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "ready", body["status"])
	})

	t.Run("MCP endpoint accepts POST requests", func(t *testing.T) {
		mcpPayload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		bodyBytes, err := json.Marshal(mcpPayload)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "http://localhost:18086/mcp", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("CORS headers are present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rr := httptest.NewRecorder()

		httpSrv.httpAddr.Handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestAuthenticationIntegration(t *testing.T) {
	mcpSrv := NewMCPServer()
	cfg := &Config{
		Addr:         ":18087",
		Endpoint:     "/mcp",
		AuthToken:    "test-bearer-token",
		APIKey:       "test-api-key-123",
		EnableHealth: true,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	errChan := make(chan error, 1)
	go func() {
		err := httpSrv.Start()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		select {
		case <-errChan:
		case <-time.After(3 * time.Second):
		}
	})

	t.Run("Bearer token authentication succeeds", func(t *testing.T) {
		mcpPayload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		}
		bodyBytes, _ := json.Marshal(mcpPayload)

		req, _ := http.NewRequest(http.MethodPost, "http://localhost:18087/mcp", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer test-bearer-token")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("API Key authentication succeeds", func(t *testing.T) {
		mcpPayload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		}
		bodyBytes, _ := json.Marshal(mcpPayload)

		req, _ := http.NewRequest(http.MethodPost, "http://localhost:18087/mcp", bytes.NewReader(bodyBytes))
		req.Header.Set("X-API-Key", "test-api-key-123")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("No authentication fails", func(t *testing.T) {
		mcpPayload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params":  map[string]interface{}{},
		}
		bodyBytes, _ := json.Marshal(mcpPayload)

		req, _ := http.NewRequest(http.MethodPost, "http://localhost:18087/mcp", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Health endpoint accessible without auth", func(t *testing.T) {
		resp, err := http.Get("http://localhost:18087/health")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestConcurrentClientConnections(t *testing.T) {
	mcpSrv := NewMCPServer()
	cfg := &Config{
		Addr:         ":18088",
		Endpoint:     "/mcp",
		EnableHealth: true,
		EnableCORS:   true,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	errChan := make(chan error, 1)
	go func() {
		err := httpSrv.Start()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		select {
		case <-errChan:
		case <-time.After(3 * time.Second):
		}
	})

	concurrentClients := 10
	results := make(chan bool, concurrentClients)

	for i := 0; i < concurrentClients; i++ {
		go func(clientID int) {
			resp, err := http.Get("http://localhost:18088/health")
			if err != nil {
				results <- false
				return
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				results <- false
				return
			}

			var bodyMap map[string]string
			err = json.Unmarshal(body, &bodyMap)
			if err != nil {
				results <- false
				return
			}

			results <- (bodyMap["status"] == "healthy")
		}(i)
	}

	successCount := 0
	for i := 0; i < concurrentClients; i++ {
		if <-results {
			successCount++
		}
	}

	assert.Equal(t, concurrentClients, successCount, "All concurrent clients should receive healthy response")
}

func TestSessionIdleTTL(t *testing.T) {
	mcpSrv := NewMCPServer()
	cfg := &Config{
		Addr:       ":18089",
		Endpoint:   "/mcp",
		SessionTTL: 1 * time.Second,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	assert.Equal(t, 1*time.Second, httpSrv.config.SessionTTL)
}

func TestHTTPErrorHandler(t *testing.T) {
	mcpSrv := NewMCPServer()
	cfg := &Config{
		Addr:     ":18090",
		Endpoint: "/mcp",
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	errChan := make(chan error, 1)
	go func() {
		err := httpSrv.Start()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
		select {
		case <-errChan:
		case <-time.After(3 * time.Second):
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "http://localhost:18090/mcp", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("non-existent endpoint returns 404", func(t *testing.T) {
		resp, err := http.Get("http://localhost:18090/nonexistent")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
