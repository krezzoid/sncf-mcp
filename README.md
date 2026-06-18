# sncf-mcp

A [Model Context Protocol](https://modelcontextprotocol.io/) server that gives
AI agents structured access to the French railway network (SNCF) — journey
planning, station lookup, departures, and disruptions — through the official
SNCF / [Navitia](https://doc.navitia.io/) open-data API.

Built in Go on the official [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk).

> **Why this exists.** I've led engineering teams for a long time and wanted to stay close to the layer that actually makes AI agents useful:
> their tooling. MCP is that layer. This project is a deliberately small, production-shaped server — the interesting part is not the API wrapper, it's
> the engineering envelope around it (typed client, hermetic tests, CI, security posture, and the design decisions written down as ADRs).

## What it does

The server exposes a small, focused set of tools rather than a thin mirror of every API endpoint:

- **`find_station`** — resolve a free-text name ("Lyon Part Dieu") to station candidates.
- **`plan_journey`** — itineraries between two locations, with optional departure/arrival time.
- **`next_departures`** — upcoming departures from a station, with real-time delays.
- **`get_disruptions`** — active service disruptions, optionally scoped to a station.

## Example

An agent disambiguates a station, then plans a journey. The output is the lean,
context-friendly shape, not the raw Navitia payload (see [ADR-0005](docs/adr/0005-lean-output-contract.md)):

`find_station({"query": "Lyon"})` →

```json
{
  "stations": [
    { "id": "stop_area:SNCF:87723197", "name": "Lyon Part-Dieu (Lyon)", "quality": 90 },
    { "id": "stop_area:SNCF:87722025", "name": "Lyon Perrache (Lyon)", "quality": 70 }
  ]
}
```

`plan_journey({"from": "Paris Gare de Lyon", "to": "stop_area:SNCF:87723197"})` →

```json
{
  "journeys": [
    {
      "departure": "2026-06-20T14:00:00+02:00",
      "arrival": "2026-06-20T15:56:00+02:00",
      "duration_minutes": 116,
      "transfers": 0,
      "legs": [
        {
          "mode": "TGV INOUI", "train": "6607",
          "from": "Paris Gare de Lyon", "to": "Lyon Part Dieu",
          "departure": "2026-06-20T14:00:00+02:00", "arrival": "2026-06-20T15:56:00+02:00"
        }
      ]
    }
  ]
}
```

When a name can't be resolved, the tool returns a helpful message (not a raw
error) steering the agent to rephrase or call `find_station` first.

## Design decisions

The choices that distinguish this from a quick wrapper are documented as Architecture Decision Records in [`docs/adr/`](docs/adr/):

1. [Station resolution via the `/places` endpoint, not a bundled CSV](docs/adr/0001-station-resolution-places-vs-csv.md)
2. [Deliberately not scraping ticket prices](docs/adr/0002-no-price-scraping.md)
3. [The official Go SDK over `mark3labs/mcp-go`](docs/adr/0003-official-go-sdk-vs-mark3labs.md)
4. [stdio as the default transport, HTTP as opt-in](docs/adr/0004-transport-stdio-vs-streamable-http.md)
5. [A stable, lean output contract for tool results](docs/adr/0005-lean-output-contract.md)
6. [Reactive retry with jittered backoff; no proactive rate limiter](docs/adr/0006-retry-and-rate-limiting.md)

A key piece of value is the **transform layer** (`internal/transform`): Navitia responses are large and deeply nested, which wastes an LLM's context window, so
the server projects them down to a compact shape carrying only what an agent needs to reason about a trip.

The full build sequence and checkpoints live in [`docs/PROJECT_PLAN.md`](docs/PROJECT_PLAN.md).

## Architecture

```
cmd/sncf-mcp/       entrypoint: config, logging, graceful shutdown
internal/navitia/   typed, context-aware API client (decoupled from MCP)
internal/transform/ projects heavy Navitia JSON into lean, LLM-friendly output
internal/tools/     MCP tool handlers (thin orchestration only)
internal/server/    wires tools into the SDK server and selects transport
testdata/           JSON fixtures for hermetic tests
docs/               Documentations and Architecture Decision Records
```

The `navitia` package has no MCP dependency, so it is unit-tested in isolation against an `httptest` server. No test touches the network.

## Getting started

You need Go 1.26+ and a free SNCF API key (<https://numerique.sncf.com/startup/api/>).

```sh
# Build and test from source
make build
make test

# Run over stdio
SNCF_API_KEY=your-key ./sncf-mcp
```

Or run it in a container (distroless, ~10 MB):

```sh
docker build -t sncf-mcp .
docker run --rm -i -e SNCF_API_KEY=your-key sncf-mcp
```

> Released binaries (via GoReleaser) and `go install` are served from a public
> mirror repository; see [docs/RELEASING.md](docs/RELEASING.md).

### Use with Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sncf": {
      "command": "/absolute/path/to/sncf-mcp",
      "env": { "SNCF_API_KEY": "your-key" }
    }
  }
}
```

## Security

- The API key is read only from the `SNCF_API_KEY` environment variable and is never logged. It is sent as an HTTP Basic auth header (never in a URL), and API errors are regression-tested not to contain it.
- Logs are structured (`slog`) and go to **stderr**; **stdout** is reserved for the stdio transport, so nothing leaks into the protocol stream.
- Tool inputs are validated: empty, unresolved, or malformed arguments return a clear message rather than a panic or a raw error.
- The container image is built `FROM scratch`-style (distroless, nonroot, no shell) for a minimal attack surface.
- CI runs `golangci-lint` (including `gosec`) and `govulncheck` on every push.

## Limitations

- **No ticket prices.** They are not part of the SNCF open-data API, and this server does not scrape them — see [ADR-0002](docs/adr/0002-no-price-scraping.md).
- Data covers TGV, TER, Transilien, and Intercités (theoretical + real-time).
- Times are local to **Europe/Paris**, rendered in RFC3339 with the correct offset (e.g. `…+02:00`).
- On 429/5xx the client retries with jittered backoff and honors `Retry-After` ([ADR-0006](docs/adr/0006-retry-and-rate-limiting.md)); it does not pre-emptively rate-limit, so please still respect the API's limits.

## Status

All four tools — `find_station`, `plan_journey`, `next_departures`, and
`get_disruptions` — are implemented, hardened (retry/backoff, input validation,
timezone-correct output), and covered by hermetic and golden tests.

## License

MIT — see [LICENSE](LICENSE).
