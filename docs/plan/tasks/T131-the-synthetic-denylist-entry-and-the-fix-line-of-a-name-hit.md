---
id: T131
title: "The synthetic denylist entry and the fix line of a name hit"
milestone: M1
module: ci-release
fate: merged
owns:
  - tools/ci/denylist.yaml
  - tools/ci/internal/hygiene/testdata/**
  - tools/ci/internal/hygiene/fixline*.go
  - tools/ci/internal/hygiene/hygiene*.go
  - tools/ci/internal/pushedrange/pushedrange*.go
  - tools/ci/fast_test.go
needs: []
after:
  - T105
  - T128
ask_first:
  - checks
  - approvals
sitting: none
merged: 2026-10-18
pr: 146
---

# T131 - The synthetic denylist entry and the fix line of a name hit

Merged on 2026-10-18 in pull request #146. The text below is the one
written at the re-plan.

Goal: One entry of the real denylist written by `go run ./tools/ci
hygiene add` for a made-up name whose plaintext is committed under the
hygiene package's testdata, and the test that finds it through the real
file; and the fix line of 10 10.2 ("reword this line; after one refused
rewrite, or if it is your own identity, hand the location to the
maintainer, who re-authors the commit") appended to every name finding
of `hygiene` and of the pushed range. The name is made up, so an agent
may hold it. `after: T128` for the shared file.

Re-plan (2026-10-18): Changed. `owns` adds the files the fix line
reaches: `hygiene*.go` and `pushedrange*.go`, where the name findings
are made and tested, and `tools/ci/fast_test.go`, whose cases of the
pushed range read them. T128 names the same files and comes first.

Re-plan (2026-10-12, stage B): New. The hygiene fixtures and the real
denylist are one package, so the fix line of a forbidden-name hit joins
this task.

`tools/ci/denylist.yaml` is on the `approvals` surface as well as on
`checks` (05 5.3), so both lines are forecast.

- Implements: REQ-0767 (10 10.1), REQ-0863 (10 10.2), REQ-0898 (10
  10.2).
- Operator: no
- Acceptance:
  - The hygiene test reads the real file and asserts the matcher finds
    the plaintext written together and with each separator; removing the
    entry turns it red.
  - One constant holds the fix line; every "name" finding of the path
    rule, the line rule, the file rule and the pushed range ends with
    it, asserted by the cases "a path hit carries the fix line" and "a
    pushed-range hit carries the fix line".
- Verify: `go test ./tools/ci/... -run
  '^Test(SyntheticEntry|HygieneRules|FastPushedRange)'`.
