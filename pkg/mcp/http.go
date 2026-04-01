package mcp

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Addr         string
	Endpoint     string
	BaseURL      string
	AuthToken    string
	APIKey       string
	SessionTTL   time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	EnableCORS   bool
	EnableHealth bool
}

func DefaultConfig() *Config {
	return &Config{
		Addr:         ":8080",
		Endpoint:     "/mcp",
		SessionTTL:   30 * time.Minute,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  180 * time.Second,
		EnableCORS:   true,
		EnableHealth: true,
	}
}

// HTTPServer wraps the Streamable HTTP MCP server.
type HTTPServer struct {
	config   *Config
	mcpSrv   *server.MCPServer
	httpSrv  *server.StreamableHTTPServer
	httpAddr *http.Server
}

// NewHTTPServer creates a new Streamable HTTP MCP server.
func NewHTTPServer(mcpSrv *server.MCPServer, cfg *Config) *HTTPServer {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Create Streamable HTTP server with options
	opts := []server.StreamableHTTPOption{
		server.WithEndpointPath(cfg.Endpoint),
		server.WithSessionIdleTTL(cfg.SessionTTL),
	}

	httpSrv := server.NewStreamableHTTPServer(mcpSrv, opts...)

	// Create HTTP server with custom handler
	mux := http.NewServeMux()

	// Register MCP endpoint
	mux.Handle(cfg.Endpoint, httpSrv)

	// Add authentication middleware if configured
	var handler http.Handler = mux
	if cfg.AuthToken != "" || cfg.APIKey != "" {
		handler = withAuthentication(*cfg, mux)
	}

	// Add CORS middleware if enabled
	if cfg.EnableCORS {
		handler = withCORS(handler)
	}

	// Add health endpoint if enabled
	if cfg.EnableHealth {
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","server":"MyPageIndexGo"}`))
		})
		mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready","server":"MyPageIndexGo"}`))
		})
	}

	httpAddr := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &HTTPServer{
		config:   cfg,
		mcpSrv:   mcpSrv,
		httpSrv:  httpSrv,
		httpAddr: httpAddr,
	}
}

// Start begins serving HTTP connections.
func (s *HTTPServer) Start() error {
	log.Info().
		Str("addr", s.config.Addr).
		Str("endpoint", s.config.Endpoint).
		Str("transport", "streamable_http").
		Msg("🚀 PageIndex MCP Server (HTTP) starting")

	if s.config.EnableHealth {
		log.Info().
			Str("url", fmt.Sprintf("http://localhost%s/health", s.config.Addr)).
			Msg("🏥 Health endpoint enabled")
	}

	if s.config.AuthToken != "" || s.config.APIKey != "" {
		log.Info().Msg("🔒 Authentication enabled")
	} else {
		log.Warn().Msg("⚠️  Authentication disabled - NOT recommended for production")
	}

	log.Info().
		Str("url", fmt.Sprintf("http://localhost%s%s", s.config.Addr, s.config.Endpoint)).
		Msg("📡 MCP endpoint ready")

	return s.httpAddr.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	log.Info().Msg("🛑 Shutting down HTTP server...")
	return s.httpAddr.Shutdown(ctx)
}

// GetEndpoint returns the full MCP endpoint URL.
func (s *HTTPServer) GetEndpoint() string {
	return fmt.Sprintf("http://localhost%s%s", s.config.Addr, s.config.Endpoint)
}

func (s *HTTPServer) GetEndpoints() map[string]string {
	return map[string]string{
		"mcp":    s.GetEndpoint(),
		"health": fmt.Sprintf("http://localhost%s/health", s.config.Addr),
		"ready":  fmt.Sprintf("http://localhost%s/ready", s.config.Addr),
	}
}

// withAuthentication creates authentication middleware.
func withAuthentication(cfg Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/ready" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip OPTIONS requests (CORS preflight)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Try Bearer token authentication
		if cfg.AuthToken != "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				expected := "Bearer " + cfg.AuthToken
				if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Try API Key authentication
		if cfg.APIKey != "" {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				if subtle.ConstantTimeCompare([]byte(apiKey), []byte(cfg.APIKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Authentication failed or not provided
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// withCORS creates CORS middleware.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, Mcp-Session-Id, Accept")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
