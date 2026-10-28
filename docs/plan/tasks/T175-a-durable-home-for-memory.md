---
id: T175
title: "a durable home for memory"
milestone: M5
module: memstore
fate: deferred
owns: []
needs:
  - T174
after: []
ask_first:
  - gitsafe
  - spec
sitting: none
---

# T175 - a durable home for memory

Deferred on 2026-10-28, the day it was written: it is the row "a
durable home for memory" of the index's Deferred decisions, and Q14.
Reopened by: the first project that moves its state into memory, which
is this repository at the first `handoff` entry of T174; or Q14 at the
first release-candidate tag, whichever comes first. `owns` is written
when it reopens.

The design the row records: a private git repository per project for
`<name>-env/memory/`, its `.git` kept by the host outside every mounted
directory (`--separate-git-dir`), committed and pushed by romeu on the
host after each `sync`, `run`, `pull` and preservation, never by a
sandbox, so the layout allowlist of 08 8.1 still refuses a `.git`
inside a memory dir and the sandbox sees plain files. The ADR that
decides it amends ADR 0021's "machine-local in v1".

- Implements: none: the row of the index and Q14 are its trace until it
  reopens.
- Operator: decides the repository's host and name, and the host git
  identity that commits.
- Acceptance: written when it reopens.
- Verify: written when it reopens.
