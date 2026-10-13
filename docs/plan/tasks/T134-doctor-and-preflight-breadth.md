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

- Implements: REQ-0011 (index), REQ-0020 (index), REQ-0048 (01 1.2),
  REQ-0056 (01 1.3), REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-0063 (01
  1.4), REQ-0065 (01 1.4), REQ-0141 (02 2.3), REQ-0145 (02 2.3),
  REQ-0713 (08 8.5).
- Operator: no
- Acceptance, one row per slug:
  - `secret-argv` and `secret-unknown-project`: a fixture argv that
    prints a marker string: `doctor` passes and the marker appears in no
    output, file or log; an argv that exits 1 or prints nothing fails;
    the run carries exactly `HOME` and `PATH=/usr/bin:/bin`, through
    `internal/procgroup` with its class.
  - `workspace-files`: the re-derivation rule of 01 1.2.
  - `egress-applied`.
  - `salvage-ref-missing`: a fixture record whose `refsPrefix` has a ref
    missing from its clone fails `doctor` with `salvage-ref-missing`
    naming the record; a deleted salvage dir is no finding.
  - `record-size`.
  - The fix hints of the paragraph after the table of 04 4.2.
  - `doctor --json` reports each check by slug with its result.
- Verify: `go test ./internal/cmd/romeu/doctor/...
  ./internal/preflight/...`.
