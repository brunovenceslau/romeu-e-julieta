---
id: T016
title: "The version handshake, on a synthetic session"
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

# T016 - The version handshake, on a synthetic session

Re-plan (2026-10-12): folded into T066; its rows are copied into T066's
Acceptance. This file keeps the id so that earlier citations resolve;
the text below is the one written before the re-plan, and T066 is where
it is built.

- Implements: 06 6.3, 05 5.2 (I30), 10 10.1 (E2E hybrid).
- Operator: no.
- Ask-first, as forecast before the re-plan: `sbxdrv`.
- Acceptance:
  - At the hybrid level on both Linux runners, the compatibility check
    passes for a julieta of an accepted protocol, with the expected
    sha256, in a read-only directory; a protocol mismatch, a
    `SHA256SUMS` mismatch and a writable bin mount each fail it with
    exit 2.
  - The session the fake `sbx` replays is hand-written, lives in the
    test's own data and is named `synthetic`; it is not under
    `e2e/testdata/sbx/<version>/`, which holds block A recordings only
    (10 10.3). T026 replaces it with the A4 recording.
  - `internal/sbxdrv` gains the two argv builders the check needs, `sbx
    version` and `sbx env exec`. They are the first rows of the argv
    table that T019 completes.
  - The test supplies `render.json` and `SHA256SUMS` as fixtures;
    `render` writes them from T039 on.
  - The function and the two builders are **dark** until T066 calls it.
    The I4 and I25 checks of T029 cover the builders when they land; the
    fake's exit 99 already holds here.
- Verify: `go test -tags e2e ./e2e/...`.
