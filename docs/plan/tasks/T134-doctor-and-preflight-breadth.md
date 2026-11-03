---
id: T134
title: "`doctor` and preflight breadth"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T049
  - T068
  - T033
after: []
ask_first: []
sitting: none
---

# T134 - `doctor` and preflight breadth

Goal: The `doctor` rows and preflight checks that T046 and T049 do not
build, each a file of its own, plugged into the seams those tasks name.

Re-plan (2026-10-12, stage B): The deferred row "doctor and preflight
breadth" of the plan now has a file; trigger: the first incident a
missing row would have caught; or the first release-candidate tag. When
it reopens, `owns` is `internal/cmd/romeu/doctor/<row>.go` for the files
it adds, `internal/preflight/<row>.go`, and the seam files it extends by
one line, `internal/cmd/romeu/doctor/rows.go` and
`internal/preflight/step5.go` (rule 14 of `docs/plan.md`).

Re-plan (2026-10-30, decided by the operator): T049 built the
`secret-argv` row and the schema of `doctor --json`
(`romeu-doctor.v1`), so neither is this task's any more.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-0061 and REQ-0062 (01 1.4) now name the confirmed prompt of `romeu
upgrade` as gate 1 too; T196 builds it, and the preflight of this task
is unchanged.

- Implements: REQ-0011 (index), REQ-0020 (index), REQ-0048 (01 1.2),
  REQ-0056 (01 1.3), REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-0063 (01
  1.4), REQ-0065 (01 1.4), REQ-0141 (02 2.3), REQ-0145 (02 2.3),
  REQ-0713 (08 8.5).
- Operator: no
- Acceptance, one row per slug:
  - `secret-unknown-project`: a secret entry for a project no spec
    names warns, naming the entry.
  - `workspace-files`: the re-derivation rule of 01 1.2.
  - `egress-applied`.
  - `salvage-ref-missing`: a fixture record whose `refsPrefix` has a ref
    missing from its clone fails `doctor` with `salvage-ref-missing`
    naming the record; a deleted salvage dir is no finding.
  - `record-size`.
  - The fix hints of the paragraph after the table of 04 4.2.
- Verify: `go test ./internal/cmd/romeu/doctor/...
  ./internal/preflight/...`.
