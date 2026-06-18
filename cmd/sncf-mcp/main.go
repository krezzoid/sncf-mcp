// Command sncf-mcp is a Model Context Protocol server exposing the French
// railway (SNCF) journey-planning API to AI agents.
//
// Usage:
//
//	SNCF_API_KEY=xxxx sncf-mcp                 # stdio (default)
//	SNCF_API_KEY=xxxx sncf-mcp -transport http -addr :8080
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	// Embed the timezone database so station times render with the correct
	// Europe/Paris offset even on a minimal image (distroless) that ships no
	// zoneinfo. See ADR-0005.
	_ "time/tzdata"

	"github.com/krezzoid/sncf-mcp/internal/server"
)

func main() {
	var (
		transport = flag.String("transport", "stdio", "transport: stdio | http")
		addr      = flag.String("addr", ":8080", "listen address when -transport=http")
	)
	flag.Parse()

	// Structured logging to stderr. stdout is reserved for the stdio transport,
	// so nothing else may write there.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	apiKey := os.Getenv("SNCF_API_KEY")
	if apiKey == "" {
		slog.Error("SNCF_API_KEY is not set; get a free key at https://numerique.sncf.com/startup/api/")
		os.Exit(1)
	}

	// Cancel the context on SIGINT/SIGTERM for a clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := server.Config{APIKey: apiKey, Transport: *transport, HTTPAddr: *addr}

	slog.Info("starting sncf-mcp", "transport", *transport, "version", server.Version)
	if err := server.Run(ctx, cfg); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}
