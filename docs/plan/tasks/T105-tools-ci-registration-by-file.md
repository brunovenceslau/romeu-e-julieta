---
id: T105
title: "`tools/ci` registration by file"
milestone: M0
module: ci-bootstrap
fate: new
owns:
  - tools/ci/**
  - .github/ask-first.yaml
needs: []
after: []
ask_first:
  - checks
  - approvals
  - ask-first
sitting: none
---

# T105 - `tools/ci` registration by file

Goal: Each check moves to `tools/ci/internal/<check>/` with its tests
and testdata; one `check_<check>.go` per check whose `init` registers
the name, the usage line, membership in `fast` and `all`, the order key
and the generator outputs; the runner sorts by order key; `generate`
iterates the registry and emits `docs/reference/checks.md`, one line per
check (name, what it guards, `fast` or `all`, a link to the package
doc), a generated output that every later check task regenerates (rule
1 of `docs/plan.md`); `fast_test.go`, `all_test.go` and `main_test.go`
split per check; `lintconfig_test.go` split by linter group. The
`approvals` surface names `tools/ci/askfirst.go`, so the glob follows
the moved package in `.github/ask-first.yaml`, the one edit this task
makes there; T107 then takes that file.

- Implements: 10 10.2; 12 12.3.
- Operator: no
- Acceptance:
  - A golden of the step lists of `fast` and `all` taken before the move
    equals the one taken after.
  - A test registers a fixture check by adding two paths and edits none.
  - The behaviour of every check is unchanged: its tests move with it
    and pass; the hardening is frozen, and a further fix is a pending
    item unless a real escape is shown.
  - Green on the four runners.
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/ci generated`
  exits 0 with `docs/reference/checks.md` present.
