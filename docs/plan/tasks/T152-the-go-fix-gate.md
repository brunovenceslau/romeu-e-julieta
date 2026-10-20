---
id: T152
title: "The go fix gate"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - tools/ci/check_fix.go
  - tools/ci/internal/fix/**
  - tools/ci/fast_test.go
  - tools/ci/all_test.go
  - tools/ci/checks_test.go
  - docs/spec/10-testing-style.md
  - internal/canon/kind.go
  - internal/cli/errors_table_test.go
  - internal/cli/generate_errors_test.go
  - internal/manifest/manifest_test.go
  - internal/memstore/check.go
  - tools/ci/adrdir/adrdir_test.go
  - tools/ci/internal/askfirst/askfirst.go
  - tools/ci/internal/askfirst/askfirst_test.go
  - tools/ci/internal/check/printer_test.go
  - tools/ci/internal/drift/drift.go
  - tools/ci/internal/drift/drift_test.go
  - tools/ci/internal/drift/generate_test.go
  - tools/ci/internal/imports/imports.go
  - tools/ci/internal/lint/config_directives_test.go
  - tools/ci/internal/lint/config_targets_test.go
  - tools/ci/internal/plan/plan.go
  - tools/ci/main_test.go
  - tools/kitpin/internal/pin/fetch.go
  - tools/kitpin/internal/pin/frontend.go
  - tools/mutants/main_test.go
needs: []
after:
  - T148
  - T149
  - T150
ask_first:
  - checks
  - digests
  - approvals
  - kits
  - ask-first
sitting: none
---

# T152 - The go fix gate

Goal: a step of `fast` and `all`, decided on 2026-10-20, that runs
`go fix -diff` over the module and fails when the diff is not empty,
and the commit that brings the tree to pass it. In the pinned toolchain
(go 1.27.2) `go fix` runs the modernizers of `go tool fix`, so the step
holds the code to the idioms of the Go version `go.mod` names
(`slices.Contains`, `strings.Cut`, `errors.AsType` and the others the
toolchain lists). The step reports and never rewrites: its failure shows
the diff and names the command that applies it, and the fix is to run
that command and commit what it writes.

The step reads the module as the lint step does: once per lint target
(`lint.Targets`, linux and darwin on amd64 and arm64, cgo off) and with
the tags of the tagged suites (`e2e,host`). The test of the lint
package over the targets holds that every Go file builds for one of
them with those tags, so a change go fix would make in a darwin file,
or in a file of the e2e suite, fails on every machine, and a run on
Linux and a run on macOS give the same verdict. On origin/main
(e3cbf8f) the diff is the same 20 files for each of the four targets,
with or without the tags. A warm run takes about 1s per target; a cold
one about 15s to 20s.

The tool is the toolchain's own `go fix`
([ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md)):
no fixer is written here, and the step adds no dependency.

The files go fix changes on origin/main belong to other tasks, so
`owns` lists each of them, and `ask_first` the surfaces they are on.
The commit that applies go fix lands after the pull requests open on
2026-10-20 that change three of them.

- Implements: REQ-1254 (10 10.2).
- Operator: no.
- Acceptance:
  - `tools/ci fast` and `tools/ci all` run one fix step per lint
    target, after vet: `go fix -diff -tags e2e,host ./...` with the
    pinned go, in the environment of the lint step for that target.
  - A step whose run prints a diff fails, whatever its exit status, with
    the diff sorted by file (go fix prints the files in an order that
    changes from run to run), clipped to 200 lines and then the list
    of the files it changes, and the line that names
    `GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go fix -tags e2e,host ./...`;
    a run that fails without a diff keeps what go printed.
  - Tests over fixture modules with the pinned go: a clean module
    passes on every target; a fixable file fails on every target; a
    fixable darwin file fails on the darwin targets alone and a linux
    one on the linux targets alone; a fixable file of the e2e suite
    fails, and passes a run without the tags (the positive control);
    a module that does not build fails without the line.
  - The tree passes the step: one commit, `style: apply go fix`, holds
    what `go fix -tags e2e,host ./...` writes, and no hand edit.
- Verify: `go test ./tools/ci/... -run 'TestSteps|TestRead|TestFastSteps|TestAllSteps'`;
  `go run ./tools/ci all` exits 0.
