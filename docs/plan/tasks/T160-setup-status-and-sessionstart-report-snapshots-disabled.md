---
id: T160
title: "setup, status and SessionStart report snapshots disabled"
milestone: M1
module: julieta-core
fate: new
owns:
  - internal/cmd/julieta/setup/snapshots_disabled*.go
  - internal/cmd/julieta/setup/setup.go
  - internal/cmd/julieta/setup/state.go
  - internal/cmd/julieta/setup/steps.go
  - internal/cmd/julieta/setup/helper_test.go
  - internal/cmd/julieta/status/helper_test.go
  - internal/cmd/julieta/status/status_test.go
  - internal/cmd/julieta/handoff/show_test.go
  - internal/hooks/actions/disabled*.go
  - internal/hooks/actions/snapshot.go
  - e2e/julieta_snapshot_test.go
needs:
  - T060
  - T051
  - T054
  - T059
  - T154
after:
  - T053
  - T079
  - T155
ask_first: []
sitting: none
---

# T160 - setup, status and SessionStart report snapshots disabled

Goal: the part of T060's `disabled` acceptance item that T060 left
(decided on 2026-10-21): `julieta setup` writes `disabled` into a
repo's `snapshot/heads.json`, and `setup`, `julieta status` and the
SessionStart print report "snapshots disabled for <dir>" (08 8.4,
REQ-0692; 04 4.3, REQ-0396 and REQ-0424; 08 8.2, REQ-0665).

The spec reads one way: "`julieta setup` writes `disabled: true` into
its `heads.json`" is the `snapshot/heads.json` of 03 3.13, the file
`julieta snapshot` writes, and not another record. The report is the
warning `snapshots-disabled` (04 4.1, Warnings; 08 8.2 lists it among
SessionStart's warnings), recorded by `setup` in julieta state as
`fetch-failed` and `agent-memory-writable` are, with the message
"snapshots disabled for <dir>" (the dir escaped by `termsafe`). Status
and SessionStart already print every warning of julieta state, and
`julieta-status/v1` already carries them in `warnings`, so neither
command, nor the format, changes: the warning is a new value of an
existing member. `romeu status` shows `disabled` from heads.json
(04 4.2) and is another task's.

The write is `actions.RecordDisabled`, in `internal/hooks/actions` since
`setup` already imports that package and the snapshot's own write,
lock and `HooksPath` read live there. Under the store lock of the
repo's memory dir: a heads.json that already says what the config says
is left alone (setup is idempotent); one that says otherwise is
rewritten with its refs and `createdAt` kept; none is made only for a
disabled repo (no refs, `createdAt` the clock of setup), since a repo
that is not disabled has nothing to say before its first snapshot. When
the key is gone, the next setup writes `disabled: false` back, so the
file does not outlive the config it describes. A heads.json that does
not decode is an entry of the report's `errors`, left as it is.

- Implements: REQ-0396 (04 4.3), REQ-0424 (04 4.3), REQ-0665 (08 8.2),
  REQ-0692 (08 8.4).
- Operator: no.
- Acceptance:
  - With a repo-local `core.hooksPath`, `julieta setup` makes
    `snapshot/heads.json` with `disabled: true`, no refs and the clock
    as `createdAt`, prints `warning snapshots-disabled: snapshots
    disabled for <dir>` on stderr and lists it in `--json` `warnings`;
    a second setup leaves the file byte for byte as it is.
  - With a heads.json of an earlier snapshot, setup sets `disabled` and
    keeps its refs and `createdAt`.
  - With no repo-local `core.hooksPath`, setup warns nothing, makes no
    heads.json, and sets `disabled` back to false in one that says true.
  - A heads.json setup cannot decode is one entry of `errors`, is left
    as it is, and the warning is still given.
  - `julieta status` (text on stderr, `--json` `warnings`) and the
    SessionStart print (`julieta handoff show --hook`) show the
    warning, which the SessionStart keeps (only `default-branch-moved`
    is cleared).
  - In the container e2e, after a setup with a repo-local
    `core.hooksPath`, heads.json says `disabled: true` with no commit
    having run a snapshot, and setup, status (text and `--json`) and
    `handoff show --hook` show the warning.
- Verify: `go test -race ./internal/cmd/julieta/... ./internal/hooks/...`,
  `go test -tags e2e ./e2e/... -run '^TestJulietaSetupSnapshotsDisabled'
  -v`, and `go run ./tools/ci all`.
