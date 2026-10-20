# Session of 2026-10-18, third part: the generated header

- The T008 stack (#128, #129, #109, #130) merged; T008 is ticked
  merged in pull request #109.
- #132: ADR 0016. The linters read generated files
  (`linters.exclusions.generated: disable`, `generated-is-used: false`
  for `unused`); revive, 48 staticcheck checks and go vet's stdversion
  still skip one, which is accepted for the outputs of `go generate`
  alone. Every generated file opens with the full generated header,
  after mockgen's. ADR 0007 rule 8, ADR 0015 decision 7, REQ-0879 and
  REQ-1117 follow. Decided on 2026-10-18.
- #133: T008's task file says its `*_gen.go` file carries the header.
- #134: the lint configuration, the generated fixtures, the revive guard
  `TestGeneratedGoFilesKeepReviveRules`, one header writer in
  `tools/ci/internal/check` and its copy in `internal/cli`, held to one
  text; every output regenerated.
- Gate: code-reviewer and security-auditor; applied as #134 lists. Left
  for later: pending note
  "drop-the-revive-guard-when-golangci-lint-lets-revive-read-generated-files".
