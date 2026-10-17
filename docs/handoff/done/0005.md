# Session of 2026-10-17: the step-table rows get their ids

- #67: the `hygiene` and `workflows` rows of the step table in 10 10.2
  carry REQ-1240 and REQ-1241, numbered by `tools/ci reqs allocate`.
  - The two checks register the ids and `docs/reference/checks.md`
    shows them in place of `-`.
  - T001 and T002, both merged, cite them, as REQ-1201 asks of every
    live id.
  - The step-table test used the real `hygiene` row as its row
    without an id; it now adds a synthetic row, since every row holds
    one.
  - Gate: code-reviewer and security-auditor. Fixed: the two comments
    that still said the rows hold no id.
  - Green on the four runners; merged on 2026-10-17, proved by the
    second parent of 60dc652.
  - This closes "step-table-rows-without-a-req-id".

Lessons for the process:

- A fixture that borrows a real row for its negative case breaks the
  day that row is fixed. A synthetic row keeps the case.
