---
id: T161
title: "Dependency bumps run as a script, not by hand"
milestone: M6
module: ci-bootstrap
fate: deferred
owns: []
needs:
  - T088
after: []
ask_first:
  - checks
  - dependencies
  - kits
sitting: none
---

# T161 - Dependency bumps run as a script, not by hand

Goal: a version bump is deterministic work, so it runs as a script and
an agent's effort goes only where judgment beats a script (reading a
changelog, deciding whether to take a release). This task is the plan,
written docs only on 2026-10-22 under "Work authorized while the
operator is away on 2026-10-21" (01m5eg0qefcvwdx05q4vnqcwfe), for the
pending note "Dependency bumps run as a script, not by hand"
(01m5e14094yhqhsxq2g941pwrf). It measures what a maintained update bot
covers of this repository's pins, proposes a `tools/ci` subcommand for
the rest only, and proposes how the scheduled run of `tools/ci pins` is
decided. It decides nothing that is the maintainer's: every decision
below is an open question (the last section).

Scope of this text: this repository. It is written here, in this task
file, and nowhere else; nothing in it is a decision record (ADR 0039).
Library first (ADR 0038): the bot is the library; the subcommand is
proposed only for what the table shows no bot covers.

Re-plan (2026-10-22): new, and deferred to M6 beside T088, whose
`tools/ci pins` it reuses for the newest version of a pin (REQ-1070,
REQ-1073). `owns` is written when the task reopens. Reopened by: the
maintainer's answer to the first open question below; otherwise the next
pin bump. The pending note's own reopen event fired on 2026-10-20
(GO-2026-6809); this task is the plan that event asked for.

## What is pinned

Read on 2026-10-22 from the tree at origin/main f84fdebb: `mise.toml`
and `mise.lock` (five tools), `go.mod` and `go.sum`, `kits/pins.yaml`
(frontend, workload, herdr, mise), the `uses` lines and the mise step of
`.github/workflows/ci.yml` and `x64.yml`, and the `reuseImage` constant
of `tools/ci/internal/license/license.go` (line 23). REQ-1061 (12 12.1)
is the one list of tool pins; "Toolchain pins" in `docs/plan.md` adds
the Actions, the Go dependencies and the kit pins.

## What a bot covers, measured

Sources, each read on 2026-10-22. The Renovate documentation site
(docs.renovatebot.com) answered HTTP 403 to the fetch tool, so the
manager documentation was read from the source tree of
`github.com/renovatebot/renovate` at its default branch, files
`lib/modules/manager/<manager>/readme.md`, `index.ts` and, for mise,
`extract.ts`, `backends.ts` and `upgradeable-tooling.ts` (the latest
release then was 44.176.2, published 2026-10-21; the default branch may
be ahead of it). Dependabot: "Supported ecosystems and repositories" at
`docs.github.com/en/code-security/dependabot/ecosystems-supported-by-dependabot/supported-ecosystems-and-repositories`
(its ecosystem table has no mise, asdf or `.tool-versions` row), the README of `dependabot/dependabot-core`
(latest release v0.401.0, 2026-10-19), and the directory listing of
that repository's default branch, which has no `mise` directory. "Not
documented" below means those sources do not say.

