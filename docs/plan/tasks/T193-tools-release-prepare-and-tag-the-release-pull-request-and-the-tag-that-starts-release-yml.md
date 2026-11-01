---
id: T193
title: "tools/release prepare and tag: the release pull request and the tag that starts release.yml"
milestone: M3
module: ci-release
fate: new
owns:
  - tools/release/internal/prepare/**
  - tools/release/internal/tag/**
  - tools/release/main.go
needs:
  - T192
  - T088
after: []
ask_first:
  - release
sitting: none
---

# T193 - tools/release prepare and tag: the release pull request and the tag that starts release.yml

Goal: the two maintainer commands of a release (12 12.6): `prepare`,
which generates the changelog section on a branch and opens the release
pull request, and `tag`, which tags the merge commit of that pull
request and starts `release.yml` on `main` with the tag as its input.

Added on 2026-11-01 (ADR 0044; decided on 2026-10-31, scope this
repository): per release the operator runs `tools/release prepare
--version <tag>`, merges the pull request, and runs `tools/release tag
<tag>`. The tag goes on the merge commit of the release pull request and
on no later one: a pull request merged after it would be an entry the
section lacks, which the drift check of `notes` (T088) refuses, and the
attestation records the commit the run started on, so `tag` refuses
when `origin/main` has moved (REQ-1338). Both commands run `gh` by the
path `mise which` resolves (REQ-1053) and are tested against the `gh`
helper binary of T088 (10 10.1).

- Implements: REQ-1338 (10 10.2), REQ-1093 (12 12.2), REQ-1096 (12
  12.2), REQ-1342 (12 12.6), REQ-1343 (12 12.6), REQ-1345 (12 12.6).
- Operator: yes: runs `prepare` and, after the merge, `tag`, for each
  release; the first release's commands and output go under this pull
  request's Evidence.
- Acceptance:
  - `prepare --version <tag>` refuses a tag not of the form of REQ-0899
    and REQ-1337, a tag whose `<major>.<minor>.<patch>` differs from
    `VERSION` (the check of `tools/release build`), a tag that exists
    locally or on `origin`, a dirty tree, and a `HEAD` that is not
    `origin/main` after a fetch; then makes the branch `release/<tag>`,
    runs T192's `changelog`, commits `CHANGELOG.md` with the subject
    `chore(release): <tag>` and a body
    that carries the sections of 12 12.4 (`## Why` naming the release,
    `## What changed`, `## Evidence` with the `changelog --stdout`
    output, `## Middleware` as `generator: tools/release changelog`,
    `## Lessons` as `none`), pushes the branch and opens the pull
    request with `gh pr create`; a fixture per refusal, and the helper
    records the `gh` argv.
  - `tag <tag>` fetches, refuses unless `origin/main` is a merge commit
    whose subject matches `^chore\(release\): <tag> \(#[0-9]+\)$` and
    the first section of `CHANGELOG.md` at it is the tag's, refuses a tag that exists, then creates the tag on
    that commit, pushes it, runs `gh workflow run release.yml --ref
    main -f tag=<tag>` and prints the Actions page of the workflow; the
    helper records the argv and the order (tag pushed before the
    dispatch); a fixture per refusal and for a `main` that moved.
  - Neither command writes outside the branch and the tag it names, and
    neither takes a `GH_TOKEN` of its own: `gh` runs with the
    operator's login, as J1 step 2 does.
- Verify: `go test ./tools/release/internal/prepare/...
  ./tools/release/internal/tag/... -v`, which lists the fixture cases by
  name.
