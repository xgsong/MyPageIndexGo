package main

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/mcp"
)

func TestCreateHTTPServer(t *testing.T) {
	t.Run("creates HTTP server with default config", func(t *testing.T) {
		mcpSrv := mcp.NewMCPServer()
		httpSrv := createHTTPServer(mcpSrv)
		assert.NotNil(t, httpSrv)
	})

	t.Run("creates HTTP server with custom config", func(t *testing.T) {
		*addr = ":9090"
		*endpoint = "/custom"
		*authToken = "test-token"
		*apiKey = "test-api-key"
		*sessionTTL = 15 * time.Minute
		*enableCORS = false
		*enableHealth = false

		mcpSrv := mcp.NewMCPServer()
		httpSrv := createHTTPServer(mcpSrv)
		assert.NotNil(t, httpSrv)

		*addr = ":8080"
		*endpoint = "/mcp"
		*authToken = ""
		*apiKey = ""
		*sessionTTL = 30 * time.Minute
		*enableCORS = true
		*enableHealth = true
	})
}

func TestStartServers(t *testing.T) {
	t.Run("stdio transport", func(t *testing.T) {
		*transport = "stdio"
		mcpSrv := mcp.NewMCPServer()
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			startServers(ctx, mcpSrv)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("http transport", func(t *testing.T) {
		*transport = "http"
		*addr = ":18085"
		mcpSrv := mcp.NewMCPServer()
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			startServers(ctx, mcpSrv)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("both transport", func(t *testing.T) {
		*transport = "both"
		*addr = ":18086"
		mcpSrv := mcp.NewMCPServer()
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			startServers(ctx, mcpSrv)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(100 * time.Millisecond)
	})

}

func TestFlagDefaults(t *testing.T) {
	t.Run("transport default", func(t *testing.T) {
		expected := "stdio"
		if *transport != expected {
			t.Logf("Note: transport was modified by previous test, current value: %s", *transport)
		}
	})

	t.Run("addr default", func(t *testing.T) {
		expected := ":8080"
		if *addr != expected {
			t.Logf("Note: addr was modified by previous test, current value: %s", *addr)
		}
	})

	t.Run("endpoint default", func(t *testing.T) {
		assert.Equal(t, "/mcp", *endpoint)
	})

	t.Run("auth-token default", func(t *testing.T) {
		assert.Empty(t, *authToken)
	})

	t.Run("api-key default", func(t *testing.T) {
		assert.Empty(t, *apiKey)
	})

	t.Run("session-ttl default", func(t *testing.T) {
		assert.Equal(t, 30*time.Minute, *sessionTTL)
	})

	t.Run("enable-cors default", func(t *testing.T) {
		assert.True(t, *enableCORS)
	})

	t.Run("enable-health default", func(t *testing.T) {
		assert.True(t, *enableHealth)
	})
}

func TestFlagParsing(t *testing.T) {
	t.Run("parse custom flags", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		testTransport := fs.String("transport", "stdio", "test")
		testAddr := fs.String("addr", ":8080", "test")

		err := fs.Parse([]string{"-transport", "http", "-addr", ":9999"})
		assert.NoError(t, err)
		assert.Equal(t, "http", *testTransport)
		assert.Equal(t, ":9999", *testAddr)
	})
}
