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

Goal: The `canon` package: every digest the gates compare, and the one
ULID writer.

Re-plan (2026-10-12): Kept as written before the re-plan, with the ULID
function every writer of an id calls (T033, T035, T059 and T061).

- Implements: REQ-0066 (01 1.4), REQ-0254 (03 3.12), REQ-0255 (03 3.12),
  REQ-0257 (03 3.12), REQ-0258 (03 3.12), REQ-0259 (03 3.12), REQ-0260
  (03 3.12), REQ-0752 (10 10.1), REQ-0753 (10 10.1), REQ-0754 (10 10.1),
  REQ-0755 (10 10.1), REQ-0756 (10 10.1), REQ-0757 (10 10.1), REQ-0758
  (10 10.1), REQ-0759 (10 10.1), REQ-0760 (10 10.1), REQ-0761 (10 10.1),
  REQ-0762 (10 10.1), REQ-0763 (10 10.1), REQ-0764 (10 10.1), REQ-0765
  (10 10.1), REQ-0766 (10 10.1), REQ-0767 (10 10.1), REQ-0768 (10 10.1),
  REQ-0769 (10 10.1), REQ-0770 (10 10.1), REQ-0771 (10 10.1), REQ-0772
  (10 10.1), REQ-0773 (10 10.1), REQ-0774 (10 10.1).
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
