---
id: T125
title: "`tools/schemagen`, `tools/ci schema` and the license allowlist"
milestone: M1
module: spec
fate: merged
owns:
  - tools/schemagen/**
  - tools/ci/internal/schema/**
  - tools/ci/check_schema.go
  - tools/ci/internal/license/**
  - tools/ci/check_license.go
  - go.mod
  - go.sum
  - tools/ci/generator_schemas.go
  - tools/ci/checks_test.go
  - tools/ci/all_test.go
  - VERSION
  - REUSE.toml
needs:
  - T105
after:
  - T008
  - T010
  - T132
ask_first:
  - checks
  - dependencies
sitting: none
merged: 2026-10-18
pr: 160
---

# T125 - `tools/schemagen`, `tools/ci schema` and the license allowlist

Merged on 2026-10-18 in pull request #160. The text below is the one
written at the re-plan.

Goal: The reflect-based JSON Schema generator and the `schema` check of
10 10.2, in the `spec` module where 12 12.2 puts them, before T135 (the
first real schema), T021 (lane A's root) and T051; the JSON Schema
validator enters `go.mod` here as its first importer (rule 7 of
`docs/plan.md`), and with the first dependency the `license` step gains
the module-graph allowlist. Its first schema type is a fixture type of
its own tests; the first schema under `schemas/` is T135's.

Re-plan (2026-10-18): Changed. The three design questions are
answered (decided on 2026-10-18). The root file `VERSION` names the
next release (REQ-1249), and the `$schema` hint of a YAML file under
`examples/` names that release; `tools/release` reads the same file,
and `REUSE.toml` gives it its license. The fixture type's schema is
written by a test over a fixture tree, not committed under `schemas/`,
which holds product schemas only; `tools/ci/generator_schemas.go`
registers the generator that writes `schemas/*.json` for the types
registered with `tools/schemagen`. `tools/release` (T103) is the
deferred reader of `VERSION`: T125 writes the file and the `schema`
check reads it, and T103 reads it when it lands. T125 owns the files that pin the
registered checks and the steps of `all`, which gain the `schema`
check and the second step of `license`: `tools/ci/checks_test.go` and
`tools/ci/all_test.go`. It comes after T010 and T132, which own them
too.

Re-plan (2026-10-12, stage B): New. Lane A's root, `spec` (T021), needs
the generator and the check, and T051 no longer waits in front of it.

- Implements: REQ-0118 (02 2.1), REQ-1249 (02 2.1), REQ-0159 (03
  (opening)), REQ-1250 (10 10.2), REQ-1049 (12 12.1), REQ-0836 (10
  10.2), REQ-0848 (10 10.2).
- Operator: no
- Acceptance:
  - `go generate` writes a schema for the generator's fixture type, and
    `tools/ci schema` fails a committed schema that differs from the
    generated one and an `examples/` or testdata file that does not
    validate (one fixture each).
  - `tools/ci schema` fails a YAML file under `examples/` without the
    `$schema` hint of the release that ships it, with a fixture.
  - The `license` step reads the licenses of the module graph and fails
    a module whose license is not on the committed allowlist compatible
    with GPL-3.0-only; fixtures for an allowed, a disallowed and an
    unknown license.
- Verify: `go test ./tools/ci/... ./tools/schemagen/...`; `go run
  ./tools/ci all` exits 0.
