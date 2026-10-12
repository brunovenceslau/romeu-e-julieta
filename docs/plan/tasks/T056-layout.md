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

Re-plan (2026-10-12): deferred to M6; `romeu run` attaches a single pane
until this lands (04 4.2 step 7 is reached by no command); the `julieta`
kit ships herdr as 06 6.4 says. Reopened by: the first project whose
daily work needs two panes at start. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 03 3.2 (the execution rule), 04 4.3 (`layout up`, `pane
  run`), 10 10.1 (Golden), 11 11.1 (A13).
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - `layout up --dry-run --json` matches the golden `layout.apply`
    requests for the example run layout.
  - `pane run` runs each step as `mise exec -C <pane dir> -- <argv>`,
    stops at a failing step with its status and drops to a login shell.
  - `layout up` applies against herdr at the version and hash of the pin
    file, in the container; it checks the socket protocol number against
    the one A13 recorded (T026), fails closed on another (Q16), and
    reports "layout stale" when the `run` digest differs. The number
    comes from the probe, not from the pinned binary, which would prove
    nothing.
- Verify: `go test -tags e2e ./e2e/...`.