| Pin | File | Renovate | Dependabot | Notes |
|---|---|---|---|---|
| `go` 1.27.3 | `mise.toml` | yes: manager `mise`, core tool `go`, GitHub tags of `golang/go` with the `go` prefix stripped | not documented (no mise ecosystem found) | handling of rc and weekly tags: not documented |
| `go` 1.27.3 | `go.mod` (`go` line) | partly: manager `gomod` does not bump the `go` directive by default, only with a package rule `rangeStrategy: bump`; it does update a `toolchain` line, which this file has none of | partly: ecosystem `gomod` updates modules; the `go` directive: not documented | the `go` line and the `mise.toml` pin must move together; grouping across two managers: not read |
| `gh` 2.102.0 | `mise.toml` | yes: `mise` registry tool, GitHub releases of `cli/cli` | not documented | the lock entry is `aqua:cli/cli`; the aqua backend is documented as supported except `http` package types and `version_filter` |
| `golangci-lint` 2.14.0 | `mise.toml` | partly: resolved through `mise registry --json` and the registry backend (documented to work when that backend is supported); its own entry was not in the files read | not documented | the backend it resolves to was not checked |
| `gopls`, `govulncheck` | `mise.toml` (`go:` backend) | yes: backend `go`, Go module datasource | not documented | no checksum in the lock; integrity rests on the Go checksum database (REQ-1063, REQ-1071) |
| the lock of all five | `mise.lock` | partly: the manager has `supportsLockFileMaintenance` and reads `mise.lock`; its lock maintenance runs `mise lock --bump`, which is for fuzzy selectors (exact pins like this repository's are bumped in `mise.toml` as normal updates, under `updatePinnedDependencies`); `mise lock` is documented as running repository-defined behavior, enabled for self-hosted only by `allowedUnsafeExecutions: mise` | not documented | whether the hosted Renovate app runs `mise lock`: not documented. No checksum handling was found in `extract.ts`: the sha256 values are not compared with any published file |
| Go dependencies (12 direct) | `go.mod`, `go.sum` | yes: manager `gomod`; `postUpdateOptions: gomodTidy` runs `go mod tidy`; `go.sum` is updated by running `go` | yes: ecosystem `gomod`, version and security updates | indirect updates are off by default in Renovate |
| `actions/checkout`, `actions/cache`, `jdx/mise-action` | `ci.yml`, `x64.yml` | partly: manager `github-actions` updates a SHA pinned `uses` with a version comment and keeps it pinned; the documented comment sits after the SHA on the same line, while this repository writes it on the line above (`# actions/checkout v7.0.1`); a bare SHA is disabled by default | yes: ecosystem `github-actions`, version and security updates; it updates the version comment when it is on the same line as `uses` | moving the comment to the line of `uses`, which both bots need, is a change to `.github/workflows/**` |
| mise version and the sha256 per runner | `with:` of the mise step and the `mise_sha256` matrix of `ci.yml`, `x64.yml` | no: the manager extracts "some version inputs from certain actions"; `jdx/mise-action` is not named; a sha256 is not a version | not documented | REQ-1065 and REQ-1068: this mise and the one of `kits/pins.yaml` are one version |
| mise version and its two linux sha256 | `kits/pins.yaml` | no: the file is YAML of no manager read; a `customManagers` regex can read a version, and the page does not say it can rewrite a standalone sha256 | not documented | `tools/kitpin mise <version>` writes it and verifies the signed `SHASUMS256.txt` with `kits/mise-minisign.pub` |
| herdr version and sha256 | `kits/pins.yaml` | no, as above | not documented | `tools/kitpin herdr <version>` |
| kit frontend `ref` (tag and digest) | `kits/pins.yaml` | not documented: the `dockerfile` manager reads Dockerfile and Containerfile names only; a custom regex with the docker datasource was not read | not documented (`docker` ecosystem: Dockerfiles; digests not confirmed) | `tools/kitpin frontend` |
| workload `ref` (digest) | `kits/pins.yaml` | not documented, as above | not documented | `tools/kitpin workload <ref>`; `julieta pin check` reports a spec's workload that is behind |
| `reuse` image tag and digest | `tools/ci/internal/license/license.go` | no: a Go constant is read by no manager; only a custom regex could | no | REQ-1066, REQ-1067: the index digest names two linux manifests |

What the bots leave: the Go bump as one change (the `go.mod` line and the
`mise.toml` pin), the comparison of the lock's sha256 values with the
published files, the mise pin in three files (`kits/pins.yaml`, `ci.yml`, `x64.yml`), the kit pins and the
`reuse` image. What no source says: whether a hosted bot may run `mise
lock`, and so whether a bot's `mise.toml` bump arrives with a current
`mise.lock`. `julieta lock --check` of REQ-0619 checks that each tool
has a lock entry covering the two linux platforms, not that the locked
version equals the `mise.toml` pin, so as written it does not catch a
stale version; which check would was not measured.

## Proposal 1: a `tools/ci bump` subcommand, only for the rest

If the maintainer adopts Renovate for the rows marked yes, the
subcommand does what the table leaves. If no bot is adopted, it is the
whole bump and the bot rows become its steps. Either way it never
commits and never pushes; it leaves a working tree for a reviewed diff
("Toolchain pins": each pin is a reviewed diff).

Two modes, the second the smaller and first to build:

1. `go run ./tools/ci bump verify`: for every platform of every tool in
   `mise.lock` that has a checksum, compare it with the published file
   (for Go, the `.sha256` files of dl.google.com, the step of #221; for
   `gh`, the checksums of its release), report each mismatch, change no
   file. It is the step a bot cannot do and a hand bump forgets, and it
   is useful on a bot's pull request too. Network, GET over HTTPS to a
   fixed host list, as REQ-1070's sources are (T088's rules for requests).
