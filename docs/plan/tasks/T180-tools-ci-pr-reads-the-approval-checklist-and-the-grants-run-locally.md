---
id: T180
title: "tools/ci pr reads the approval checklist and the grants, run locally"
milestone: M2
module: ci-bootstrap
fate: new
owns:
  - tools/ci/check_pr.go
  - tools/ci/checks_test.go
  - tools/ci/internal/askfirst/**
  - docs/spec/12-engineering.md
  - .github/pull_request_template.md
needs:
  - T003
after: []
ask_first:
  - approvals
  - checks
sitting: none
---

# T180 - tools/ci pr reads the approval checklist and the grants, run locally

Goal: `go run ./tools/ci pr` judges the approval lines of a pull
request body against the ask-first surfaces its diff touches and the
grants of `docs/grants.yaml`, run by hand on a saved payload or on a
title and a body file before the pull request exists. It is the first
half of the first slice of T005 (that file's re-plan of 2026-10-12);
T181 runs it in the base-defined pr job, so the check gates every pull
request.

Decided on 2026-10-29, scope this repository: the approval lines are
checked by a machine now, not at M6, because the `formats` surface
(#314) landed with no check behind it. The approval line becomes a
checklist item: the maintainer's tick, not the author's typed line, is
the approval.

1. 12 12.4 changes from the plain line to the checklist form, in the
   same diff as `approvalLine`: one `> [!IMPORTANT]` alert in the body,
   and inside it one `> - [ ] Approval: <surface id> - <what is asked>`
   line per surface. A ticked line (`[x]`) is the approval, and only
   the maintainer ticks it; an unticked line is a request. REQ-1144 to
   REQ-1147, REQ-1157 to REQ-1159 and REQ-1162 (whose "the line is
   typed by the PR's author" no longer holds) are rewritten for that
   form, the approval hint of `.github/pull_request_template.md` shows
   the checklist, and REQ-1161 says whether a line with a grant phrase needs the tick
   (decided by this task's pull request and put to the operator before
   it is ready). With one account acting for everyone (ADR 0008), the
   tick is a record and a guard against mistakes, as the line was.
2. The reader lives in the askfirst package, which `approvals` already
   names, and `tools/ci/check_pr.go` is a thin `pr` subcommand over it.
   The askfirst package reads `.github/ask-first.yaml` with the one
   reader the generator of 12 12.3 uses (invariant (e) of T005).
3. Out of scope, left in T005: the sections, Middleware, Lessons,
   Prior art, goldens, Conventional Commits, prose, `Fixes-release:`,
   guard tags, the forbidden names over the title and body, the
   template's other hints and 05. Out of scope, in T181: the workflow, the head read
   as git objects in the job, and invariants (a), (b) and (c).

- Implements: REQ-0537 (05 5.3), REQ-0538 (05 5.3), REQ-0817 (10 10.2),
  REQ-0818 (10 10.2), REQ-0819 (10 10.2), REQ-1130 (12 12.4), REQ-1134
  (12 12.4), REQ-1144 (12 12.4), REQ-1145 (12 12.4), REQ-1146 (12 12.4),
  REQ-1147 (12 12.4), REQ-1156 (12 12.4), REQ-1157 (12 12.4), REQ-1158
  (12 12.4), REQ-1159 (12 12.4), REQ-1160 (12 12.4), REQ-1161 (12 12.4),
  REQ-1162 (12 12.4).
- Operator: decide, before the pull request is ready, whether a grant
  line needs the tick, and approve the 12 12.4 wording of the checklist.
- Acceptance:
  - `TestPRApprovalLine` holds one case per row of the table of T005's
    acceptance, titled with the row's case title verbatim, each body
    line rewritten as a ticked checklist line inside the alert, each
    outcome the one 12 12.4 states. It adds these rows, each outcome
    written in 12 12.4: an unticked line (fails: not approved), an
    uppercase `[X]` (the outcome GitHub's rendering gives), a ticked
    line outside the alert (fails: no line for `decisions`), and a body
    with two alerts.
  - A touched ask-first surface without its approval line fails, with
    the list read at the base and at the head and joined; a line for an
    id neither list has fails; two lines for one id fail; a line that
    starts with `Approval: ` and is not an approval line fails as
    malformed.
  - A grant phrase `granted at <sitting> for <task id>` passes on a
    grantable surface only when `docs/grants.yaml` at the base commit
    holds that grant for that surface and task; fixtures: present
    passes, absent fails, present only at the head fails, the phrase on
    `decisions` fails. A grants entry for a surface outside the
    grantable list, or for a task id the plan does not define, fails.
  - The changed paths are those between the merge base of `base.sha`
    and `head.sha`, and `head.sha`, both names of a rename included: a
    fixture that merges the default branch into the head touches no
    surface; `**` matches zero segments in a fixture.
  - A walk from `base.sha` to `head.sha` that finds no commit fails.
  - A payload with neither accepted shape, or without one of the seven
    fields of REQ-0817, fails.
  - A local run with `--title` and `--body` reads the local base..HEAD
    range and makes no network read; a local run on a saved payload
    file, with `head.sha` present, makes none either.
  - Invariant (e): `pr` and the generator of 12 12.3 read
    `.github/ask-first.yaml` and `docs/grants.yaml` with one reader; a
    test asserts both call it, and a second reader inlined in either
    turns it red.
- Verify: `go test ./tools/ci/... -run '^TestPR'`, and `go run ./tools/ci
  pr --title <title> --body <file>` on this task's own pull request
  body, whose output is pasted under Evidence.
