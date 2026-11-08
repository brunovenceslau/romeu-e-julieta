---
id: T026
title: "The block A results and recordings"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T019
  - T020
after: []
ask_first: []
sitting: O3
---

# T026 - The block A results and recordings

Re-plan (2026-10-12): deferred to M6; block A results and recordings
arrive with the harness; the facts the code needs first come from daily
use and from sitting M2, each marked with its source. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. T030's two rows are copied below: the A4
recording replaces T016's synthetic session here. The text below is the
one written before the re-plan; `needs` names the tasks whose code it
reads (the definitions of T019, the check of T020), and `owns` is
written when the task reopens.

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

- Implements: REQ-0142 (02 2.3), REQ-0158 (02 2.6), REQ-0566 (06 6.1),
  REQ-0588 (06 6.3), REQ-0750 (10 10.1), REQ-0924 (10 10.3), REQ-0943
  (10 10.4), REQ-0979 (11 (opening)), REQ-0981 (11 11.1), REQ-0982 (11
  11.1), REQ-0983 (11 11.1), REQ-0984 (11 11.1), REQ-0985 (11 11.1),
  REQ-0986 (11 11.1), REQ-0987 (11 11.1), REQ-0988 (11 11.1), REQ-0989
  (11 11.1), REQ-0990 (11 11.1), REQ-0991 (11 11.1), REQ-0992 (11 11.1),
  REQ-0993 (11 11.1), REQ-0994 (11 11.1), REQ-0995 (11 11.1), REQ-1368
  (11 11.1), REQ-1369 (11 11.1), REQ-1028
  (11 11.4).
- Operator: block O3 produces it; the maintainer opens it. A16's result
  file is committed from the maintainer's shell, and its **[review]**
  part is read in the pull request.
- Ask-first, as forecast before the re-plan: none for results alone;
  each surface a resolution touches.
- Acceptance:
  - The pull request holds the `probe-result.v1` files under
    `docs/probes/`, from both hosts, and the recordings under
    `e2e/testdata/sbx/<version>/`, and nothing else.
  - It reaches `main` like any change, not by a direct push, because a
    direct push is public before any check reads it: a branch in the
    maintainer's plain host clone with the hook enabled, `go run
    ./tools/ci fast`, the diff of everything staged read by the
    maintainer before the push, then a pull request, so the hook,
    `hygiene`, the pushed range and `pr` read host data before it is
    public.
  - A results-only branch holds no code, so the hook runs `main`'s own
    `tools/ci` on the host. When a resolution commit joins the branch
    (below), the maintainer reads that diff in the plain clone before
    checking the branch out, as item c of block O1 requires of code that
    runs on the host (10 10.2).
  - `tools/ci probes` passes. An overturned result lands with the
    results; its fix is its own pull request, and `tools/ci acceptance`
    (T090) fails until `resolvedBy` is set (11 11.4).
  - Pass, for block O3: `go run ./tools/ci probes --require-pass
    A1,A2,A3,A4,A5,A6,A9,A10,A11,A12,A13,A14,A15,A16,A17,A18,A19` exits 0 on the
    branch with both hosts' results; a `fail` or `inconclusive` verdict
    does not merge.
  - While the branch holds results alone, `git diff --name-only
    origin/main...HEAD` lists only paths under `docs/probes/` and
    `e2e/testdata/sbx/`.
  - The sha256 of each `sbxenv.yaml` golden is in the A5 result, one per
    file under `internal/render/testdata/`, as T019 defines A5.
  - A4 re-measures the workload user's home that the pin file carries
    with `source: sitting M2` (T012).
- Acceptance (from T030, folded):
  - The hybrid test of T016 replays the A4 recording of the
    non-interactive `sbx env exec` shape (04 4.2, `run` step 5), and the
    synthetic session is deleted in the same pull request.
  - A test over `e2e/*_test.go` and `e2e/hybrid_test.go` fails when the
    fake `sbx` is given a session from anywhere but
    `e2e/testdata/sbx/<version>/`, so a hand-written session cannot
    return after this task; the fake's own unit test data under
    `e2e/fakesbx/` is outside its scope.
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes
  --require-pass A1,A2,A3,A4,A5,A6,A9,A10,A11,A12,A13,A14,A15,A16,A17,A18,A19`.
