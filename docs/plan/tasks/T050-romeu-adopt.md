---
id: T050
title: "`romeu adopt`"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T047
after: []
ask_first:
  - contracts
  - gates
sitting: none
---

# T050 - `romeu adopt`

Re-plan (2026-10-12): deferred to M6; memory leaves the sandbox in M1,
so a recreate loses nothing salvage cannot capture; the preflight
refuses a sandbox romeu has no generation for, and the maintainer
removes it by hand. Reopened by: the first host-state loss, or the first
sandbox created outside romeu that must be kept. The text below is the
one written before the re-plan; `needs` lists its earlier dependencies
with a folded id replaced by the id that absorbed it, and `owns` is
written when the task reopens.

- Implements: 04 4.2 (`adopt`), 01 1.6 ("Generation"), 05 5.2 (I26).
- Operator: no.
- Ask-first, as forecast before the re-plan: `contracts`, `gates`.
- Acceptance:
  - `adopt` without a terminal, with a workspace path that differs, with
    an open generation or with no such sandbox exits 2; with a stale
    toolchain it exits 3.
  - After `adopt` the record has `adopted: true` and an unknown recreate
    digest, and the preflight of T046 and `sync` step 9 accept the
    sandbox they refused before.
  - That `salvage` and `rm` succeed after `adopt` is tested in T068.
- Verify: `go test -tags e2e ./e2e/...`.
