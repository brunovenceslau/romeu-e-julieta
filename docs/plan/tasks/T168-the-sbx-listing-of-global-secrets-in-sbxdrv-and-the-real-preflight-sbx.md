---
id: T168
title: "The sbx listing of global secrets in `sbxdrv` and the real `preflight.Sbx`"
milestone: M3
module: sbxdrv
fate: new
owns:
  - internal/sbxdrv/secret*.go
  - e2e/fakesbx/secret*.go
  - internal/preflight/sbx*.go
  - internal/preflight/listings*.go
  - internal/sbxdrv/guard.go
  - internal/sbxdrv/builders_test.go
  - e2e/testdata/sbx/0.49.2/secrets-settings.jsonl
needs:
  - T029
  - T046
after: []
ask_first:
  - sbxdrv
sitting: O3
---

# T168 - The sbx listing of global secrets in `sbxdrv` and the real `preflight.Sbx`

Goal: the command that lists the global secrets of sbx, in `sbxdrv`, and
the real `preflight.Sbx` adapter that the preflight of 01 1.5 calls, so
the secret check (I22, `sbx-global-secret`, RJ-329) reads the host's
sbx and not a fixture. Decided on 2026-10-23, scope this repository.

The parsers of `internal/preflight/listings.go` (T046) read a
hand-written session marked `synthetic: true` (T046, rule 10 of
`docs/plan.md`). This task moves them to read the recording of A9 (11
11.1), which lists the global secrets and the global allow rules as
`doctor` would. It is gated on A9: the listing command's argv and output
are the ones A9 records, not the ones inferred, so the task waits for the
sitting that runs block A (O3).

- Implements: none: the requirement ids stay with the tasks that own the
  preflight (T046) and the sbx driver (T029).
- Operator: no. The recording is the operator's, at O3.
- Acceptance:
  - `sbxdrv` has the listing command of global secrets, an argv builder
    from A9's recording, and a parser whose output the preflight
    consumes; the fake sbx (`e2e/fakesbx`) answers it with the recorded
    shape.
  - `preflight.Sbx` is the real adapter over `sbxdrv`, and the parsers
    of `listings.go` read the recording of A9; an output of another
    shape is `upstream-shape` (RJ-310).
  - A test fails the preflight with `sbx-global-secret` when the
    recording lists a global secret for a service a project uses, and
    passes it when the list is empty.
- Verify: `go test ./internal/sbxdrv/... ./internal/preflight/...`.
