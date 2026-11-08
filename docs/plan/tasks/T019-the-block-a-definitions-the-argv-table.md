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

Amended on 2026-11-06 and 2026-11-07 (the sbx 0.51 and kit schema m.12
amendments, as decided on 2026-11-07): the table of 11 11.1 gains A18
(the caches across sandboxes and recreates, REQ-1368) and A19 (the
engine of the `sbx/claude` workload and `dockerd@1`, REQ-1369; its
workload part is measured first by hand inside T205's measurement,
beside A9's deny, and this probe re-records it), and extends:
- A3: each mount at its target of 02 2.5, the `julieta` link, and a
  write, as the agent and as root, to each bind sbx makes itself (the
  env file, each kit source, each skill dir of an `agent-skill@1` entry),
  whose read-only flag is recorded;
- A4: `sbx inspect`, the mount fields of `sbx ls --json` for a running
  and for a stopped sandbox (an empty list included) and the source,
  target and read-only flag of each bind sbx adds itself (the host's
  skills store among them, under the `skills.defaultMode` in force;
  added on 2026-11-07), the unreadable state, exit 125 of `env exec` and its status when the daemon is
  stopped, `--no-input` per argv shape and the two offers of sbx 0.51,
  each provoked where the harness can; a shape provoked on neither host
  is recorded as `not-provoked`, keeps no `--no-input`, and is named in
  the result, whose synthetic session 10 10.3 admits (REQ-0924);
- A5: the goldens with `target`, `entrypoint` and `cmd` and no
  `skills` key, the golden with a `skills` key added (refused by hand
  on 2026-11-07, sbx v0.51.0; an acceptance reopens the skills row of
  Deferred decisions), and a `~/.sbxenv.yaml`
  planted under a home the harness creates, never the operator's: the
  harness refuses to start while the operator's own file exists and
  removes its plant on every exit path; where sbx cannot be pointed at
  another home, A5 plants nothing in the operator's home and records
  the user-level file case as `not-provoked` (block A plants nothing but
  A2's one global setting, Q29);
- A9: the `--created-via` value, and the host and port deny of Q2, read
  again beside a portless allow of the same host and after a same-name
  `sbx env rm` and `sbx env run -d --clone`;
- A15: `sbx env exec <dir> -- true` on a stopped sandbox whose env file
  in `<dir>` changed, and `sbx env exec` on a running one with a changed
  env file, each read back through `sbx ls --json`, `sbx inspect`, `sbx
  policy ls`, a marker and the sandbox id (T210 records both first by
  hand); and the in-place apply measurement T210 records first by hand
  and this probe re-records: on a running and on a stopped sandbox, `sbx
  env run -d <dir>` with the env file changed in one kind at a time (a
  kit added, changed, removed; the workload; a `ports` entry; each size
  option of 03 3.1; `agent`; a repo added, removed; a `target` or
  `readOnly`; a secrets binding added, removed), whether the instance
  stays and the change took effect as read back through sbx, an apply
  repeated and one killed mid-apply, a declined plan, the attached `sbx
  env run <dir>` on an equal env file carrying the `entrypoint` and
  `cmd` of 03 3.3 with and without `--no-input`, and that entrypoint
  and `cmd` on an attached, a detached and a cloned `env run`; A15 is
  re-run at each floor bump.
The argv list of REQ-0997 grows with `sbx inspect`, the attached `sbx
env run <dir>`, `--no-input`, `sbx secret ls` and the settings listing.
A18 and A19 record only, so each passes once its observations are
recorded (10 10.3, REQ-0950).

- Implements: REQ-0142 (02 2.3), REQ-0158 (02 2.6), REQ-0199 (03 3.3),
  REQ-0203 (03 3.3), REQ-0566 (06 6.1), REQ-0588 (06 6.3), REQ-0928 (10
  10.3), REQ-0938 (10 10.3), REQ-0943 (10 10.4), REQ-0976 (11
  (opening)), REQ-0977 (11 (opening)), REQ-0979 (11 (opening)), REQ-0980
  (11 (opening)), REQ-0981 (11 11.1), REQ-0982 (11 11.1), REQ-0983 (11
  11.1), REQ-0984 (11 11.1), REQ-0985 (11 11.1), REQ-0986 (11 11.1),
  REQ-0987 (11 11.1), REQ-0988 (11 11.1), REQ-0989 (11 11.1), REQ-0990
  (11 11.1), REQ-0991 (11 11.1), REQ-0992 (11 11.1), REQ-0993 (11 11.1),
  REQ-0994 (11 11.1), REQ-0995 (11 11.1), REQ-0997 (11 11.1), REQ-0998
  (11 11.1), REQ-0999 (11 11.1), REQ-1368 (11 11.1), REQ-1369 (11 11.1).
- Operator: feeds block O3.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - A definition exists for each probe id of 11 11.1 (A1 to A6, A9 to
    A19), each with its expectation written before any run, and a test
    compares the definitions with the ids the table lists. A3 records
    the mount facts; what romeu and julieta do with the plants is C6 of
    block B (11 11.2). A16 runs in the maintainer's host shell and
    records the token's type and scopes, or the repository list and
    permissions read by hand (**[review]**), never the value, through
    the recorder's write function; A17 is arch-sensitive and counts only
    with both hosts.
  - The table of 11 11.1 ("Recorded sessions") is committed as data: the
    `sbx` argv shapes romeu needs, the ones 04 4.2 names for `sync`,
    `run`, `adopt`, `stop`, `salvage`, `rm`, `retire`, `status`,
    `doctor` and `pull`, and the three policy commands of 07 7.5. The
    sessions the fake `sbx` starts from are derived from each task of
    this plan whose end-to-end tests call the fake, T041, T042, T043,
    T047, T048, T050, T055, T066, T067, T068, T069, T073, T074, T075,
    T077, T080 and T081, and not from the journeys alone: the six CI
    journeys J2, J3b, J6, J7, J10 and J11 (10 10.1), each with the state
    its sandbox starts in: removed, stopped or created outside romeu (10
    10.3). That covers `adopt`, `stop` and `pull`, `salvage` alone and
    `recreate`, whose call orders no journey session holds, and stateful
    replay fails a call order no session holds. T016 is not among those
    tasks: its session is synthetic, and T026 replaces it with the A4
    recording. T055, T077, T080 and T081 run `sync` or a J2 step through
    it and reuse its sessions.
  - Block A records the three failure sessions of 11 11.1, which replace
    the synthetic ones of T029.
  - A4's definition records the two non-interactive `sbx env exec <dir>
    -- ...` shapes of 04 4.2 (`run`, steps 5 and 6) beside the `-it`
    forms 11 11.1 lists, so T026 and T066 replay them. That is the
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
  - A4's provoked sandbox, one whose state sbx cannot read or whose sbx
    records were removed by hand, is named in the result and removed
    before the run ends, by the recovery of REQ-0102 (a daemon restart,
    announced before A4 starts); the harness skips the provocation, and
    records it as `not-provoked`, while other sandboxes run on the host
    (amended on 2026-11-07).
  - A2 sets the global `env.rememberHostCommands` (11 11.1) and records
    the prior value in its result. A12's agent is one the harness
    starts, with a key it generates.
- Verify: `go run ./e2e/probes --block A --out <dir>` against the helper
  binary in a test.
