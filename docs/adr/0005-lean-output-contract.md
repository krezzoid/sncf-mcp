# ADR-0005: A stable, lean output contract for tool results

**Status:** Accepted
**Date:** 2026-06-17
**Deciders:** Project author

## Context

Navitia responses are large and deeply nested. A single `/journeys` reply can be
several hundred kilobytes: full geometries (`geojson`), per-stop schedules, fare
and CO₂ breakdowns, pagination links, feed-publisher metadata, and more. Almost
none of that helps a model answer "how do I get from A to B?", and shipping it
verbatim wastes the context window (and the user's tokens), buries the useful
fields, and couples the agent to Navitia's wire format.

The `internal/transform` package already projects each response down to a small
shape (`LeanJourney`, `LeanLeg`, `LeanStation`, `LeanDeparture`,
`LeanDisruption`). But two things were left undecided:

1. **Is this projection a contract?** The JSON field names these types produce
   are what the agent and the calling model actually see. If they change, every
   prompt, example, and downstream consumer built against them can break — yet
   nothing recorded that they are an interface, nor stopped them from drifting.
2. **What stops accidental drift?** A refactor that renamed `duration_minutes`
   or dropped `realtime` would pass every existing unit test, because those tests
   assert individual fields, not the whole shape.

## Decision

Treat the `Lean*` types in `internal/transform` as the project's **public output
contract**:

- Their **JSON field names and semantics are stable**. Adding an optional field
  is backwards-compatible; renaming, removing, or repurposing a field is a
  **breaking change** and must be deliberate (and, once the server is released,
  reflected in a version bump).
- The contract is **documented here** (the tables below are canonical) and in the
  godoc on each `Lean*` type.
- The contract is **enforced mechanically** by golden-file tests
  (`internal/transform/golden_test.go`): a recorded Navitia response is projected
  and compared byte-for-byte against a checked-in `testdata/golden/*.golden.json`.
  Any change to an output shape fails CI on purpose; regenerating the goldens
  (`go test ./internal/transform -update`) makes the diff explicit and reviewable.

### The contract

`find_station` → `{ "stations": [LeanStation] }`

| Field | Type | Meaning |
|-------|------|---------|
| `id` | string | Navitia stop-area id, usable as a `plan_journey` `from`/`to` |
| `name` | string | Display name |
| `quality` | int | Navitia match quality (higher is better) |

`plan_journey` → `{ "journeys": [LeanJourney] }`

| Field | Type | Meaning |
|-------|------|---------|
| `departure` | string | RFC3339 departure timestamp |
| `arrival` | string | RFC3339 arrival timestamp |
| `duration_minutes` | int | Total journey duration |
| `transfers` | int | Number of transfers |
| `legs` | array | Public-transport legs (walks/transfers folded into `transfers`) |

`LeanLeg`: `mode`, `train`, `from`, `to`, `departure`, `arrival` — all strings.

`next_departures` → `{ "departures": [LeanDeparture] }`

| Field | Type | Meaning |
|-------|------|---------|
| `direction` | string | Terminus / headsign direction |
| `mode` | string | Commercial mode (e.g. "TER", "TGV INOUI") |
| `train` | string | Headsign / train number |
| `scheduled` | string | RFC3339 timetabled departure |
| `expected` | string | RFC3339 real-time departure (equals `scheduled` when none) |
| `delay_minutes` | int | Minutes late, clamped at 0 |
| `realtime` | bool | Whether backed by real-time data |

`get_disruptions` → `{ "disruptions": [LeanDisruption] }`

| Field | Type | Meaning |
|-------|------|---------|
| `status` | string | "active" / "future" / "past" |
| `severity` | string | Severity name |
| `effect` | string | e.g. "SIGNIFICANT_DELAYS", "NO_SERVICE" |
| `message` | string | First human-readable message |

> **Timestamps.** Times are rendered in RFC3339 with the correct Europe/Paris
> offset (e.g. `2026-06-20T14:00:00+02:00`). The Navitia "basic" datetime carries
> no offset, so the transform attaches Europe/Paris explicitly, and the binary
> embeds the zone database (`time/tzdata`) so this also holds on minimal images.
> (Earlier revisions rendered a bare `Z`; corrected.)

## Options Considered

### Option A: Return Navitia responses verbatim

**Pros:** zero projection code; nothing to maintain; loses no data.
**Cons:** hundreds of KB per call; wastes context window and tokens; the useful
signal is buried; the agent is coupled to Navitia's wire format and its churn.

### Option B: Project ad hoc, with no stated contract (status quo before this ADR)

**Pros:** lean output already; simple.
**Cons:** the field names are a de-facto API that nobody promised to keep stable;
a rename slips through because unit tests check fields, not shapes; no single
place documents what the agent sees.

### Option C: An explicit lean contract, documented and golden-pinned (chosen)

**Pros:** small, predictable output; one canonical description; accidental drift
fails CI; intentional changes are visible in golden diffs and reviewable.
**Cons:** a little ceremony — golden files to regenerate when the output changes
on purpose; the contract must be honored going forward.

## Trade-off Analysis

The transform layer is the main piece of engineering value in this project (per
the README): it is what makes the tools usable inside a model's context window.
Leaving its output undocumented and unpinned undercuts that — the output is an
interface whether or not we admit it. Option C costs only the discipline of
regenerating a golden file when we *intend* to change a shape, and in return it
documents the interface once and turns every accidental change into a failing
test. Option A throws away the layer's whole reason to exist; Option B keeps the
benefit but none of the guarantees.

## Consequences

- **Easier:** one place to look up what each tool returns; safe refactoring of the
  transform internals as long as the golden output is unchanged; intentional shape
  changes are explicit and reviewable.
- **Harder:** changing the output requires regenerating goldens and is recognized
  as breaking; new tools must add a golden case and a contract entry here.
- **Revisit when:** the server is versioned for release → tie any contract change
  to the version.

## Action Items

1. [x] Document the contract (this ADR) and reference it from the transform godoc.
2. [x] Add golden-file tests that pin every `Lean*` shape.
3. [x] When adding a tool, add a golden case and a contract entry here.
4. [ ] On the first tagged release, restate the contract as part of the public API surface.
