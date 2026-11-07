---
id: T047
title: "The live changes of `sync`"
milestone: M4
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/sync/live*.go
  - e2e/scenarios/j11/live*
  - internal/cmd/romeu/sync/steps.go
  - e2e/sync_live_test.go
needs:
  - T046
  - T043
  - T103
  - T190
  - T203
after: []
ask_first: []
sitting: none
---

# T047 - The live changes of `sync`

Goal: The live changes of `sync`.

Re-plan (2026-10-12): Kept, in M4. Its scenario files live under
`e2e/scenarios/j11/` beside T043's, with their own names; its e2e file
is `e2e/sync_live_test.go`; it extends `sync/steps.go`, the ordered step
list of T043, by one line (rule 14 of `docs/plan.md`), which the `after`
edge admits.

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): step 9 of `sync` hands no env file to sbx.
For a present sandbox, running or stopped, whose live recreate digest
differs from the open generation's `createdWith`, or whose generation
was adopted, it warns `recreate-needed` with the hint that the one
function of REQ-1377 gives for the sandbox's state: `romeu recreate
<name>` while it runs; for a stopped one, undo the change, `romeu sync
<name>`, rerun, with the warnings REQ-1377 lists; the adopted text on
either. The in-place apply waits behind the closed gate of `run` step 3
(REQ-1376), so `romeu run <name>` is never the hint. The function is
T203's, in `internal/preflight`, so this task needs T203 (REQ-0344,
REQ-1377).

- Implements: REQ-0192 (03 3.2), REQ-0344 (04 4.2), REQ-0355 (04 4.2),
  REQ-1377 (04 4.1), REQ-0488 (05 5.2), REQ-0511 (05 5.2), REQ-0634 (07
  7.5), REQ-0635 (07 7.5).
- Operator: no.
- Acceptance:
  - With a running sandbox, `sync` runs the rest of the preflight, then
    reconciles egress with the recorded argv and verifies it with `sbx
    policy check`.
  - The reconcile of a running sandbox writes each domain into
    `egressApplied` before its `allow` call and removes it after its
    `rm` call, so a reconcile cut short converges on the next run.
  - A gate 1 or a gate 2 change makes a `sync` whose sandbox is running
    exit 3 before a mutating call (I7); a foreign sandbox of the
    project's name makes it exit 2 with `RJ-203` (I26).
  - The `secrets` field follows its `apply` tag: recreate-class, or `sbx
    secret set --command` if the A10 result overturned the default
    (Q18).
  - It warns `recreate-needed` when the live recreate digest differs
    from the open generation's `createdWith`, or the generation was
    adopted, with the sandbox running and with it stopped, and makes no
    sbx call for it: an e2e case per state asserts the slug, the hint of
    REQ-1377 for that state (`romeu recreate <name>` while running; the
    undo, `romeu sync <name>` and rerun hint while stopped) and that the
    fake `sbx` records no `sbx env run`.
  - The J11 scenario function runs in CI.
- Verify: `go test -tags e2e ./e2e/... -run '^TestSyncLive' -v`. The
  pattern names the `TestSyncLive*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
