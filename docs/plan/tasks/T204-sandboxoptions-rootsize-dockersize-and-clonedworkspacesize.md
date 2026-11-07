---
id: T204
title: "sandboxOptions rootSize, dockerSize and clonedWorkspaceSize"
milestone: M3
module: spec
fate: new
owns:
  - internal/spec/sandboxoptions*.go
  - internal/spec/project.go
  - examples/**
  - internal/render/env*.go
  - internal/render/testdata/**
needs:
  - T021
  - T037
after:
  - T200
  - T055
ask_first:
  - gates
  - formats
sitting: none
---

# T204 - sandboxOptions rootSize, dockerSize and clonedWorkspaceSize

Goal: the project spec takes the three size options of sbx 0.51 beside
`cpus` and `memory`, ungated and recreate-class, and the render passes
them through.

Added on 2026-11-06 (scope this repository), with the specification
amended first: the sbx 0.51 and kit schema m.12 amendments of 03 3.1
and 03 3.2.

1. The fields (REQ-1356): `sandboxOptions.rootSize`, `dockerSize` and
   `clonedWorkspaceSize`, each a byte size of the `memory` form
   (`512m`, `20g`), with no bound of romeu's: what sbx refuses is sbx's
   error at run time, and A15 records what a change does under a
   stopped sandbox (T019).
2. The tags (REQ-0192): on the committed list of ungated fields with
   the rest of `sandboxOptions`, `apply:"recreate"`, stripped from
   `renderDigest`, present in the recreate digest; the generated field
   table of `docs/reference/project.md` and `schemas/project.v1.json`
   follow from the types.
3. The render: the keys reach the env file under `sandboxOptions`, and
   the example of 03 3.2 and `examples/` carry `rootSize`.

- Implements: REQ-1356 (03 3.1), REQ-0192 (03 3.2), REQ-0067 (01 1.4).
- Operator: yes, the approval lines for `internal/spec/project.go`
  (ask-first `gates`) and `schemas/**` (ask-first `formats`).
- Acceptance:
  - The validator accepts `20g` and `512m` for each of the three fields
    and refuses `20G`, `0`, `-1g`, `1t` and a bare number, naming the
    field; an omitted field renders nothing.
  - The I7 flip-table test shows each of the three fields outside the
    widening set and inside the recreate digest, and `go generate`
    passes with them on the ungated list.
  - The env file golden of a spec that sets all three carries them
    under `sandboxOptions`; the schema and the reference page
    regenerate with the three rows.
- Verify: `go test ./internal/spec/... ./internal/render/... -run
  '^(TestSandboxOptionsSizes|TestEnvSandboxOptionsSizes)' -v`. The
  pattern names the functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
