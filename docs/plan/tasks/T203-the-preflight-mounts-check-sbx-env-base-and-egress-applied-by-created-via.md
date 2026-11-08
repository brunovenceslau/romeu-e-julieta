---
id: T203
title: "The preflight mounts check, sbx-env-base and egress-applied by --created-via"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/preflight/step6*.go
  - internal/preflight/mounts*.go
  - internal/preflight/envbase*.go
  - internal/preflight/errors.go
  - internal/cmd/romeu/doctor/env_base*.go
  - internal/cmd/romeu/doctor/egress_applied*.go
  - internal/cmd/romeu/doctor/skills_share_writable*.go
  - e2e/preflight_mounts_test.go
  - e2e/doctor_env_base_test.go
  - internal/preflight/recreate*.go
  - internal/preflight/preflight.go
  - internal/preflight/preflight_test.go
  - internal/preflight/step5.go
  - internal/preflight/step5_test.go
  - internal/sbxdrv/policy_ls*.go
  - e2e/testdata/sbx/0.51.0/egress.jsonl
  - e2e/doctor_egress_applied_test.go
needs:
  - T046
  - T049
  - T202
after: []
ask_first:
  - contracts
  - sbxdrv
sitting: none
---

# T203 - The preflight mounts check, sbx-env-base and egress-applied by --created-via

Goal: the preflight lets sbx tell romeu what it mounted and what it
merged: step 6 compares the mounts `sbx ls --json` lists with the
render's, step 5 and `doctor` fail on a user-level `~/.sbxenv.yaml`,
and `egress-applied` reads only the rules `sbx policy allow` made.

Added on 2026-11-06 (scope this repository), with the specification
amended first: the sbx 0.51 and kit schema m.12 amendments of 01 1.5,
04 4.2, 04 4.4 and 05 5.2 (I22).

1. The mounts check (REQ-1354, REQ-1364, `RJ-368`): where the recorded
   `sbx ls --json` shape lists the sandbox's host bind mounts, each
   mount of the render (the primary clone, every memory dir and
   `.romeu/bin`) must be listed at its target with its source present;
   a mount not listed, at another target or marked missing is
   `mounts-mismatch`, exit 2, hint `romeu recreate <name>`. A mount
   romeu did not render is not compared; `status` prints it (T197). The
   protective commands print the failure and go on (REQ-0078), and
   `rm`, `retire` and `recreate` pass every step but this one before
   their removal half.
2. `sbx-env-base` (REQ-1361, REQ-1363, `RJ-367`): preflight step 5 and
   the `doctor` row fail when `~/.sbxenv.yaml` exists on the host; the
   hint names the file to move away; `doctor` reports it as a finding
   of `RJ-328`, a **pre** check.
3. `egress-applied` (REQ-0392): the `doctor` row compares
   `egressApplied` with the per-sandbox rules `sbx policy ls` reports
   as made by `sbx policy allow` (`--created-via`, the value the A9
   recording shows), so the rules sbx provisioned from the descriptors
   are left out; until A9 records the value, the synthetic session
   carries the one the sbx 0.45 notes name, marked `synthetic: true`.

Amended on 2026-11-07 (the closed gate and the closed mounts list, as
decided on 2026-11-07): the task moves to M3, since `run` (T066) needs
it and sitting M3 needs `run`. Item 1 is corrected and extended, and
items 4 and 5 are added.

