# Dependency bumps run as a script, not by hand

What is at stake: medium. Each bump done by hand spends an agent's
context on steps that are the same every time, and a step forgotten
(a lock not regenerated, a checksum not compared) passes review as
easily as it is skipped.

Scope: this repository.

Decided on 2026-10-20: a version bump is deterministic work, so it
runs as a script, and an agent's effort goes only where judgment beats
a script (reading a changelog, deciding whether to take a release).

Measured on 2026-10-20: the plan detects a stale pin deterministically
but bumps it by hand. `tools/ci pins` (REQ-1072, REQ-1073; T088, M6,
deferred, not built yet) is to report each tool pin behind a release,
run by hand; its scheduled run is a deferred decision (`docs/spec.md`,
Deferred decisions; `docs/plan.md`, Toolchain pins);
`julieta lock --check` (REQ-0619) checks that `mise.lock` agrees with
`mise.toml`; the `vulnerabilities` step finds an urgent bump only by
turning every later pull request red. "Toolchain pins" in
`docs/plan.md` says each pin is a reviewed diff, and no record names a
bump tool. The Go bump to 1.27.3 of 2026-10-20 (#221) took five hand
steps: edit `go.mod`; edit `mise.toml`; `mise lock`; compare the four
sha256 values with the `.sha256` files of dl.google.com; run
`go run ./tools/ci all`.

That bump answered GO-2026-6809, a security advisory against the pinned
Go, which is the reopen event of the deferred scheduled run in
`docs/spec.md` ("a security advisory published against a pinned tool,
however it is learned"): that deferral's trigger fired on 2026-10-20.

Options, to decide when reopened:

- A maintained update bot that already reads `go.mod`, `go.sum` and
  mise files, such as Renovate, opening one pull request per bump
  (library first, ADR 0038; the `dependencies` and `approvals` surfaces,
  ask first). The maintainer still merges.
- A `tools/ci` subcommand that applies one bump, regenerates the lock
  and compares the checksums, for the pins a bot does not cover.

Reopen: now, for the scheduled run, whose trigger fired with
GO-2026-6809; otherwise at the next pin bump, or when T088 is planned.
A `vulnerabilities` step turned red by a published fix is that same
event, not a second one.

Planned on 2026-10-22 in T161 (#239): the bot coverage measured, the
`tools/ci` subcommand proposed, and the scheduled run's options listed
as open questions in that task file, for the maintainer to answer.
