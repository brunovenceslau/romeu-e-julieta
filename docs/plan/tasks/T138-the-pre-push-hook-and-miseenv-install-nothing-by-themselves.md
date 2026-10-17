---
id: T138
title: "The pre-push hook and `miseEnv` install nothing by themselves"
milestone: M0
module: ci-release
fate: merged
owns:
  - .githooks/pre-push
needs: []
after:
  - T105
ask_first:
  - checks
sitting: none
merged: 2026-10-17
pr: 69
---

# T138 - The pre-push hook and `miseEnv` install nothing by themselves

Merged on 2026-10-17 in pull request #69. The text below is the one
written at the re-plan.

Goal: `MISE_AUTO_INSTALL=false` and `MISE_NOT_FOUND_AUTO_INSTALL=false`
join the hook's fixed settings and `miseEnv` of the mise package of
`tools/ci` (its file after T105's split, admitted by `after: T105`), so
a missing tool is the probe's hint and never a build under the caller's
module settings; the test that the hook paragraph of 12 12.1 names holds
the hook's `env -i` line to `hookAllow` and the fixed settings, hints
included (65d90c10, item 7); the hook's comment says the probe runs in a
subshell. T001's merged acceptance text stays the record it is.

Re-plan (2026-10-12, stage B): New, an M0 task on the `checks` surface,
since the hook runs on every push and the window should not wait for
daily use.

- Implements: REQ-0858 (10 10.2), REQ-0891 (10 10.2), REQ-1042 (12
  12.1), REQ-1047 (12 12.1), REQ-1048 (12 12.1).
- Operator: no
- Acceptance:
  - Measured before the change, under Evidence: with `GONOSUMDB=*`
    exported in the caller and `govulncheck` absent from the mise
    installs, one push through the hook: whether the probe or the checks
    installed govulncheck under the caller's settings, measured end to
    end.
  - After the change the same push stops at the probe's hint and
    installs nothing; a test runs the hook against that environment and
    asserts no install.
  - The hook test holds the `env -i` line to `hookAllow` and the fixed
    settings, the two new variables included, and the three hints; the
    hook's comment names the subshell.
- Verify: `go test ./tools/ci/... -run '^TestHook'`; `go run ./tools/ci
  fast` exits 0.
