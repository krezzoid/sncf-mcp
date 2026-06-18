// Package server wires the tool handlers into an MCP server and runs it over
// the selected transport. Transport choice is discussed in ADR-0004; the
// default is stdio, which is what local MCP clients (Claude Desktop, Cursor)
// launch directly.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
	"github.com/krezzoid/sncf-mcp/internal/tools"
)

// Version is the server version reported in the MCP implementation info. It is
// "dev" for local builds and stamped to the release tag at build time via
// -ldflags "-X .../internal/server.Version=..." (see docs/RELEASING.md).
var Version = "dev"

// Config holds runtime configuration for the server.
type Config struct {
	APIKey    string // SNCF API key
	Transport string // "stdio" (default) or "http"
	HTTPAddr  string // listen address when Transport == "http", e.g. ":8080"
}

// build constructs the MCP server and registers the available tools.
//
// NOTE: the exact SDK constructor/transport names should be confirmed against
// the go-sdk version pinned in go.mod (run `go doc github.com/modelcontextprotocol/go-sdk/mcp`).
// The AddTool / handler signatures below follow the official v1 examples.
func build(cfg Config) *mcp.Server {
	client := navitia.New(cfg.APIKey)
	h := tools.New(client)

	srv := mcp.NewServer(&mcp.Implementation{Name: "sncf-mcp", Version: Version}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_station",
		Description: "Resolve a station or city name to ranked SNCF station candidates. Use it to disambiguate a name (e.g. 'Lyon') before plan_journey, then pass a returned id.",
	}, h.FindStation)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "plan_journey",
		Description: "Plan train itineraries between two locations on the French rail network. Accepts station/city names (resolved automatically) or stop_area ids from find_station.",
	}, h.PlanJourney)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "next_departures",
		Description: "List upcoming departures from a station on the French rail network, with real-time delays.",
	}, h.NextDepartures)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_disruptions",
		Description: "List active service disruptions on the French rail network, optionally scoped to a station.",
	}, h.GetDisruptions)

	return srv
}

// Run builds the server and serves it over the configured transport, blocking
// until the context is cancelled or the client disconnects.
func Run(ctx context.Context, cfg Config) error {
	srv := build(cfg)

	switch cfg.Transport {
	case "", "stdio":
		// Local clients launch the process and speak over stdin/stdout.
		return srv.Run(ctx, &mcp.StdioTransport{})
	case "http":
		// Streamable HTTP for remote/hosted use. A single shared server here;
		// see ADR-0004 for the statefulness caveats before scaling this out.
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		// ReadHeaderTimeout guards against Slowloris-style attacks (gosec G114).
		httpSrv := &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}
		return httpSrv.ListenAndServe()
	default:
		return fmt.Errorf("unknown transport %q (want 'stdio' or 'http')", cfg.Transport)
	}
}
