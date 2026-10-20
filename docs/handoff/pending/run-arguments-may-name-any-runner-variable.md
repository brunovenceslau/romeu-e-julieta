# Run arguments may name any runner variable

What is at stake: small; the value is quoted, so there is no shell
injection, and no `tools/ci` subcommand reads arguments today. A branch
name chosen by a pull request's author can still reach a program's
argv.

The workflows grammar admits any `"$NAME"` word in a run line (`runVar`
in `tools/ci/internal/workflows/workflows.go`); `go run ./tools/ci all
"$GITHUB_HEAD_REF"` in `ci.yml` is admitted by the grammar (security audit of T144,
2026-10-20). Only the release publish job limits the name, to
`"$GITHUB_REF_NAME"` (#188). An allowlist of variable names would close
it.

Reopen: when a `tools/ci` subcommand starts to read an argument a
workflow passes, or the next change to the workflows grammar.
