---
id: T154
title: "julieta status lists the recorded hook failures"
milestone: M1
module: julieta-core
fate: new
owns:
  - internal/cmd/julieta/status/section_failures*.go
  - internal/cmd/julieta/status/report.go
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
---

# T154 - julieta status lists the recorded hook failures

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
clear a failure when the same hook next succeeds. A file of the store
that cannot be read is one entry of the report's `errors`, and the
failures that can be read are still listed.

`julieta-status/v1` gains the member without a new version. REQ-0163
says any change to a format's fields bumps its version; no release has
shipped the format yet (the repository has no tag), and REQ-0424 lists
the hook failures as part of the v1 report from the start, so the
member completes v1 rather than changing it. The generated schema, the
valid fixture and the e2e decoder of every field gain it. Reopened by
the first release: from then on a new member of the report is a new
version.

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
  - A file of the store that cannot be read is one entry of `errors`,
    and the failure beside it is still listed.
  - In the container e2e, a commit whose tracked `pre-commit` fails
    shows in `julieta status --json` and in the text, with its hook,
    repo, stage and exit status, and a later commit that passes clears
    it.
  - `schemas/julieta-status.v1.json` is generated with the member, and
    the valid fixture carries one failure.
- Verify: `go test -race ./internal/cmd/julieta/status/...`, `go test
  -tags e2e ./e2e/... -run '^TestJulietaStatus' -v`, and `go run
  ./tools/ci all`.
