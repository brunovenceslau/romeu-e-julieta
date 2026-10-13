---
id: T104
title: "The plan as task files"
milestone: M0
module: docs
fate: merged
owns:
  - docs/plan.md
  - docs/plan/**
  - docs/notes/julieta-by-hand.md
needs: []
after:
  - T121
ask_first: []
sitting: none
merged: 2026-10-13
pr: 43
---

# T104 - The plan as task files

Merged on 2026-10-13 in pull request #43. The text below is the one
written by the re-plan.

Goal: `docs/plan.md` rewritten as the narrative; `docs/plan/tasks/` with
one file per task, a stub per folded or removed id and a file with
`merged:` and `pr:` filled for each merged id; and
`docs/notes/julieta-by-hand.md`. The tree row of 02 2.1 that names the
directory was applied on 2026-10-12 (ADR 0012; T122).

Re-plan (2026-10-12): This is the branch `docs/replan` itself, rebased
on the pass that placed the requirement ids; `after: T121`, whose
coverage half reads these task files, and no code: the check that
reads the files is T121's.

- Implements: REQ-0829 (10 10.2), REQ-1089 (12 12.2), REQ-1090 (12
  12.2), REQ-1101 (12 12.2).
- Operator: no
- Acceptance:
  - Every id of the earlier plan and of the re-plan has one file under
    `docs/plan/tasks/`; every file's front matter parses.
  - The narrative states the rules of "How the work is done", the
    milestones with what each delivers, the deferred decisions and the
    accepted risks, and names the 2026-10-01 answer to plan question 1
    as reopened by the re-plan.
  - Every live task's Implements line cites requirement ids, resolved
    from the allocated specification by a script and never by hand.
  - `docs/notes/julieta-by-hand.md` reads "in the kit format the daily
    sandboxes accept today" for the interim kit.
  - No file of this repository holds a name of the hashed denylist
    (index, Boundaries): a capability is named by what it does.
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/ci reqs`
  exits 0; every file under `docs/plan/tasks/` parses and the ids are a
  gapless sequence from T001, checked by a one-off script outside the
  tree that reads each front matter as YAML (T106 then makes that
  `tools/ci plan`).
