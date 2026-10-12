---
id: T115
title: "Recreate wave 1"
milestone: M2
module: none
fate: new
owns: []
needs:
  - T114
  - T081
  - T077
  - T053
after: []
ask_first: []
sitting: M2
---

# T115 - Recreate wave 1

Goal: One project per slot: `/handoff --final`, push, `julieta snapshot
--all`, remove, recreate with the product kits and the personal kit in
the hand-written project definition. The fourth edit of rule 9 lands
here: the kit entries with their args, `julietaBin` set to the bridge's
bin mount path and `signingKey` to the public key, each with its undo
in the runbook; `romeu sync` writes both at sitting M3 (T116). `julieta
install` and `lock` (T053) are in the wave, so each recreated repository
gets its `mise.toml` and `mise.lock` (the M2 row of `docs/plan.md`).

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. Each
definition edit is an ask-first change there, under that repository's
own rules, since the kit entries and their args enter every sandbox
(rule 9 of `docs/plan.md`).

- Implements: 09 (J6 and J10, by hand); 06 6.4.
- Operator: runs the slots as part 2 of sitting M2.
- Acceptance:
  - Every daily sandbox is built from the `julieta`, `julieta-claude`
    and `git-ssh-sign` product kits plus the personal kit.
  - The runbook holds the kit entries, their args and the undo of each.
  - In each recreated sandbox `ssh-add -L` lists exactly one key, the
    `signingKey` of the definition, and the host socket it reaches is
    the dedicated signing-only agent loaded with `ssh-add -c`, so each
    signature asks for confirmation: the by-hand form of I34 until T080
    lands (the I34 row of Accepted risks in `docs/plan.md`).
  - The loss window (uncommitted work since the last snapshot) is an
    accepted risk until M4 (Accepted risks).
- Verify: in each recreated sandbox, read at sitting M2: `julieta
  status --json` exits 0, and `ssh-add -L` prints one line, the signing
  key.
