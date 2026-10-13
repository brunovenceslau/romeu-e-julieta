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
is folded in, and its rows are copied below.

- Implements: REQ-0060 (01 1.4), REQ-0072 (01 1.4), REQ-0079 (01 1.6),
  REQ-0080 (01 1.6), REQ-0082 (01 1.6), REQ-0089 (01 1.6), REQ-0092 (01
  1.6), REQ-0094 (01 1.6), REQ-0098 (01 1.6), REQ-0100 (01 1.6),
  REQ-0101 (01 1.6), REQ-0102 (01 1.6), REQ-0103 (01 1.6), REQ-0104 (01
  1.6), REQ-0148 (02 2.4), REQ-0149 (02 2.4), REQ-0150 (02 2.4),
  REQ-0228 (03 3.8), REQ-0229 (03 3.8), REQ-0231 (03 3.8), REQ-0232 (03
  3.8), REQ-0239 (03 3.8), REQ-0240 (03 3.8), REQ-0260 (03 3.12),
  REQ-0283 (04 4.1), REQ-0284 (04 4.1), REQ-0351 (04 4.2), REQ-0447 (04
  4.4), REQ-0454 (04 4.4), REQ-0455 (04 4.4), REQ-0456 (04 4.4),
  REQ-0457 (04 4.4), REQ-0513 (05 5.2), REQ-0514 (05 5.2), REQ-0515 (05
  5.2), REQ-0516 (05 5.2), REQ-0517 (05 5.2), REQ-0570 (06 6.1),
  REQ-0571 (06 6.1), REQ-0755 (10 10.1), REQ-0776 (10 10.1), REQ-0961
  (10 10.6), REQ-1044 (12 12.1), REQ-1110 (12 12.3), REQ-1207 (01 1.6),
  REQ-1208 (01 1.6), REQ-1209 (01 1.6), REQ-1210 (01 1.6), REQ-1211 (01
  1.6), REQ-1212 (01 1.6), REQ-1213 (01 1.6), REQ-1214 (01 1.6),
  REQ-1215 (01 1.6), REQ-1216 (01 1.6), REQ-1217 (01 1.6), REQ-1218 (01
  1.6).
- Operator: no.
- Acceptance:
  - A record is written by temp file, fsync and rename, mode 0600; the
    state directory is 0700.
  - A second process that takes `romeu.lock` exits 1 with `RJ-101`. The
    commands that take `romeu.lock` are the eleven of 04 4.1 Locking,
    and `status`, `doctor`, `handoff` and `version` take none, asserted
    by a table over the command definitions.
  - `toolchain.json`, the project record and the descriptor cache have
    generated `state-*.v1` schemas.
  - A state record that does not decode is never written over: a test
    corrupts `projects/<name>.json`, runs a mutating command and asserts
    the file is byte-identical and the exit is `RJ-334`; `status` and
    `doctor` report it.
  - Each row of both tables has a hand-written case asserting its Effect
    at state level (01 1.6), and the case titled "every transition row
    has an Effect case" fails when the registered `(from, event)` keys
    differ from the table's rows.
  - The generation rows for a create that times out keep the record, and
    the next `run` goes on or closes the generation as lost (01 1.6).
  - The descriptor cache record is read first by `sync` step 6; T043
    asserts the read.
  - The package takes the clock and the ULID entropy source as injected
    values, every ULID comes from `canon.NewULID`, and its tests fix
    both (10 10.4).
- Acceptance (from T034, folded):
  - Each machine is a transition table; the generated tests exercise
    each row and assert each missing pair illegal, with exit 2 and an
    error id.
  - The candidate and generation diagrams that `ARCHITECTURE.md` links
    are generated from the tables, each in its own file under
    `docs/diagrams/` (12 12.3). They are the only generated diagrams:
    the promotion commit is the numbered procedure of 01 1.6, and the
    probe has no lifecycle diagram (12 12.8).
- Verify: `go test ./internal/state/...`; `go run ./tools/ci generated`.
