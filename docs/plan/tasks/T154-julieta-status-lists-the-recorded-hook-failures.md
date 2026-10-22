---
id: T154
title: "julieta status lists the recorded hook failures"
milestone: M1
module: julieta-core
fate: merged
owns:
  - internal/cmd/julieta/status/section_failures*.go
  - internal/cmd/julieta/status/report.go
  - internal/cmd/julieta/status/status.go
  - internal/cmd/julieta/status/sections.go
  - internal/hooks/failures.go
  - internal/cmd/julieta/handoff/state.go
  - internal/cmd/julieta/hooks/run.go
  - docs/reference/julieta.md
  - docs/reference/julieta/status.md
  - internal/cmd/julieta/status/testdata/valid.json
  - schemas/julieta-status.v1.json
  - e2e/julieta_status_test.go
needs:
  - T051
  - T054
  - T059
  - T153
after: []
ask_first: []
sitting: none
merged: 2026-10-21
pr: 227
---

# T154 - julieta status lists the recorded hook failures

Merged on 2026-10-21 in pull request #227. The text below is the one
written when it was built.

Goal: `julieta status` lists the hook failures recorded in julieta
state, as REQ-0424 and REQ-0675 say: SessionStart, `julieta status` and
`romeu status` surface each one until a later run of the same hook
succeeds. T054's store, `hook-failures/` below julieta state
(`internal/hooks`, `hooks.Failures`), holds the git hooks' failures,
and since T059 the session hooks'; since T153 a git hook's stage can
be `system`. Today only the SessionStart print reads the store (pending
note "julieta status reads the hook failure store",
0000000000782g86tsyzte1t16, which this task closes).

One section file, `section_failures.go`, registered in the set of
`sections.go` (plan rule 14), reads the store and fills a new member of
the report, `hookFailures`: each failure with its time, exit status,
hook, message, path, repo and stage, the oldest first. The text form
prints one stdout line for each failure, since a recorded failure is
part of the report and not a warning of 04 4.1 (it has no slug). The
section writes nothing: the dispatcher and the session hooks already
clear a failure when the same hook next succeeds. Each file of the
store that cannot be trusted is one entry of the report's `errors`, and
the failures that can be read are still listed. The path of the store
is one helper, `hooks.FailuresDir`, which the dispatcher (`julieta hooks
run`), the session hooks (`internal/cmd/julieta/handoff`) and the
section call, in place of three copies of the directory name; the
package doc and the help of `julieta status` name the failures.

`julieta-status/v1` gains the member without a new version: decided on
2026-10-21, formats evolve by the rules of protobuf, where adding a
member is backward compatible ("Formats evolve by the rules of
protobuf", 01m5egnpxhheh7197j08zdr618). REQ-0163 said that any change
to a format's fields bumps its version; T162 ("Readers of formats
between binaries ignore unknown members") amended it. The generated
schema, the valid fixture and the e2e decoder of every field gain the
member.

- Implements: REQ-0424 (04 4.3), REQ-0675 (08 8.2).
- Operator: no.
- Acceptance:
  - With a tracked-stage and a system-stage failure of git hooks and a
    SessionStart failure recorded through `internal/hooks` in the real
    store below `$HOME`, `julieta status --json` lists the three, the
    oldest first, each with its hook, repo, stage, exit status, path or
    message, and the text form prints one line for each on stdout,
    with a line feed in a message escaped; after the store clears two
    of them, as the dispatcher does when the same hook passes, status
    lists the third only.
  - With no store, `hookFailures` is an empty list and the text prints
    no failure line.
  - Two files of the store it cannot trust, one with an unknown member
    and one whose repo and hook name another file, are two entries of
    `errors`, one line each, and the failure beside them is still
    listed.
  - A failure with neither a path nor a message ends its text line at
    its time, with no separator after it.
  - In the container e2e, a commit whose tracked `pre-commit` fails
    shows in `julieta status --json` and in the text, with its hook,
    repo, stage and exit status, and a later commit that passes clears
    it.
  - `schemas/julieta-status.v1.json` is generated with the member, and
    the valid fixture carries one failure.
- Verify: `go test -race ./internal/cmd/julieta/status/...`, `go test
  -tags e2e ./e2e/... -run '^TestJulietaStatus' -v`, and `go run
  ./tools/ci all`.
