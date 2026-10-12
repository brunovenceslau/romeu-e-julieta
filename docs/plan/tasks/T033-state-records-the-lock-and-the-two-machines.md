---
id: T033
title: "`state`: records, the lock and the two machines"
milestone: M3
module: state
fate: kept
owns:
  - internal/state/**
needs:
  - T021
  - T017
after: []
ask_first:
  - gates
sitting: none
---

# T033 - `state`: records, the lock and the two machines

Earlier title: `state`: records, atomic writes and the lock.

Goal: `state`: records, atomic writes, the lock, the two machines as
transition tables with generated tests and diagrams, the descriptor
cache, the toolchain acknowledgement record.

Re-plan (2026-10-12): Kept; T034 (the candidate and generation machines)
is folded in, and its acceptance rows apply as written there.

- Implements: 02 2.4, 03 3.8, 04 4.1 (Locking).
- Operator: no.
- Acceptance:
  - A record is written by temp file, fsync and rename, mode 0600; the
    state directory is 0700.
  - A second process that takes `romeu.lock` exits 1 with `RJ-101`.
  - `toolchain.json`, the project record and the descriptor cache have
    generated `state-*.v1` schemas.
- Verify: `go test ./internal/state/...`.
