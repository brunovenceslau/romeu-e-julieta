---
id: T111
title: "The bridge: julieta by hand in every daily sandbox"
milestone: M1
module: none
fate: new
owns: []
needs:
  - T051
  - T054
  - T059
  - T060
  - T078
after: []
ask_first: []
sitting: M1
---

# T111 - The bridge: julieta by hand in every daily sandbox

Goal: Per project definition: a read-only mount of one shared bin
directory holding the host-built linux julieta and `SHA256SUMS`, a mount
of the memory directory in its final host layout, and one environment
variable holding the hand-written manifest; an interim personal kit, in
the format the pinned frontend accepts, that installs the SessionStart
and SessionEnd hook lines of 08 8.2 (each a wrapper that prints one
warning line and exits 0 when the mount is absent, so a skipped hook is
never silent; the SessionStart one runs `julieta setup` before `handoff
show --hook`, the order `romeu run` keeps in 04 4.2, so the dispatcher
is the global `core.hooksPath` before the session's first commit or
push) and the two skills pinned from this repository at a SHA; a
runbook with each step and its undo.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. Each step is
an ask-first change there, under that repository's own rules, since it
changes what enters every sandbox (rule 9 of `docs/plan.md`).

Re-plan (2026-10-12): The bridge is outside the product: nothing of it
enters a spec section or a product command. T079 and T114 remove the
interim kit (Decisions this plan defers).

- Implements: 08 8.2; 03 3.6 (the manifest format, read as
  `docs/notes/julieta-by-hand.md` says).
- Operator: applies the three edits and the host build, two projects
  first, then every project; then one week of daily use.
- Acceptance:
  - `julieta handoff show --hook` prints in each daily sandbox.
  - `julieta status` exits 0 in each.
  - A commit in a daily sandbox runs `.githooks/pre-commit` through the
    dispatcher.
  - A test push from each daily sandbox, on a throwaway branch, runs
    both push hooks through the dispatcher: the repository's tracked
    `.githooks/pre-push` and the names gate of the config repository
    (T112); the hook output shows both, before any other hook wiring
    of that sandbox is changed.
  - With the bin mount absent, each wrapper prints its warning line
    and exits 0.
  - The runbook holds the undo of each step.
- Verify: in each daily sandbox, read at sitting M1: `julieta status
  --json` exits 0 and lists the manifest's repositories; `julieta
  handoff show --hook` prints the built-in memory line of 08 8.1 at
  least; the test push's hook output names both hooks.
