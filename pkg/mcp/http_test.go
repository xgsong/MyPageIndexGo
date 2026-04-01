package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Equal(t, "/mcp", cfg.Endpoint)
	assert.Equal(t, 30*time.Minute, cfg.SessionTTL)
	assert.Equal(t, 60*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 120*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 180*time.Second, cfg.IdleTimeout)
	assert.True(t, cfg.EnableCORS)
	assert.True(t, cfg.EnableHealth)
}

func TestNewHTTPServer(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")

	t.Run("with default config", func(t *testing.T) {
		httpSrv := NewHTTPServer(mcpSrv, nil)
		require.NotNil(t, httpSrv)
		assert.Equal(t, ":8080", httpSrv.config.Addr)
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			Addr:         ":9090",
			Endpoint:     "/api/mcp",
			AuthToken:    "test-token",
			APIKey:       "test-api-key",
			SessionTTL:   15 * time.Minute,
			EnableCORS:   false,
			EnableHealth: false,
		}
		httpSrv := NewHTTPServer(mcpSrv, cfg)
		require.NotNil(t, httpSrv)
		assert.Equal(t, ":9090", httpSrv.config.Addr)
		assert.Equal(t, "/api/mcp", httpSrv.config.Endpoint)
		assert.Equal(t, "test-token", httpSrv.config.AuthToken)
		assert.Equal(t, "test-api-key", httpSrv.config.APIKey)
		assert.Equal(t, 15*time.Minute, httpSrv.config.SessionTTL)
		assert.False(t, httpSrv.config.EnableCORS)
		assert.False(t, httpSrv.config.EnableHealth)
	})
}

