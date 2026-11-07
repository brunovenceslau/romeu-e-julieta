---
id: T207
title: "The apply swap tag and the recreate digest over both classes"
milestone: none
module: none
fate: removed
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T207 - The apply swap tag and the recreate digest over both classes

Removed on 2026-11-07, before it was built: the swap class it would
have added (a third apply tag, a split of the recreate digest, a
read-back after the swap) left the specification the same day, when
`run` step 3 came to hand a changed env file to sbx, running or
stopped, and to branch on sbx's exit alone (04 4.2, REQ-0353; 01 1.7,
recreate needed). T066 carries that step.

- Implements: none: removed.
- Operator: no.
- Acceptance: none.
- Verify: none.
