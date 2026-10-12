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
sandboxes run on every push lives in the config repository, with its
tests, and the pre-push wiring of every sandbox runs it from there
through the hook dispatcher once `julieta setup` has installed it
(T054). A Go port is deferred with a trigger.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The step is
an ask-first change there, under that repository's own rules, since it
changes what guards every push (rule 9 of `docs/plan.md`).

Re-plan (2026-10-12): No dependency and an irreversible-harm surface, so
it goes first, and it is verified again after sitting M1, once the
dispatcher is the global `core.hooksPath` in every sandbox (T111): a
sandbox whose wiring changed under it would otherwise lose the gate
without a word.

- Implements: none (this repository keeps its own hashed denylist, 10
  10.2).
- Operator: wires the gate and runs the two verifications.
- Acceptance:
  - A push from a sandbox runs the gate, before sitting M1 through the
    pre-push wiring and after it through the dispatcher.
  - A push of a commit that holds a planted made-up name, added to the
    gate's list for the test, is refused, and the hook output names
    the gate.
- Verify: from a sandbox, one push of a throwaway branch that holds the
  planted name exits non-zero with the gate's refusal in the hook
  output, and the same push without it exits 0; both reads taken
  before and again after sitting M1.
