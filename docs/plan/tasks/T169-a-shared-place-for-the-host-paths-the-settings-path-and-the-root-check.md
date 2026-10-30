---
id: T169
title: "A shared place for the host paths: the settings path and the root check"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/hostpaths/**
  - internal/cmd/romeu/init/**
  - internal/cmd/romeu/approve/**
  - internal/cmd/romeu/sync/step1_lock.go
  - docs/spec/02-layouts.md
  - docs/spec/12-engineering.md
needs:
  - T041
  - T042
after:
  - T164
  - T165
  - T043
ask_first:
  - contracts
sitting: none
---

# T169 - A shared place for the host paths: the settings path and the root check

Goal: one place for the host paths and the root check, used by `init`,
`approve`, `sync` and `doctor`, in place of the copies each command
grows. `sync` step 1 (T043) holds a copy of `approve`'s `loadSettings`,
which this task moves too (decided on 2026-10-29).
Decided on 2026-10-23, scope this repository; no existing task covers
it.

Main has, in `internal/cmd/romeu/init/paths.go` (T041, #259),
`settingsPath`, `realPath`, `within`, `rootProblem` and `dirProblem`.
T042, when merged, carries a copy of the settings path in
`internal/cmd/romeu/approve/settings.go`, which this task replaces; so
it needs T042.

The package (`internal/hostpaths`, a row the task adds to the tree of
02 2.1) holds the settings path (`ROMEU_SETTINGS`, `XDG_CONFIG_HOME`,
`HOME`; REQ-0148, REQ-0150), the state directory path
(`XDG_STATE_HOME`), `realPath`, and the root check with the rules of
`root-unsafe` as 04 4.2 and I20 state them since 2026-10-23: a root
inside a git repository, equal to `$HOME`, not absolute, not owned by
the user, holding or sitting inside the host settings or state
directory, and a root or `<root>/<cfg>-env` that is a symlink, not
owned by the user or writable by its group or others. `init` and
`approve` call it and delete their copies; `doctor` (T049) calls it for
its `root-unsafe` row.

Decided on 2026-10-30, scope this repository, written here: the
settings path stays built by `spec.SettingsPath` (`internal/spec`,
T041) and the state directory by `state.DefaultDir`, which predate the
package and which `hostpaths` imports, so neither is a copy;
`hostpaths.SettingsPath` adds the path `cli.Env` holds, and every
command takes the path from it. The package's path joins the
`romeu-cli` row of the module table of 12 12.2, which the tree check
requires of a package under `internal/`. The memory push
(`internal/cmd/romeu/all/memory_push.go`, T175), outside this file set,
still builds the settings path; the call-site test lists it as
pending, and its one-line move to `hostpaths.SettingsPath` lands in a
change of its own.

- Implements: REQ-1277 (04 4.2), REQ-1281 (05 5.2), REQ-1282 (05 5.2).
- Operator: no.
- Acceptance:
  - One table test per rule of the root check, including a root inside
    the settings directory, a root inside the state directory, a root
    that is a symlink, a `<root>/<cfg>-env` that is a symlink, and a
    root writable by its group or others; each fails with the message
    that names the rule.
  - `init` and `approve` hold no path or root code of their own, and
    their e2e cases pass unchanged.
  - No other package builds the settings path from the environment (a
    test over the call sites).
- Verify: `go test ./internal/hostpaths/... ./internal/cmd/romeu/...`;
  `go test -tags e2e ./e2e/... -run '^TestInit' -v`.
