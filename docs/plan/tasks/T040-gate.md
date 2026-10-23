---
id: T040
title: "`gate`"
milestone: M3
module: gate
fate: merged
owns:
  - internal/gate/**
needs:
  - T021
  - T033
  - T007
after:
  - T123
ask_first:
  - gates
sitting: none
merged: 2026-10-23
pr: 253
---

# T040 - `gate`

Merged on 2026-10-23 in pull request #253. The text below is the one
written when it was built.

Goal: `gate`.

Re-plan (2026-10-12): Kept as written before the re-plan; `after: T123`,
since the record of the two gates is a review order (12 12.5):
[ADR 0018, gate toolchain changes per machine and project widening per project](../../adr/0018-gate-toolchain-changes-per-machine-and-project-widening-per-project.md).

Re-plan (2026-10-22): `internal/state` keeps the full widening set of an
approval as `map[string]any` (`state.WideningSet`) for now. This task
replaces the map with the real type of the set. The widening digest is
never recomputed from the stored map, since a large integer goes through
a float64 in it; it is computed from the typed set, and the stored digest
is only compared.

Re-plan (2026-10-23): the last sentence of the 2026-10-22 re-plan no
longer holds. `Build` refuses a set that would not read back from JSON
as itself, so the widening digest is computed again from the stored set
whenever a project record is read or written, and must equal the stored
digest; the stored set's render digest must equal the approval's, and
its project the record's. A record that fails these checks is refused
on read as record-unreadable (RJ-334) and left untouched, and refused on
write before anything is written. The generated `internal/gate/*_gen.go`
and `internal/gate/*_gen_test.go` are outputs of rule 1, owned by no
task.

- Implements: REQ-0055 (01 1.3), REQ-0059 (01 1.4), REQ-0066 (01 1.4),
  REQ-0067 (01 1.4), REQ-0071 (01 1.4), REQ-0191 (03 3.2), REQ-0192 (03
  3.2), REQ-0272 (04 4.1), REQ-0488 (05 5.2), REQ-0518 (05 5.2),
  REQ-0575 (06 6.1), REQ-0638 (07 7.6), REQ-0732 (10 10.1), REQ-0754 (10
  10.1), REQ-0790 (10 10.1), REQ-1050 (12 12.1), REQ-1106 (12 12.3).
- Operator: no.
- Acceptance:
  - The widening set and the recreate digest field lists are generated
    from the struct tags; the generated flip table shows that each
    `gate:"widening"` field flips the digest and each field on the
    untagged list of T021 does not (I7); a field on neither fails the
    generator, so the table is never green by omission.
  - The diff is not truncated: a 10 000-line kit file change appears in
    full, the summary is repeated above the prompt, and a binary file is
    shown as size and sha256, flagged (I29). The diff text has a golden
    under `internal/gate/testdata/` (10 10.1).
  - The golden's fixture changes one item of each class (a capability, a
    gated domain, a secret argv, the workload repository, a content-only
    kit file); the golden shows the summary first, in that order and
    marked, then content-only kit changes with file and line counts, the
    kit file list with modes, the workload repository highlighted and
    the host-settings argv behind each secret name, and the summary
    repeated above the prompt (01 1.4).
  - The prompt needs a terminal: it is written to stderr and the TTY
    condition is stdin; a `/dev/ptmx` test drives `y` and `N` on stdin
    and reads stderr (10 10.1).
  - The record of the two gates, ADR 0018, is merged (12 12.5).
- Verify: `go test ./internal/gate/...`.
