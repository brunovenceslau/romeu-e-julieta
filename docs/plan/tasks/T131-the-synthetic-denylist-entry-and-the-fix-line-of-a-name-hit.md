---
id: T131
title: "The synthetic denylist entry and the fix line of a name hit"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/denylist.yaml
  - tools/ci/internal/hygiene/testdata/**
  - tools/ci/internal/hygiene/fixline*.go
needs:
  - T105
after:
  - T128
ask_first:
  - checks
sitting: none
---

# T131 - The synthetic denylist entry and the fix line of a name hit

Goal: One entry of the real denylist written by `go run ./tools/ci
hygiene add` for a made-up name whose plaintext is committed under the
hygiene package's testdata, and the test that finds it through the real
file; and the fix line of 10 10.2 ("reword this line; after one refused
rewrite, or if it is your own identity, hand the location to the
maintainer, who re-authors the commit") appended to every name finding
of `hygiene` and of the pushed range. The name is made up, so an agent
may hold it. `after: T128` for the shared file.

Re-plan (2026-10-12, stage B): New. The hygiene fixtures and the real
denylist are one package, so the fix line of a forbidden-name hit joins
this task.

- Implements: REQ-0863 (10 10.2), REQ-0898 (10 10.2).
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
