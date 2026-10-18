# No generated-code header and no package named generated

Decided on 2026-10-17, after "generators-live-with-their-data".

- A `*_gen.go` file carries no generated-code header. ADR 0007 rule 8
  and the lint step of 10 10.2 stand, so golangci-lint skips no generated file and every
  finding in one stays visible.
- No package is named `generated`: `tools/ci/internal/generated` becomes
  `tools/ci/internal/drift`, in a pull request of its own on the
  `checks` surface before T008's rework; the step keeps the name
  `generated` (REQ-0822).
- The generator of the per-command reference pages lives in
  `internal/cli/clidoc`, the one importer of `cobra/doc`, which only
  `tools/ci` imports (10 10.7, REQ-1247).
- ADR 0015 records these decisions (decisions 7 and 8) and is accepted.
