---
id: T164
title: "`romeu init --git-credential-helper`, so a private config repo clones at J1 step 3"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/init/**
  - internal/gitsafe/credential*.go
  - e2e/init_test.go
  - e2e/internal/romeu/credential*.go
  - e2e/scenarios/j1/**
  - docs/spec/04-cli.md
  - docs/spec/09-journeys.md
needs:
  - T041
after: []
ask_first:
  - contracts
  - gitsafe
sitting: none
---

# T164 - `romeu init --git-credential-helper`, so a private config repo clones at J1 step 3

Goal: `romeu init --git-credential-helper <name-or-path>`, so a private
config repository can be cloned at J1 step 3, and the error
`config-clone-failed` for a clone that fails. Decided on 2026-10-23,
scope this repository; it is the next task built after the pull request
that records the decisions of that day, so it is ready to start.

`init` clones the config repository before the operator can edit host
settings (J1 step 5), so a private one fails at the clone: the hardened
git of 05 5.1 uses only the helper that `tools.gitCredentialHelper`
names (REQ-0464), and the settings do not exist yet. Reported on 2026-10-23 from a private config repository on macOS: the
failure surfaces as RJ-348 `internal`, which is for a defect of the code
and not for a clone that git refused.

The flag takes the value `tools.gitCredentialHelper` takes (03 3.4,
REQ-1267), refuses a value that rule refuses, records it in the
`settings.yaml` that `init` writes, and the clone uses it for the config
repository's host only. Scoping it with `credential.<url>.helper`, in
place of the global key of REQ-0464, is a proposal that the task settles
against REQ-0464 (it amends that requirement if it changes it, and
`gitsafe` is in `ask_first` for that reason).

A clone that fails is the new error `config-clone-failed`, exit 1, with
the git output (escaped, bounded) in the details and the hint `decide:
check the URL and your access; a private config repo needs
--git-credential-helper`. The id is the next free `RJ-` number when the
task starts, read from 04 4.4 and the `errors.go` files. The task
creates `internal/cmd/romeu/init/errors.go`, to which T165 then adds its
rows. The spec amendments (the `init` row of 04 4.2, J1 step 3, the 04 4.4 row)
are made in this task, with the ids `go run ./tools/ci reqs allocate`
hands out, which the Implements line then cites.

- Implements: REQ-1283 (04 4.2), REQ-1284 (04 4.2), REQ-1285 (04 4.2),
  REQ-1286 (04 4.4), allocated by this task with its spec amendment
  (12 12.11).
- Operator: no.
- Acceptance:
  - In the e2e, against an origin served over TLS that requires
    authentication (10 10.1), `init` without the flag fails with
    `config-clone-failed`, exit 1, the git output in the details, and
    nothing left under the root; with `--git-credential-helper <path>`
    and a test helper it clones to `<root>/<cfg>-env/<dir>`.
  - The settings written hold `tools.gitCredentialHelper` with the
    value, and a test holds that the helper is passed for the config
    repository's host only (the argv of the clone, and no other git
    call of `init`).
  - A value REQ-1267 refuses is refused before any write, exit 2.
  - The `init` row of 04 4.2, J1 step 3 and the row of 04 4.4 name the
    flag and the error; `docs/reference/errors.md` is regenerated.
- Verify: `go test -tags e2e ./e2e/... -run '^TestInit' -v`; the
  reviewer reads the `TestInit*` functions this task adds in the `-v`
  output (10 10.8).
