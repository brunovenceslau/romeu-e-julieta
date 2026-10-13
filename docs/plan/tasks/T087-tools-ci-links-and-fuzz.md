---
id: T087
title: "`tools/ci links` and `fuzz`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T085
  - T133
after: []
ask_first:
  - checks
  - dependencies
  - approvals
sitting: none
---

# T087 - `tools/ci links` and `fuzz`

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. It needs T133 for `tools/ci mutate`,
which `fuzz.yml` calls, and forecasts `approvals`, since it adds
`fuzz.yml` under `.github/workflows/**`. The text below is the one
written before the re-plan; `owns` is written when the task reopens.

- Implements: REQ-0011 (index), REQ-0795 (10 10.1), REQ-0873 (10 10.2),
  REQ-0874 (10 10.2), REQ-1111 (12 12.3), REQ-1121 (12 12.3).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`, `dependencies`
  (`fuzz.yml`).
- Acceptance:
  - `links` fetches each external URL the docs cite, fails a fixture
    page with one dead URL, and is called by no workflow.
  - `fuzz` runs each fuzz target for a fixed time, and the scheduled
    `fuzz.yml`, which runs from a `schedule` cron once a day on `main`,
    calls it and then `tools/ci mutate`; the file passes `tools/ci
    workflows`. A test lists every `Fuzz*` function of the module and
    fails when `fuzz` would miss one, so a run with zero targets cannot
    pass.
- Verify: `go test ./tools/ci/... -run '^Test(Links|Fuzz)'`; `go run
  ./tools/ci links`; `go run ./tools/ci fuzz`.
