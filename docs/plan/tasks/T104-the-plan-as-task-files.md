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
  - tools/ci/reqs.go
  - tools/ci/reqs_test.go
needs: []
after:
  - T121
ask_first:
  - checks
sitting: none
---

# T104 - The plan as task files

Goal: `docs/plan.md` rewritten as the narrative; `docs/plan/tasks/` with
one file per task, a stub per folded or removed id and a file with
`merged:` and `pr:` filled for each merged id;
`docs/notes/julieta-by-hand.md`; and the coverage half of `tools/ci
reqs` over `docs/plan/tasks/*.md`: every Implements entry a live id,
every live id cited by a task of fate `kept`, `new`, `deferred` or
`merged`, the deferred ones reported apart; a left-v1, folded or removed
task is not read, as 12 12.11 and the `reqs` row of 10 10.2 say. The
tree row of 02 2.1 that names the directory was applied on 2026-10-12
(ADR 0012; T122).

Re-plan (2026-10-12): This is the branch `docs/replan` itself, rebased
on the pass that placed the requirement ids; `after: T121`, which holds
the spec half in the same two files, and `checks`, since the coverage
half is a check.

- Implements: REQ-0829 (10 10.2), REQ-1082 (12 12.2), REQ-1089 (12
  12.2), REQ-1090 (12 12.2), REQ-1101 (12 12.2), REQ-1201 (12 12.11).
- Operator: no
- Acceptance:
  - Every id T001 to T138 has one file under `docs/plan/tasks/`; every
    file's front matter parses.
  - The narrative states the rules of "How the work is done", the
    milestones with what each delivers, the deferred decisions and the
    accepted risks, and names the 2026-10-01 answer to plan question 1
    as reopened by the re-plan.
  - Every live task's Implements line cites requirement ids, resolved
    from the allocated specification by a script and never by hand.
  - The coverage half has its cases: "an Implements entry whose id is
    retired or unknown", "a live id that no task cites", "a folded task
    is not read", "the coverage half is silent without task files", "a
    deferred-only id is reported and passes", "an Implements line that
    wraps is read whole", "an untracked task file does not satisfy the
    coverage half" and "a task file without a fate is left to the plan
    check"; the tree passes.
  - `docs/notes/julieta-by-hand.md` reads "in the kit format the daily
    sandboxes accept today" for the interim kit.
  - No file of this repository holds a name of the hashed denylist
    (index, Boundaries): a capability is named by what it does.
- Verify: `go run ./tools/ci all` exits 0; `go run ./tools/ci reqs`
  exits 0; `ls docs/plan/tasks | wc -l` prints 138; a one-off script
  outside the tree reads the front matter of every task file as YAML and
  exits 0 (T106 then makes that `tools/ci plan`).
