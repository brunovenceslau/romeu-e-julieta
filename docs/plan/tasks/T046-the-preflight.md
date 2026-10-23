---
id: T046
title: "The preflight"
milestone: M3
module: romeu-cli
fate: merged
owns:
  - internal/preflight/**
needs:
  - T029
  - T033
after: []
ask_first:
  - contracts
sitting: none
merged: 2026-10-23
pr: 261
---

# T046 - The preflight

Merged on 2026-10-23 in pull request #261. The text below is the one
written when it was built.

Goal: The preflight, one function per step.

Re-plan (2026-10-12): Kept, narrowed to the rows I22, I23 and I26 need;
the breadth is T134 (deferred with its trigger, Decisions this plan
defers). "The A9 recording" in the I22 row below is the hand-written
session of the global secret and allow-rule listing, marked `synthetic:
true` (T029, rule 10 of `docs/plan.md`), which A9 re-records when it
runs. `internal/preflight/` is a row of the tree of 02 2.1. The package
runs no e2e against the release build, so T103 leaves `needs`.

- Implements: REQ-0047 (01 1.2), REQ-0057 (01 1.3), REQ-0058 (01 1.3),
  REQ-0073 (01 1.5), REQ-0074 (01 1.5), REQ-0075 (01 1.5), REQ-0076 (01
  1.5), REQ-0077 (01 1.5), REQ-0078 (01 1.5), REQ-0308 (04 4.2),
  REQ-0347 (04 4.2), REQ-0365 (04 4.2), REQ-0371 (04 4.2), REQ-0372 (04
  4.2), REQ-0378 (04 4.2), REQ-0379 (04 4.2), REQ-0380 (04 4.2),
  REQ-0395 (04 4.2), REQ-0436 (04 4.4), REQ-0506 (05 5.2), REQ-0507 (05
  5.2), REQ-0511 (05 5.2), REQ-0713 (08 8.5), REQ-0751 (10 10.1),
  REQ-0968 (10 10.7), REQ-1203 (01 1.5), REQ-1204 (01 1.5), REQ-1205 (01
  1.5), REQ-1206 (01 1.5).
- Operator: no.
- Acceptance:
  - The preflight is one function that runs its six steps in the order
    of 01 1.5 and stops at the first failure with the exit of its row. A
    table test covers each step. `preflight/step5.go` holds the list of
    host checks, the seam T080 and T134 extend by one line (rule 14 of
    `docs/plan.md`).
  - Step 5 holds the checks marked **pre** that this phase can build:
    the `sbx` globals (I22), and the symlink, mise, direnv and VS Code
    trust rows, with `HOME` in a temp dir (I23). The signing row lands
    with T080.
  - The I22 fixtures are synthesized files under the preflight package's
    `testdata/synthetic/`, apart from the recordings and marked
    synthetic in their first line, because no listing of a well-kept
    host holds what they must show (Q29): one global secret whose
    service name equals a `secrets` entry of the project under test, and
    one per broad-rule shape in a table (`**`, a wildcard over a whole
    top-level domain, a permissive default network policy). Each parses
    with the parser of T029 the way the A9 recording of the global
    secret and allow-rule listing parses, and a fixture that does not is
    a test failure. A listing of an unrecorded shape, or one that cannot
    be parsed, fails step 5 with `upstream-shape` (`RJ-310`); a row of a
    recorded shape whose scope is not one sandbox counts as global
    (`sbx-global-secret`, `sbx-broad-allow`) and the preflight exits 2;
    a fixture row of an unknown shape proves it. The preflight exits 2
    on each (01 1.5, step 5); it passes on the A9 recording itself and
    on a global secret for a service no project uses. Two fixtures,
    asserted separately: a recording with no global row passes. A
    recording with a global row of placeholder values passes only
    because they name no used service and no broad pattern.
  - A listing with a changed container shape and one with an unparseable
    body each make the preflight exit 2 with `upstream-shape`
    (`RJ-310`); `doctor` reports the same two fixtures (T049).
  - Step 6 refuses a sandbox romeu has no open generation for, with
    `RJ-203` naming `romeu adopt`, and one whose workspace path differs
    (I26); the I26 cases "adopt accepts a sandbox the preflight refused"
    and "after adopt, salvage and rm succeed on the adopted sandbox" are
    T050's.
  - Step 6, the table: a `removing` generation with its sandbox present
    passes for `rm`, `recreate` and `retire`, and every other **P**
    command, `salvage --from-host` included, exits 2 with `RJ-336`.
  - It touches no network: it passes with the registry and the origins
    unreachable (10 10.1).
  - No **P** command exists yet; `sync` (T043) is its first caller, and
    the live changes (T047) call the rest of it.
- Verify: `go test ./internal/... -run '^TestPreflight'`, which lists
  the step and the I22 tests by name.
