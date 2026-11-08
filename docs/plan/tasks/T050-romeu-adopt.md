---
id: T050
title: "`romeu adopt`"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T047
  - T068
after: []
ask_first:
  - contracts
  - gates
sitting: none
---

# T050 - `romeu adopt`

Narrowed on 2026-11-08: a stopped sandbox whose recreate digests
differ is now started with `sbx env exec <dir> -- true`, and refused
only for an adopted generation or, by `run` and `pull`, when the spec
removed a kit or a secrets binding since the create (04 4.2, REQ-1387,
REQ-1405, REQ-1406). T217 builds it; where this task's text below says
such a sandbox is refused, T217's text wins.

Re-plan (2026-10-12): deferred to M6; memory leaves the sandbox in M1,
so a recreate loses nothing salvage cannot capture; the preflight
refuses a sandbox romeu has no generation for, and the maintainer
removes it by hand. Reopened by: the first host-state loss, or the first
sandbox created outside romeu that must be kept; or the first release
candidate. The text below is the one written before the re-plan; `needs`
lists its earlier dependencies with a folded id replaced by the id that
absorbed it, and `owns` is written when the task reopens, with its
import file at `internal/cmd/romeu/all/import_adopt.go`.

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): `adopt` warns before its prompt, running or
stopped, that once the adopted sandbox stops, for any cause, no romeu
command can start, salvage or remove it until T210 lifts the refusal,
and that its disk persists; on a stopped sandbox it says this is already
the case (REQ-0312; REQ-0553's reason narrowed to a running sandbox).
`adopt` is not refused. On the adopted generation `run` never hands sbx
romeu's env file and refuses a stopped sandbox (REQ-1378), `salvage`,
`rm`, `recreate`, `retire` and `pull` refuse a stopped one (REQ-1387),
the mounts check is skipped whole and `status` prints its mounts
(REQ-1383), and `status` and `sync` warn that it was adopted. The last
acceptance line below therefore holds on a running adopted sandbox;
when this task reopens it cites REQ-1378 and REQ-1383 for its cases and
adds the warning's case, with the sandbox running and stopped, to its
acceptance.

- Implements: REQ-0090 (01 1.6), REQ-0233 (03 3.8), REQ-0312 (04 4.2),
  REQ-0511 (05 5.2), REQ-0553 (05 5.4), REQ-0790 (10 10.1).
- Operator: no.
- Ask-first, as forecast before the re-plan: `contracts`, `gates`.
- Acceptance:
  - `adopt` without a terminal, with a workspace path that differs, with
    an open generation or with no such sandbox exits 2; with a stale
    toolchain it exits 3. The prompt goes to stderr, and stdin decides
    the TTY.
  - After `adopt` the record has `adopted: true` and an unknown recreate
    digest, and the preflight of T046 and `sync` step 9 accept the
    sandbox they refused before.
  - After `adopt`, `salvage` and `rm` succeed on the adopted sandbox
    (I26).
- Verify: `go test -tags e2e ./e2e/...`.
