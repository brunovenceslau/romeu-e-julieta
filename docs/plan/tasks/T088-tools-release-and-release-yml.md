---
id: T088
title: "`tools/release` and `release.yml`"
milestone: M3
module: ci-release
fate: kept
owns:
  - .github/workflows/release.yml
  - tools/release/internal/build/fuzzgate*.go
  - tools/release/internal/verify/**
  - tools/release/internal/notes/**
  - tools/release/internal/publish/**
  - tools/release/internal/gh/**
  - tools/release/main.go
  - tools/ci/internal/workflows/release*.go
  - e2e/scenarios/j1/step2_*.go
needs:
  - T079
  - T081
  - T103
  - T127
  - T192
after:
  - T041
  - T170
ask_first:
  - release
  - dependencies
  - checks
  - approvals
sitting: none
---

# T088 - `tools/release` and `release.yml`

Goal: `release.yml` and the three publish-job subcommands of
`tools/release` that T103 did not build: `verify`, `notes` and
`publish`.

Re-plan (2026-10-12): deferred to M6; release only; `go build` and the
`--dry-run` build of T103 serve the maintainer until then. Reopened by:
the first adopter other than the maintainer; or the maintainer's
environments all on the new tools.

Re-plan (2026-11-01, decided on 2026-10-31, scope this repository): reopened before its trigger, since release automation and
`romeu upgrade` go before the M3 critical path, and narrowed to slice 1:
`release.yml`, `verify`, `notes`, `publish` and the `gh` helper their
tests share. What changed in the specification since the earlier text:
every tag carries a suffix until `fuzz.yml` exists (REQ-1337), and a
prerelease tag skips the fuzz gate of REQ-0902 while it does not, so
T087 leaves `needs`; the workflow starts by `workflow_dispatch` on
`main` with the tag as its input, and its first job refuses unless the
tag's commit is the commit the run started on (REQ-1338), which closes
the index row on how `release.yml` is started; the verification contract
gains `--source-ref refs/heads/main` (REQ-1339); `notes` prints the
section of `CHANGELOG.md` and refuses one that differs from what
git-cliff renders at the tag (REQ-1347; ADR 0044), so it reads no GitHub
API and the job needs no `pull-requests` or `issues` permission; the
pin-freshness check `tools/ci pins` of the earlier text is slice 2,
T198, deferred with the earlier trigger; the index row on a check that reads the rulesets
again has `release.yml` landing as its trigger, so this pull request
decides it or opens the `deferred` issue that names the row. `fuzzgate.go`
is T103's merged file and changes here by the prerelease rule only;
the `workflows` fixtures of `release.yml` are files of their own, after
T170, which owns that package.

- Implements: REQ-0006 (index), REQ-0031 (index), REQ-0556 (05 5.4),
  REQ-0730 (09 (opening)), REQ-0773 (10 10.1), REQ-0811 (10 10.2),
  REQ-0814 (10 10.2), REQ-0870 (10 10.2), REQ-0875 (10 10.2), REQ-0899
  (10 10.2), REQ-0900 (10 10.2), REQ-0901 (10 10.2), REQ-0902 (10 10.2),
  REQ-0906 (10 10.2), REQ-0907 (10 10.2), REQ-0909 (10 10.2), REQ-0911
  (10 10.2), REQ-0912 (10 10.2), REQ-0914 (10 10.2), REQ-1011 (11 11.2),
  REQ-1053 (12 12.1), REQ-1058 (12 12.1), REQ-1226 (10 10.2), REQ-1227
  (10 10.2), REQ-1337 (10 10.2), REQ-1338 (10 10.2), REQ-1339 (10 10.2),
  REQ-1347 (12 12.6), REQ-1350 (12 12.6).
- Operator: item f of 10 10.2: turn on immutable releases before the
  first tag; the API answer under this pull request's Evidence. The
  first run of `release.yml` reads each endpoint with inferred
  permissions; its log goes under the Evidence of the pull request that
  fixes them, if any.
- Ask-first: `release`, `dependencies` and `approvals` (the workflow
  file), `checks` (the `workflows` fixtures).
- Acceptance:
  - `release.yml` starts by `workflow_dispatch` with the one input
    `tag`, checked against the tag form before any step uses it; a
    first job refuses unless `github.ref` is `refs/heads/main`, the tag
    exists and the commit it names equals `github.sha` (a guard; the
    enforcement is `--source-ref` on the consumer side, REQ-1338); the
    checkout of the publish job takes the full history and the tags,
    which `notes` needs; one
    test job per runner of 10 10.2 runs `tools/ci all` on that commit
    with the read permissions of `ci.yml`; one publish job on
    `ubuntu-26.04` needs them all, runs no test, runs `go run ./tools/ci
    setup` first and the release steps with `GOPROXY=off`, in the order
    build, attest (`actions/attest-build-provenance`, pinned by sha),
    verify, notes, publish, with the permissions of the publish job row
    of T127 and no cache restore; no job holds a signing credential.
    `tools/ci workflows` has a fixture for each, and one that fails a
    workflow started by a `push` of a tag, which is how the file never
    comes from the tagged commit.
  - `tools/release build` skips the fuzz gate for a tag with a suffix
    while `.github/workflows/fuzz.yml` is not in the tree, and never for
    a tag without one; a fixture each, beside the six of REQ-0902.
  - `tools/release verify` runs `gh attestation verify` with the five
    flags of the verification contract, starting the `gh` of
    `mise.lock` by the path `mise which` resolves, never through `mise
    exec` (12 12.1).
  - `tools/release notes --version <tag>` prints the section of the tag
    from `CHANGELOG.md` at `HEAD`, computes the Upgrading part through
    T192's code, renders `git-cliff --current --strip all` with it, and
    refuses when the two differ or the section is missing; fixtures:
    equal, a hand edit, a pull request merged after the release pull
    request, a missing section.
  - `publish` creates the release as a draft, uploads every asset and
    publishes it in the same run with the section of `notes` as its
    body, so no release is left a draft and no asset is added after
    publication (10 10.2; GitHub's immutable releases refuse that); a
    tag with a suffix after the patch number is marked a prerelease;
    the test asserts the publish call comes after the last upload.
  - `verify`, `notes` and `publish` are each tested against a `gh`
    helper binary re-executed from the test and a recorded API fixture
    (10 10.1), so the first tag, which is published once and not
    retried, is not their first run.
  - The verification contract's attestation entry and the checksum line
    are one block of J1's `commands.yaml`, with `--source-ref
    refs/heads/main`, the tag its one variable (10 10.2), in
    `e2e/scenarios/j1/step2_*.go`; the scenario test expands it and runs
    `shasum -c` and the two `gh` lines against a fixture release served
    by the `gh` helper of this task.
  - The `tools/ci` test of REQ-0911 holds the `gh` pinned in `mise.toml`
    at or above the floor of 12 12.1, read from that page, with its
    three cases, as the earlier text asked.
- Verify: `go test ./tools/release/... -v`, which lists the
  per-subcommand tests by name; `go run ./tools/ci workflows`; `go run
  ./tools/release build --version v1.0.0-alpha.1 --dry-run`.
