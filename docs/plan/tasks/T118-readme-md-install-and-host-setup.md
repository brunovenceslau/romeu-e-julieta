---
id: T118
title: "`README.md`: install and host setup"
milestone: M5
module: docs
fate: new
owns:
  - README.md
needs:
  - T116
after: []
ask_first: []
sitting: none
---

# T118 - `README.md`: install and host setup

Goal: The J1 install paragraph and the host setup steps (the dedicated
signing agent, `kit.allowedSources`, secrets), in the heading order of
ADR 0001 rule 12, so T095 extends it.

- Implements: 09 (J1); ADR 0001 rule 12.
- Operator: no
- Acceptance:
  - A reader with the two hosts' prerequisites reaches `romeu run` from
    the page alone.
  - The headings are the ones ADR 0001 rule 12 fixes, in order.
- Verify: `go run ./tools/ci fast` exits 0.
