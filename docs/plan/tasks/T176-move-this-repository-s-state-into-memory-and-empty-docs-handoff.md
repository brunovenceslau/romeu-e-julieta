---
id: T176
title: "move this repository's state into memory and empty docs/handoff"
milestone: M3
module: none
fate: new
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

Goal: this repository's state leaves `docs/handoff/` for julieta
memory, once the durable copy of T175 is in use for this project and
the handoff type of T174 and the index of T172 are in its sandbox
(ADR 0042, ADR 0043). Owns `docs/handoff/**` and `AGENTS.md` when it
starts; the payload's handoff checker is the kit repository's.

What it does: `now.md` becomes the open `handoff`
entry; each note of `pending/` a `project` entry whose first body line
is `reopen when: ...`; each `gotchas/` note a `feedback` or `reference`
entry; `decisions/` notes that are the product's become ADRs and the
rest `project` entries; `done/` stays as the history of closed
handoffs. `AGENTS.md` points at `julieta memory index` instead of
`docs/handoff/now.md`.

- Implements: none: a documentation move.
- Operator: yes, the public repository loses its handoff tree.
- Acceptance:
  - Every open pending note and gotcha is an entry (`julieta memory
    list --status open` names each by title); `now.md`'s next steps and
    operator items are the open handoff; `docs/handoff/` holds only
    `about.md`, which says where the state went, and `done/`.
  - `AGENTS.md` names `julieta memory index` as the current state, and
    `tools/ci docs` passes.
- Verify: `julieta memory index`, `go run ./tools/ci docs`.
