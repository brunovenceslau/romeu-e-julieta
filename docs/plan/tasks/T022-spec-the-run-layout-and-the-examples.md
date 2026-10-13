---
id: T022
title: "`spec`: the run layout and the examples"
milestone: none
module: spec
fate: folded
absorbed_by: T021
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T022 - `spec`: the run layout and the examples

Re-plan (2026-10-12): folded into T021; its rows are copied into T021's
Acceptance. This file keeps the id so that earlier citations resolve;
the text below is the one written before the re-plan, and T021 is where
it is built.

- Implements: 03 3.2 ("Run layout"), 02 2.1 (`examples/`), 03 (opening).
- Operator: no.
- Ask-first, as forecast before the re-plan: `gates` (if `project.go`
  changes).
- Acceptance:
  - The validator refuses a pane id used twice, two focused panes in a
    tab, a ratio outside 0.1 to 0.9, an `env` key on the refused list
    and a `cwd` with a `..` element.
  - `examples/` holds an example config repo whose files validate
    against the generated schemas in `tools/ci schema`.
- Verify: `go test ./internal/spec/...`.
