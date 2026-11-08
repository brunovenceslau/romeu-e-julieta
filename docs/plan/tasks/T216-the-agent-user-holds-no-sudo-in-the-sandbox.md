---
id: T216
title: "The agent user holds no sudo in the sandbox"
milestone: M2
module: kits
fate: new
owns:
  - kits/julieta-claude/**
  - internal/cmd/julieta/setup/sudo*.go
  - internal/cmd/julieta/setup/steps.go
  - internal/cmd/julieta/setup/setup_test.go
  - e2e/kit_julieta_claude_test.go
needs: []
after:
  - T201
ask_first:
  - kits
sitting: none
---

# T216 - The agent user holds no sudo in the sandbox

Goal: the agent user holds no root in a romeu sandbox other than the
workload's Docker engine (I35): the `julieta-claude` kit takes the
agent's `sudo` away at install, and `julieta setup` fails at every boot
when `sudo -n true` still succeeds as the agent.

Added on 2026-11-08 (scope every adopter of the product), as decided on
2026-11-08: A19, measured by hand on 2026-11-07 on sbx v0.51.0, found
`sudo -n` working for the agent on `docker.io/docker/sbx-kit-claude`
and on `docker.io/sbx/claude`; Q33 accepts the engine only, and the
question had come back in several sessions, so the rule is an invariant
(05 5.2, I35) and this task is the code that holds it. Until it lands,
the sudo row of 05 5.4 (REQ-1400) records the gap. Its `after` edge is
the overlap on `kits/julieta-claude/**` and the setup steps with T201.

1. An install step of the `julieta-claude` kit that runs as root
   (`user: "0"`, as `os-base`'s does) removes every sudoers rule that
   grants the agent user and takes the agent out of the `sudo` group,
   and leaves the `docker` group (06 6.4, REQ-1399).
2. A `julieta setup` step runs `sudo -n true` as the agent and fails
   when it succeeds, with an error id added through `tools/new error`
   and a hint that names the workload pin.
3. By hand on the maintainer's host, recorded in the pull request: on
   the pinned workload with the kit, `sudo -n true` as the agent fails,
   `docker run hello-world` still runs as the agent, and the workload's
   entrypoint and the attached `layout up` still start. If the engine
   or the entrypoint needs the agent's `sudo`, the task stops and the
   maintainer decides.
4. Once it lands, the pull request removes the sudo row of 05 5.4
   (REQ-1400) and says so in Q33, under the `spec` approval line.

- Implements: REQ-1399 (06 6.4), REQ-1400 (05 5.4), REQ-1401 (05 5.2).
- Operator: yes: the hand check of item 3, and the `kits` and `spec`
  approval lines.
- Acceptance:
  - The E test of I35: a fixture workload whose agent has a sudoers
    rule and the `sudo` group; after the kit installs, `sudo -n true`
    as the agent fails and the agent is still in `docker`.
  - The U test of I35: `julieta setup` fails when `sudo -n true`
    succeeds, and passes when it fails.
  - The hand record of item 3 is in the pull request.
- Verify: `go run ./tools/ci all`.
