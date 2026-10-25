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
  - T035
  - T079
after: []
ask_first: []
sitting: M1
---

# T111 - The bridge: julieta by hand in every daily sandbox

Goal: Per project definition: a read-only mount of one shared bin
directory holding the host-built linux julieta and `SHA256SUMS`, a mount
of the memory directory in its final host layout, and one environment
variable holding the hand-written manifest, encoded with `go run
./tools/manifest encode` (its `repos[].base` the
origin SHA each clone started from, 03 3.6, so a snapshot bundles only
the work since it; its `generation` one ULID per project, kept in the
runbook), with beside it the entry of the `julieta-claude` kit of T079,
pinned at a SHA of this repository and its `julietaBin` arg set to the
bin mount's path, listed after the sandbox agent payload's kit so its
skills are the ones installed; the kit brings the SessionStart and
SessionEnd hook lines of 08 8.2 (each a wrapper that prints one warning
line and exits 0 when the mount is absent, so a skipped hook is never
silent), the PreToolUse guard of `julieta hooks guard`, the agent's
own memory directory made non-writable, and the two skills of
`skills/`; once per created sandbox, a
runbook step runs `julieta setup` before the first session, the order
`romeu run` keeps in 04 4.2, so the dispatcher is the global
`core.hooksPath` before the session's first commit or push; a fourth
edit that
removes the store's writer kit and the mount of the store from the
definition; once, outside the definitions, the removal of the inbox
skill from the sandbox agent payload, with its own undo, a global change
that lands before the first wave and reaches every sandbox at once. In a
daily sandbox the julieta skill of T078 (`julieta memory add`) is then
the route for anything worth keeping and the handoff skill the route for
handoffs; a runbook with each step and its undo. The open-items
cut-over, per project: the one-off entry of the store's open items into
`julieta memory` is a command of the config repository that writes each
item's source id as a tag of its entry (`source-` and the id's
timestamp digits and hash, since a store id is not of the tag form of
03 3.9; 03 3.9 gives tags no behaviour but `lesson`), skips an item whose tag is already present
and prints the counts on both sides, with one recorded list that gives
each item its destination (a memory entry id, or closed); it runs again
at the end of the sitting, after the store's writer has left every
sandbox (the fourth edit and the payload change), and both counts are in
the runbook. The host build of the store's tool stays a writer of the
store, outside every sandbox, so T119's closing read still matters.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. Each step is
an ask-first change there, under that repository's own rules, since it
changes what enters every sandbox (rule 9 of `docs/plan.md`). The pull
request whose Evidence carries sitting M1's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): The bridge is outside the product: nothing of it
enters a spec section or a product command. T079 and T114 were to
remove an interim kit; the re-plan of 2026-10-23 below drops it. From sitting M1 on julieta is
the one path for new items, and the count-verified entry of the store's
open items is this task's (item 19 of Decisions of the re-plan, amended
on 2026-10-13); T119 reads at sitting M5 that the store holds no item
that the open-items list of sitting M1 does not name.

Re-plan (2026-10-23): No interim kit. The skills of `skills/` are the
official ones and the `julieta-claude` kit of T079 carries them and the
hook lines into the daily sandboxes from sitting M1 on, so this task
needs T079; the sandbox agent payload's own `handoff` skill is not kept
in step with them (Decisions this plan defers). The first wave is
romeu-e-julieta and verona; T112's sandbox gate lands during the week of
daily use, before the end-of-sitting read. Wave 1 reads first that a
daily sandbox's definition loads the `julieta-claude` kit at all, since
no daily sandbox has loaded a kit of the pinned frontend yet; if it
does not, this task stops with a decision record before any other step.

- Implements: REQ-0659 (08 8.1), REQ-0673 (08 8.2).
- Operator: applies the four edits and the host build, two projects
  first, then every project; runs the one-off entry of the open items
  per project; one week of daily use; runs the entry again at the end of
  the sitting. Before the first wave, removes the inbox skill once from
  the sandbox agent payload, which lives outside this repository and the
  config repository's definitions: an ask-first change of the repository
  that holds it, one approval, with its undo in the runbook. Rebuilds
  the shared bin directory at each sitting and whenever a daily sandbox
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
  - The runbook holds the undo of each step, the fourth edit's and the
    payload change's included.
  - No daily sandbox holds the store's writer kit, the mount of the
    store or the inbox skill after the fourth edit and the payload
    change.
  - One list per project, each open item of the store with its
    destination: a memory entry id, or closed.
  - The entry command skips an item whose `source-` tag is present
    and prints the counts on both sides; both counts, from the first run
    and the end-of-sitting run, are in the runbook.
- Verify: in each daily sandbox, read at sitting M1: `gh pr merge` is
  refused by the guard with the route in its text; `julieta status
  --json` exits 0 and lists the manifest's repositories; `julieta
  handoff show --hook` prints the built-in memory line of 08 8.1 at
  least; the test push's hook output names both hooks; `julieta version
  --json` in each daily sandbox names the SHA the runbook's last rebuild
  line names; `julieta memory list` in each project's sandbox shows the
  entries the open-items list names; the runbook holds the two count
  pairs of each project, from the first run and the end-of-sitting run;
  in each daily sandbox, the store's writer kit, the mount of the store
  and the inbox skill are absent.
