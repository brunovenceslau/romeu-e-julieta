---
id: T017
title: "`canon`"
milestone: M1
module: canon
fate: kept
owns:
  - internal/canon/**
needs: []
after: []
ask_first:
  - digests
sitting: none
---

# T017 - `canon`

Goal: The `canon` package: every digest the gates compare.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 03 3.12.
- Operator: no.
- Acceptance:
  - `canon.Digest(kind, v)` hashes `"romeu/<kind>/v1"`, a NUL byte and
    the canonical JSON; a test shows that two kinds over one value
    differ.
  - Each kind of the table in 03 3.12 is a typed constant, and a value
    of one kind does not compile against a digest of another: a
    compile-fail harness builds a fixture package that mixes two kinds
    with `go build` and expects the failure.
- Verify: `go test ./internal/canon/...`.
