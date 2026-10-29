---
id: T126
title: "Accept ADR 0011"
milestone: M6
module: docs
fate: new
owns:
  - docs/adr/0008-*.md
  - docs/adr/0010-*.md
  - docs/adr/0011-*.md
  - docs/spec.md
needs: []
after:
  - T122
  - T108
  - T182
ask_first:
  - decisions
  - spec
sitting: O8
---

# T126 - Accept ADR 0011

Goal: The pull request that accepts ADR 0011 after the App checks its
Consequences list: the status word, the `Superseded in part by` lines of
ADR 0008 and ADR 0010, and the removal of the index row on required
status checks without the administrator bypass. Block O8 names it in
"Needs merged first". The App checks are run from a session of the
sandbox kit repository.

Re-plan (2026-10-12, stage B): New. The acceptance of a Proposed record
is a pull request of its own, so block O8 has a task to name.

- Implements: REQ-1167 (12 12.5).
- Operator: each App check of the record's Consequences, under Evidence.
- Acceptance:
  - The status lines and the row removal are in the diff.
  - `tools/ci sequences` (when it exists) and `tools/ci generated` pass.
- Verify: `go run ./tools/ci generated` exits 0.
