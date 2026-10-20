---
id: T021
title: "`spec`: the project spec, host settings, the run layout and the examples"
milestone: M3
module: spec
fate: kept
owns:
  - internal/spec/**
  - examples/**
  - docs/spec/03-formats.md
needs:
  - T017
  - T125
after:
  - T122
  - T051

ask_first:
  - gates
  - contracts
  - spec
sitting: none
---

# T021 - `spec`: the project spec, host settings, the run layout and the examples

Earlier title: `spec`: the project spec and host settings.

Goal: `spec`: the project spec, host settings and run layout types with
the `gate` and `apply` tags, `rules.go`, strict decode, the generated
validators and schemas, the untagged-field list (I7), the reference
pages and `examples/`.

Re-plan (2026-10-12): Kept; T022 (the run layout and the examples) is
folded in: one package, one agent, and its rows are copied below.
`tools/schemagen` and `tools/ci schema` land in T125 (gap 15 under
"Questions for the maintainer" in `docs/plan.md`), so this task needs
T125 and adds its two types to the generator; the field-table generator
of `docs/reference/project.md` is its own. Its schemas and reference
pages are generated outputs (rule 1 of `docs/plan.md`), owned by no
task. It owns `docs/spec/03-formats.md` for the one edit the
specification assigns it: the format tables of 03 3.2 and 03 3.4 that
its generated pages replace by a link (rule 6), whose requirement ids
are retired with `Replaced by: <the generated page>`.

Re-plan (2026-10-20): Changed. T051 gave up the edit of the manifest
table of 03 3.6, because no generator writes a `docs/reference/` page
for the manifest and `schemas/manifest.v1.json` does not carry the
meaning of each field. This task's field-table generator also writes
the manifest's page, from `internal/manifest` and its struct tags, and
replaces the table of 03 3.6 by a link to it, which is REQ-0173.

- Implements: REQ-0019 (index), REQ-0038 (00 0.4), REQ-0054 (01 1.3),
  REQ-0066 (01 1.4), REQ-0067 (01 1.4), REQ-0118 (02 2.1), REQ-0119 (02
  2.1), REQ-0133 (02 2.2), REQ-0134 (02 2.2), REQ-0159 (03 (opening)),
  REQ-0160 (03 (opening)), REQ-0172 (03 (opening)), REQ-0173 (03 (opening)), REQ-0174 (03 3.1),
  REQ-0175 (03 3.1), REQ-0176 (03 3.1), REQ-0177 (03 3.1), REQ-0178 (03
  3.1), REQ-0179 (03 3.1), REQ-0180 (03 3.1), REQ-0181 (03 3.1),
  REQ-0182 (03 3.1), REQ-0183 (03 3.1), REQ-0184 (03 3.1), REQ-0186 (03
  3.1), REQ-0187 (03 3.1), REQ-0188 (03 3.1), REQ-0189 (03 3.1),
  REQ-0190 (03 3.1), REQ-0191 (03 3.2), REQ-0192 (03 3.2), REQ-0193 (03
  3.2), REQ-0194 (03 3.2), REQ-0195 (03 3.2), REQ-0197 (03 3.2),
  REQ-0205 (03 3.4), REQ-0489 (05 5.2), REQ-0490 (05 5.2), REQ-0491 (05
  5.2), REQ-0494 (05 5.2), REQ-0495 (05 5.2), REQ-0509 (05 5.2),
  REQ-0576 (06 6.2), REQ-0752 (10 10.1), REQ-0793 (10 10.1), REQ-0836
  (10 10.2), REQ-0960 (10 10.6), REQ-1044 (12 12.1), REQ-1106 (12 12.3),
  REQ-1107 (12 12.3), REQ-1117 (12 12.3), REQ-1222 (05 5.2).
- Operator: no.
- Acceptance:
  - Strict decode: the decoder is `encoding/json/v2` with
    `RejectUnknownMembers(true)` for every owned format and
    `go.yaml.in/yaml/v3` with `KnownFields(true)` for the YAML ones; a
    fixture with a duplicate key and one with an unknown key are each
    refused; `command`, `argv` or `env` under `secrets` fails (I8).
  - The validators are generated from `rules.go`; a table and a fuzz
    target with committed seeds cover the name and path rules of 03 3.1
    (I12), and a YAML round-trip fuzz covers each string field (I11).
    The two rules of 03 3.1 added by the stage B pass sit in `rules.go`:
    `projects/` holds `*.yaml` only, and one spec names the config
    project, the second marked romeu-only.
  - `internal/spec/versions.go` holds one reader of `schema:
    <name>/v<n>`: a newer version is `format-newer` (`RJ-311`) with the
    hint `install the release that wrote it`; for `project.v1` and
    `host-settings.v1` a version older than N-1 is `format-unsupported`
    (`RJ-337`); a table test covers each format the package decodes.
  - Host settings refuse a relative or missing `argv[0]` and a shell
    string, and an argv element starting with `ghp_`, `gho_`, `ghs_`,
    `ghu_`, `ghr_` or `github_pat_` or matching `^[0-9a-f]{40}$` with
    `secret-in-argv` (`RJ-314`), at load, one fixture per shape (I25).
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
  - `examples/kits/` holds an example personal kit (CC0-1.0), and
    `examples/.github/workflows/validate.yml` pins a julieta release by
    version and archive sha256, verifies it against `checksums.txt`
    before it runs, and passes `julieta pin check --workflows` (T055);
    each YAML file under `examples/` carries the `$schema` hint of the
    release that ships it.
- Acceptance (from T022, folded):
  - The validator refuses a pane id used twice, two focused panes in a
    tab, a ratio outside 0.1 to 0.9, an `env` key on the refused list
    and a `cwd` with a `..` element.
  - `examples/` holds an example config repo whose files validate
    against the generated schemas in `tools/ci schema`.
- Verify: `go test ./internal/spec/...`; `go run ./tools/ci schema` (the
  check of T125, over this task's two schemas).
