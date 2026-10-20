---
id: T127
title: "The release publish job row of `tools/ci workflows`"
milestone: M1
module: ci-release
fate: merged
owns:
  - tools/ci/internal/workflows/release*.go
  - tools/ci/internal/workflows/workflows.go
needs: []
after:
  - T105
  - T013

ask_first:
  - checks
sitting: none
merged: 2026-10-20
pr: 188
---

# T127 - The release publish job row of `tools/ci workflows`

Merged on 2026-10-20 in pull request #188. The text below is the one
written at the re-plan.

Goal: The grammar row of the release workflow (10 10.2, the publish job:
`ubuntu-26.04`, needs every test job, no test, `tools/ci setup` first,
`GOPROXY=off`, attestation permissions), left by round 8 (R8-10-1) with
no task file. T088 needs it.

Re-plan (2026-10-12, stage B): New. A change to merged work is a task,
so it has an owner and an `after` edge: `after: T105` for the moved
file layout, and `after: T013`, which owns the workflows package whole,
so the smaller task waits. Re-plan (2026-10-20): `owns` adds
`workflows.go`, since the package dispatches by file only for `ci.yml`,
and the permissions, step `env` and `github.token` refusals live there.

- Implements: REQ-0768 (10 10.1), REQ-0808 (10 10.2).
- Operator: no
- Acceptance:
  - A fixture per admitted and per refused step of the publish job.
  - The grammar still refuses the forms it refuses today.
- Verify: `go test ./tools/ci/... -run '^TestWorkflowsRelease'`.
