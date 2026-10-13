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
one-way supersede link found in review. The deferral is a risk accepted
by name in 05 5.4. The text below is the one written before the re-plan;
`needs` lists its earlier dependencies with a folded id replaced by the
id that absorbed it, and `owns` is written when the task reopens.

- Implements: REQ-0560 (05 5.4), REQ-0561 (05 5.4), REQ-0562 (05 5.4),
  REQ-0826 (10 10.2), REQ-0827 (10 10.2), REQ-0867 (10 10.2), REQ-1083
  (12 12.2), REQ-1164 (12 12.5), REQ-1165 (12 12.5), REQ-1166 (12 12.5),
  REQ-1169 (12 12.5), REQ-1170 (12 12.5), REQ-1171 (12 12.5), REQ-1193
  (12 12.9).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - A fixture fails for each rule of the step: a duplicate ADR number, a
    filename that differs from `slug(Title)`, a status outside the set,
    a one-way supersede link, whole or in part (`Supersedes in part` and
    `Superseded in part by`), a cited record that is neither Accepted
    nor Proposed with the word `proposed` in every sentence that cites
    it, a Deferred decisions row with an empty cell, a duplicate id, a
    referenced id or id range that is not defined.
  - The two fixtures of the Proposed rule: "a Proposed record cited with
    the word proposed passes" and "a Proposed record cited without it
    fails".
  - A command of 04 4.2 or 04 4.3 with no row in the table of 00 0.5
    fails, and a row written as a command (a code span starting with
    `romeu` or `julieta`) that names none fails, a fixture each way.
  - A `T<nnn>`, `O<n>` or `checkpoint C<n>` token in `docs/spec.md` or
    `docs/spec/` fails (12 12.5: the specification names no plan id).
  - A gap in the ADR numbers passes (ADR 0009, decision 4).
  - It passes on this repository's `docs/spec.md`, `docs/spec/` and
    `docs/adr/`.
- Verify: `go test ./tools/ci/... -run '^TestSequences'`, which runs the
  fixture lines; `go run ./tools/ci sequences`; `go run ./tools/ci
  generated`.
