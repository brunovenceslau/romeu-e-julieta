---
id: T080
title: "The signing rule"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/sync/signing_rule.go
  - internal/cmd/romeu/doctor/signing.go
  - internal/preflight/signing.go
  - internal/sbxdrv/env_signing.go
  - e2e/signing_test.go
needs:
  - T025
  - T081
  - T043
  - T046
  - T049
after: []
ask_first:
  - sbxdrv
  - formats
sitting: none
---

# T080 - The signing rule

Goal: The signing rule: `sync` refuses the kit without the one-key
socket, `doctor`'s signing row, `SSH_AUTH_SOCK` on the project's sbx
calls, as one file each in those packages.

Re-plan (2026-10-12): Kept, pulled forward: the signing kit (T081, M2)
is in daily use before romeu renders it, so the kit lands first, this
task needs it, and no `sync` of the maintainer's renders it before this
task because the slots of sitting M3 (T116) wait for this task. The e2e
uses the real kit, not a fixture of its name. Between M2 and this task
the rule holds by hand: the dedicated signing-only socket loaded with
`ssh-add -c`, and the `ssh-add -L` read of T115 in every recreated
sandbox (the I34 row accepted by name in 05 5.4, linked from Accepted
risks in `docs/plan.md`); that row closes when this task lands. Its four
files plug into the named seams of rule 14: `doctor/rows.go`,
`preflight/step5.go`, `sync/rules.go` and `sbxdrv/env.go`'s `register`.
The `sbxdrv` forecast is for `env_signing.go`.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-0296 (04 4.1) now admits `gh` in `upgrade` and `doctor`; nothing
changes for the signing rule.

- Implements: REQ-0075 (01 1.5), REQ-0296 (04 4.1), REQ-0384 (04 4.2),
  REQ-0520 (05 5.2), REQ-0549 (05 5.4), REQ-0600 (06 6.4), REQ-0601 (06
  6.4).
- Operator: no.
- Acceptance:
  - `sync` refuses to render a kit named `git-ssh-sign` with exit 2 and
    `RJ-204` unless the socket is set, reachable and holds one key equal
    to `signingKey`. The rule is keyed by the kit's name; the kit (T081,
    M2) lands first and this task needs it, so the e2e uses the real
    kit, and no `sync` renders that kit before this task lands (sitting
    M3 waits for it). No merge forwards the host's whole agent in
    between: the host socket is the dedicated signing-only one.
  - git-ssh-sign's capability set appears in the gate 2 diff of a
    project that adds it, and the rule holds on the first `sync` that
    renders it.
  - `doctor` and the preflight of that project's **P** commands run the
    same check.
  - For a project that uses the kit, the `sbx` calls carry
    `SSH_AUTH_SOCK=<signing.agentSocket>` and no other project's do.
- Verify: `go test -tags e2e ./e2e/... -run '^TestSigning' -v`. The
  pattern names the `TestSigning*` functions this task adds, and the
  reviewer reads them in the `-v` output (10 10.8).
