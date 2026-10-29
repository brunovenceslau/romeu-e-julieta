---
id: T190
title: "`egress`: romeu applies only the domains it owns, each with its port"
milestone: M3
module: egress
fate: new
owns:
  - internal/egress/derive.go
  - internal/egress/derive_test.go
  - internal/egress/derive_git_test.go
  - docs/spec/07-mise-egress.md
needs:
  - T036
after:
  - T184
ask_first:
  - catalog
  - spec
sitting: none
---

# T190 - `egress`: romeu applies only the domains it owns, each with its port

Goal: no rule romeu adds is wider than what the derivation meant.
Measured on 2026-10-29 with sbx v0.50.1 (found while building T184,
decided the same day, scope this repository): sbx applies the runtime
hosts of the descriptors itself, with their ports, as a rule romeu
cannot edit, and `sbx policy allow network --sandbox <name> <domain>`
with no port opens every port of the domain (80, 8080 and 2222 of
`portquiz.net` answered).

1. `Derive` tells apart the domains that only the descriptors declared
   (step 6 of 07 7.5), so the application of REQ-0634 skips them
   (REQ-1316); they stay in the result for gate 2's upload flag, the
   digest and the manifest's applied set.
2. Decide in 07 7.5 the port of each rule romeu applies: 443 by
   default, the port of the URL for a lock host, and the form of a
   catalog or `egress.extra` entry that needs another; REQ-0634's argv
   becomes `<domain>:<port>` once the block A recording of that form
   is read (the probe of 2026-10-29 read `sbx policy ls --json`, whose
   resources carry `host:port`).

- Implements: REQ-1316 (07 7.5), REQ-0634 (07 7.5).
- Operator: approves the port rule of item 2 (`spec`).
- Acceptance:
  - A table test of `Derive`: a domain only a kit declared is marked
    and is not among the domains to apply; the same domain also
    produced by the catalog or a lock is applied.
  - 07 7.5 states the port of each applied rule, and the result of
    `Derive` carries it.
  - `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/egress/`.
