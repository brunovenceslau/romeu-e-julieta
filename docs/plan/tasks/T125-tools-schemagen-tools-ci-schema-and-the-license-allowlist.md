---
id: T125
title: "`tools/schemagen`, `tools/ci schema` and the license allowlist"
milestone: M1
module: spec
fate: new
owns:
  - tools/schemagen/**
  - tools/ci/internal/schema/**
  - tools/ci/check_schema.go
  - tools/ci/internal/license/**
  - tools/ci/check_license.go
  - go.mod
  - go.sum
needs:
  - T105
after:
  - T008
ask_first:
  - checks
  - dependencies
sitting: none
---

# T125 - `tools/schemagen`, `tools/ci schema` and the license allowlist

Goal: The reflect-based JSON Schema generator and the `schema` check of
10 10.2, in the `spec` module where 12 12.2 puts them, before T135 (the
first real schema), T021 (lane A's root) and T051; the JSON Schema
validator enters `go.mod` here as its first importer (rule 7 of
`docs/plan.md`), and with the first dependency the `license` step gains
the module-graph allowlist. Its first schema type is a fixture type of
its own tests; the first schema under `schemas/` is T135's.

Re-plan (2026-10-12, stage B): New. Lane A's root, `spec` (T021), needs
the generator and the check, and T051 no longer waits in front of it.

- Implements: REQ-0118 (02 2.1), REQ-0159 (03 (opening)).
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
