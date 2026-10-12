---
id: T021
title: "`spec`: the project spec, host settings, the run layout and the examples"
milestone: M3
module: spec
fate: kept
owns:
  - internal/spec/**
  - examples/**
needs:
  - T017
  - T051
after: []
ask_first:
  - gates
  - contracts
sitting: none
---

# T021 - `spec`: the project spec, host settings, the run layout and the examples

Earlier title: `spec`: the project spec and host settings.

Goal: `spec`: the project spec, host settings and run layout types with
the `gate` and `apply` tags, `rules.go`, strict decode, the generated
validators and schemas, the untagged-field list (I7), the reference
pages and `examples/`.

Re-plan (2026-10-12): Kept; T022 (the run layout and the examples) is
folded in: one package, one agent. Its acceptance rows apply as written
there. `tools/schemagen` and `tools/ci schema` land in T051 with their
first input, the manifest (gap 15 under "Questions for the maintainer"
in `docs/plan.md`); this task adds its two types to the generator, and
the field-table generator of `docs/reference/project.md` is its own.
Its schemas and reference pages are generated outputs (rule 1 of
`docs/plan.md`), owned by no task.

- Implements: 03 3.1, 03 3.2, 03 3.4, 01 1.4 (the struct tags), 12 12.3,
  S10, 05 5.2 (I8, I11, I12).
- Operator: no.
- Acceptance:
  - Strict decode: an unknown key fails; `command`, `argv` or `env`
    under `secrets` fails (I8).
  - The validators are generated from `rules.go`; a table and a fuzz
    target with committed seeds cover the name and path rules of 03 3.1
    (I12), and a YAML round-trip fuzz covers each string field (I11).
  - `schemas/project.v1.json` and `schemas/host-settings.v1.json` are
    generated.
  - The generated reference holds the field table of
    `docs/reference/project.md`, from the `gate` and `apply` tags, and
    the rule table, from `rules.go` (12 12.3).
  - A reference page per file format is generated with its schema (S10):
    this task writes the generator and the pages of its two formats, and
    each later task that adds a schema adds its page through it.
  - Every field of the spec types is classified:
    `internal/spec/project.go` holds, beside the tags, the list of the
    fields that carry no `gate:"widening"` tag (today `sandboxOptions`
    and `run`, I7), and the generator of the widening set fails when the
    untagged fields of the types differ from that list. So a new field
    is never left out of the gate by omission, and the list is on the
    `gates` surface with the tags.
- Verify: `go test ./internal/spec/...`; `go run ./tools/ci schema`
  (the check of T051, over this task's two schemas).
