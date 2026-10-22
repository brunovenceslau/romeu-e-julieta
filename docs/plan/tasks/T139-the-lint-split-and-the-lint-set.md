---
id: T139
title: "The lint split and the lint set"
milestone: M1
module: ci-bootstrap
fate: merged
owns:
  - .golangci.yml
  - tools/ci/internal/setup/setup.go
  - tools/ci/internal/setup/setup_test.go
  - tools/ci/internal/lint/**
  - tools/ci/fast_test.go
  - tools/ci/all_test.go
  - tools/ci/checks_test.go
  - tools/ci/internal/drift/**
  - tools/ci/check_generated.go
  - docs/spec/10-testing-style.md
  - tools/ci/internal/hygiene/hygiene_test.go
  - tools/ci/internal/mise/misefiles_test.go
  - tools/ci/internal/mise/which_test.go
  - tools/ci/internal/moduletree/moduletree_test.go
  - tools/ci/internal/license/license.go
  - tools/ci/git/gittest/gittest.go
  - tools/mutants/main_test.go
  - tools/new/main_test.go
  - internal/cli/testsupport/testsupport.go
  - internal/cli/testsupport/testsupport_test.go
  - internal/gitsafe/gitsafe_test.go
  - internal/gitsafe/helper_test.go
  - internal/signing/signing_test.go
  - go.mod
needs: []
after:
  - T010
  - T105
  - T125
ask_first:
  - checks
  - dependencies
  - gitsafe
  - signing
sitting: none
merged: 2026-10-18
pr: 162
---

# T139 - The lint split and the lint set

Merged on 2026-10-18 in pull request #162. The text below is the one
written at the re-plan.

Goal: `config_test.go` split by linter group under the lint package
T105 made, and the lint set of item 16 of Decisions of the re-plan in
`docs/plan.md`, decided on 2026-10-18: option A (`errorlint`,
`usetesting`, `thelper`, `forbidigo` with the Boundaries patterns and
revive `package-comments`)
with the `tidy` rule of `generated` (`go mod tidy` changes neither
`go.mod` nor `go.sum`), one fixture violation per enabled linter.

Re-plan (2026-10-18): Changed. The new linters report findings in
today's tree: 156 of thelper (151 helpers without `t.Helper()`, 5
`testing.TB` parameters not named `tb`), 1 of usetesting, 1 of revive
`package-comments` and 38 of forbidigo, none of errorlint. The files
that hold them belong to other tasks, so `owns` lists each of them (the
whole lint and drift packages, which this task changes throughout, and
otherwise one file each), `tools/ci/check_generated.go` for the Guards
text of `generated`, and `docs/spec/10-testing-style.md`, where
REQ-1224, REQ-1225 and the `generated` row (REQ-0822) now name the lint
set and the `tidy` rule, and a new requirement states the boundary run.

The tidy rule runs offline, and tidy loads the tests of each
dependency, whose modules `go mod download` does not fetch, so T139
also owns `tools/ci/internal/setup/setup.go` and its test: setup runs
`go mod tidy -diff`, which writes nothing, with the module proxy, to
fill the module cache.

forbidigo cannot be scoped by the package of the caller: its `pkg`
setting names the package of the referenced symbol, and the only other
scoping, `linters.exclusions` and nolint, is barred by ADR 0007 rule 8.
So the lint step makes a second golangci-lint run for each target, with
its own configuration that enables forbidigo alone, over the packages
outside an allow list; the allow list is one Go list in the lint
package, and `lint.go` and its tests are owned for that. An empty list
of packages fails the step instead of dropping it, and a test over the
real module holds the list non-empty.

The patterns are the output ones: `fmt.Print*`, `os.Stdout`,
`os.Stderr`, `log.` and `text/template`. `exec.Command(` is not a
pattern: 04 4.1 admits `os/exec` in `internal/procgroup` only
(REQ-0298, REQ-0299), and the call-site test in
`internal/procgroup/callsite_test.go` holds that more strictly than a
pattern could, so the call-site scan owns it. `"sh", "-c"` (REQ-0959)
cannot be matched by forbidigo, which reads identifiers and selectors
and no string argument; a Go AST test in the lint package, over the
real module, fails a call whose string arguments include "sh" followed
by "-c", with a one-violation fixture, so that pattern is held and not
recorded as a gap.

04 4.1 names no set of packages for the output patterns, so the allow
list (`Allowed` in `tools/ci/internal/lint/boundaries.go`) is a decision
of this task, derived from REQ-0269 (all output through the CLI writer)
and the composition roots: the writer, the main packages, and the
helper processes of `internal/procgroup`. Open: the operator confirms
the list in the pull request. A test helper process is judged like any
code and no exclusion is allowed, so `drift_test.go` writes its failing
generator's message with `println`, and `signing_test.go` makes its
short socket directory from `os.TempDir()`: usetesting flags
`os.MkdirTemp`, and the name `t.TempDir` gives is too long for the
104-byte socket path. These two are decided workarounds.

The task comes after T010 (itself after T132) and T125, which own
`tools/ci/all_test.go` and `tools/ci/internal/license/license.go`, and
T053, which owns `tools/ci/internal/mise/misefiles_test.go` and lands
much later, comes after T139; it also owns
`tools/ci/checks_test.go`, which pins the steps of the lint check and
gains the second run. Its ask-first forecast adds `dependencies`,
`gitsafe` and `signing`, the surfaces of files it owns. The file name in
the goal was `lintconfig_test.go` and is `config_test.go`. The `tidy`
rule fails today's tree, whose `go.mod` lists `github.com/spf13/pflag`
as indirect while a package imports it, so the task owns `go.mod` for
that one line, after T125, which owns it and `go.sum`. The finding of
the rule says that tidy changed the file and that the fix is to commit
tidy's result.

Re-plan (2026-10-18): Changed. Item 16 of Decisions of the re-plan, the
lint set, was decided on 2026-10-18 (option A with the `tidy` rule), so
the task no longer waits for sitting M0.

Re-plan (2026-10-12, stage B): New, split from T105 so that the move of
the checks is one frozen-behaviour pull request and the new rules are
another; a lint fixture that flakes on one runner then holds no M1 task
behind it.

- Implements: REQ-1224 (10 10.2), REQ-1225 (10 10.2), REQ-1251 (10 10.2), REQ-1252 (10 10.2).
- Operator: no; item 16 of Decisions of the re-plan (the lint set) was
  decided on 2026-10-18.
- Acceptance:
  - `.golangci.yml` enables the doc-comment, error-string and
    commented-out-code checkers, testifylint with `enable-all: true`,
    errorlint, usetesting, thelper and revive `package-comments`; it
    disables no linter and excludes no finding. forbidigo runs apart,
    with its own configuration, over the packages outside the allow list.
  - One fixture test per enabled linter proves it still reports, each
    titled by the linter, and the fixtures of the rule 14 checkers and
    of testifylint stay green after the split.
  - A fixture proves forbidigo reports a forbidden output call in a
    package outside the allow list and in none inside it, and a test
    fails a call with "sh" then "-c".
  - The `tidy` rule of `generated` fails a tree whose `go mod tidy`
    changes `go.mod` or `go.sum`, with a fixture.
  - Green on the runners of `ci.yml`, and of `x64.yml` when it runs
    (10 10.2, Runners).
- Verify: `go run ./tools/ci all` exits 0; `go test ./tools/ci/... -run
  '^TestLint'`.

Re-plan (2026-10-21): changed by T157. The `generated` row (REQ-0822) names the race step in place of `unit` in the order it holds against the steps that run code of the change.
