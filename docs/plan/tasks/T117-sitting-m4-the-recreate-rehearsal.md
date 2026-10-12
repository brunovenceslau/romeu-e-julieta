---
id: T117
title: "Sitting M4: the recreate rehearsal"
milestone: M4
module: none
fate: new
owns: []
needs:
  - T068
after: []
ask_first: []
sitting: M4
---

# T117 - Sitting M4: the recreate rehearsal

Goal: One `romeu recreate` rehearsal on one project with a planted dirty
worktree, a stash and an ignored file.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox.

- Implements: 09 (J6, J10); 08 8.5.
- Operator: plants the cases and runs the rehearsal.
- Acceptance:
  - Every planted case is in the salvage and restored.
  - J6 and J10 pass in CI with the fake sbx; on the host, the
    rehearsal's `romeu recreate` transcript and salvage listing, read by
    the maintainer, are the Evidence.
- Verify: `romeu recreate` and the salvage listing, read at the sitting.
