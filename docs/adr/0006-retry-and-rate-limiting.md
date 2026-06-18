# ADR-0006: Reactive retry with jittered backoff; no proactive rate limiter

**Status:** Accepted
**Date:** 2026-06-18
**Deciders:** Project author

## Context

The SNCF/Navitia API enforces rate limits and can return transient failures
(HTTP 429 Too Many Requests, and 5xx). The client must cope with both without
giving up on the first hiccup or, conversely, hammering the API.

Phase 1 added reactive handling: retry on 429/5xx with exponential backoff,
honoring a `Retry-After` header when present, bounded by a retry count and a
maximum per-attempt delay, and cancellable via context. Phase 6 ("rate limiting
/ backoff finalized") asks whether to go further — e.g. a proactive client-side
rate limiter that throttles requests before they are sent.

The server's primary user is a single developer running it locally against their
own key (see [ADR-0004](0004-transport-stdio-vs-streamable-http.md)). It issues a
handful of requests per agent turn, not a sustained high-volume stream.

## Decision

Keep a **reactive** strategy and finalize it; do **not** add a proactive limiter.

- Retry only idempotent GETs, on 429 and 5xx, up to a configurable number of
  attempts (default 3).
- Backoff is exponential from a configurable base (default 500ms), capped at
  `maxBackoff` (10s), with **equal jitter** (half fixed, half random) so retries
  don't synchronize.
- A server-provided `Retry-After` (seconds or HTTP-date) overrides the computed
  backoff.
- Every wait respects context cancellation, so the caller's deadline bounds the
  total time spent retrying.
- No new dependency: jitter uses `math/rand/v2` (not security-sensitive).

We explicitly do **not** add a token-bucket / `golang.org/x/time/rate` limiter.

## Options Considered

### Option A: Reactive retry + jittered backoff, no limiter (chosen)

**Pros:** correct and sufficient for a low-volume local server; stdlib-only; the
API's own `Retry-After` drives throttling when it matters; context bounds the
total wait. **Cons:** under a hypothetical high-throughput, multi-tenant
deployment it could still burst into the rate limit before the first 429.

### Option B: Add a proactive token-bucket rate limiter

**Pros:** smooths bursts; "production-shaped" for a hosted, multi-user service.
**Cons:** a new dependency and tuning knobs for a problem this server does not
have; the limit is per-key and the single local user is unlikely to approach it;
premature for the current scope (cf. the HTTP-transport decision in ADR-0004).

## Trade-off Analysis

Reactive backoff already honors the server's explicit `Retry-After`, which is the
authoritative signal about the rate limit. A proactive limiter would add code and
a dependency to pre-empt a situation a single local user rarely reaches, and
would need a configured rate that we would only be guessing at. Equal jitter is
the one real gap worth closing now: it de-correlates retries at zero dependency
cost. If the server ever grows a hosted, multi-user deployment (ADR-0004), a
proactive limiter should be revisited together with that.

## Consequences

- **Easier:** transient failures are absorbed; retries are de-correlated; total
  wait is bounded by the caller's context.
- **Harder:** no pre-emptive throttling — a future high-volume use could still
  trip the limit before backing off.
- **Revisit when:** a hosted/multi-user deployment appears, or sustained 429s
  show up in practice → add a proactive limiter then.

## Action Items

1. [x] Add equal jitter to the backoff and document the strategy here.
2. [ ] Revisit a proactive limiter if/when hosting lands (ties to ADR-0004).
