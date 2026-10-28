---
id: T118
title: "`README.md`: install and host setup"
milestone: M5
module: docs
fate: new
owns:
  - README.md
  - tools/ci/headings.yaml
needs:
  - T116
after: []
ask_first:
  - checks
sitting: none
---

# T118 - `README.md`: install and host setup

Goal: The J1 install paragraph and the host setup steps (the dedicated
signing agent, `kit.allowedSources`, secrets), in the heading order of
ADR 0001 rule 12 as ADR 0009 decision 6 amends it, so T095 extends it;
`tools/ci/headings.yaml` with the README list, on the `checks` surface.

- Implements: REQ-0547 (05 5.4), REQ-0602 (06 6.4), REQ-1058 (12 12.1),
  REQ-1087 (12 12.2), REQ-1183 (12 12.8).
- Operator: no
- Acceptance:
  - A reader with the two hosts' prerequisites reaches `romeu run` from
    the page alone.
  - `tools/ci/headings.yaml` holds the README list (the nine headings of
    ADR 0001 rule 12 as ADR 0009 decision 6 amends it) and this page
    matches it.
  - The host setup registers `signingKey` on GitHub as a signing key
    only, installs an `SSH_ASKPASS` program first, and adds the key to
    the dedicated agent with `ssh-add -c` (06 6.4, J1 step 5).
- Verify: `go run ./tools/ci fast` exits 0.

Re-plan (2026-10-28): the host setup also says how the dedicated agent
is started and kept alive, how sbx is pointed at it, and how the
operator checks that it is in force, so that nothing of the sandbox
definitions' note on it is lost when that repository retires. Measured
there on 2026-10-08 and on 2026-10-28 (sbx v0.50.1): a LaunchAgent on
macOS runs `ssh-agent -D -a <socket>` and the key is loaded again after
each boot; `sbx settings set ssh.agentSocketPath <socket>` is host-wide
(empty, sbx forwards the socket of each client), and `sbx daemon
restart` applies it and stops the running sandboxes; after each restart
and each boot `ssh-add -L` in a sandbox lists exactly one key, the
signing key, and whether sbx fails closed or falls back to the client's
agent when the socket is missing is measured once and recorded, since
it is not documented. The daily sandboxes of 2026-10-28 list two keys:
the step was decided on 2026-10-08 and not yet run.
