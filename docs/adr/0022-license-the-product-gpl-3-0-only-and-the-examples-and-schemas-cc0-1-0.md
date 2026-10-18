# 22. License the product GPL-3.0-only and the examples and schemas CC0-1.0

Date: 2026-10-18

## Status

Proposed

## Context

The repository holds a product, and files that users copy into their
own config repos: `examples/` and `schemas/`. The license of each part
sets what an adopter, and a legal team behind one, may do
([00 0.4](../spec/00-scope.md#04-license)).

Round 1 chose GPL-3.0-or-later for the product and asked, as Q1, about
the license of the examples and the schemas; round 2 chose CC0-1.0 for
`examples/` and `schemas/` ([round 1](../reviews/round-1.md) and
[round 2](../reviews/round-2.md)). The lens set of the review protocol
lists GPL-3.0-only with CC0 under those two directories as a
choice the first page must state, because a mixed-license repository
raises questions at adopters with a legal team
([lenses](../reviews/lenses.md)). The text of the specification now
says "only", with no "or any later version" option. The code
and the specification moved to GPL-3.0-only on 2026-10-02; no reason is
recorded in the commit message, the reviews or the notes.

### Alternatives considered

- **GPL-3.0-or-later for the product.** The first form, in round 1.

## Decision

The product is GPL-3.0-only, version 3 of the GPL only. `examples/`
and `schemas/` are CC0-1.0, because users copy them into their own
config repos. The mechanism of the split is in
[00 0.4](../spec/00-scope.md#04-license).

## Consequences

- A kit copied from a product kit stays GPL-3.0-only; to start a kit
  without that, copy the example kits.
- Each release archive carries the license text and the notices of the
  linked modules, and a module whose license is not compatible with
  GPL-3.0-only fails the `license` step.
- The README states the license and what it means for a config repo.
