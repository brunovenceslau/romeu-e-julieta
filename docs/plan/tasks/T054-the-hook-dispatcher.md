---
id: T054
title: "The hook dispatcher"
milestone: M1
module: julieta-core
fate: kept
owns:
  - internal/hooks/**
  - internal/cmd/julieta/hooks/**
  - internal/cmd/julieta/all/import_hooks.go
  - e2e/julieta_hooks_test.go
  - internal/cmd/julieta/setup/hooks_path.go
  - internal/cmd/julieta/setup/steps.go
needs:
  - T135
  - T023
after:
  - T051
  - T035
ask_first: []
sitting: none
---

# T054 - The hook dispatcher

Goal: The hook dispatcher: `hooks run <hook>`, the chain of 04 4.3,
failures recorded in julieta state, julieta's own actions one file each
under `internal/hooks/actions/`.

Re-plan (2026-10-12): Kept. The `pre-commit` lock check lands with T053
and the `post-commit` snapshot with T060, both dark until then. The
manifest type comes from T135; `after: T051` and `after: T035` for the
setup seam file `setup/steps.go`, which this task extends by one line
after the memory check's line (the setup order of 04 4.3), so the two
one-line edits never meet in a rebase;
`internal/hooks/actions/actions.go` is the hook-name-to-action seam the
action files register into (rule 14 of `docs/plan.md`).

- Implements: REQ-0154 (02 2.5), REQ-0297 (04 4.1), REQ-0301 (04 4.1),
  REQ-0306 (04 4.1), REQ-0397 (04 4.3), REQ-0408 (04 4.3), REQ-0409 (04
  4.3), REQ-0505 (05 5.2), REQ-0619 (07 7.3), REQ-0675 (08 8.2),
  REQ-0686 (08 8.4), REQ-0691 (08 8.4), REQ-0740 (10 10.1), REQ-1128 (12
  12.4).
- Operator: no.
- Acceptance:
  - In the container, the dispatcher runs julieta's own action, then
    `.githooks/<hook>`, then `$GIT_DIR/hooks/<hook>`, passing arguments
    and stdin as 04 4.3 says; the first non-zero status stops, except a
    failure of julieta's own `snapshot`, which is recorded and does not
    stop the chain (04 4.3; I21).
  - The hook stages run repository content on purpose, inside the
    sandbox only (I21); they have no timeout. A stage ended by a signal
    is reported as `exited 128+<signal>`; a `hook-failed` event's
    `exit_code` stays as 13 13.2 defines it.
  - `setup` installs the dispatcher as the global `core.hooksPath` (04
    4.3): after `julieta setup`, a `git commit` in the container runs
    `.githooks/pre-commit` and `$GIT_DIR/hooks/post-commit` through it
    (I21).
  - A failure is recorded in julieta state and shown until a later run
    of the same hook succeeds.
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaHooks' -v`. The
  pattern names the `TestJulietaHooks*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
