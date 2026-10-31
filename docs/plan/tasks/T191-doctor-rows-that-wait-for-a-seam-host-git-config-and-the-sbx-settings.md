---
id: T191
title: "Doctor rows that wait for a seam: host git config and the sbx settings"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/doctor/host_git_config*.go
  - internal/cmd/romeu/doctor/sbx_remember_commands*.go
  - internal/cmd/romeu/doctor/sbx_allowed_sources*.go
  - internal/gitsafe/host_config*.go
  - internal/sbxdrv/settings_ls*.go
  - e2e/testdata/sbx/0.49.2/secrets-settings.jsonl
needs:
  - T049
after: []
ask_first:
  - gitsafe
  - sbxdrv
sitting: O3
---

# T191 - Doctor rows that wait for a seam: host git config and the sbx settings

Goal: the three rows of `romeu doctor` that T049 did not build, each a
file of its own registered through `doctor/rows.go` (rule 14 of
`docs/plan.md`).

Re-plan (2026-10-30, decided by the operator, scope this repository):
split from T049, which built every other row of its list. Each row waits
for a read that has no seam yet. `host-git-config` needs a verb of
`internal/gitsafe` that reads the host global git config for the keys of
its one Go slice (`gitsafe.NeutralizedKeys`); a parser of the config by
hand is ADR 0038's question. `sbx-remember-commands` and
`sbx-allowed-sources` need an sbx settings read in `internal/sbxdrv`: a
builder, a parser and a session, whose argv and output probe A1 records
at sitting O3 (11 11.1); until then a session written by hand and marked
`synthetic: true` stands in (rule 10 of `docs/plan.md`). This task owns
the verb, `internal/gitsafe/host_config*.go`, and the builder with its
parser, `internal/sbxdrv/settings_ls*.go`, named the way those packages
name a verb and a builder (`global.go`, `secret_ls.go`); both are
ask-first (`gitsafe`, `sbxdrv`), so each carries its approval line. The
session of the settings read is a line of the session file of the
secrets and settings calls,
`e2e/testdata/sbx/0.49.2/secrets-settings.jsonl`, which the builder's
test holds (T029's rule: a builder has a session).

- Implements: REQ-0370 (04 4.2), REQ-0373 (04 4.2), REQ-0374 (04 4.2).
- Operator: no.
- Acceptance:
  - Each row has a fixture with `HOME` in a temp dir, and its test
    asserts the row's slug in `romeu doctor --json`, not only the exit
    status (10 10.1): the host global git config, which reads the one
    Go slice of `internal/gitsafe` (T023), a warning with the
    review-checkout risk; `env.rememberHostCommands` not true, a
    warning; `kit.allowedSources` that does not admit the workload
    registries, a failure.
  - A settings listing of an unrecorded shape is the error
    `upstream-shape` on the rows that read it, and `doctor` exits 1.
- Verify: `go test ./internal/cmd/romeu/doctor/ -run
  '^TestDoctor(HostGitConfig|SbxRememberCommands|SbxAllowedSources)'
  -v`, which names the row tests this task adds.
