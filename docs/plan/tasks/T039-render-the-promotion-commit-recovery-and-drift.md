---
id: T039
title: "`render`: the promotion commit, recovery and drift"
milestone: M3
module: render
fate: merged
owns:
  - internal/render/promote*.go
  - internal/render/drift*.go
  - internal/render/testdata/promote/**
  - tools/ci/internal/callsites/**
  - tools/ci/check_callsites.go
  - internal/procgroup/callsite_test.go
  - .github/ask-first.yaml
  - docs/spec/05-security.md
needs:
  - T033
  - T038
after:
  - T105
ask_first:
  - checks
  - procgroup
  - ask-first
  - spec
sitting: none
merged: 2026-10-29
pr: 315
---

# T039 - `render`: the promotion commit, recovery and drift

Goal: `render`: the promotion commit, recovery and drift, with fault
injection and the call-site check.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: REQ-0029 (index), REQ-0047 (01 1.2), REQ-0081 (01 1.6),
  REQ-0082 (01 1.6), REQ-0084 (01 1.6), REQ-0085 (01 1.6), REQ-0086 (01
  1.6), REQ-0087 (01 1.6), REQ-0226 (03 3.7), REQ-0256 (03 3.12),
  REQ-0262 (03 3.13), REQ-0342 (04 4.2), REQ-0385 (04 4.2), REQ-0386 (04
  4.2), REQ-0429 (04 4.4), REQ-0479 (05 5.2), REQ-0481 (05 5.2),
  REQ-0498 (05 5.2), REQ-0499 (05 5.2), REQ-0500 (05 5.2), REQ-0582 (06
  6.3), REQ-0751 (10 10.1), REQ-1060 (12 12.1), REQ-1311 (10 10.2).
- Operator: no.
- Acceptance:
  - The four steps run in order; a test hook stops after each, and
    recovery finishes a promotion whose candidate verifies against its
    `filesDigest`, or refuses with exit 4 when it does not.
  - An existing kit dir with the right digest is verified and left
    unchanged, one whose content differs fails; when the candidate no
    longer verifies, recovery restores `render.json` with the previous
    digests plus step 2's, exits 4 with
    `interrupted-promotion-abandoned`, and a following `status` reports
    no drift.
  - A hand edit of `sbxenv.yaml`, of a kit file or of
    `.romeu/bin/SHA256SUMS` is reported as drift (I15).
  - The I16 scan fails each listed call kind outside a function the
    admitted-call-site table names (05 5.2): `os/exec` is admitted in
    `internal/procgroup` only, and its runner in `sbxdrv` and `gitsafe`
    (and the doctor's secret-argv row); a fixture package that starts
    `gh` fails.
  - The `os/exec` call-site scan of 04 4.1 (Subprocesses, item 4),
    which T137 lands as a test in `internal/procgroup/callsite_test.go`,
    moves into `tools/ci callsites` beside the I16 scan (REQ-0481),
    with its positive control and its seam rule, and the test file is
    removed; its admitted list is `internal/procgroup` alone, as T105
    leaves it. The whole of `tools/ci/internal/callsites/**`, which
    then holds the admitted list and the guarded lists of the scan,
    joins the globs of the `procgroup` surface in `.github/ask-first.yaml`,
    with `.github/CODEOWNERS` regenerated, the way T107 puts
    `tools/ci/internal/askfirst/**` on `approvals` (decided on
    2026-10-13).
- Verify: `go test ./internal/render/...`; `go test ./tools/ci/... -run
  '^TestCallsites'`, which runs the fixtures of the call-site check; `go
  run ./tools/ci callsites` exits 0 on the tree.
