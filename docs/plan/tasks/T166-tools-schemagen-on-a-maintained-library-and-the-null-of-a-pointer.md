---
id: T166
title: "`tools/schemagen` on a maintained library, and the null of a pointer"
milestone: M3
module: spec
fate: new
owns:
  - tools/schemagen/**
  - schemas/**
  - go.mod
  - go.sum
  - docs/spec/03-formats.md
  - docs/spec/12-engineering.md
  - docs/handoff/pending/00000000008ks9tvtz3bk21p28-hand-written-code-a-maintained-library-could-replace.md
needs:
  - T021
  - T037
after: []
ask_first:
  - dependencies
sitting: none
---

# T166 - `tools/schemagen` on a maintained library, and the null of a pointer

Goal: item 5 of the pending note "Hand-written code a maintained
library could replace" (00000000008ks9tvtz3bk21p28), under
[ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md):
`tools/schemagen` derives the JSON Schemas from the Go types with
`github.com/invopop/jsonschema` instead of its own reflection, and the
question of `null` for a pointer is settled in the same change.
Decided on 2026-10-23, scope this repository. The dependency is added
by the orchestrator of the pull request, after the maintainer approves
it there: a builder told of an approval by another agent refuses to add
a dependency. The library's maintenance, license and fit are checked
online at the task, and a swap that does not fit leaves the generator as
it is, with the reason written in the pull request (ADR 0038,
decision 3).

The question carried with it: the generator admits no `null` for a
pointer, so a member that has no value is omitted and never written as
`null` (REQ-1273, 03 3.7, decided on 2026-10-23: `promoting` and
`lockSha256` of `render.json`). The swap keeps that rule, and the task
checks the library's treatment of a pointer and of `omitempty` against
it for every type that has a schema.

03 3.1 says "no dependency" for the generator and 12 12.1 lists the
modules; the task amends both, as T146 did (REQ-1049).

- Implements: REQ-1273 (03 3.7).
- Operator: no.
- Acceptance:
  - `go run ./tools/schemagen` produces, for every schema under
    `schemas/`, a document that `tools/ci schema` validates the same
    fixtures against; a schema that differs from the committed one in
    more than the order or spelling the library fixes is explained in
    the pull request.
  - A test holds that no generated schema admits `null` for a member
    the specification omits when empty, and that `render.json` written
    by `internal/render` validates against `schemas/render.v1.json`.
  - 03 3.1 no longer says "no dependency", and REQ-1049 (12 12.1) lists
    the module.
  - The pending note's item 5 reads "Done by T166 (#<number>) on <date>"
    with the library and version, or the reason it was kept.
- Verify: `go test ./tools/schemagen/...`; `go run ./tools/ci schema`;
  `go generate ./...` leaves the tree unchanged.
