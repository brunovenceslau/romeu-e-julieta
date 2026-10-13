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
variable holding the hand-written manifest (its `repos[].base` the
origin SHA each clone started from, 03 3.6, so a snapshot bundles only
the work since it; its `generation` one ULID per project, kept in the
runbook); an interim personal kit, in the kit format the daily sandboxes
accept today (the pinned frontend of 06 6.1 is first exercised at the
kit-frontend go/no-go, T113), that installs the SessionStart and
SessionEnd hook lines of 08 8.2 (each a wrapper that prints one warning
line and exits 0 when the mount is absent, so a skipped hook is never
silent; the SessionStart one runs `julieta setup` before `handoff show
--hook`, the order `romeu run` keeps in 04 4.2, so the dispatcher is the
global `core.hooksPath` before the session's first commit or push) and
the two skills pinned from this repository at a SHA; a runbook with each
step and its undo.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. Each step is
an ask-first change there, under that repository's own rules, since it
changes what enters every sandbox (rule 9 of `docs/plan.md`). The pull
request whose Evidence carries sitting M1's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): The bridge is outside the product: nothing of it
enters a spec section or a product command. T079 and T114 remove the
interim kit (Decisions this plan defers). The count-verified entry of
the store's open items is T119's, unless the operator moves the cut-over
to M1 (item 19 of Decisions open at the re-plan).

- Implements: REQ-0659 (08 8.1), REQ-0673 (08 8.2).
- Operator: applies the three edits and the host build, two projects
  first, then every project; then one week of daily use. Rebuilds the
  shared bin directory at each sitting and whenever a daily sandbox
  needs a merged fix, with a cross-build of `main` on the host (`go
  build` for linux, or T103's `--dry-run` build once it lands), its
  sha256 in `SHA256SUMS`; each rebuild is one runbook line with the SHA
  of `main`. The edits of a wave land as one pull request of the config
  repository, applied by a script of that repository that takes the
  project name, so the wave is one approval and one undo.
- Acceptance:
  - `julieta handoff show --hook` prints in each daily sandbox.
  - `julieta status` exits 0 in each.
  - A commit in a daily sandbox runs `.githooks/pre-commit` through the
    dispatcher.
  - A test push from each daily sandbox, on a throwaway branch, runs
    both push hooks through the dispatcher: the repository's tracked
    `.githooks/pre-push` and the names gate of the config repository
    (T112); the hook output shows both, before any other hook wiring of
    that sandbox is changed. After sitting M1 the names gate runs
    through the dispatcher on a test push, read again.
  - In each daily sandbox a snapshot after one commit holds that commit
    and not the history reachable from `base`, read with `git bundle
    list-heads`, the bundle's size in the runbook.
  - With the bin mount absent, each wrapper prints its warning line and
    exits 0.
  - The runbook holds the undo of each step.
- Verify: in each daily sandbox, read at sitting M1: `julieta status
  --json` exits 0 and lists the manifest's repositories; `julieta
  handoff show --hook` prints the built-in memory line of 08 8.1 at
  least; the test push's hook output names both hooks; `julieta version
  --json` in each daily sandbox names the SHA the runbook's last rebuild
  line names.