func TestHTTPServerGetEndpoint(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")
	cfg := &Config{
		Addr:     ":8080",
		Endpoint: "/mcp",
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	endpoint := httpSrv.GetEndpoint()
	assert.Equal(t, "http://localhost:8080/mcp", endpoint)
}

func TestHTTPServerGetEndpoints(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")
	cfg := &Config{
		Addr:         ":8080",
		Endpoint:     "/mcp",
		EnableHealth: true,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	endpoints := httpSrv.GetEndpoints()
	require.Contains(t, endpoints, "mcp")
	require.Contains(t, endpoints, "health")
	require.Contains(t, endpoints, "ready")

	assert.Equal(t, "http://localhost:8080/mcp", endpoints["mcp"])
	assert.Equal(t, "http://localhost:8080/health", endpoints["health"])
	assert.Equal(t, "http://localhost:8080/ready", endpoints["ready"])
}

func testAuthenticationScenario(t *testing.T, cfg Config, setValidAuth func(*http.Request), setInvalidAuth func(*http.Request)) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	authHandler := withAuthentication(cfg, handler)

	t.Run("valid credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		setValidAuth(req)
		rr := httptest.NewRecorder()

		authHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		setInvalidAuth(req)
		rr := httptest.NewRecorder()

		authHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("no credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		rr := httptest.NewRecorder()

		authHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestWithAuthentication(t *testing.T) {
	t.Run("Bearer token authentication", func(t *testing.T) {
		cfg := Config{
			AuthToken: "secret-token",
		}
		testAuthenticationScenario(t, cfg,
			func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer secret-token")
			},
			func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer wrong-token")
			},
		)
	})

	t.Run("API Key authentication", func(t *testing.T) {
		cfg := Config{
			APIKey: "secret-api-key",
		}
		testAuthenticationScenario(t, cfg,
			func(req *http.Request) {
				req.Header.Set("X-API-Key", "secret-api-key")
			},
			func(req *http.Request) {
				req.Header.Set("X-API-Key", "wrong-key")
			},
		)
	})

	t.Run("skip auth for health endpoints", func(t *testing.T) {
		cfg := Config{
			AuthToken: "secret-token",
			APIKey:    "secret-api-key",
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authHandler := withAuthentication(cfg, handler)

		t.Run("health endpoint", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rr := httptest.NewRecorder()

			authHandler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})

		t.Run("ready endpoint", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rr := httptest.NewRecorder()

			authHandler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	})

	t.Run("skip auth for OPTIONS requests", func(t *testing.T) {
		cfg := Config{
			AuthToken: "secret-token",
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		authHandler := withAuthentication(cfg, handler)

		req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
		rr := httptest.NewRecorder()

		authHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestWithCORS(t *testing.T) {
	cfg := &Config{
		Addr:         ":8080",
		Endpoint:     "/mcp",
		EnableCORS:   true,
		EnableHealth: true,
	}
	httpSrv := NewHTTPServer(server.NewMCPServer("test", "1.0.0"), cfg)

	t.Run("CORS headers present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		rr := httptest.NewRecorder()

		httpSrv.httpAddr.Handler.ServeHTTP(rr, req)

		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), "POST")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Contains(t, rr.Header().Get("Access-Control-Expose-Headers"), "Mcp-Session-Id")
	})

	t.Run("OPTIONS preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
		rr := httptest.NewRecorder()

		httpSrv.httpAddr.Handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestHealthEndpoints(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")
	cfg := &Config{
		Addr:         ":8080",
		Endpoint:     "/mcp",
		EnableHealth: true,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	t.Run("health endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		httpSrv.httpAddr.Handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var response map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "MyPageIndexGo", response["server"])
	})

	t.Run("ready endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()

		httpSrv.httpAddr.Handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var response map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "ready", response["status"])
		assert.Equal(t, "MyPageIndexGo", response["server"])
	})
}

func TestConfigValidation(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")

	t.Run("empty config uses defaults", func(t *testing.T) {
		cfg := DefaultConfig()
		httpSrv := NewHTTPServer(mcpSrv, cfg)
		require.NotNil(t, httpSrv)
		assert.Equal(t, ":8080", httpSrv.config.Addr)
		assert.Equal(t, "/mcp", httpSrv.config.Endpoint)
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		httpSrv := NewHTTPServer(mcpSrv, nil)
		require.NotNil(t, httpSrv)
		assert.Equal(t, ":8080", httpSrv.config.Addr)
		assert.Equal(t, "/mcp", httpSrv.config.Endpoint)
	})

	t.Run("custom endpoint path", func(t *testing.T) {
		cfg := &Config{
			Endpoint: "/api/v1/mcp",
		}
		httpSrv := NewHTTPServer(mcpSrv, cfg)
		require.NotNil(t, httpSrv)
		assert.Equal(t, "/api/v1/mcp", httpSrv.config.Endpoint)
	})
}

func TestSessionManagement(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")
	cfg := &Config{
		Addr:       ":18082",
		Endpoint:   "/mcp",
		SessionTTL: 5 * time.Minute,
	}
	httpSrv := NewHTTPServer(mcpSrv, cfg)

	assert.Equal(t, 5*time.Minute, httpSrv.config.SessionTTL)
}

func TestServerConcurrency(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0.0")

	t.Run("multiple server instances", func(t *testing.T) {
		cfg1 := &Config{Addr: ":18083", Endpoint: "/mcp1"}
		cfg2 := &Config{Addr: ":18084", Endpoint: "/mcp2"}

		httpSrv1 := NewHTTPServer(mcpSrv, cfg1)
		httpSrv2 := NewHTTPServer(mcpSrv, cfg2)

		require.NotNil(t, httpSrv1)
		require.NotNil(t, httpSrv2)

		assert.Equal(t, ":18083", httpSrv1.config.Addr)
		assert.Equal(t, ":18084", httpSrv2.config.Addr)
		assert.Equal(t, "/mcp1", httpSrv1.config.Endpoint)
		assert.Equal(t, "/mcp2", httpSrv2.config.Endpoint)
	})
}

func TestStdioServer(t *testing.T) {
	t.Run("creation", func(t *testing.T) {
		mcpSrv := server.NewMCPServer("test", "1.0.0")
		stdioSrv := NewStdioServer(mcpSrv)
		require.NotNil(t, stdioSrv)
	})

	t.Run("shutdown", func(t *testing.T) {
		mcpSrv := server.NewMCPServer("test", "1.0.0")
		stdioSrv := NewStdioServer(mcpSrv)

		ctx := context.Background()
		err := stdioSrv.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestMCPServerIntegration(t *testing.T) {
	t.Run("full HTTP server setup with tools", func(t *testing.T) {
		mcpSrv := NewMCPServer()
		require.NotNil(t, mcpSrv)

		cfg := &Config{
			Addr:         ":18085",
			Endpoint:     "/mcp",
			EnableHealth: true,
		}
		httpSrv := NewHTTPServer(mcpSrv, cfg)
		require.NotNil(t, httpSrv)

		assert.Equal(t, ":18085", httpSrv.config.Addr)
		assert.Equal(t, "/mcp", httpSrv.config.Endpoint)
	})

	t.Run("server name and version constants", func(t *testing.T) {
		assert.Equal(t, "MyPageIndexGo", MCPServerName)
		assert.Equal(t, "1.0.0", MCPServerVersion)
	})
}
