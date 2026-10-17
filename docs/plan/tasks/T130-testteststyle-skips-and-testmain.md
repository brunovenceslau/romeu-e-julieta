---
id: T130
title: "`TestTestStyle`: skips and `TestMain`"
milestone: M1
module: ci-release
fate: merged
owns:
  - tools/ci/internal/lint/teststyle*.go
  - tools/ci/all_test.go
  - tools/ci/main_test.go
needs: []
after:
  - T105

ask_first:
  - checks
sitting: none
merged: 2026-10-17
pr: 112
---

# T130 - `TestTestStyle`: skips and `TestMain`

Merged on 2026-10-17 in pull request #112. The text below is the one
written at the re-plan.

Goal: The rules of 10 10.1 and 10 10.6 that `TestTestStyle` on `main`
lacks: `t.Skip`, `t.SkipNow`, `t.Skipf`, `testing.Short`, and a
`TestMain` whose exit status is not the one `m.Run` returned (either
spelling, 10 10.6); and the three skip sites of `main` reworked. `owns`
names the two test files whose skips it reworks by their names on
`main`; the writer takes their post-T105 names from T105's split.

Re-plan (2026-10-12, stage B): New. The meta-test on `main` holds fewer
rules than the pages it cites. `after: T105`, for the moved file
layout, is an order and not a data dependency (rule 2).

- Implements: REQ-0964 (10 10.6).
- Operator: no
- Acceptance:
  - A fixture file fails for each form, the `TestMain` rule in both
    spellings (one fixture that ends in `os.Exit(m.Run())`, one that
    stores the result, cleans up and exits it, one that exits 0 whatever
    `m.Run` returned); the repository passes.
  - The `uname -m` cases of the architecture tests take `same` and
    `other` from a table keyed by `runtime.GOARCH` and, when the table
    has no entry, run the FAIL cases alone instead of skipping; the
    working-tree read of the hook test becomes `require.NoError` (10
    10.2 already requires the `.git` directory of a clone).
- Verify: `go test ./tools/ci/... -run '^TestTestStyle'`.
