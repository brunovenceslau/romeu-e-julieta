---
id: T208
title: "The workload pin move to docker.io/sbx/claude"
milestone: M4
module: kitpin
fate: new
owns:
  - kits/pins.yaml
  - examples/**
  - e2e/gate_workload_move_test.go
  - internal/cmd/romeu/sync/rule_manifest_size.go
  - internal/cmd/julieta/spec/manifest.go
needs:
  - T205
  - T055
  - T213
  - T209
after:
  - T204
ask_first:
  - kits
sitting: none
---

# T208 - The workload pin move to docker.io/sbx/claude

Goal: the product's workload pin moves from
`docker.io/docker/sbx-kit-claude` to `docker.io/sbx/claude`, the
built-in Claude sbx 0.51 publishes as a v3 kit, once T205 has taken its
decision; where that decision is the per-sandbox deny, this task also
waits for T209.

Added on 2026-11-06 (scope this repository): Q2 of the index and 06
6.2, as decided on 2026-11-06. The measurement and the decision are
T205's; this task is the pin change, a pull request of its own.

1. `go run ./tools/kitpin workload <digest>` on the resolved digest of
   `sbx/claude`, whose manifest list covers linux/amd64 and
   linux/arm64; `julieta pin workload` on the example specs;
   `workloadRepositories` of the host-settings example and the `agent`
   handle the descriptor declares written into the examples (03 3.2).
2. The pin bump's probes run again (11 11.4: A12, A15, C3, C5, B2's S5
   part and A19, Q2); until block A, the sitting facts, each marked
   with its source.
3. `needs: T209` is added by T205's decision when the deny branch is
   taken, before this task starts.

Amended on 2026-11-07 (as decided on 2026-11-07, decisions P and S):
the pin moves only where T205's two conditions allow it: the plain-HTTP
condition of REQ-1374 and the engine condition of REQ-1386, which A19,
run by hand with A9 in T205's measurement, settles. The engine starts
through the image annotation `com.docker.sandboxes.start-docker`, which
gate 2 shows as the `dockerd@1` grant only once T213 lands (REQ-1385,
REQ-1391), so this task needs T213. The workload user's home, read in
T205's note, must equal the `/home/agent` that the size rule of `sync`
and the stand-in manifest of `julieta spec validate` hard-code as
`sandboxHome`, or this task moves both constants with the pin (V2.39).
T077 leaves `needs` (merged long ago), and T201 leaves `after`: no path
is shared, and T213 already orders this task after T201 (V2.52).

- Implements: REQ-0578 (06 6.2), REQ-0579 (06 6.2), REQ-0190 (03 3.1),
  REQ-0577 (06 6.2).
- Operator: yes, the approval line for `kits/pins.yaml` (ask-first
  `kits`), and the probe runs of item 2 on the hosts.
- Acceptance:
  - `kits/pins.yaml` names `docker.io/sbx/claude` by digest, `tools/ci
    kits` passes, and every example spec and host-settings file names
    the new repository and the agent handle the descriptor declares.
  - The gate 2 diff of a project moving from the old pin to the new one
    highlights the workload repository and lists the capability
    changes, the runtime egress pair among them.
  - `julieta pin workload` reports the derived `provides` without
    `deb/tzdata` and `deb/zsh`, so `os-base` stays in the kit set.
  - The pull request names T205's outcome for each condition: A19
    showed that the agent user cannot reach the engine's socket, or
    the 05 5.4 row of REQ-1386 accepts that elevation by name and Q33
    is amended; a pull request without one of the two does not move the
    pin (REQ-1386).
  - The gate 2 diff of that move lists the `dockerd@1` grant the
    start-docker annotation normalizes to, marked first, through T213
    (REQ-1385).
  - The workload user's home recorded in T205's note equals
    `/home/agent`, or the two `sandboxHome` constants take the recorded
    home and their tests pass with it.
- Verify: `go run ./tools/ci kits && go test -tags e2e ./e2e/... -run
  '^TestGateWorkloadMove' -v` (the case of the engine grant included). The pattern names the function this task
  adds, and the reviewer reads it in the `-v` output (10 10.8).
