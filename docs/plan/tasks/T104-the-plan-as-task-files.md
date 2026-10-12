---
id: T104
title: "The plan as task files"
milestone: M0
module: docs
fate: new
owns:
  - docs/plan.md
  - docs/plan/**
  - docs/notes/julieta-by-hand.md
needs: []
after: []
ask_first: []
sitting: none
---

# T104 - The plan as task files

Goal: `docs/plan.md` rewritten as the narrative; `docs/plan/tasks/` with
one file per task, a stub per folded or removed id and a file with
`merged:` and `pr:` filled for each merged id;
`docs/notes/julieta-by-hand.md`. The tree row of 02 2.1 that names the
directory is amendment 1, carried by T108.

Re-plan (2026-10-12): This is the branch `docs/replan` itself.

- Implements: 02 2.1; 12 12.2 (Re-planning).
- Operator: no
- Acceptance:
  - Every id T001 to T120 has one file under `docs/plan/tasks/`; every
    file's front matter parses.
  - The narrative states the rules of "How the work is done", the
    milestones with what each delivers, the deferred decisions and the
    accepted risks, and names the 2026-10-01 answer to plan question 1
    as reopened by the re-plan.
  - No file of this repository holds a name of the hashed denylist
    (index, Boundaries): a capability is named by what it does.
- Verify: `go run ./tools/ci all` exits 0; `ls docs/plan/tasks | wc -l`
  prints 120; a one-off script outside the tree reads the front matter
  of every task file as YAML and exits 0 (T106 then makes that
  `tools/ci plan`).
