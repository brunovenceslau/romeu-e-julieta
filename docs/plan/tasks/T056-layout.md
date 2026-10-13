---
id: T056
title: "`layout`"
milestone: M6
module: layout
fate: deferred
owns: []
needs:
  - T021
  - T051
  - T012
  - T026
after: []
ask_first: []
sitting: none
---

# T056 - `layout`

Re-plan (2026-10-12): deferred to M6; T066 lands a minimal `julieta
layout up` that execs a login shell, so `romeu run` attaches a single
pane until this task replaces its body; the `julieta` kit ships herdr as
06 6.4 says. Reopened by: the first project whose daily work needs two
panes at start; or the first release candidate. Its ids `RJ-319` and
`RJ-320` leave `internal/cli/errorids_exempt.yaml` when it lands. The
text below is the one written before the re-plan; `needs` lists its
earlier dependencies with a folded id replaced by the id that absorbed
it, and `owns` is written when the task reopens, with its import file at
`internal/cmd/julieta/all/import_layout.go`.

- Implements: REQ-0090 (01 1.6), REQ-0196 (03 3.2), REQ-0297 (04 4.1),
  REQ-0306 (04 4.1), REQ-0358 (04 4.2), REQ-0418 (04 4.3), REQ-0419 (04
  4.3), REQ-0696 (08 8.5), REQ-0732 (10 10.1).
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - `layout up --dry-run --json` matches the golden `layout.apply`
    requests for the example run layout.
  - `pane run` runs each step as `mise exec -C <pane dir> -- <argv>`,
    with no timeout, stops at a failing step with its status and drops
    to a login shell; a step a signal ended is reported as `exited
    128+<signal>`.
  - `layout up` applies against herdr at the version and hash of the pin
    file, in the container, through `internal/procgroup` with the layout
    class of the table of 04 4.1; it checks the socket protocol number
    against the one A13 recorded (T026), fails closed on another (Q16),
    and reports "layout stale" when the `run` digest differs. The number
    comes from the probe, not from the pinned binary, which would prove
    nothing.
- Verify: `go test -tags e2e ./e2e/...`.
