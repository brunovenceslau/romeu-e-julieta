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
  - T111
after: []
ask_first: []
sitting: M2
---

# T115 - Recreate wave 1

Goal: One project per slot: `/handoff --final`, push, `julieta snapshot
--all`, remove, recreate with the product kits and the personal kit in
the hand-written project definition. The fifth edit of rule 9 lands
here: the kit entries with their args, `julietaBin` set to the bridge's
bin mount path and `signingKey` to the public key, each with its undo in
the runbook; `romeu sync` writes both at sitting M3 (T116). `julieta
install` and `lock` (T053) are in the wave, so each recreated repository
gets its `mise.toml` and `mise.lock` (the M2 row of `docs/plan.md`). J6
steps 1, 2 and 5 run by hand, with `julieta snapshot --all` in place of
the salvage of step 3, which waits for T068.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The wave's
definition edits land as one pull request of the config repository,
applied by a script of that repository that takes the project name, so
the wave is one approval and one undo; each is an ask-first change
there, under that repository's own rules, since the kit entries and
their args enter every sandbox (rule 9 of `docs/plan.md`). The pull
request whose Evidence carries sitting M2's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

- Implements: REQ-0548 (05 5.4), REQ-0549 (05 5.4), REQ-0602 (06 6.4),
  REQ-1002 (11 11.2).
- Operator: runs the slots as part 2 of sitting M2.
- Acceptance:
  - Every daily sandbox is built from the `julieta`, `julieta-claude`
    and `git-ssh-sign` product kits plus the personal kit.
  - The runbook holds the kit entries, their args and the undo of each.
  - In each recreated sandbox `ssh-add -L` lists exactly one key, the
    `signingKey` of the definition, and the host socket it reaches is
    the dedicated signing-only agent loaded with `ssh-add -c`, so each
    signature asks for confirmation: the by-hand form of I34 until T080
    lands (the I34 row accepted by name in 05 5.4, linked from Accepted
    risks in `docs/plan.md`).
  - Read at sitting M2 and recorded with `source: sitting M2`: what a
    commit in a recreated sandbox does when the confirmation is not
    answered within the agent's timeout (fails with no object written,
    or waits), and for how long; the I34 line of the plan's Accepted
    risks names that outcome and the away window it covers.
  - No change to the personal kit's instructions, settings merge or
    plugin pins lands between the first and the last slot of the wave; a
    change found needed waits for the wave's end.
  - The loss window (uncommitted work since the last snapshot) is an
    accepted risk until M4 (Accepted risks).
- Verify: in each recreated sandbox, read at sitting M2: `julieta status
  --json` exits 0, and `ssh-add -L` prints one line, the signing key.
