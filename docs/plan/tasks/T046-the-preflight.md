---
id: T046
title: "The preflight"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/preflight/**
needs:
  - T029
  - T033
  - T103
after: []
ask_first: []
sitting: none
---

# T046 - The preflight

Goal: The preflight, one function per step.

Re-plan (2026-10-12): Kept, narrowed to the rows I22, I23 and I26 need;
the breadth is deferred with its trigger (Decisions this plan defers).
Read differently until M6, in the I22 row below: "the A9 recording" is
the hand-written session of the global secret and allow-rule listing,
marked `synthetic: true` (T029, rule 10 of `docs/plan.md`), which A9
re-records when it runs. `internal/preflight/` joins the tree of 02 2.1
through T108 (amendment 13).

- Implements: 01 1.5, 04 4.2 (the rows of the doctor table marked
  **pre**), 05 5.2 (I15, I22, I23, I26).
- Operator: no.
- Acceptance:
  - The preflight is one function that runs its six steps in the order
    of 01 1.5 and stops at the first failure with the exit of its row. A
    table test covers each step.
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
    a test failure. A row the parser cannot classify as scoped to one
    sandbox counts as global and the preflight exits 2; a fixture row of
    an unknown shape proves it. The preflight exits 2 on each (01 1.5,
    step 5); it passes on the A9 recording itself and on a global secret
    for a service no project uses. Two fixtures, asserted separately: a
    recording with no global row passes. A recording with a global row
    of placeholder values passes only because they name no used service
    and no broad pattern.
  - Step 6 refuses a sandbox romeu has no open generation for, with
    `RJ-203` naming `romeu adopt`, and one whose workspace path differs
    (I26).
  - It touches no network: it passes with the registry and the origins
    unreachable (10 10.1).
  - No **P** command exists yet; `sync` (T043) is its first caller, and
    the live changes (T047) call the rest of it.
- Verify: `go test ./internal/... -run '^TestPreflight'`, which lists
  the step and the I22 tests by name.
