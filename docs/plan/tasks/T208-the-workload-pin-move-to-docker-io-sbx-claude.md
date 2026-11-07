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
needs:
  - T205
  - T055
  - T077
after:
  - T201
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
- Verify: `go run ./tools/ci kits && go test -tags e2e ./e2e/... -run
  '^TestGateWorkloadMove' -v`. The pattern names the function this task
  adds, and the reviewer reads it in the `-v` output (10 10.8).
