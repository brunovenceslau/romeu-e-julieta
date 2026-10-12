---
id: T055
title: "`julieta spec validate` and `julieta pin`"
milestone: M3
module: julieta-core
fate: kept
owns:
  - internal/cmd/julieta/spec/**
  - internal/cmd/julieta/pin/**
  - cmd/julieta/import_spec.go
  - cmd/julieta/import_pin.go
  - e2e/julieta_spec_test.go
  - e2e/julieta_pin_test.go
  - e2e/scenarios/j2/step1_*.go
needs:
  - T021
  - T032
  - T031
  - T036
  - T051
after:
  - T043
ask_first: []
sitting: none
---

# T055 - `julieta spec validate` and `julieta pin`

Goal: `julieta spec validate` and `julieta pin`.

Re-plan (2026-10-12): Kept. It feeds the config repo's `validate.yml`.
Its J2 step lives in its own file, `e2e/scenarios/j2/step1_*.go`, in
the directory T043 owns, so this task comes `after` T043 (rule 1 of
`docs/plan.md`).

- Implements: 04 4.3 (`spec validate`, `pin workload`, `pin check`), 07
  7.6, 06 6.2, 09 (J2 step 1, J9).
- Operator: feeds block O4.
- Acceptance:
  - `spec validate` exits 2 on a decode or rule error, per file and
    across files; `--catalog` lists each unknown `backend:tool` and lock
    host and exits 6 (`check-findings`) when it lists one; a decode error
    stays exit 2.
  - `pin workload` rewrites the digest after it verifies that the
    manifest list covers linux/amd64 and linux/arm64, through the
    guarded client of `oci`.
  - `pin check` reports a pin that is not a digest or is behind, and
    with `--workflows` the julieta release pinned in the config repo's
    workflows (04 4.3); it exits 6 (`check-findings`) when it reports
    one.
  - The J2 scenario function gains its step 1, `julieta spec validate`
    in the config project's sandbox, at the hybrid level.
- Verify: `go test ./internal/... -run '^Test(SpecValidate|Pin)'`, which
  lists the tests by name; `go test -tags e2e ./e2e/... -run
  '^(TestJulietaSpec|TestJulietaPin)' -v`. The pattern names the
  `TestJulietaSpec*` and `TestJulietaPin*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
