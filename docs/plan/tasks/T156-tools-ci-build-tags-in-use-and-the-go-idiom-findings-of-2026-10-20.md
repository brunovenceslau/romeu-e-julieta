---
id: T156
title: "tools/ci: build tags in use and the Go idiom findings of 2026-10-20"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - tools/ci/internal/taggedsuites/**
  - tools/ci/check_taggedsuites.go
  - tools/ci/internal/lint/lint.go
  - tools/ci/internal/lint/lint_test.go
  - tools/ci/internal/fix/**
  - tools/ci/check_fix.go
  - tools/ci/internal/imports/imports.go
  - tools/ci/internal/imports/imports_test.go
  - tools/ci/internal/license/modules.go
  - tools/ci/internal/license/modules_test.go
  - tools/ci/internal/race/**
  - tools/ci/check_race.go
  - tools/ci/internal/container/**
  - tools/ci/check_container.go
  - tools/ci/internal/coverage/**
  - tools/ci/check_coverage.go
  - tools/ci/internal/citest/**
  - tools/ci/fast_test.go
  - tools/ci/all_test.go
  - internal/moduleroot/**
  - e2e/internal/container/**
  - e2e/container_test.go
  - docs/spec/10-testing-style.md
  - docs/reference/checks.md
  - docs/plan/tasks/T002-tools-ci-workflows-tools-ci-all-coverage.md
  - docs/plan/tasks/T013-the-container-e2e-on-the-workload-s-base.md
  - docs/plan/tasks/T103-tools-release-build-dry-run.md
  - docs/plan/tasks/T152-the-go-fix-gate.md
needs: []
after:
  - T155
  - T150
ask_first:
  - checks
sitting: none
---

# T156 - tools/ci: build tags in use and the Go idiom findings of 2026-10-20

Goal: the second part of "One race run, test tags and parallel CI jobs"
(01m5drzqsp6zg34xzmywckwb7b): the findings of the Go idiom audit of
2026-10-20 in `tools/ci` and the e2e harness, fixed as defects, and
one source for the tags that the lint, fix, imports, license and
tagged-suite steps pass. The steps run the same tests after this task
as before it; what changes is where each value comes from and the
reason each flag states. The `unit` step is left as it is: T157
removes it.

1. **Tags.** Four lists name the tags of the tagged suites, each with
   `host`: `lint.BuildTags` (`tools/ci/internal/lint/lint.go:45`), which
   the fix step reads, `tagSets` of `imports` (`imports.go:76`) and of
   `license` (`modules.go:169`), and `taggedsuites.Suites`
   (`taggedsuites.go:27`). No file of the module carries
   `//go:build host` (measured on 2026-10-21: the constraints in use
   are `e2e` in 6 files, `unix` in 70, `linux || darwin` in 2 and
   `linux` in 1), since the host suite of 10 10.1 is written at block
   B. `taggedsuites.Suites` becomes the one source the other three
   read, and this task removes `host` from it. `host` enters `Suites`
   in the pull request of T075, the task that writes the first file of
   `e2e/host/` (deferred today); T075 names
   `tools/ci/internal/taggedsuites/taggedsuites.go` in its `owns` when
   it is re-planned. Two tests hold the list both ways, so neither
   direction drifts: a tag of `Suites` that no file carries fails, and
   a tag of a `//go:build` line of the module that is not in `Suites`
   fails, the GOOS and GOARCH names, `unix`, `cgo` and the `go1.N`
   tags left out (the names `go/build` knows).
2. **`-count=1`.** The race and e2e steps pass it "because tests
   that scan the repository would otherwise pass from the test cache"
   (REQ-0833, and REQ-0837 by a pointer to the unit row, REQ-0824). The
   go command's test cache records the files and directories a test
   opens and the variables it reads in process, so that reason holds
   only for a test that reads the tree through a subprocess (git, the
   go command). Each step keeps the flag only when such a test runs in
   it, and its comment and its row name one. REQ-0837 gets its own
   reason, not a pointer to the row that T157 retires. `tools/mutants`
   (`runner.go:402`, `runner.go:414`) is left as it is: it runs `go
   test` on a clone whose source each mutant changes, so a cached result
   cannot stand in for a mutant, and the flag there costs nothing; T136
   holds the mutation runner's pending items.
3. **The e2e step.** Its argv is written by hand next to the race
   step's (`-json -race -count=1 -timeout=14m -tags e2e`,
   `tools/ci/internal/container/container.go:132`). One function builds
   the `go test` argv of both steps from their tags and their extra
   flags, so the race flags cannot drift apart. The infrastructure-error
   file named by `ROMEU_E2E_INFRA_FILE` (`e2e/internal/container/container.go:56`)
   exists so that no text a test prints can turn a failure into an
   infrastructure error (REQ-0798); it stays unless the builder finds a
   way with the same property, and the package doc of each side says
   why it is a file and not the output.
4. **`ModuleRoot`.** Two copies: `citest.ModuleRoot`
   (`tools/ci/internal/citest/citest.go:109`) and
   `container.ModuleRoot` (`e2e/internal/container/container.go:591`).
   One stays, in `internal/moduleroot`, a new package under the root
   `internal/`, which both `tools/ci` and the e2e suite may import (a
   package under `tools/ci/internal` or `e2e/internal` is visible to
   one of them only); `citest.ModuleRoot` keeps its `*testing.T`
   signature over it. No open task owns that path (measured on
   2026-10-21).
5. **The cover profile.** `tools/ci/internal/coverage` parses the
   profile by hand (`parseProfile`, `coverage.go:101`). `go tool cover
   -func` reports per function and a total, not per package, so it does
   not give the per-package percentage of REQ-0834 by itself; the
   per-package numbers the go command prints are the candidate. The
   builder replaces the parser with what the go command gives, or keeps
   it with the reason in its package doc, compared on the profile of a
   real run either way. No new dependency.

Tags select files, never remove them: `go test -tags e2e ./...` runs
every untagged test too. So the e2e step keeps its one directory,
`./e2e/...`, the home of the tagged suite (the maintainer's question
below), and the race run keeps `./...`.

- Implements: REQ-0832 (10 10.2), REQ-1254 (10 10.2), REQ-0833 (10
  10.2), REQ-0837 (10 10.2), REQ-0834 (10 10.2).
- Operator: no.
- Acceptance:
  - `Suites` names `e2e` and not `host`; the fix, lint, imports and
    license steps read their tags from it, and a test pins each argv
    (`fast_test.go`, `lint_test.go`, `fix_test.go` and the imports and
    license tests no longer hold `e2e,host`).
  - A test fails when `Suites` names a tag that no file of the module
    carries, and another when a `//go:build` line of the module names a
    tag outside `Suites` and outside the toolchain's names, each with a
    fixture tree; the rows of REQ-0832 and REQ-1254 say the tags come
    from `Suites`, and 10 10.1 and 11 keep the `host` suite as written.
  - The race and e2e steps pass `-count=1` only where the row or the
    comment names a test that reads the tree through a subprocess;
    REQ-0837 states its own reason; a test pins each argv.
  - The race and e2e argv come from one function, and a test pins both;
    the e2e step still reads `-json` and still fails a run with no test,
    a skipped test and a forged infrastructure line (the cases of
    `tools/ci/internal/container` as they are).
  - One `ModuleRoot`, in `internal/moduleroot`; `git grep -n 'func
    ModuleRoot'` finds one, and `go run ./tools/ci imports` passes.
  - `tools/ci coverage` gives the same findings as before on the profile
    of a real race run, compared in the pull request.
  - `docs/reference/checks.md` is regenerated where a usage line
    changes, and the task files that cite a changed id (T002, T013,
    T103, T152) are updated in the same pull request (12 12.11); `go
    run ./tools/ci reqs` passes.
- Verify: `go test -race ./tools/ci/... ./e2e/internal/...
  ./internal/moduleroot/...`, `go vet -tags e2e ./e2e/...`, `go run
  ./tools/ci reqs` and `go run ./tools/ci all`.

Question for the maintainer: the note says that filters are build tags
and that no directory list selects packages. Since a tag adds files and
removes none, `go test -race -tags e2e ./...` runs every unit test a
second time, about 4 minutes on a Linux runner on 2026-10-20. This task
keeps `./e2e/...` for the e2e step, the one directory of the tagged
suite; T157 drops the directory list of the unit step with the step
itself. Recommended: that reading; the other is one Linux run
`go test -race -tags e2e ./...` that is both the race and the e2e, which
changes what the coverage profile holds (REQ-0834).
