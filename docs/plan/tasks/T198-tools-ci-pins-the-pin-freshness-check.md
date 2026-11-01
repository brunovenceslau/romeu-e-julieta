---
id: T198
title: "tools/ci pins: the pin-freshness check"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T088
after: []
ask_first:
  - checks
sitting: none
---

# T198 - tools/ci pins: the pin-freshness check

Goal: `tools/ci pins`, which reports the freshness of each tool pin of
the list of 12 12.1 against its published versions.

Split on 2026-11-01 from T088 (scope this repository): when T088
reopened narrowed to `release.yml`, `verify`, `notes` and `publish`,
the pin-freshness check of its earlier text became this task, deferred
with T088's earlier trigger: the first adopter other than the
maintainer; or the maintainer's environments all on the new tools.
`owns` is written when it reopens. The text below is the one T088
carried before the re-plan.

- Implements: REQ-1067 (12 12.1), REQ-1070 (12 12.1), REQ-1072 (12
  12.1), REQ-1073 (12 12.1), REQ-1074 (12 12.1), REQ-1075 (12 12.1),
  REQ-1076 (12 12.1), REQ-1077 (12 12.1), REQ-1078 (12 12.1).
- Operator: no.
- Acceptance:
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
- Verify: `go test ./tools/ci/... -run '^TestPins'`, which lists the
  tests by name; `go run ./tools/ci pins`.
