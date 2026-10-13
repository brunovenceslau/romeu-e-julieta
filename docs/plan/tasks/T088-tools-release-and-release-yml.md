---
id: T088
title: "`tools/release` and `release.yml`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T079
  - T081
  - T087
  - T103
  - T127
after:
  - T041
ask_first:
  - release
  - dependencies
  - checks
  - approvals
sitting: none
---

# T088 - `tools/release` and `release.yml`

Re-plan (2026-10-12): deferred to M6; release only; `go build` and the
`--dry-run` build of T103 serve the maintainer until then. Reopened by:
the first adopter other than the maintainer; or the maintainer's
environments all on the new tools. It needs T127 for the publish job row
of `tools/ci workflows`, comes after T041 for the J1 step 2 entry, and
forecasts `approvals`, since it adds `release.yml`. The text below is
the one written before the re-plan; `owns` is written when the task
reopens, with `e2e/scenarios/j1/step2_*.go` among its files.

- Implements: REQ-0006 (index), REQ-0031 (index), REQ-0556 (05 5.4),
  REQ-0730 (09 (opening)), REQ-0752 (10 10.1), REQ-0753 (10 10.1),
  REQ-0754 (10 10.1), REQ-0755 (10 10.1), REQ-0756 (10 10.1), REQ-0757
  (10 10.1), REQ-0758 (10 10.1), REQ-0759 (10 10.1), REQ-0760 (10 10.1),
  REQ-0761 (10 10.1), REQ-0762 (10 10.1), REQ-0763 (10 10.1), REQ-0764
  (10 10.1), REQ-0765 (10 10.1), REQ-0766 (10 10.1), REQ-0767 (10 10.1),
  REQ-0768 (10 10.1), REQ-0769 (10 10.1), REQ-0770 (10 10.1), REQ-0771
  (10 10.1), REQ-0772 (10 10.1), REQ-0773 (10 10.1), REQ-0774 (10 10.1),
  REQ-0811 (10 10.2), REQ-0814 (10 10.2), REQ-0870 (10 10.2), REQ-0875
  (10 10.2), REQ-0899 (10 10.2), REQ-0900 (10 10.2), REQ-0901 (10 10.2),
  REQ-0906 (10 10.2), REQ-0907 (10 10.2), REQ-0909 (10 10.2), REQ-0911
  (10 10.2), REQ-0912 (10 10.2), REQ-1011 (11 11.2), REQ-1053 (12 12.1),
  REQ-1058 (12 12.1), REQ-1067 (12 12.1), REQ-1070 (12 12.1), REQ-1072
  (12 12.1), REQ-1073 (12 12.1), REQ-1074 (12 12.1), REQ-1075 (12 12.1),
  REQ-1076 (12 12.1), REQ-1077 (12 12.1), REQ-1078 (12 12.1), REQ-1079
  (12 12.2), REQ-1093 (12 12.2), REQ-1173 (12 12.6), REQ-1174 (12 12.6),
  REQ-1175 (12 12.6), REQ-1176 (12 12.6), REQ-1177 (12 12.6), REQ-1190
  (12 12.9).
- Operator: item f of 10 10.2: turn on immutable releases before the
  first release candidate; the API answer under this pull request's
  Evidence.
- Ask-first, as forecast before the re-plan: `release`, `dependencies`,
  `checks`.
