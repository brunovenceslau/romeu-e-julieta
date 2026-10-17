---
id: T140
title: "The spec-to-tools/ci identifier and module-tree tests"
milestone: M1
module: ci-bootstrap
fate: merged
owns:
  - tools/ci/internal/moduletree/**
  - tools/ci/internal/identifiers/**
needs: []
after:
  - T105
ask_first:
  - checks
sitting: none
merged: 2026-10-17
pr: 93
---

# T140 - The spec-to-tools/ci identifier and module-tree tests

Merged on 2026-10-17 in pull request #93. The text below is the one
written at the re-plan.

Goal: Two repository tests: one reads the tree of 02 2.1, the module
table of 12 12.2 (the rows it names per module) and `coverTrees` of S9,
and fails on a package module directory one of them lacks; the other
reads every identifier a page under `docs/spec/` names as "`<name>` of
the <x> package of `tools/ci`" and fails one that package does not
declare (`go/ast` over `tools/ci`).

Re-plan (2026-10-12, stage B): New, split from T105 so that a new rule
is not reviewed inside a frozen-behaviour move.

- Implements: REQ-1082 (12 12.2), REQ-1228 (10 10.6).
- Operator: no
- Acceptance:
  - The module-tree test checks only the package modules that an id
    names; fixtures: a package under `internal/` with no module row,
    and a module row with no directory.
  - The identifier test holds `docs/spec/` and not `docs/adr/`, and
    freezes no path; a fixture page that names an identifier the
    package lacks fails.
  - `internal/` holds small single-purpose packages and no `utils`
    package: the module-tree test fails a directory under `internal/`
    whose name is `util`, `utils`, `helper`, `helpers` or `common`.
- Verify: `go test ./tools/ci/... -run '^TestModuleTree|^TestIdentifiers'`.
