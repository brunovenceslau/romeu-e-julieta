---
id: T069
title: "`romeu retire`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/retire/**
  - cmd/romeu/import_retire.go
  - e2e/scenarios/j7/**
needs:
  - T068
  - T048
after: []
ask_first: []
sitting: none
---

# T069 - `romeu retire`

Goal: `romeu retire`.

Re-plan (2026-10-12): Kept as written before the re-plan.

- Implements: 04 4.2 (`retire`), 09 (J7), 02 2.3 (the invariants of the
  tree).
- Operator: no.
- Acceptance:
  - The J7 scenario function passes in CI: `retire` lists the unpushed
    and salvage-only work per repo, exits 5 on a declined confirmation
    or without a terminal and without `--force`, moves `<name>-env/` to
    `.attic/<name>/<ts>/`, drops the project from the workspace files
    and moves its state to the state attic.
  - Nothing is deleted.
  - On a `removing` generation with its sandbox present, `retire` runs
    the fingerprint cases of I17 that T068 runs for `rm`: an equal
    fingerprint resumes at `sbx env rm`, and one that differs salvages
    again and ends as `RJ-338` (04 4.4) says; on an exit with `RJ-338`
    or `RJ-306` nothing is moved (the tree, the workspace files, the
    state), and after the resume the move of J7 completes (round 17,
    R17-08; round 18, R18-05, R18-A9).
- Verify: `go test -tags e2e ./e2e/... -run '^TestRetire' -v`. The
  pattern names the `TestRetire*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
