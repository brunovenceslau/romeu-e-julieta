---
id: T145
title: "The ULIDs of canon from oklog/ulid"
milestone: M1
module: canon
fate: merged
owns:
  - internal/canon/ulid.go
  - internal/canon/ulid_test.go
  - go.mod
  - go.sum
  - docs/spec/12-engineering.md
needs:
  - T017
after: []
ask_first:
  - digests
  - dependencies
sitting: none
merged: 2026-10-20
pr: 201
---

# T145 - The ULIDs of canon from oklog/ulid

Merged on 2026-10-20 in pull request #201. The text below is the one
written when it was built.

Goal: the first swap of
[ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md),
decision 5: `internal/canon` encodes, decodes and stamps ULIDs through
`github.com/oklog/ulid/v2` (v2.1.2, Apache-2.0), approved as a new
dependency on 2026-10-20. `canon` keeps only what the library does not
hold: lowercase text and the refusal of uppercase (REQ-0258), and the
order of one source, where an id written in a millisecond already used,
or before it, is the last one plus one.

- Implements: none: a swap under ADR 0038; the requirements stay T017's.
- Operator: no.
- Acceptance:
  - The API of `canon` (`ULID`, `ParseULID`, `NewULID`, `ULIDSource`,
    `NewULIDSource`, the text methods and the errors) does not change,
    and no caller changes.
  - Every test of T017 in `internal/canon/ulid_test.go` passes unchanged,
    bar the import test, which admits `github.com/oklog/ulid/v2` by name
    and still refuses any other import outside the standard library.
  - A test holds the lowercase text of `canon` equal to the library's
    text lowercased, and `ParseULID` equal to the library's parse for
    every lowercase id of a fixed set, so the two cannot drift.
  - REQ-1049 (12 12.1) lists the module.
- Verify: `go test ./internal/canon/...`.
