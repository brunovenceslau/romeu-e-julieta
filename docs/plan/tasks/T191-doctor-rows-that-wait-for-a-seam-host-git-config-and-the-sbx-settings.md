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
  - internal/cmd/romeu/doctor/sbx_skills_shared*.go
  - internal/cmd/romeu/doctor/sbx_skills_share_writable*.go
  - internal/gitsafe/host_config*.go
  - internal/sbxdrv/settings_ls*.go
  - e2e/testdata/sbx/0.51.0/secrets-settings.jsonl
needs:
  - T049
after: []
ask_first:
  - gitsafe
  - sbxdrv
sitting: O3
---

# T191 - Doctor rows that wait for a seam: host git config and the sbx settings

Goal: the three rows of `romeu doctor` that T049 did not build, and
the `skills-shared` row added on 2026-11-07, each a file of its own
registered through `doctor/rows.go` (rule 14 of `docs/plan.md`).

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
`e2e/testdata/sbx/0.51.0/secrets-settings.jsonl`, which the builder's
test holds (T029's rule: a builder has a session).

Amended on 2026-11-07 (as decided on 2026-11-07, after the hand
measurement of sbx v0.51.0 that day showed the env file refuses a
`skills` key; scope this repository): the row `skills-shared` (04 4.2)
reads `skills.defaultMode` from the same settings listing and warns,
never blocking, while it is not `off`, since each sandbox created then
gets the host's skills store read-only (03 3.3, REQ-0200; 05 5.4). Its
hint names the sbx settings command that sets the key to `off`, says
that by sbx's description the setting reaches only sandboxes created
after the change, so an existing sandbox keeps the share until `romeu
recreate <name>` (A4 records whether an env-file create honours
`off`), says to keep no secret in the store, and says that a sandbox
with a writable share writes into every other sandbox.

Amended on 2026-11-08 (as decided on 2026-11-08, security finding M3 of
the review of the sbx 0.51 amendments; scope this repository): the row
`skills-share-writable` (04 4.2, REQ-1398) reads every sandbox
`sbx ls --json` lists, romeu's or not, and warns for each whose bind of
the host's skills store is not read-only.

- Implements: REQ-0370 (04 4.2), REQ-0373 (04 4.2), REQ-0374 (04 4.2),
  REQ-1397 (04 4.2), REQ-1398 (04 4.2).
- Operator: no.
- Acceptance:
  - Each row has a fixture with `HOME` in a temp dir, and its test
    asserts the row's slug in `romeu doctor --json`, not only the exit
    status (10 10.1): the host global git config, which reads the one
    Go slice of `internal/gitsafe` (T023), a warning with the
    review-checkout risk; `env.rememberHostCommands` not true, a
    warning; `kit.allowedSources` that does not admit the workload
    registries, a failure; `skills.defaultMode` `readonly` or
    `readwrite`, the warning `skills-shared` whose hint names the
    setting, the recreate, the secret and the writable share, and
    `off`, no finding; a listing without the key falls under the next
    item, since sbx lists every key with its source (seen on
    2026-11-07).
  - `skills-share-writable`: a listing with a sandbox romeu did not
    create whose store bind is writable, a warning that names it; every
    store bind read-only, no finding; a sandbox listed with no mounts,
    skipped and named.
  - A settings listing of an unrecorded shape is the error
    `upstream-shape` on the rows that read it, and `doctor` exits 1.
- Verify: `go test ./internal/cmd/romeu/doctor/ -run
  '^TestDoctor(HostGitConfig|SbxRememberCommands|SbxAllowedSources|SbxSkillsShared|SbxSkillsShareWritable)'
  -v`, which names the row tests this task adds.
