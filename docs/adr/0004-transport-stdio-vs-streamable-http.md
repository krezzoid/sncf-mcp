# ADR-0004: Default to stdio transport; HTTP is opt-in

**Status:** Accepted
**Date:** 2026-06-16
**Deciders:** Project author

## Context

MCP servers can speak over different transports. The official Go SDK supports
two relevant ones:

- **stdio** — the client launches the server as a subprocess and communicates
  over stdin/stdout. This is how local clients (Claude Desktop, Cursor) work.
- **Streamable HTTP** — the server runs as a network service that clients
  connect to over HTTP. Suited to remote/hosted, multi-user scenarios.

A known ecosystem caveat: Streamable HTTP currently leans on stateful
connections, which sits awkwardly with standard cloud patterns (load balancers,
autoscaling, serverless). A stateless transport is the highest-priority item on
the MCP roadmap, but until it lands, hosting an HTTP MCP server is more involved
than the demos suggest.

This project's primary user is a developer running it locally against their own
SNCF API key. It is a portfolio/utility server, not a hosted multi-tenant
service.

## Decision

Default to **stdio**. Provide **Streamable HTTP** behind an explicit
`-transport http` flag for users who want remote access, but treat it as
advanced/secondary and document the statefulness caveat. Do not invest in
production HTTP hosting (auth, scaling, multi-tenant key handling) at this stage.

## Options Considered

### Option A: stdio only

| Dimension | Assessment |
|-----------|------------|
| Complexity | Lowest |
| Fits primary use | Yes — local clients |
| Hosting story | None |
| Deployment risk | None |

**Pros:** simplest; zero network surface; matches how the target clients work.
**Cons:** no remote/shared use at all.

### Option B: stdio default + HTTP opt-in (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one extra branch + handler |
| Fits primary use | Yes — stdio default |
| Hosting story | Possible, with documented caveats |
| Deployment risk | Contained — HTTP is opt-in and unsupported for scale |

**Pros:** great local default; an escape hatch for remote use without committing
to a hosting story; demonstrates awareness of the transport trade-off.
**Cons:** the HTTP path is intentionally minimal and not production-hardened.

### Option C: HTTP-first / hosted service

**Pros:** multi-user, "real product" shape.
**Cons:** drags in auth, scaling, secret management, and the current
stateful-transport limitation — large scope for little benefit to the actual
primary user. Out of scope for a local utility.

## Trade-off Analysis

The primary user runs the server locally, so stdio is the correct default — it
is simplest and matches the clients. Offering HTTP as an opt-in costs almost
nothing (one branch and a handler) and signals that the transport trade-off was
understood, while explicitly *not* building a hosting stack avoids a large,
premature investment that the stateful-transport caveat would make doubly
wasteful right now.

## Consequences

- **Easier:** zero-config local use; minimal network surface by default.
- **Harder:** anyone using `-transport http` for real must add auth, TLS, and
  scaling themselves; that path is deliberately unsupported for production.
- **Revisit when:** the MCP stateless-transport work lands and a genuine hosted
  use case appears → design hosting properly then.

## Action Items

1. [x] Implement stdio as the default and HTTP behind `-transport http`.
2. [x] Document the statefulness caveat here and in the README.
3. [ ] If hosting is ever needed, write a follow-up ADR covering auth and scaling.
