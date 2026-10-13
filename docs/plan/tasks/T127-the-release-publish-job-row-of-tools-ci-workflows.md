---
id: T127
title: "The release publish job row of `tools/ci workflows`"
milestone: M1
module: ci-release
fate: new
owns:
  - tools/ci/internal/workflows/release*.go
needs:
  - T105
after: []
ask_first:
  - checks
sitting: none
---

# T127 - The release publish job row of `tools/ci workflows`

Goal: The grammar row of the release workflow (10 10.2, the publish job:
`ubuntu-26.04`, needs every test job, no test, `tools/ci setup` first,
`GOPROXY=off`, attestation permissions), left by round 8 (R8-10-1) with
no task file. T088 needs it.

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge.

- Implements: REQ-0808 (10 10.2).
- Operator: no
- Acceptance:
  - A fixture per admitted and per refused step of the publish job.
  - The grammar still refuses the forms it refuses today.
- Verify: `go test ./tools/ci/... -run '^TestWorkflowsRelease'`.
