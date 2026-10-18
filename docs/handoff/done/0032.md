# Session of 2026-10-18, first part: generators with their data, and drift

- #119, #120: ADR 0015 accepted, with decisions 7 and 8 changed: the
  generator of the per-command reference pages lives in
  `internal/cli/clidoc`, the one importer of `cobra/doc`, which only
  `tools/ci` imports (REQ-1247, a row of 10 10.7), and the generator of
  the error table's pages in `internal/cli`. T008 owns them and
  `tools/ci/generator_cli.go`. A `*_gen.go` file carries no
  generated-code header. Approved on 2026-10-17: the `decisions` surface
  for ADR 0015.
- REQ-0121 and plan rule 4 amended: the package of the `generated`
  check is `tools/ci/internal/drift`, since no package is named
  `generated`, and a generator lives in the package that owns its data
  and is registered by its own `tools/ci/generator_<name>.go`.
- `tools/ci/internal/generated` became `tools/ci/internal/drift`;
  `check.Generator` and `check.RegisterGenerator` replace
  `Check.Outputs`. The generators moved next to their data: CODEOWNERS
  to `askfirst`, the index of the decision records to `adrdir`,
  `checks.md` to `check`, with `GeneratedNotice` in `check`. The
  `checks` surface names `tools/ci/internal/drift/**` and
  `tools/ci/adrdir/**`.
