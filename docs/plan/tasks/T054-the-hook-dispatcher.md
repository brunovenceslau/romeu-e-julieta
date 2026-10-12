---
id: T054
title: "The hook dispatcher"
milestone: M1
module: julieta-core
fate: kept
owns:
  - internal/hooks/**
  - internal/cmd/julieta/hooks/**
  - cmd/julieta/import_hooks.go
  - e2e/julieta_hooks_test.go
  - internal/cmd/julieta/setup/hooks_path.go
needs:
  - T051
  - T023
after: []
ask_first: []
sitting: none
---

# T054 - The hook dispatcher

Goal: The hook dispatcher: `hooks run <hook>`, the chain of 04 4.3,
failures recorded in julieta state, julieta's own actions one file each
under `internal/hooks/actions/`.

Re-plan (2026-10-12): Kept. The `pre-commit` lock check waits for T053
and the `post-commit` snapshot for T060, both dark until then.

- Implements: 04 4.3 (`hooks run`), 08 8.2 (recorded failures), 12 12.4
  (the dispatcher runs the tracked hook), 05 5.2 (I21).
- Operator: no.
- Acceptance:
  - In the container, the dispatcher runs julieta's own action, then
    `.githooks/<hook>`, then `$GIT_DIR/hooks/<hook>`, passing arguments
    and stdin as 04 4.3 says; the first non-zero status stops (I21).
  - `setup` installs the dispatcher as the global `core.hooksPath` (04
    4.3): after `julieta setup`, a `git commit` in the container runs
    `.githooks/pre-commit` and `$GIT_DIR/hooks/post-commit` through it
    (I21).
  - A failure is recorded in julieta state and shown until a later run
    of the same hook succeeds.
  - `pre-commit` runs `lock --check` when a mise file is staged.
- Verify: `go test -tags e2e ./e2e/... -run '^TestJulietaHooks' -v`. The
  pattern names the `TestJulietaHooks*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
