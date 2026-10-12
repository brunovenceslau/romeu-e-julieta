---
id: T004
title: "`tools/ci sequences`"
milestone: M6
module: ci-bootstrap
fate: deferred
owns: []
needs:
  - T003
after: []
ask_first:
  - checks
sitting: none
---

# T004 - `tools/ci sequences`

Re-plan (2026-10-12): deferred to M6; `tools/new adr` and `tools/ci
plan` (T106) remove the main errors it would catch; T106 checks the task
files until this check lands. Reopened by: the first duplicate id or
one-way supersede link found in review. The text below is the one
written before the re-plan; `needs` lists its earlier dependencies with
a folded id replaced by the id that absorbed it, and `owns` is written
when the task reopens.

- Implements: 10 10.2 (the `sequences` step and the id table), ADR 0001
  rules 6 and 7, rule 7 as ADR 0009 amends it.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture fails for each rule of the step: a duplicate ADR number,
    a filename that differs from `slug(Title)`, a status outside the
    set, a one-way supersede link, whole or in part (`Supersedes in
    part` and `Superseded in part by`), a cited record that is not
    Accepted,
    a Deferred decisions row with an empty cell, a duplicate id, a
    referenced id or id range that is not defined.
  - A gap in the ADR numbers passes (ADR 0009, decision 4).
  - It passes on this repository's `docs/spec.md`, `docs/spec/` and
    `docs/adr/`.
- Verify: `go test ./tools/ci/... -run '^TestSequences'`, which runs the
  fixture lines; `go run ./tools/ci sequences`.
