---
id: T139
title: "The lint split and the lint set"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - .golangci.yml
  - tools/ci/internal/lint/config*.go
  - tools/ci/internal/lint/testdata/**
needs: []
after:
  - T105
ask_first:
  - checks
sitting: none
---

# T139 - The lint split and the lint set

Goal: `lintconfig_test.go` split by linter group under the lint package
T105 made, and the lint set of item 16 of Decisions of the re-plan in
`docs/plan.md`, decided on 2026-10-18: option A (`errorlint`,
`usetesting`, `thelper`, `forbidigo` with the Boundaries patterns and
revive `package-comments`)
with the `tidy` rule of `generated` (`go mod tidy` changes neither
`go.mod` nor `go.sum`), one fixture violation per enabled linter.

Re-plan (2026-10-18): Changed. Item 16 of Decisions of the re-plan, the
lint set, was decided on 2026-10-18 (option A with the `tidy` rule), so
the task no longer waits for sitting M0.

Re-plan (2026-10-12, stage B): New, split from T105 so that the move of
the checks is one frozen-behaviour pull request and the new rules are
another; a lint fixture that flakes on one runner then holds no M1 task
behind it.

- Implements: REQ-1224 (10 10.2), REQ-1225 (10 10.2).
- Operator: no; item 16 of Decisions of the re-plan (the lint set) was
  decided on 2026-10-18.
- Acceptance:
  - `.golangci.yml` enables the doc-comment, error-string and
    commented-out-code checkers, testifylint with `enable-all: true`
    and the linters of item 16; it disables no linter and excludes no
    finding.
  - One fixture test per enabled linter proves it still reports, each
    titled by the linter, and the fixtures of the rule 14 checkers and
    of testifylint stay green after the split.
  - The `tidy` rule of `generated` fails a tree whose `go mod tidy`
    changes `go.mod` or `go.sum`, with a fixture.
  - Green on the runners of `ci.yml`, and of `x64.yml` when it runs
    (10 10.2, Runners).
- Verify: `go run ./tools/ci all` exits 0; `go test ./tools/ci/... -run
  '^TestLint'`.
