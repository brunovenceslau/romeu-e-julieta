---
id: T015
title: "The fake `sbx`"
milestone: none
module: probes
fate: folded
absorbed_by: T029
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T015 - The fake `sbx`

Re-plan (2026-10-12): folded into T029; its rows are copied into T029's
Acceptance. This file keeps the id so that earlier citations resolve;
the text below is the one written before the re-plan, and T029 is where
it is built.

- Implements: 10 10.3, 05 5.2 (I4, I25).
- Operator: no.
- Ask-first, as forecast before the re-plan: `sbxdrv`.
- Acceptance:
  - The map names no owner for `e2e/fakesbx`; see [Questions for the
    maintainer](../../plan.md#questions-for-the-maintainer).
  - On recordings of its own test data: an argv shape absent from the
    recording fails the test with the argv (Replay only); a call out of
    the recorded order fails (Stateful replay); for `env exec`, the argv
    before `--` is matched and the command after it is forwarded.
  - It exits 99 on `--auto-approve` (I4) and on a value form of `sbx
    secret set` (I25), whatever the recording holds.
  - The placeholder normalizer has a unit test (10 10.1).
- Verify: `go test ./e2e/fakesbx/...`.
