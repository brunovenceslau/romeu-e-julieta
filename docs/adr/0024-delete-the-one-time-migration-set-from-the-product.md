# 24. Delete the one-time migration set from the product

Date: 2026-10-18

## Status

Proposed

## Context

`julieta memory import` and `verify`, the `import/` allowlist entry, the
memory-entry digest kind, the repo evidence kind, `tools/ci acceptance
--file` and the C1 check carried one person's migration into the generic
product. Review round 8 raised this as item SC2 of [round 8](../reviews/round-8.md#decisions-for-the-operator). It
supersedes the round 6 decision (op-migration-set) that the set stays in
v1. The recommendation of the review carried the cost: six concepts for
one run on one machine, which the scope and second-adopter lenses both
priced.

### Alternatives considered

- **Keep the six items**, rewrite the scope cell as a general need, and
  mark the commands operator-only. Not taken.
- **Delete them** and run the one-time import as a script in the
  operator's config repo. Taken; the specification already lets a config
  repo carry its own `acceptance.json`
  ([02 2.2](../spec/02-layouts.md)).

## Decision

Decided on 2026-10-08: the six migration items are deleted from the
product. The one-time import runs as a script in the operator's config
repo.

## Consequences

- The plan loses the task that built the import and parts of two later
  tasks.
- The product has six concepts fewer to specify, test and keep.
- The round 6 decision to keep the set in v1 is reversed by this
  record, knowingly.
