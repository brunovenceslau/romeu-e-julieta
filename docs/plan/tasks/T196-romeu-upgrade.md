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

Built on 2026-11-02 (scope this repository): the root package is
`changelog` (`changelog.go`, imported as `changelog` by `upgrade`); its
`Since` cuts the file at the level-2 headings of REQ-1350, the rule
`tools/release/internal/changelog` reads, written again since a product
package imports nothing under `tools/`, and the file's own rule, not a
Markdown parse (ADR 0038); a dev build's own version has no section, so
it prints none. The command uses the standard library alone (`net/http`,
`archive/tar`, `compress/gzip`, `crypto/sha256`): no module is added.
The injected values are three variables of `release.go`, the API host,
the asset host and a root certificate, which the e2e sets with `-ldflags
-X`; a release sets none and reads `api.github.com` and
`release-assets.githubusercontent.com`, the asset host 07 7.4 measured.
An asset is read by its id on the API host, and the client follows a
redirect to the asset host alone, over HTTPS. The verification argv is
two functions of `verify.go`, `tagCommitArgv` and `attestArgv`, which
follow ADR 0045, decision 1, as merged. These choices are provisional,
each open to the maintainer's review: the preview prints and the running
romeu asks, through `gate.Confirm`, so the preview's stdin stays closed
as REQ-0270 says of every other subprocess; its question, `install this
romeu and acknowledge this toolchain? [y/N]`, replaces gate 1's at the
end of what `approve --toolchain` prints; its environment adds
`ROMEU_SETTINGS` and `XDG_STATE_HOME` to the one of 4.1, so the new
binary reads the settings and the state the running one read; a preview
that fails is `upgrade-verify-failed`, since the archive did not prove
it runs here; `--check` prints a third line, `current`, when the two
versions are the same; the running romeu holds the lock from after the
checks of REQ-1326 until the swap, as `run` releases it before its exec.
The edits outside `owns` the work forced: the class `Preview` (2 min)
of `internal/procgroup` and its row in the Starting values of 04 4.1,
since the preview had no class; `doctor.GhAtFloor` in
`internal/cmd/romeu/doctor/gh_floor.go`, T195's reader exported for
`upgrade`, with its test; `approve.TextOf` and `approve.Acknowledge` in
`internal/cmd/romeu/approve/approve.go`, which the preview and the
finish call and `approve` itself now uses; `upgrade` in the list of
commands that take the lock (04 4.1, Locking, REQ-0284, whose rule its
Writes cell meets) and in the list of `internal/state/lock_commands_test.go`;
`changelog.go` among the files `tools/ci/internal/lint` lets hold a
`go:embed`, as 02 2.1 (REQ-0129) names it; and the reference pages of
`docs/reference/`, regenerated, with the line of the new page in
`.gitattributes`. The gh of the unit tests is the test binary run again
as a helper process (`TestHelperProcess`, the form of
`internal/procgroup`), behind a wrapper script that prints its answer,
since the package's tests may not write to the process's streams. The e2e answers y on a machine with no
clone of the config repo, so the new binary records gate 1 and the
steps of `sync --all` stop at step 2 (`config-clone-missing`, exit 2).

Reviewed on 2026-11-02 (code review and security audit; scope this
repository), and changed so. Three of the provisional choices above are
now the text of the specification, for the maintainer's approval
through the `contracts` line: the preview prints and the running romeu
asks (REQ-1348), the preview's environment adds `ROMEU_SETTINGS` and
`XDG_STATE_HOME` (REQ-1348, not the general list of REQ-0296), and a
preview that fails is `upgrade-verify-failed` (REQ-1333, with an archive
that does not extract); REQ-1327 names the 30 s timeout as built and a
cap of 1 MiB for `checksums.txt`. `--finish` is bound to the swap that
just happened (REQ-1330), so a process of the same user cannot run it
to acknowledge gate 1 with no prompt: it needs a terminal on its stdin
and stderr, as `install` does, and the finish token. The token is part
of the interface every later release keeps, as `--preview` and
`--finish` are, since the old binary writes it and the new one reads
it: the file `upgrade-finish` of the romeu state directory (02 2.4),
mode 0600, created with `O_EXCL` and `O_NOFOLLOW` right before the
exec, in the line format `romeu-upgrade-finish/1`, `version <running
version>`, `sha256 <hex of the new binary>`, `expires <RFC 3339 UTC>`,
each line ended by a line feed (`token.go`); `--finish` opens it with
`O_NOFOLLOW`, deletes it, reads at most 4 KiB, and refuses one that is
missing, not a regular file of the user with mode 0600, of another
format, expired or more than 60 s ahead, of another sha256 than its own
executable, or of another version than its argument. Provisional, each
open to review: the swap writes the token after the rename and before
the exec, so a token that does not write leaves the new binary
installed with the message of a failed exec; the swap removes a stale
token before it writes, and removes its own after a failed exec; the
token's expiry comes from the command's clock (`env.Now`, 60 s).
Before the rename the swap checks the temporary file with `Lstat`
(regular, the user's, mode 0755) and its sha256 against the one
`extract` computed from the verified archive, or it is
`upgrade-verify-failed` and deletes it. `runningBinary` refuses a
directory another user owns or its group or others may write (RJ-365,
the message naming its mode); the attestation names the archive to gh
by its absolute path; no URL reaches an error with its query, user or
fragment, the signed redirect of the asset host included. The captures
of gh's output take a lock (procgroup copies stdout and stderr in two
goroutines). The edits outside `owns` this review forced: `checkGhFloor`
of `internal/cmd/romeu/doctor/gh_floor.go` now calls `GhAtFloor` and
maps its refusals (unset warns, a refused file or a version below the
floor fails, any other error is an error, a timeout is
`subprocess-timeout`), with doctor's tests unchanged; `approve.TextOf`
takes the record its caller loaded, so `approve` loads it once;
`releaseLDFlags` of `tools/release/internal/build/build.go`, with
`TestReleaseLDFlags`, which holds that a release sets `-X` for
`main.version`, `main.commit` and `main.date` alone; the line of
`upgrade-finish` in the state tree of 02 2.4 and a sentence on the
three test seams under REQ-0940 (10 10.4). The recorded fixture of 10
10.1 is `testdata/releases-recorded.json`, the release list of
goreleaser/goreleaser read unauthenticated on 2026-11-02 (its
provenance in `testdata/REUSE.toml`), beside the hand-written
`testdata/releases-handmade.json` of the draft-first and prerelease
cases.

Deferred, each with the event that reopens it: a warning or a flag for
a downgrade, before the first stable tag (v1.0.0); a sweep of
`.romeu-upgrade-*` files a crash left beside the binary, at the first
report of one; holding the lock through the exec and keeping
`.romeu-previous` for a rollback, if a failed exec is ever reported; a
check at the workflow level that a release build injects nothing but
the version stamp, beyond `TestReleaseLDFlags`, with T088's build of
`release.yml`.

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
