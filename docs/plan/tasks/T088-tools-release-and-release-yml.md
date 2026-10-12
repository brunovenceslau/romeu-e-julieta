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
after: []
ask_first:
  - release
  - dependencies
  - checks
sitting: none
---

# T088 - `tools/release` and `release.yml`

Re-plan (2026-10-12): deferred to M6; release only; `go build` and the
`--dry-run` build of T103 serve the maintainer until then. Reopened by:
the first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 10 10.2 ("Release and bootstrap"), 12 12.6, 12 12.1, 02
  2.1 (Embedding), S1, 10 10.2 (`tools/ci pins`).
- Operator: no.
- Ask-first, as forecast before the re-plan: `release`, `dependencies`,
  `checks`.
- Acceptance:
  - `tools/release build` is T103's; `release.yml` runs it, and
    `go run ./tools/release build --version v1.0.0 --dry-run` embeds
    the four product kits of T079 and the catalog.
  - `tools/release notes` groups Conventional Commit subjects since the
    last tag that is not a prerelease, with each pull request's Why.
  - `tools/release verify` runs `gh attestation verify` with the signer
    workflow, starting the `gh` of `mise.lock` by the path `mise which`
    resolves, never through `mise exec` (12 12.1);
    `publish` marks a tag with a suffix as a prerelease and never
    creates a draft.
  - `notes`, `verify` and `publish` are each tested against a `gh`
    helper binary re-executed from the test and a recorded API fixture
    (10 10.1), so the candidate
    tag, which the tag ruleset lets nobody retry under the same name, is
    not their first run.
  - `release.yml` passes `tools/ci workflows`.
  - `gh` is in `mise.lock`, with a sha256 for each of the four
    platforms, from PR #27 (round 8, DR7), merged, and the lint
    row of 10 10.2 admits it.
  - The `tools/ci` test of 10 10.2 holds the `gh` pinned in `mise.toml`
    at or above the floor of 12 12.1, initially 2.68.0, read from that
    page, with its three cases: a pin below the floor fails, a pin at
    the floor passes, and a page with no floor it can parse fails.
  - `tools/ci pins` reports the pin freshness of each tool pin in the
    list of [12 12.1](../../spec/12-engineering.md#121-tech-stack), by the
    rules of that section: its sources and endpoints, every page read,
    the version parse, "newest", the lowercase status, plain text with
    one line per pin in table order, and exit 0 whatever the statuses,
    non-zero only on a usage or internal error.
  - Its requests are GET over HTTPS only, to a fixed list of hosts (the
    sources of 12 12.1: `go.dev`, `api.github.com`,
    `proxy.golang.org`, `hub.docker.com`), each with a body-size cap
    and a timeout, and none carries an `Authorization` header.
  - Its tests use one fixture per source, with rows for a pin behind,
    current and ahead; an API error, reported as `unknown`; a pin that
    does not parse, reported as `unknown`; a pin and a release equal
    but for a leading `v`, reported as `current`; a prerelease and a
    draft newer than the pin, and a suffixed version on a source with
    no prerelease flag, none of which marks it; a tag that is not a
    version; a tag with control characters, printed through
    `termsafe`; and, for the GitHub releases and the image tags, the
    newest release on the second page. They show that each file that
    holds a pin of that list is byte-identical after a run, and that
    `all` does not start the check. It is run once by hand on the real
    releases before this task merges, and its output is in the pull
    request body.
- Verify: `go test ./tools/release/...`, which lists the per-subcommand
  tests by name; `go test ./tools/ci/... -run '^TestPins'`, which lists
  the tests by name; `go run ./tools/release build --version v1.0.0
  --dry-run`; `go run ./tools/ci pins`.