2. `go run ./tools/ci bump <pin> [<version>]`: with no version, the
   newest from the sources of `tools/ci pins`; then
   1. refuse a dirty tree;
   2. edit every file that holds the pin, from one table kept beside
      REQ-1061: Go is `go.mod` and `mise.toml`; mise is `kits/pins.yaml`
      (by running `go run ./tools/kitpin mise <version>` as a subprocess:
      its writing and verifying code is in `tools/kitpin/internal/pin`,
      which `tools/ci` cannot import, and exporting a package instead
      touches `kits` and the call-site rules of `procgroup`) and the
      `version` and `mise_sha256` values of both workflows, the macOS
      sha256, which `kits/pins.yaml` does not hold, taken from the same
      signed `SHASUMS256.txt`; `reuse` is `license.go`; a `mise.toml` tool is one line;
   3. run `mise lock` in the environment of `mise.MiseEnv` of `tools/ci/internal/mise` (REQ-1071), so
      the lock is regenerated the way setup does;
   4. run `bump verify` on the result;
   5. run the probes REQ-1069 names for that pin, if the pin is one;
   6. run `go run ./tools/ci all` and print the diff and the result.

Library search (ADR 0038), to repeat when it reopens: `mise lock`
regenerates the lock, `tools/kitpin` and its `minisign` package write
and verify the kit pins, and `tools/ci pins` reads the sources, so the
new code is the table of files and the checksum comparison. A library
for the checksum comparison was not searched; none is claimed.

Surfaces the pull request of this task would touch: `checks` (the
subcommand lives in `tools/ci`), `kits` if a package of `tools/kitpin`
is exported for it, and `dependencies` only if a fixture or
a bump of its own is committed. A bump the subcommand performs later
touches `dependencies` (and `kits`, `approvals` for the workflows),
each with its approval line in that pull request.

## Proposal 2: how the scheduled run of `tools/ci pins` is decided

The row is in "Deferred decisions" of `docs/spec.md` (line 344); its
trigger, "a security advisory published against a pinned tool", fired on
2026-10-20 with GO-2026-6809. Options:

- A. Keep it by hand. No surface touched. It repeats the failure the
  pending note measured: nothing reports a stale pin between runs.
- B. A scheduled workflow (cron) that runs `tools/ci pins` and opens or
  updates an issue. Touches `.github/workflows/**` (`dependencies` and
  `approvals`, per pull request) and `tools/ci` (`checks`); the grammar
  of `tools/ci workflows` would need a row for `schedule` and for the
  issue write permission (not measured).
- C. The bot's schedule is the scheduled run for the pins the bot
  covers, and `pins` stays by hand for the rest (kit pins, `reuse`).
- D. Run `tools/ci pins` at each checkpoint of the handoff routine, in a
  session. No workflow, no surface; the cadence is the maintainer's.

Recommendation: C with D if a bot is adopted, since the bot's schedule
then answers the trigger for the pins it covers and D covers the rest
with no new workflow; B if no bot is adopted, weekly, because then
nothing else runs on a clock. Not A. Reopen: a stale-pin report that
reached the maintainer late, or an advisory against a pin the
`vulnerabilities` step does not scan (it names Go and modules; for
`gh`, `golangci-lint` and mise this was not measured).

## Open questions (the maintainer's)

1. Adopt a bot: Renovate (the only one of the two whose documents name
   mise and `mise.lock`), Dependabot (`gomod` and `github-actions`
   only, as far as read), or none? Adopting installs an app or adds a
   config file: outward facing and ask first. A Renovate or Dependabot
   config file is under no glob of `.github/ask-first.yaml` today; it
   would belong to `dependencies`, which is a change to `ask-first.yaml`
   (the `ask-first` surface).
2. If a bot: may it open pull requests for the `mise` and `github-actions`
   managers once the comments of the `uses` lines are moved to the line
   of the SHA, and is a stale `mise.lock` on its pull request accepted,
   given that no check measured here turns a stale locked version red
   (REQ-0619 checks entries and platforms, not versions)?
3. Build `bump verify` first, `bump <pin>` second, or neither?
4. Which option for the scheduled run (A to D)?
5. Does a pin that only a script covers (the `reuse` digest, the
   frontend and workload digests) stay by hand, as the second row of
   "Deferred decisions" (`docs/spec.md`, line 345) says?
6. No new dependency is proposed by this task; a bot is a service, not
   an import. Say so if the maintainer reads it otherwise.

- Implements: none (a plan; the requirement rows for `bump` are written
  when the task reopens, 12 12.11).
- Operator: yes for the first question: adopting a bot.
- Acceptance (when reopened, after the answers):
  - `bump verify` exits non-zero and names the platform when one
    sha256 of `mise.lock` differs from the published file, and exits 0
    on the current lock, each against a fixture served locally.
  - `bump go <version>` on a copy of the tree changes `go.mod` and
    `mise.toml` to that version, regenerates `mise.lock`, and the
    verify step passes; a dirty tree is refused.
  - A bump of `mise` changes `kits/pins.yaml`, `ci.yml` and `x64.yml`
    to one version, and `tools/ci kits` passes (REQ-1068).
  - `go run ./tools/ci all` exits 0.
- Verify: `go test ./tools/ci/... -run '^TestBump'`,
  `go run ./tools/ci bump verify`, and `go run ./tools/ci all`.
