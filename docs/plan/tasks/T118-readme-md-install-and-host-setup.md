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

- Implements: REQ-0520 (05 5.2), REQ-0547 (05 5.4), REQ-0549 (05 5.4), REQ-0602 (06 6.4), REQ-1058 (12 12.1),
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
operator checks that it is in force (REQ-0549: `ssh-add -L` in a
sandbox lists exactly one key, the signing key, after the setup, after
each `sbx daemon restart` and after each boot), so that nothing of the
sandbox definitions' note on it is lost when that repository retires
(its `docs/ssh-agent-dedicated-host-socket.md` and its ADR 0033). The
page keeps each claim with its source. Measured: on 2026-10-08, in a
sandbox of those definitions, the agent socket is a relay to the host
and lists the host agent's two keys; on 2026-10-28, on the host with
sbx v0.50.1, `ssh.agentSocketPath` is empty, forwarding is on, and the
daily sandboxes list two keys. Read in sbx's documentation, not
measured: `sbx settings set ssh.agentSocketPath <socket>` is host-wide
(empty, sbx forwards the socket of each client) and a daemon restart
applies it; that the restart stops the running sandboxes is read in a
third-party guide, and on the host the restart does not always succeed,
so the page says to check `sbx daemon status` after it. Not measured,
written as the note proposes it: on macOS a LaunchAgent runs
`ssh-agent -D -a <socket>` and the key is loaded again after each boot;
on Linux the equivalent (a systemd user unit) is written when measured.
Whether sbx fails closed or falls back to the client's agent when the
socket is missing is what Q19 leaves to probes A12 and B2; the page
cites the probe's record.
