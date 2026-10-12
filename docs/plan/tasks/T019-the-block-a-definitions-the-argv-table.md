---
id: T019
title: "The block A definitions, the argv table and the goldens"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T018
  - T012
  - T017
  - T066
after: []
ask_first: []
sitting: none
---

# T019 - The block A definitions, the argv table and the goldens

Re-plan (2026-10-12): deferred to M6; the argv table and the goldens are
written with the fake's hand-written sessions (T029) and with `render`
(T037); block A re-measures them before the release candidate. Reopened
by: a real-sbx behaviour the fake cannot model breaks a daily run; or
the first release candidate. The text below is the one written before
the re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: 11 11.1, 10 10.3 (Replay only, Stateful replay, Known
  argv), 04 4.2, 07 7.5 (Application), 03 3.3, 10 10.4.
- Operator: feeds block O3.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - A definition exists for each probe id of 11 11.1: A1, A2, A3, A4,
    A5, A6, A9, A10, A11, A12, A13 and A14, each with its expectation
    written before any run. A3 records the mount facts; what romeu and
    julieta do with the plants is C6 of block B (11 11.2).
  - The table of 11 11.1 ("Recorded sessions") is committed as data: the
    `sbx` argv shapes romeu needs, the ones 04 4.2 names for `sync`,
    `run`, `adopt`, `stop`, `salvage`, `rm`, `retire`, `status`,
    `doctor` and `pull`, and the three policy commands of 07 7.5. The
    sessions the fake `sbx` starts from are derived from each task of
    this plan whose end-to-end tests call the fake, T030, T041, T042,
    T043, T044, T045, T047, T048, T050, T055, T066, T067, T068, T069,
    T073, T074, T075, T077, T080 and T081, and not from the journeys
    alone: the five CI journeys J2, J3b, J7, J10 and J11 (10 10.1), each
    with the state its sandbox starts in: removed, stopped or created
    outside romeu (10 10.3). That covers `adopt`, `stop` and `pull`,
    `salvage` alone and `recreate`, whose call orders no journey session
    holds, and stateful replay fails a call order no session holds. T016
    is not among those tasks: its session is synthetic, and T030
    replaces it with the A4 recording. T055, T077, T080 and T081 run
    `sync` or a J2 step through it and reuse its sessions.
  - A4's definition records the two non-interactive `sbx env exec <dir>
    -- ...` shapes of 04 4.2 (`run`, steps 5 and 6) beside the `-it`
    forms 11 11.1 lists, so T030 and T066 replay them. That is the
    plan's addition to A4, by the rule of the table.
  - A test fails when a row of that table is exercised by no block A
    definition. A shape block A did not record cannot be replayed, and
    recording it later means a second sitting on both hosts.
  - A test walks `e2e/` and fails an end-to-end test that calls the fake
    and declares no session, and a declared session that is no row of
    the table. The other direction, a row that no test declares, is
    checked in T075, when every such test exists.
  - The `sbxenv.yaml` goldens that A5 runs `sbx env plan` on are written
    here, under `internal/render/testdata/`, in the shape of 03 3.3,
    with kit directory names computed by `canon`'s `kit-tree` digest
    over fixture kits. T037 must produce the same bytes. A5's definition
    hashes every file under that directory, so each golden has a
    recorded hash.
  - A11, A12 and A13 read the frontend, the workload and herdr from the
    pin file of T012.
  - A9 lists the global secrets and the global allow rules as `doctor`
    would read them (11 11.1), and plants nothing (Q29). It lists once
    while its own scoped rule still exists, and A10's scoped secret too
    when A10 ran first in the run and its sandbox is still live, before
    its `rm --resource` and `env rm` steps; if A10 runs second, only the
    scoped rule is listed. The spec gives no order between A9 and A10;
    the listing point and that order are the plan's addition to A9. 11
    11.1 lists after the removals, and the recording would then hold no
    row that tells a global one from a scoped one. It records two
    observations, `globalRuleRowSeen` and `globalSecretRowSeen`. The
    recorded shape of that listing is what the synthesized I22 fixtures
    of T046 and T049 must parse as.
  - A2 sets the global `env.rememberHostCommands` (11 11.1) and records
    the prior value in its result. A12's agent is one the harness
    starts, with a key it generates.
- Verify: `go run ./e2e/probes --block A --out <dir>` against the helper
  binary in a test.
