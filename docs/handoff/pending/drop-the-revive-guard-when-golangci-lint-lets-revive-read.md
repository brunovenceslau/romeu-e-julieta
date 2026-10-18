# Drop the revive guard when golangci-lint lets revive read generated files

golangci-lint v2.14.0 always passes `ignore-generated-header: false` to
revive (`pkg/golinters/revive/revive.go`), so revive skips a file with
the generated header whatever `linters.exclusions.generated` says (ADR
0016). `TestGeneratedGoFilesKeepReviveRules` in
`tools/ci/internal/lint/config_test.go` holds revive's two rules of ADR
0001 rule 14 in those files instead, and the fixture test holds the
skip.

When a golangci-lint bump lets revive read generated files, the fixture
test fails on the skipped revive fixtures under `generated/`. Then drop
the `skipped` field, the guard, `reviveGenerated` and
`reviveRuleProblems`, and amend ADR 0016's consequences in a new record
or a dated change.

Reopen: that failure, or a golangci-lint bump (re-read revive's
`isGenerated` in `lint/linter.go` either way).
