# Dependency bumps run as a script, not by hand

What is at stake: medium. Each bump done by hand spends an agent's
context on steps that are the same every time, and a step forgotten
(a lock not regenerated, a checksum not compared) passes review as
easily as it is skipped.

Scope: this repository.

Decided on 2026-10-20: a version bump is deterministic, so that an
agent's work goes only where judgment beats a script (reading a
changelog, deciding whether to take a release).

Measured on 2026-10-20: the plan detects a stale pin deterministically
but bumps it by hand. `tools/ci pins` (REQ-1072, REQ-1073; T088, M6,
deferred) reports each tool pin behind a release, run by hand, its
scheduled run deferred (`docs/plan.md`, Decisions this plan defers);
`julieta lock --check` (REQ-0619) checks that `mise.lock` agrees with
`mise.toml`; the `vulnerabilities` step finds an urgent bump only by
turning every later pull request red. "Toolchain pins" in
`docs/plan.md` says each pin is a reviewed diff, and no record names a
bump tool. The Go bump to 1.27.3 of 2026-10-20 took five hand steps:
edit `go.mod` and `mise.toml`, `mise lock`, compare the four sha256
values with the `.sha256` files of dl.google.com, run
`go run ./tools/ci all`.

Options, to decide when reopened:

- A maintained update bot that already reads `go.mod`, `go.sum` and
  mise files, such as Renovate, opening one pull request per bump
  (library first, ADR 0038; the `dependencies` and `approvals` surfaces,
  ask first). The maintainer still merges.
- A `tools/ci` subcommand that applies one bump, regenerates the lock
  and compares the checksums, for the pins a bot does not cover.

Reopen: at the next pin bump, when T088 is planned, or when the
`vulnerabilities` step turns a pull request red again for a release
already published.
