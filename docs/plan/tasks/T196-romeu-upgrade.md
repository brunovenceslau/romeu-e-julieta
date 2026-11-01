---
id: T196
title: "romeu upgrade"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/upgrade/**
  - internal/cmd/romeu/all/import_upgrade.go
  - e2e/upgrade_test.go
  - e2e/scenarios/j9/**
  - changelog.go
  - changelog_test.go
  - tools/ci/internal/callsites/**
needs:
  - T192
  - T195
  - T042
  - T103
after: []
ask_first:
  - contracts
  - procgroup
  - gates
  - checks
sitting: none
---

# T196 - romeu upgrade

Goal: `romeu upgrade [--version <tag>] [--check]` (04 4.2): install a
published release on this host, after the operator reads what changed
and confirms, and acknowledge it as gate 1.

Added on 2026-11-01 (decided on 2026-10-31 and 2026-11-01, scope this
repository): a release made from one host is installed on any other by
one command, with the same checks as J1 step 2 (the sha256 in Go, the
attestation through the `gh` of `tools.gh`), so the binaries in use stop
being old. The prompt comes before the swap: the running romeu downloads
and verifies, runs the new binary once as a preview subprocess, asks,
and a `no` leaves the binary and `toolchain.json` as they were. The
changelog is embedded from `CHANGELOG.md`; `go:embed` does not cross a
parent directory, so the embed is a package at the root of the
repository (`changelog.go`), whose name this task picks and `tools/ci
imports` admits. The confirmation is gate 1 (REQ-0061): on `yes` the new
binary, in place of the process, writes `toolchain.json` through
`approve`'s code and runs the steps of `sync --all`. The releases API
host, the asset host and the version are injected values (10 10.4), so
the tests serve a fixture release; the first run against a real release
waits for T088's first tag. T042's `approve.Changes` and `approve.Text`
print the toolchain part of the preview; the preview and the `gh` steps
are admitted call sites of I16, and `syscall.Exec` joins the scan of
REQ-0500 with the swap step as its one admission.

- Implements: REQ-0061 (01 1.4), REQ-0062 (01 1.4), REQ-1320 (04 4.1),
  REQ-1321 (04 4.1), REQ-1323 (04 4.2), REQ-1324 (04 4.2), REQ-1325 (04
  4.2), REQ-1326 (04 4.2), REQ-1327 (04 4.2), REQ-1328 (04 4.2),
  REQ-1348 (04 4.2), REQ-1329 (04 4.2), REQ-1330 (04 4.2), REQ-1332 (04
  4.4), REQ-1333 (04 4.4), REQ-1334 (04 4.4), REQ-1335 (04 4.4),
  REQ-1349 (04 4.4), REQ-0500 (05 5.2), REQ-1336 (05 5.2), REQ-1341 (12
  12.6), REQ-1350 (12 12.6).
- Operator: yes: the first upgrade on the maintainer's host, from a
  dev build of this task installed by hand (`v0.0.0-dev+<commit>`, the
  first binary that has `upgrade`) to the first release of T088, with
  its output under this pull request's Evidence or the next.
- Acceptance:
  - The root package embeds `CHANGELOG.md`; a test with a fixture file
    shows the sections between two versions, newest first, and the own
    section alone when the running version is not a tag of the file
    (`v0.0.0-dev+<commit>`, REQ-0065).
  - The pick: on recorded API fixtures (10 10.1), from
    `/releases?per_page=10` of the repository compiled into romeu, the
    newest published release that is not a draft, a prerelease
    included; `--version`
    pins one and refuses one that does not exist; a release equal to
    the running version prints `current`, exit 0; `--check` prints the
    running and the picked versions, needs no TTY and no `tools.gh` and
    makes no other read. Every request is GET over HTTPS to the two
    injected hosts, with no `Authorization` header, a 30 s timeout and
    the caps of REQ-1327; a non-2xx answer, a timeout, a body over the
    cap, and a release without the darwin archive of `runtime.GOARCH`
    or without `checksums.txt` are `upgrade-download-failed` (RJ-362),
    and nothing is written.
  - Before any download: no TTY is a usage error; `tools.gh` unset,
    not executable or below the floor (T195's reader) is
    `upgrade-gh-unusable` (RJ-364); a binary directory, links
    resolved, that the user cannot write is `upgrade-binary-unwritable`
    (RJ-365).
  - The sha256 of the archive is checked against `checksums.txt` in
    Go, and the attestation entry of the verification contract runs
    through `tools.gh` with the fixed argv (`--repo`,
    `--signer-workflow`, `--source-digest`, `--source-ref
    refs/heads/main`, `--deny-self-hosted-runners`) and the environment
    of REQ-0296; a `gh` helper re-executed from the test records both;
    a mismatch or a non-zero `gh` is `upgrade-verify-failed` (RJ-363)
    with `gh`'s output escaped and cut to 4 KiB, and the running binary
    is unchanged.
  - The new binary is extracted under a temporary name beside the
    running one and run as a preview subprocess (`upgrade --preview
    <running version>`, a flag the help does not list and every later
    release keeps, as `--finish <running version>` is for the exec),
    which prints the changelog sections, escaped through `termsafe`,
    and the toolchain changes of `approve`, then the prompt; any answer but `y` deletes the temporary file and is
    `upgrade-declined` (RJ-366), exit 1, and an fs snapshot diff shows
    nothing else changed; `y` renames the temporary file over the
    running binary and replaces the process (`syscall.Exec`) with the
    new one, which writes `toolchain.json` without a second prompt and
    runs the steps of `sync --all`; the e2e uses the `--dry-run` build
    of T103 as the new binary and the fake sbx.
  - The five error ids are declared in the package's `errors.go`; the
    call-site table admits the `gh` step, the preview step and the
    `syscall.Exec` of the swap, and a fixture package that calls
    `syscall.Exec` elsewhere fails the scan.
  - `e2e/scenarios/j9/` holds the first step of J9 as the command and
    the prompt it prints.
- Verify: `go test ./internal/cmd/romeu/upgrade/... -v`, which lists the
  cases by name; `go test -tags e2e ./e2e/... -run '^TestUpgrade' -v`;
  `go run ./tools/ci callsites`.
