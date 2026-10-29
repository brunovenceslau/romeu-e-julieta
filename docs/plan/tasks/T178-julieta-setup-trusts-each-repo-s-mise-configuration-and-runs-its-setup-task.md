---
id: T178
title: "julieta setup trusts each repo's mise configuration and runs its setup task"
milestone: M2
module: julieta-core
fate: new
owns:
  - internal/mise/repo_task*.go
  - internal/cmd/julieta/setup/repo_setup*.go
  - internal/cmd/julieta/setup/install*.go
  - internal/cmd/julieta/setup/steps.go
  - internal/cmd/julieta/setup/setup.go
  - internal/cmd/julieta/setup/*_test.go
  - e2e/julieta_setup_test.go
  - docs/spec/04-cli.md
  - docs/spec/08-memory-handoff-salvage.md
  - mise.toml
  - docs/handoff/gotchas/0000000000k5wq4gpchaf1ydgt-a-recreated-sandbox-needs-mise-install-and-tools-ci-setup.md
needs:
  - T051
after:
  - T177
ask_first:
  - dependencies
sitting: none
---

# T178 - julieta setup trusts each repo's mise configuration and runs its setup task

Goal: after `setup`, `mise` works in the agent's shell without asking
and each repo has run what its own checks need, so a recreated sandbox
needs no command by hand. Decided on 2026-10-28 with the specification
amended first: REQ-1306 and REQ-1307 (04 4.3); this task's pull
request adds `repo-setup-failed` to the warnings list of 04 4.1 and the
SessionStart list of 08 8.2 with the code (`TestWarningSlugs`).
`after: T177` for the shared e2e file of `setup`.

1. Trust (REQ-1306): before `install`, `setup` runs `mise trust` on each
   configuration file `install` reads, inside the sandbox only, with the
   mise environment of `install` (REQ-0403) but `MISE_TRUSTED_CONFIG_PATHS`
   and `MISE_PARANOID` unset, as the agent's shell has them, so the trust
   is recorded where that shell reads it. A repo with no mise
   configuration is skipped. The host side of I23 is
   untouched: no host command trusts anything.
2. The repo's task (REQ-1307): after `install`, in each repo whose mise
   configuration defines the task `setup` (as `mise tasks ls --json`
   lists it), `mise run setup` with the same environment and the 30 min
   budget 04 4.1 gives it beside `mise install`; a non-zero
   exit or a timeout is the warning `repo-setup-failed` (`<repo> exit
   <n>`) and `setup` goes on; a repo without the task is skipped
   silently.
3. This repository defines the task as `go mod download`, which fills
   the module cache the checks read with `GOPROXY=off`; `tools/ci setup`
   stays the pipeline's and a bare machine's (its pinned mise comes from
   the sandbox definitions' mise kit inside a sandbox). The gotcha of the
   recreated sandbox says `julieta setup` now does it, until T179 makes
   it automatic.

- Implements: REQ-1306 (04 4.3), REQ-1307 (04 4.3).
- Operator: yes, the approval line for `mise.toml` (ask-first
  `dependencies`).
- Acceptance:
  - In a fixture repo with a `mise.toml`, after `setup` a plain `mise
    ls` in that repo exits 0 with no trust prompt, and the trust record
    is under the sandbox's mise state, not the repo.
  - A repo whose task `setup` writes a marker file has it after `setup`;
    a task that exits 3 warns `repo-setup-failed` naming the repo and
    `exit 3`, and `setup` exits 0; a task past its budget (a test seam
    shortens it) warns the same with the timeout named; a repo without
    the task, or without mise configuration, warns nothing.
  - `repo-setup-failed` joins the warnings list of 04 4.1 and the
    SessionStart list of 08 8.2, the lag sentence under REQ-0276 goes;
    `TestWarningSlugs` passes.
  - Operator evidence: in this repository's sandbox, after `julieta
    setup` alone, `go run ./tools/ci fast` passes with no other setup
    command.
- Verify: `go test -race ./internal/cmd/julieta/setup/...`,
  `go test ./e2e -run JulietaSetup` and `go run ./tools/ci fast`.
