---
id: T112
title: "The names gate in the config repository"
milestone: M1
module: none
fate: new
owns: []
needs: []
after: []
ask_first: []
sitting: M1
---

# T112 - The names gate in the config repository

Goal: The deterministic forbidden-names gate that the maintainer's
sandboxes run on every push, with its tests, and the pre-push wiring of
every sandbox runs it through the hook dispatcher once `julieta setup`
has installed it (T054). The mechanism is (a), decided on 2026-10-12
(item 18 of Decisions of the re-plan): each guarded repository
carries its own hashed denylist and runs the hygiene check of 10 10.2
from its tracked `.githooks/pre-push`, as this repository does, so no
private file reaches any sandbox and the dispatcher's tracked link
carries it; the config repository keeps the plain list and a one-off
command that writes a guarded repository's entries from it (the `hygiene
add` form, run in a terminal on the host); if the hygiene check cannot
run outside this repository without a `checks` change, the task names
that change and its approval line. The alternative not taken was (b),
the config repository as a secondary repo of every project (02 2.6),
its plain list cloned into every sandbox. The deferred row "a Go port
of the names gate of the config repository" closes when this task
lands. The phrase
list of the lesson of 2026-10-12 (pending item aea4a5dd; its phrases
stay in the private home, never written here) joins the same private
home and the same gate as a third list.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The step is an
ask-first change there, under that repository's own rules, since it
changes what guards every push (rule 9 of `docs/plan.md`). The pull
request whose Evidence carries sitting M1's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): No dependency and an irreversible-harm surface, so
it goes first; the second verification, after sitting M1 through the
dispatcher, is T111's.

- Implements: none (12 12.11: the names gate of the config repository is
  no obligation of this specification).
- Operator: wires the gate with mechanism (a) and runs the
  verification.
- Acceptance:
  - A push from a sandbox runs the gate, before sitting M1 through the
    pre-push wiring.
  - A push of a commit that holds a planted made-up name, added to the
    gate's list for the test, is refused, and the hook output names the
    gate.
  - The entries of each guarded repository's denylist cover every name
    of the config repository's list, measured by hashing each name's
    segments and comparing, and the measurement is in the runbook
    (pending item 67fe3422's condition for running one gate).
  - A push from a rendered sandbox (sitting M3) of a commit with the
    planted name is refused by the tracked hook.
- Verify: from a sandbox, one push of a throwaway branch that holds the
  planted name exits non-zero with the gate's refusal in the hook
  output, and the same push without it exits 0, read before sitting M1;
  the read after sitting M1 is T111's.
