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
read-back after the swap) left the specification the same day.
Restated on 2026-11-07, after the decisions of that day: no class of
change is applied in place. `run` step 3 hands no changed env file to
sbx; on a running sandbox whose recreate digests differ it warns
`recreate-needed` and attaches, and a stopped one it refuses with exit
5 (04 4.2, REQ-1376; 01 1.7, recreate needed). The in-place apply is a
Deferred row of the index, opened only by T210's hand measurement; T066
builds the closed gate. Nothing points to this task as live work.

- Implements: none: removed.
- Operator: no.
- Acceptance: none.
- Verify: none.
