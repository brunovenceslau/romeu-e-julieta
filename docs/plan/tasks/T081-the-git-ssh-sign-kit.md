---
id: T081
title: "The `git-ssh-sign` kit"
milestone: M2
module: kits
fate: kept
owns:
  - kits/git-ssh-sign/**
  - e2e/kit_git_ssh_sign_test.go
needs:
  - T076
  - T113
after: []
ask_first:
  - kits
  - signing
sitting: none
---

# T081 - The `git-ssh-sign` kit

Goal: The `git-ssh-sign` kit: the descriptor, the git config files, the
allowed-signers file from the `signingKey` arg.

Re-plan (2026-10-12): Kept, pulled forward. The trust-root role moves
into that arg, which until sitting M3 is set in the project definition
by hand, the fourth edit of rule 9 (T115), and `romeu sync` writes
afterwards. Its gate 2 appearance is asserted when T043 lands.
Acceptance added: a commit in a daily sandbox is signed with the one key
and GitHub shows it verified. Until T080 lands, I34 is enforced by no
code: the host socket is the dedicated signing-only agent loaded with
`ssh-add -c`, and T115 reads `ssh-add -L` in every recreated sandbox
(the I34 row of Accepted risks in `docs/plan.md`). The socket fact of
sitting M2 (T113) lands here as
`kits/git-ssh-sign/testdata/sitting-m2.yaml`, with `source: sitting
M2`; this task needs T113 for it. Its kit e2e file is
`e2e/kit_git_ssh_sign_test.go` (rule 1).

- Implements: 06 6.4.
- Operator: the dedicated signing-only agent socket on the host.
- Acceptance:
  - The kit declares the ssh-agent capability and writes
    `gpg.format=ssh`, `commit.gpgsign=true`, `user.signingkey` and the
    allowed-signers file from the `signingKey` arg.
  - Its capability set appears in the gate 2 diff of a project that adds
    it, and the rule of T080 holds on the first `sync` that renders it.
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/... -run
  '^TestKitGitSSHSign' -v`. The pattern names the `TestKitGitSSHSign*`
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).