- Acceptance:
  - `tools/release build` is T103's; `release.yml` runs it, and `go run
    ./tools/release build --version v1.0.0 --dry-run` embeds the product
    kits present at that task (`os-base` only if sitting M2 showed a
    missing package) and the catalog.
  - `tools/release notes` opens with the link to `docs/` at the tag,
    then an Upgrading section generated from the differences since the
    last non-prerelease tag of the error ids and exit codes,
    `internal/spec/versions.go` and the embedded kit digests; it groups
    Conventional Commit subjects since that tag, and each entry carries
    its pull request's Why, author and closing-keyword issues; a pull
    request whose subject has `!` or that touches `contracts` is listed
    first; a fixture each (12 12.6).
  - `tools/release verify` runs `gh attestation verify` with the signer
    workflow, starting the `gh` of `mise.lock` by the path `mise which`
    resolves, never through `mise exec` (12 12.1).
  - `publish` creates the release as a draft, uploads every asset and
    publishes it in the same run, so no release is left a draft and no
    asset is added after publication (10 10.2; GitHub's immutable
    releases refuse that); a tag with a suffix after the patch number is
    marked a prerelease; the test against the `gh` helper asserts the
    publish call comes after the last upload.
  - `notes`, `verify` and `publish` are each tested against a `gh`
    helper binary re-executed from the test and a recorded API fixture
    (10 10.1), so the candidate tag, which is published once and not
    retried, is not their first run.
  - `release.yml` has one test job per runner of 10 10.2 running
    `tools/ci all` on the tagged commit with the read permissions of
    `ci.yml`, and one publish job on `ubuntu-26.04` that needs them all,
    runs no test, runs `go run ./tools/ci setup` first and the release
    steps with `GOPROXY=off`; the run records the runner image version
    and restores no cache; no job holds a signing credential, the
    keyless attestation is the only signature (ADR 0009, decision 19);
    `release.yml` takes its workflow file from the default branch, never
    the tagged commit, and `tools/ci workflows` has a fixture for each.
  - The pull request decides how releases come from `main` only and
    closes the index row on how `release.yml` is started and the row on
    a check that reads the rulesets again, whose trigger it is, with a
    fixture of `tools/ci workflows` or a **[review]** line under
    Evidence showing the workflow file never comes from the tagged
    commit; both rows are in the table "These rows meet a task of this
    plan" of `docs/plan.md`.
  - The verification contract's two-line attestation entry and the
    checksum line are one block of J1's `commands.yaml`, the tag its one
    variable (10 10.2): the J1 step 2 entry of
    `e2e/scenarios/j1/commands.yaml` holds the three verification
    commands of 09 as one block with `<tag>` as its one variable, in
    `e2e/scenarios/j1/step2_*.go`; the scenario test expands it and runs
    `shasum -c` and the two `gh` lines against a fixture release served
    by the `gh` helper of this task.
  - `gh` is in `mise.lock`, with a sha256 for each of the four
    platforms, from PR #27 (round 8, DR7), merged, and the lint row of
    10 10.2 admits it.
  - The `tools/ci` test of 10 10.2 holds the `gh` pinned in `mise.toml`
    at or above the floor of 12 12.1, initially 2.68.0, read from that
    page, with its three cases: a pin below the floor fails, a pin at
    the floor passes, and a page with no floor it can parse fails (round
    18, ship gate round 1, F11).
  - `tools/ci pins` reports the pin freshness of each tool pin in the
    list of [12 12.1](../../spec/12-engineering.md#121-tech-stack), by
    the rules of that section: its sources and endpoints, every page
    read, the version parse, "newest", the lowercase status, plain text
    with one line per pin in table order, and exit 0 whatever the
    statuses, non-zero only on a usage or internal error; it reads the
    `reuse` index's two linux manifests beside its attestation
    manifests.
  - Its requests are GET over HTTPS only, to a fixed list of hosts (the
    sources of 12 12.1: `go.dev`, `api.github.com`, `proxy.golang.org`,
    `hub.docker.com`), each with a body-size cap and a timeout, and none
    carries an `Authorization` header.
  - Its tests use one fixture per source, with rows for a pin behind,
    current and ahead; an API error, reported as `unknown`; a pin that
    does not parse, reported as `unknown`; a pin and a release equal but
    for a leading `v`, reported as `current`; a prerelease and a draft
    newer than the pin, and a suffixed version on a source with no
    prerelease flag, none of which marks it; a tag that is not a
    version; a tag with control characters, printed through `termsafe`;
    and, for the GitHub releases and the image tags, the newest release
    on the second page. They show that each file that holds a pin of
    that list is byte-identical after a run, and that `all` does not
    start the check. It is run once by hand on the real releases before
    this task merges, and its output is in the pull request body.
- Verify: `go test ./tools/release/...`, which lists the per-subcommand
  tests by name; `go test ./tools/ci/... -run '^TestPins'`, which lists
  the tests by name; `go run ./tools/release build --version v1.0.0
  --dry-run`; `go run ./tools/ci pins`.
