---
id: T039
title: "`render`: the promotion commit, recovery and drift"
milestone: M3
module: render
fate: kept
owns:
  - internal/render/promote*.go
  - internal/render/drift*.go
  - internal/render/testdata/promote/**
  - tools/ci/internal/callsites/**
  - tools/ci/check_callsites.go
needs:
  - T033
  - T038
after:
  - T037
ask_first:
  - checks
sitting: none
---

# T039 - `render`: the promotion commit, recovery and drift

Goal: `render`: the promotion commit, recovery and drift, with fault
injection and the call-site check.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 01 1.6 ("Promotion commit"), 01 1.2 (the drift rule), 06
  6.3 (step 1), 05 5.2 (I15, I16).
- Operator: no.
- Acceptance:
  - The four steps run in order; a test hook stops after each, and
    recovery finishes a promotion whose candidate verifies against its
    `filesDigest`, or refuses with exit 4 when it does not.
  - A hand edit of `sbxenv.yaml`, of a kit file or of
    `.romeu/bin/SHA256SUMS` is reported as drift (I15).
  - The I16 scan fails each listed call kind outside a function the
    admitted-call-site table names (05 5.2).
- Verify: `go test ./internal/render/...`; `go test ./tools/ci/... -run
  '^TestCallsites'`, which runs the fixtures of the call-site check;
  `go run ./tools/ci callsites` exits 0 on the tree.
