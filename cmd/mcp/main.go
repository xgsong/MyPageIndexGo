package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/mcp"
)

var (
	transport    = flag.String("transport", "stdio", "Transport mode: stdio, http, or both")
	addr         = flag.String("addr", ":8080", "HTTP server address (only for http/both transport)")
	endpoint     = flag.String("endpoint", "/mcp", "MCP endpoint path (only for http/both transport)")
	authToken    = flag.String("auth-token", "", "Bearer token for authentication (only for http/both transport)")
	apiKey       = flag.String("api-key", "", "API Key for authentication (X-API-Key header, only for http/both transport)")
	sessionTTL   = flag.Duration("session-ttl", 30*time.Minute, "Session idle TTL (only for http/both transport)")
	enableCORS   = flag.Bool("enable-cors", true, "Enable CORS (only for http/both transport)")
	enableHealth = flag.Bool("enable-health", true, "Enable health endpoints (only for http/both transport)")
)

func main() {
	flag.Parse()

	log.Info().Str("version", mcp.MCPServerVersion).Msg("🚀 PageIndex MCP Server starting")

	mcpSrv := mcp.NewMCPServer()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	startServers(ctx, mcpSrv)

	log.Info().Msg("✅ MCP Server ready")

	<-ctx.Done()

	log.Info().Msg("🛑 Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if *transport == "http" || *transport == "both" {
		cfg := mcp.DefaultConfig()
		cfg.Addr = *addr
		httpSrv := mcp.NewHTTPServer(mcpSrv, cfg)
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error shutting down HTTP server")
		}
	}

	log.Info().Msg("👋 Goodbye!")
}

func startServers(ctx context.Context, mcpSrv *mcp.MCPServer) {
	switch *transport {
	case "stdio":
		log.Info().Msg("📡 Using stdio transport")
		stdioSrv := mcp.NewStdioServer(mcpSrv)
		go func() {
			if err := stdioSrv.Start(); err != nil {
				log.Error().Err(err).Msg("❌ Stdio server error")
			}
		}()

	case "http":
		log.Info().Msg("🌐 Using HTTP transport")
		httpSrv := createHTTPServer(mcpSrv)
		go func() {
			if err := httpSrv.Start(); err != nil {
				log.Error().Err(err).Msg("❌ HTTP server error")
			}
		}()

	case "both":
		log.Info().Msg("🔄 Using both stdio and HTTP transports")
		stdioSrv := mcp.NewStdioServer(mcpSrv)
		httpSrv := createHTTPServer(mcpSrv)
		go func() {
			if err := stdioSrv.Start(); err != nil {
				log.Error().Err(err).Msg("❌ Stdio server error")
			}
		}()
		go func() {
			if err := httpSrv.Start(); err != nil {
				log.Error().Err(err).Msg("❌ HTTP server error")
			}
		}()

	default:
		log.Fatal().Str("transport", *transport).Msg("❌ Invalid transport mode")
	}
}

func createHTTPServer(mcpSrv *mcp.MCPServer) *mcp.HTTPServer {
	cfg := mcp.DefaultConfig()
	cfg.Addr = *addr
	cfg.Endpoint = *endpoint
	cfg.AuthToken = *authToken
	cfg.APIKey = *apiKey
	cfg.SessionTTL = *sessionTTL
	cfg.EnableCORS = *enableCORS
	cfg.EnableHealth = *enableHealth

	return mcp.NewHTTPServer(mcpSrv, cfg)
}
