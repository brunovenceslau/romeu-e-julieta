---
id: T047
title: "The live changes of `sync`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/sync/live*.go
  - e2e/scenarios/j11/live*
needs:
  - T046
  - T043
  - T103
after:
  - T043
ask_first: []
sitting: none
---

# T047 - The live changes of `sync`

Goal: The live changes of `sync`.

Re-plan (2026-10-12): Kept, in M4. Its scenario files live under
`e2e/scenarios/j11/` beside T043's, with their own names.

- Implements: 04 4.2 (`sync` step 9), 07 7.5 (Application), 09 (J11), 05
  5.2 (I7, I26).
- Operator: no.
- Acceptance:
  - With a running sandbox, `sync` runs the rest of the preflight, then
    reconciles egress with the recorded argv and verifies it with `sbx
    policy check`.
  - A gate 1 or a gate 2 change makes a `sync` whose sandbox is running
    exit 3 before a mutating call (I7); a foreign sandbox of the
    project's name makes it exit 2 with `RJ-203` (I26).
  - The `secrets` field follows its `apply` tag: recreate-class, or `sbx
    secret set --command` if the A10 result overturned the default
    (Q18).
  - It reports "recreate needed" when the recreate digest differs from
    the running generation.
  - The J11 scenario function runs in CI.
- Verify: `go test -tags e2e ./e2e/... -run '^TestSyncLive' -v`. The
  pattern names the `TestSyncLive*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
