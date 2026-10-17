---
id: T017
title: "`canon`"
milestone: M1
module: canon
fate: merged
owns:
  - internal/canon/**
needs: []
after: []
ask_first:
  - digests
sitting: none
merged: 2026-10-16
pr: 59
---

# T017 - `canon`

Merged on 2026-10-16 in pull request #59. The text below is the one
written at the re-plan.

Goal: The `canon` package: every digest the gates compare, and the one
ULID writer.

Re-plan (2026-10-12): Kept as written before the re-plan, with the ULID
function every writer of an id calls (T033, T035, T059 and T061).

- Implements: REQ-0066 (01 1.4), REQ-0254 (03 3.12), REQ-0255 (03 3.12),
  REQ-0257 (03 3.12), REQ-0258 (03 3.12), REQ-0259 (03 3.12), REQ-0260
  (03 3.12), REQ-0753 (10 10.1).
- Operator: no.
- Acceptance:
  - `canon.Digest(kind, v)` hashes `"romeu/<kind>/v1"`, a NUL byte and
    the canonical JSON; a test shows that two kinds over one value
    differ.
  - Each kind of the table in 03 3.12 is a typed constant,
    `memory-content` and `fingerprint` among them, and a value of one
    kind does not compile against a digest of another: a compile-fail
    harness builds a fixture package that mixes two kinds with `go
    build` and expects the failure.
  - One function `canon.NewULID` writes every ULID, 26 characters of
    lowercase Crockford base32, monotonic within one process, from the
    standard library alone, and `canon.ParseULID` refuses uppercase; a
    table test covers both.
- Verify: `go test ./internal/canon/...`.
