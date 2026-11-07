---
id: T203
title: "The preflight mounts check, sbx-env-base and egress-applied by --created-via"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/preflight/step6*.go
  - internal/preflight/mounts*.go
  - internal/preflight/envbase*.go
  - internal/preflight/errors.go
  - internal/cmd/romeu/doctor/env_base*.go
  - internal/cmd/romeu/doctor/egress_applied*.go
  - e2e/preflight_mounts_test.go
  - e2e/doctor_env_base_test.go
needs:
  - T046
  - T049
  - T202
after: []
ask_first:
  - contracts
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

- Implements: REQ-1354 (01 1.5), REQ-0078 (01 1.5), REQ-1361 (04 4.2),
  REQ-0392 (04 4.2), REQ-1363 (04 4.4), REQ-1364 (04 4.4), REQ-0506 (05
  5.2).
- Operator: yes, the approval line for the `errors.go` declarations of
  `RJ-367` and `RJ-368` (ask-first `contracts`).
- Acceptance:
  - With a fake session listing every rendered mount at its target,
    step 6 passes; with one missing, one at another target, one marked
    missing, and one extra mount romeu did not render, the first three
    exit 2 with `mounts-mismatch` naming the mount and the fourth
    passes; `salvage`, `stop` and `pull` print the failure and go on,
    and `run` and `sync` stop.
  - With a recorded shape that lists no mounts, step 6 compares nothing
    and reports that it did not, the line `status` prints (T197).
  - The preflight returns the mounts failure as a non-stopping result
    for the protective commands (`salvage`, `salvage --from-host`,
    `stop`, `pull`) and for the salvage half of `rm`, `retire` and
    `recreate`, and as a stopping one for `run` and `sync`, asserted in
    the preflight's unit test per command; the commands' own behaviour
    on it is T068's and T074's.
  - A `~/.sbxenv.yaml` in the test's home makes the preflight exit 2
    with `sbx-env-base` and `doctor` exit 6 with the row's finding and
    the hint; without the file both pass.
  - The `egress-applied` fixture with one rule made by `sbx policy
    allow` and one provisioned from a descriptor compares the first
    only; a listing without the `--created-via` field, or one whose
    value neither recording shows, is `upstream-shape`.
- Verify: `go test ./internal/preflight/... -run
  '^(TestStep6Mounts|TestStep5EnvBase)' -v && go test -tags e2e ./e2e/...
  -run '^(TestPreflightMounts|TestDoctorEnvBase|TestDoctorEgressApplied)'
  -v`. The patterns name the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
