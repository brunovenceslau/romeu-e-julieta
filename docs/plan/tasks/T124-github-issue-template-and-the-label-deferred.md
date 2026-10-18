---
id: T124
title: "`.github/ISSUE_TEMPLATE/` and the label `deferred`"
milestone: M1
module: docs
fate: new
owns:
  - .github/ISSUE_TEMPLATE/**
  - tools/ci/issueforms_test.go
needs: []
after: []
ask_first:
  - checks
sitting: none
---

# T124 - `.github/ISSUE_TEMPLATE/` and the label `deferred`

Goal: The two issue forms of 12 12.7 (a bug, a catalog gap) and the
label `deferred` the index's Deferred decisions use to report an event
no check detects; M1, since events can fire from M0 on. The path is on
no surface of `.github/ask-first.yaml`.

Re-plan (2026-10-18): Changed. `owns` adds `tools/ci/issueforms_test.go`,
the fixture test its Verify line runs (`go test ./tools/ci/... -run
'^TestIssueForms'`), which lives in a package of `tools/ci` and so on the
`checks` surface, now in `ask_first`.

Re-plan (2026-10-12, stage B): New. The forms had no task file before
this pass.

- Implements: REQ-0127 (02 2.1), REQ-1180 (12 12.7).
- Operator: the label is created through the API, the call under
  Evidence.
- Acceptance:
  - Each form is parsed by a fixture test (the fields 12 12.7 lists).
  - Both forms render in GitHub's issue chooser, read at the next
    sitting **[review]**.
  - The label exists.
- Verify: `go test ./tools/ci/... -run '^TestIssueForms'`.
