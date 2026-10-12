---
id: T030
title: "The version handshake, on the recorded session"
milestone: none
module: romeu-cli
fate: folded
absorbed_by: T066
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T030 - The version handshake, on the recorded session

Re-plan (2026-10-12): folded into T066, which carries its acceptance
rows. This file keeps the id so that earlier citations resolve; the text
below is the one written before the re-plan, and T066 is where it is
built.

- Implements: 06 6.3, 05 5.2 (I30), 10 10.3 (Source of truth).
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - The hybrid test of T016 replays the A4 recording of the
    non-interactive `sbx env exec` shape (04 4.2, `run` step 5), and the
    synthetic session is deleted in the same pull request.
  - A test over `e2e/*_test.go` and `e2e/hybrid_test.go` fails when the
    fake `sbx` is given a session from anywhere but
    `e2e/testdata/sbx/<version>/`, so a hand-written session cannot
    return after this task; the fake's own unit test data under
    `e2e/fakesbx/` is outside its scope.
- Verify: `go test -tags e2e ./e2e/...`.
