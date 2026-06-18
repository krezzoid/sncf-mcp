# ADR-0002: Do not scrape ticket prices

**Status:** Accepted
**Date:** 2026-06-16
**Deciders:** Project author

## Context

A natural user question is "how much does this trip cost?". Ticket pricing is
**not** part of the SNCF / Navitia open-data API — that API covers schedules,
journeys, departures, and disruptions, but not fares.

To fill the gap, one existing community server ships an "experimental" price
scraper that screen-scrapes the SNCF booking site. This is tempting (it answers
a real question) but carries real costs.

Forces: user value of price data, terms-of-service compliance, reliability,
legal/maintenance risk, and the project's positioning as production-shaped.

## Decision

Do not provide ticket prices, and do not scrape them. State this explicitly as a
documented limitation in the README and surface no price-related tool. Revisit
only if SNCF (or an official partner) exposes pricing through a supported API
with terms that allow this use.

## Options Considered

### Option A: Scrape prices from the booking site

| Dimension | Assessment |
|-----------|------------|
| User value | High — answers a common question |
| ToS / legal | Risky — typically against site terms |
| Reliability | Fragile — breaks whenever the site markup changes |
| Maintenance | High — a permanent, unpredictable upkeep tax |
| Reputation | Negative for a portfolio/production project |

**Pros:** feature completeness; immediate user value.
**Cons:** likely ToS violation; brittle; an open-ended maintenance burden;
ships legal/ethical risk into anyone who deploys the server.

### Option B: Don't provide prices; document the gap (chosen)

| Dimension | Assessment |
|-----------|------------|
| User value | Lower — no fares |
| ToS / legal | Clean |
| Reliability | High — only depends on the supported API |
| Maintenance | None for this concern |
| Reputation | Positive — a deliberate, defensible boundary |

**Pros:** legally and operationally clean; reliable; honest scope.
**Cons:** an obvious feature is absent.

## Trade-off Analysis

This is a judgement call between feature completeness and engineering integrity,
and the latter wins. A scraper trades a one-time feature for a permanent,
unbounded liability: it can break at any time without warning, may violate
terms, and pushes that risk onto every deployer. Declining a feature for a clear
reason is a stronger engineering signal than shipping a fragile one. The
limitation is easy to communicate and sets honest expectations.

## Consequences

- **Easier:** no scraping infrastructure, no markup-change firefighting, no ToS exposure.
- **Harder:** the server cannot answer "how much?"; users must check elsewhere.
- **Revisit when:** an official, supported pricing API appears with compatible terms.

## Action Items

1. [x] Document the no-prices limitation in the README.
2. [x] Ensure no tool implies price availability.
3. [ ] Watch for an official SNCF pricing API and re-evaluate if one ships.
