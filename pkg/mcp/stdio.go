package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"
)

// StdioServer wraps the stdio MCP server.
type StdioServer struct {
	mcpSrv *server.MCPServer
	ctx    context.Context
	cancel context.CancelFunc
}

// NewStdioServer creates a new stdio MCP server.
func NewStdioServer(ctx context.Context, mcpSrv *server.MCPServer) *StdioServer {
	ctx, cancel := context.WithCancel(ctx)
	return &StdioServer{
		mcpSrv: mcpSrv,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *StdioServer) Start() error {
	log.Info().
		Str("transport", "stdio").
		Msg("🚀 PageIndex MCP Server (stdio) starting")
	log.Info().
		Str("stdin", os.Stdin.Name()).
		Str("stdout", os.Stdout.Name()).
		Msg("📡 Waiting for connections on stdin/stdout")

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ServeStdio(s.mcpSrv)
	}()

	select {
	case err := <-errChan:
		return err
	case <-s.ctx.Done():
		return s.Shutdown(s.ctx)
	}
}

// Shutdown gracefully stops the stdio server.
func (s *StdioServer) Shutdown(ctx context.Context) error {
	log.Info().Msg("🛑 Shutting down stdio server...")
	fmt.Fprintln(os.Stderr, "👋 Goodbye!")
	return nil
}
