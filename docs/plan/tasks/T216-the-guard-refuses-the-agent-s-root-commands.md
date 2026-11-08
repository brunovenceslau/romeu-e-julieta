---
id: T216
title: "The guard refuses the agent's root commands"
milestone: M2
module: kits
fate: new
owns:
  - internal/cmd/julieta/hooks/guard*.go
  - skills/julieta/SKILL.md
  - e2e/kit_julieta_claude_test.go
needs: []
after:
  - T201
ask_first:
  - kits
sitting: none
---

# T216 - The guard refuses the agent's root commands

Goal: the agent runs no root command without the maintainer's
authorization (I35): the PreToolUse guard of the `julieta-claude` kit
refuses the forms of 06 6.4 item 9, and the `julieta` skill says the
rule in one line.

Added on 2026-11-08 (scope every adopter of the product), as decided on
2026-11-08: A19, measured by hand on 2026-11-07 on sbx v0.51.0, found
the agent in the `docker` group of the workload's root engine and
`sudo -n` working on both workloads. Removing `sudo` would be no
containment, since the `docker` group the agent needs gives root over
the VM, so Q33 accepts root by name and the rule is about use. The
guard is an instruction and a guard against mistakes, not containment,
and its text says so. The question had come back in several sessions;
Q33, I35 and this task close it. Its `after` edge is the overlap on
`e2e/kit_julieta_claude_test.go` with T201.

1. The guard refuses, as bash parses the command
   (`mvdan.cc/sh/v3/syntax`, as REQ-0598 does): `sudo`, `su`, `doas`,
   `pkexec`; `docker run` and `docker create` with `--privileged`, a
   `--cap-add`, `--pid=host`, `--network=host`, `--ipc=host`, or a bind
   whose source is `/`; the same inside the string of `bash -c`,
   `sh -c` and `eval`; and any command whose name is not a literal word.
2. The refusal text says that root needs the maintainer, who runs the
   command from the host with `sbx exec`.
3. The `julieta` skill gains one line with the same rule.

- Implements: REQ-1399 (06 6.4), REQ-1400 (05 5.4), REQ-1401 (05 5.2).
- Operator: no.
- Acceptance:
  - The U test of I35: the parse table, one row per refused form,
    `bash -c "s""udo true"` and `x=sudo; $x true` among them, and the
    admitted forms (`docker run hello-world`, `echo sudo`).
  - The E test of I35: a container case per refused form, as REQ-0598's
    are.
- Verify: `go test ./internal/cmd/julieta/hooks/ -run Guard -v` and
  `go run ./tools/ci e2e`.
