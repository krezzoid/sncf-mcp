# ADR-0003: Use the official Go MCP SDK, not `mark3labs/mcp-go`

**Status:** Accepted
**Date:** 2026-06-16
**Deciders:** Project author

## Context

The server needs a Go library implementing the Model Context Protocol. Two
mature options exist:

- `github.com/mark3labs/mcp-go` — the long-standing community library, widely
  adopted (imported by ~1,900 modules), originally authored by Ed Zynda.
- `github.com/modelcontextprotocol/go-sdk` — the official SDK, developed in
 collaboration with Google, which reached a stable **v1.0.0** (around one year ago) with a
  no-breaking-changes compatibility guarantee and is spec-complete for the
  current MCP revision. It supports both stdio and Streamable HTTP transports.

The official SDK explicitly acknowledges and was inspired by `mcp-go`; the two
APIs differ but translating between them is generally straightforward.

Forces: long-term maintenance and spec alignment, API stability guarantees,
idiomatic Go ergonomics, community size, and the signal the choice sends in a
portfolio project.

## Decision

Build on `github.com/modelcontextprotocol/go-sdk` (v1.x). Pin a v1 version in
`go.mod` and rely on the stability guarantee.

## Options Considered

### Option A: `mark3labs/mcp-go`

| Dimension | Assessment |
|-----------|------------|
| Maturity | Very mature, battle-tested |
| Community | Largest Go MCP user base |
| Spec alignment | Community-tracked |
| Long-term | Community-maintained, no formal guarantee |
| Team familiarity | High across the ecosystem |

**Pros:** proven, lots of examples, large user base.
**Cons:** not the canonical implementation; spec alignment and longevity depend
on community effort rather than a first-party commitment.

### Option B: Official `modelcontextprotocol/go-sdk` (chosen)

| Dimension | Assessment |
|-----------|------------|
| Maturity | Stable v1.0.0 with compatibility guarantee |
| Community | Growing fast; first-party + Google |
| Spec alignment | Spec-complete; tracks the spec authoritatively |
| Long-term | Strongest — official, committed to backward compatibility |
| Ergonomics | Idiomatic: typed input/output structs with `jsonschema` tags |

**Pros:** canonical and future-proof; clean typed-tool API (schemas derived from
Go types); first-party spec tracking; stable v1 contract.
**Cons:** younger than `mcp-go`; fewer third-party examples (for now).

## Trade-off Analysis

For a project whose explicit goal is to demonstrate sound engineering judgement,
aligning with the canonical, spec-authoritative implementation is the stronger
position than optimizing for the largest current install base. The official
SDK's v1 stability guarantee removes the main risk of adopting a newer library
(churn), and its typed-tool ergonomics — tool schemas generated from Go structs
— improve correctness and readability. The cost (a smaller pool of community
examples) is minor given the SDK ships its own examples and documentation.

## Consequences

- **Easier:** strongly-typed tools; confidence in long-term spec alignment; one
  obvious upgrade path.
- **Harder:** fewer Stack-Overflow-style answers; must read the SDK's own docs.
- **Revisit when:** the SDK goes to v2 for a new spec revision — plan a deliberate
  migration rather than tracking pre-releases.

## Action Items

1. [x] Add the official SDK to `go.mod` and wire tools via `mcp.AddTool`.
2. [ ] Run `go doc` against the pinned version to confirm transport constructor names.
3. [ ] Pin to a specific v1 minor and bump deliberately, not automatically.
