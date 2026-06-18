# ADR-0001: Resolve stations via the `/places` API, not a bundled CSV

**Status:** Accepted
**Date:** 2026-06-16
**Deciders:** Project author

## Context

Users (and agents) refer to stations by free-text name — "Lyon Part Dieu",
"Toulouse", "Gare de Lyon". The Navitia API works in terms of stable IDs
(`stop_area:SNCF:…`) or coordinates. Something must bridge names to IDs.

The existing community SNCF MCP servers solve this by shipping a static dataset
of stations and coordinates (one bundles a `train_stations_europe.csv` and looks
stations up locally; another hardcodes coordinates for major cities directly in
source). This is the single biggest design weakness in those projects.

Forces at play: accuracy of resolution, freshness of station data, maintenance
burden, handling of typos/partial names, and latency.

## Decision

Resolve names at request time through the Navitia `/places` autocomplete and
geocoding endpoint, biased to `type[]=stop_area`. Pick the highest-quality
stop-area match (Navitia returns results ranked by `quality`). Expose this both
implicitly (inside `plan_journey`) and explicitly (as the `find_station` tool)
so the agent can disambiguate when needed.

## Options Considered

### Option A: Bundled static dataset (CSV / hardcoded coordinates)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low to start, grows over time |
| Cost | Zero API calls for resolution |
| Freshness | Poor — stale the moment the network changes |
| Accuracy | Brittle on typos, partial names, renamed stations |
| Maintenance | Ongoing: someone must refresh the data |

**Pros:** no extra API call; works offline; simple to demo.
**Cons:** data rots; new/renamed stations are invisible; fuzzy matching must be
reimplemented poorly; the dataset is dead weight in the repo.

### Option B: Resolve via the `/places` endpoint (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one well-documented endpoint |
| Cost | One extra request per resolution |
| Freshness | Always current — the API is the source of truth |
| Accuracy | High — server-side ranking, autocomplete, typo tolerance |
| Maintenance | None — no data to maintain |

**Pros:** authoritative, current, typo-tolerant; no bundled data; reuses the
provider's ranking. **Cons:** an extra network round-trip; depends on the API
being reachable (but the whole server already does).

## Trade-off Analysis

The only real cost of Option B is one additional request and a dependency on the
API for resolution. Since the server cannot function without the API anyway,
that dependency is already present — Option A would add a *second*, independent
source of truth that drifts from the first. The latency cost is small and can be
mitigated later with a short-lived cache (parked in the project plan). Option A's
"benefit" (offline resolution) is irrelevant for a server whose entire job is to
call the online API.

## Consequences

- **Easier:** correct results for arbitrary names; zero data maintenance; smaller repo.
- **Harder:** each plan involves up to two extra resolution calls; we must
  handle "no match" and "ambiguous match" gracefully.
- **Revisit when:** latency or call volume becomes a concern → add a TTL cache
  for `/places` lookups.

## Action Items

1. [x] Implement `Client.Places` and use it in `plan_journey`.
2. [x] Expose `find_station` for explicit disambiguation.
3. [x] Add graceful "no station found" / ambiguity handling.
4. [ ] Consider a TTL cache once usage justifies it (plan "Future").
