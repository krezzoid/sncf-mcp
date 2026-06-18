# Project plan — sncf-mcp

This document is the build sequence for the project. Each phase has a goal, a
concrete task list, and a **checkpoint** (a definition of done you can verify
before moving on). Phases are ordered so that there is always a working,
demonstrable state — we grow a vertical slice rather than building horizontal
layers that only connect at the end.

The guiding principle: a small, finished, well-tested thing beats an ambitious
half-built one. Resist scope creep; park ideas in "Future" at the bottom.

Legend: `[ ]` todo · `[~]` in progress · `[x]` done.

---

## Phase 0 — Repository scaffold & toolchain

**Goal:** a repository that compiles, tests, lints, and runs in CI from day one.

- [x] Package layout (`cmd`, `internal/{navitia,transform,tools,server}`, `testdata`).
- [x] `go.mod` targeting Go 1.26 with the official MCP SDK.
- [x] Typed Navitia client skeleton + hermetic test harness (`httptest`).
- [x] Transform layer skeleton + table-driven test.
- [x] Two tool handlers (`find_station`, `plan_journey`) wired into the server.
- [x] CI (build, vet, test+coverage, golangci-lint, govulncheck), Dockerfile, GoReleaser, Makefile.
- [x] README, this plan, and the four ADRs.

**Checkpoint 0:** after `go mod edit -module …` and `go mod tidy`, the commands
`go build ./...`, `go vet ./...`, and `go test ./...` all succeed locally, and
the CI workflow is green on the first push. The binary starts over stdio and
reports a clear error when `SNCF_API_KEY` is missing.

---

## Phase 1 — Navitia client: `/places` and `/journeys`

**Goal:** a correct, well-tested API client, independent of MCP.

- [x] HTTP Basic auth (key as username, empty password).
- [x] `Places(query)` and `Journeys(from, to, when, arrival)`.
- [x] Typed error for non-2xx responses; context propagation; request timeout.
- [x] Retry with backoff on 429/5xx, honoring the documented rate limit.
- [x] Hermetic tests cover: happy path, empty results, and an API error.
- [x] One **integration** test behind a build tag (`//go:build integration`) that hits the real API when `SNCF_API_KEY` is set, and is skipped otherwise.

**Checkpoint 1:** `make test` passes with no network access; `make integration`
returns real journeys for a known pair (e.g. Paris → Lyon) when a key is present.
Fixtures in `testdata/` document the real response shape.

---

## Phase 2 — Transform layer

**Goal:** compact, stable, LLM-friendly output that does not blow the context window.

- [x] `Journeys()` projection (departure/arrival, duration, transfers, legs).
- [x] `Stations()` projection (id, name, quality; stop-areas only).
- [x] Decide and document the output contract (field names are part of the API the agent sees — treat changes as breaking).
- [x] Golden-file tests: a recorded heavy response in → expected lean JSON out.

**Checkpoint 2:** golden tests pin the output shape; a real Navitia response of
several hundred KB projects down to a small, readable object. Any change to the
output shape now fails a test on purpose.

---

## Phase 3 — First end-to-end tool over stdio (`plan_journey`)

**Goal:** something real, usable in a live MCP client.

- [x] `plan_journey` resolves both endpoints, computes, and transforms.
- [x] Graceful handling of "no station found" and ambiguous matches (return a helpful message, not a raw error).
- [x] Manual verification in Claude Desktop; capture the config and a sample transcript in the README.

**Checkpoint 3:** in Claude Desktop, "How do I get from Toulouse to Bordeaux
tomorrow morning?" returns sensible itineraries. The manual test procedure is
written down so it is repeatable.

---

## Phase 4 — `find_station` as a first-class tool

**Goal:** make station resolution explicit and inspectable for the agent.

- [x] `find_station` handler returning ranked candidates.
- [x] Tighten the `jsonschema` descriptions so the model calls it well.
- [x] Tests for the empty-result and address-vs-stop_area filtering paths.

**Checkpoint 4:** the agent can disambiguate "Lyon" (multiple stations) by
calling `find_station` first, then `plan_journey` with a chosen id. This is the
concrete realisation of [ADR-0001](adr/0001-station-resolution-places-vs-csv.md).

---

## Phase 5 — `next_departures` and `get_disruptions`

**Goal:** complete the four-tool surface.

- [x] Implement `Departures(stopAreaID)` + a `Departures()` transform projection.
- [x] Implement `Disruptions(stopAreaID)` + a `Disruptions()` transform projection.
- [x] Register both tools in `internal/server`.
- [x] Hermetic tests with fixtures for both.

**Checkpoint 5:** all four tools are live and tested; real-time delays and
disruptions surface in answers. Tool count stops here by design (see "Future").

---

## Phase 6 — Hardening & observability

**Goal:** the difference between a demo and something production-shaped.

- [x] Rate limiting / backoff finalized and tested.
- [x] Structured logging reviewed; confirm no PII or secrets in logs.
- [x] Input validation on all tool arguments.
- [x] `golangci-lint` and `govulncheck` clean.
- [x] README polish: limitations, security, and a short demo GIF or transcript.

**Checkpoint 6:** lint and vuln scans are clean in CI; a deliberate bad input
(e.g. malformed datetime) yields a clear, safe error rather than a panic.

---

## Future (explicitly out of scope for v0.1)

- `get_journey_details` (richer per-leg breakdown).
- Isochrones / "what's reachable in N minutes".
- Listing on the official MCP Registry.
- Caching of `/places` results to cut latency and API calls.

These are parked deliberately. Adding them before Phase 6 is scope creep.
