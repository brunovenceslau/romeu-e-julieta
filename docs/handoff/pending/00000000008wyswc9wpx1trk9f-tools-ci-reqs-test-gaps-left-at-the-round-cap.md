# tools/ci reqs: test gaps left at the round cap

Test-engineer round 3 of #45 (GO at dece74f) hit the round cap with no
Critical or High finding. Open, in `tools/ci/reqs.go`:

- Medium: (1) reqParseTask unterminated front matter guard untested (a
  mutant panics); (2) front matter without a `fate:` line; (3) reqCite
  backtick citation with a reading aid; (4) the ADR check's reqDigits
  filter (`REQ-` and `REQ-new` in an ADR must pass); (5) reqMisplaced
  TrimSpace guard (mid-line id followed by ` - `); (6) reqPagesAt chapter
  filter (allocate with docs/spec/sub/ or notes.txt); (7) reqMarks header
  bound with a table row as the last line and no newline; (8)
  reqRetiredRows with an empty table.
- Low: retired-rows upper bound off by one; reqShared tested only for
  1169; reqDigits {3,4} survives (add REQ-001); reqChapter {1,2}
  survives (add 1-x.md); delimiter-row alignment colons; tilde fence
  with a backtick in the info string; a retired row with 5 cells;
  Implements continuation by one space with an unanchored reqFate.
- Known limit, stated in 12 12.11: a deletion of the highest ids passes
  the check.

The code moved to tools/ci/internal/reqs (T105 PR-C). #64 changed it
(a byte-count test for reqMarks) without taking these gaps on.

Reopen: the next change to tools/ci/internal/reqs made for its own sake.