1. (amended) The mounts check compares each mount of the render whose
   repo the open generation records (03 3.8, `repos`), `_shared` and
   `.romeu/bin`, each at its target of 02 2.5; the primary clone keeps
   its host path and is not compared, and a repo the spec added since
   the create is `run` step 3's `recreate-needed`, not this step's. A
   repo the spec removed since the create, whose memory bind the
   listing still holds, and a mount whose target the spec changed fail
   the step with `mounts-mismatch`, even on a running sandbox that step
   3 would warn about and attach to (REQ-1354, N7). A host bind romeu
   did not render passes only on the closed list of REQ-1381: the env
   file, and each kit source and each `agent-skill@1` skill dir of a
   kit of the render, and each passes only when its source, after
   resolving links, is the env file or lies under a
   `.romeu/kits/<id>-<digest12>/` dir, and its target is the one A4
   records (the synthetic session's until then); which render's kit
   dirs count on a running sandbox whose kits changed since the create
   is part of the open question on mount-affecting changes on a running sandbox (docs/spec.md). A bind off the list fails the step for `run` and
   `sync`, and the protective commands print it and go on (REQ-1381,
   REQ-0078); the plan adds no read-only condition the spec does not
   hold (RC-1, amended on 2026-11-07). A
   listing with no mounts for a running sandbox fails the step; one
   with none for a stopped sandbox defers the comparison to right after
   the command starts it (REQ-1382), through a function this task
   exports, which returns the result with the same results as step 6:
   `run` (T066) then stops the sandbox with `sbx stop` and exits 2 with
   `mounts-mismatch`, saying the bind was live until the stop
   (REQ-1393, which names `run` only), and the salvage start (T068)
   prints it and goes on (REQ-1382, REQ-0078; amended on 2026-11-07). The comparison is skipped whole for an adopted generation
   (REQ-1383). RJ-368's hint is the one of 04 4.4 for the sandbox's
   state: restore the source and rerun, or salvage then recreate; for a
   removed repo or a changed target on a running sandbox `romeu
   recreate <name>`; on a stopped sandbox whose digests differ it ends
   with the stopped hint of item 4.
4. The recreate-needed hint and refusal (REQ-1377, `RJ-305`): one
   function in `internal/preflight/recreate*.go`, keyed by the
   sandbox's state (running or stopped) and by whether the generation
   was adopted, returns the hint of every `recreate-needed` warning and
   error: `romeu recreate <name>` for a running sandbox; for a stopped
   one, the undo of the change that moved the recreate digest, then
   `romeu sync <name>` and a rerun, naming the three sources that
   digest covers: the project spec, a secret's argv in the host
   settings, and the product-kit content of the romeu release, which
   has no romeu route until T210, so a sandbox a release will need
   recreated is started before the upgrade (REQ-1377, J9), with the
   warnings REQ-1377 lists; the adopted
   messages for an adopted generation. Beside it, the refusal that
   `run` (T066), `pull` (T067), `salvage`, `rm`, `recreate`, `retire`
   and the fingerprint read (T068, T069) make on a stopped sandbox whose
   digests differ or whose generation was adopted: exit 5 with
   `recreate-needed`, no sbx call that starts it, no record change
   (REQ-1387). `RJ-305` is declared in `internal/preflight/errors.go`.
   This task is the first that needs the hint, for RJ-368's, so it owns
   it; `status` (T197) and `sync` step 9 (T047) import it.
5. The step table and the step 5 seam: step 6's mounts result is
   appended in `preflight.go` as a printed result for the protective
   commands, and the host check of `sbx-env-base` is one line of the
   step 5 slice (rule 14 of `docs/plan.md`); the `--created-via` parse
   of item 3 is in `internal/sbxdrv/policy_ls.go` and the policy
   session of `e2e/testdata/sbx/0.51.0/egress.jsonl`, after T202, which
   owns the other sessions.

Amended on 2026-11-08 (as decided on 2026-11-08, security finding M3 of
the review of the sbx 0.51 amendments; scope every adopter of the
product): the doctor row `skills-share-writable` (04 4.2, REQ-1398)
reads every sandbox sbx lists, romeu's or not, from the structured
mounts of `sbx inspect`, and warns for each that holds a writable bind
of the skills store or of a directory above it; it reuses this task's
recognition of the store's bind (REQ-1381) and the mount parse of T202.

- Implements: REQ-1398 (04 4.2), REQ-1354 (01 1.5), REQ-0078 (01 1.5), REQ-1361 (04 4.2),
  REQ-0392 (04 4.2), REQ-1363 (04 4.4), REQ-1364 (04 4.4), REQ-0506 (05
  5.2), REQ-1377 (04 4.1), REQ-1381 (01 1.5), REQ-1382 (01 1.5),
  REQ-1383 (01 1.5), REQ-1387 (04 4.2), REQ-1393 (05 5.4).
- Operator: yes, the approval lines for the `errors.go` declarations of
  `RJ-305`, `RJ-367` and `RJ-368` (ask-first `contracts`) and for
  `internal/sbxdrv/policy_ls.go` (ask-first `sbxdrv`). Item 1's build
  waits for the maintainer's answer to the open question on mount-affecting changes on a running sandbox (docs/spec.md), which decides how step 6
  treats a repo added or removed, a mount target changed and a kit
  changed on a running sandbox, and which kit dirs the closed list
  exempts (amended on 2026-11-07).
- Acceptance:
  - `skills-share-writable`: a sandbox romeu did not create with a
    writable bind of the store, and one with a writable bind of `$HOME`,
    each a warning that names it escaped; every store bind read-only, no
    finding; a sandbox listed with no mounts, skipped and named.
  - With a fake session listing every compared mount at its target,
    step 6 passes; with one missing, one at another target, one marked
    missing, a memory bind of a repo the spec removed since the create,
    a mount whose target the spec changed, and a host bind off the
    closed list (a bind of the host skills store that is not read-only
    among them), each
    exits 2 with `mounts-mismatch` naming the mount; a render with one
    repo more than the generation records passes step 6 (amended on
    2026-11-07).
  - The read-only bind of the host's skills store passes at the source,
    target and read-only flag the session records (REQ-1381; amended on
    2026-11-07); a bind of the store that is not read-only fails with
    `mounts-mismatch`, and its hint names `skills.defaultMode` first (set
    it to `off`, or `readonly`, then `romeu recreate <name>`), since a
    recreate under the same setting fails again (04 4.4, RJ-368; decided
    on 2026-11-07).
  - An exempt bind (the env file, a kit source, a skill dir) passes at
    the target the session records, with a source under a
    `.romeu/kits/<id>-<digest12>/` dir or equal to the env file; the
    same bind at another target, or with a source elsewhere, fails with
    `mounts-mismatch` for `run` and `sync` and is printed by the
    protective commands, which go on (amended on 2026-11-07).
  - A listing that shows no mounts fails the step for a running
    sandbox; for a stopped one the step passes and the exported
    after-start comparison, called with a listing that holds a bind off
    the list, returns `mounts-mismatch` and calls no sbx command itself;
    the stop and its message are `run`'s (T066), and the salvage start
    prints the result and goes on (REQ-1382, REQ-1393).
  - For an adopted generation the step compares nothing and passes
    (REQ-1383).
  - The hint function returns `romeu recreate <name>` for a running
    sandbox, the stopped hint for a stopped one, and the two adopted
    messages, each a table case asserting the text of REQ-1377; the
    refusal helper returns exit 5 and `RJ-305` with the stopped hint,
    and the fake records no sbx call (REQ-1377, REQ-1387).
  - The preflight returns the mounts failure as a non-stopping result
    for the protective commands (`salvage`, `salvage --from-host`,
    `stop`, `pull`) and for the salvage half of `rm`, `retire` and
    `recreate`, and as a stopping one for `run` and `sync`, asserted in
    the preflight's unit test per command; the commands' own behaviour
    on it is T066's, T067's, T068's, T069's and T074's.
  - A `~/.sbxenv.yaml` in the test's home makes the preflight exit 2
    with `sbx-env-base` and `doctor` exit 6 with the row's finding and
    the hint; without the file both pass.
  - The `egress-applied` fixture with one rule made by `sbx policy
    allow` and one provisioned from a descriptor compares the first
    only; a listing without the `--created-via` field, or one whose
    value neither recording shows, is `upstream-shape`.
- Verify: `go test ./internal/preflight/... ./internal/sbxdrv/... -run
  '^(TestStep6Mounts|TestStep5EnvBase|TestRecreateHint|TestRecreateRefusal|TestMountsAfterStart|TestPolicyLsCreatedVia)'
  -v && go test -tags e2e ./e2e/...
  -run '^(TestPreflightMounts|TestDoctorEnvBase|TestDoctorEgressApplied)'
  -v`. The patterns name the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
