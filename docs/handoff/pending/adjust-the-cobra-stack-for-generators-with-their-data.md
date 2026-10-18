# Adjust the cobra stack for generators with their data

The decision "generators-live-with-their-data" (2026-10-17) differs from
what stack #118 (#115, #116, #117) says. Before the stack merges:

- ADR 0015 decision 7 (in `docs/adr/0015-*.md`, "The generator runs in
  tools/ci") changes to the new decision.
- T008's `owns` in `docs/plan/tasks/T008-*.md` changes: the lines
  `tools/ci/internal/generated/errors*.go` and `commands*.go` give way
  to the generator in `internal/cli`, the `*_gen.go` files and the
  subpackage for the reference pages.
- `docs/spec/12-engineering.md` REQ-1049 and REQ-1246 say `cobra/doc` is
  used "in tools/ci only"; that becomes "only in the generator
  subpackage and tools/ci".

Do the adjustment on #115 and #116, then run the gate again.

Open for the operator: a `*_gen.go` file and the ban on the generated-code
header (ADR 0007 rule 8; `docs/spec/10-testing-style.md` around line 605;
`tools/ci/internal/lint/config_test.go` around line 493). golangci-lint
skips a file whose header is `// Code generated ... DO NOT EDIT.`, so
either `*_gen.go` files carry no such header, or an ADR amends the rule.
Recommended: no header, which keeps every lint finding visible.

Reopen: the operator's answer, or the start of the rework of #109.
