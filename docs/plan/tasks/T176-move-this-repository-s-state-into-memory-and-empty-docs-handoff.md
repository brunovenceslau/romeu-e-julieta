---
id: T176
title: "move this repository's state into memory and empty docs/handoff"
milestone: M5
module: none
fate: deferred
owns: []
needs:
  - T175
  - T174
  - T172
after: []
ask_first: []
sitting: none
---

# T176 - move this repository's state into memory and empty docs/handoff

Deferred on 2026-10-28, the day it was written, by the row "a durable
home for memory" of the index: a project that moves its state into
memory keeps a versioned copy in its repository until memory has a
durable home, and no task removes that copy before then. Reopened by:
T175 merged and in use for this project. `owns` is written when it
reopens (`docs/handoff/**`, `AGENTS.md`, the payload's handoff checker
is the kit repository's).

What it does when it reopens: `now.md` becomes the open `handoff`
entry; each note of `pending/` a `project` entry whose first body line
is `reopen when: ...`; each `gotchas/` note a `feedback` or `reference`
entry; `decisions/` notes that are the product's become ADRs and the
rest `project` entries; `done/` stays as the history of closed
handoffs. `AGENTS.md` points at `julieta memory index` instead of
`docs/handoff/now.md`.

- Implements: none: a documentation move.
- Operator: yes, the public repository loses its handoff tree.
- Acceptance: written when it reopens.
- Verify: written when it reopens.
