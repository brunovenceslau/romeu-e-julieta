# Session of 2026-10-13, the stage-B trace pass (migrated from docs/reviews/stage-b.md)

The stage-B trace pass ran as one workflow, its output landed on the
branch, and the tool's focused gate ran three rounds. The merges are in
Done record 1.

- Decided on 2026-10-12:
  - Requirement ids are `REQ-nnnn` from one global counter, never
    reused; a retired id is recorded in the table of 12 12.11; ADRs
    carry no ids; ids are allocated only by `tools/ci reqs allocate`;
    tasks cite them in Implements; the check fails on an active id no
    task cites and on a task citing an unknown or retired id. Citing ids
    in code and tests is deferred to the first task that needs it (ADR
    0012, 12 12.11).
  - For this pass only, craft findings were applied as well as
    contradictions and behavior gaps, reversing the stage-A rule.
  - Every agent that writes, changes, tests or reviews Go loads the
    cc-skills-golang skills, starting with golang-how-to, uses gopls for
    navigation and reports the skills it loaded. The rule lives in the
    sandbox payload (see pending
    "prompts-for-the-sandbox-kit-and-the-house-kit").
  - The sixteen decisions OD-1 to OD-16 of the re-plan took their
    recommended options (docs/plan.md, "Decisions of the re-plan"). OD-14
    moved to T111 and OD-7 covers every risk row (decisions
    "re-plan-od-14-at-t111-od-7-for-every-risk-row-prefix-capture").
  - #44, the gopls pin: decisions
    "the-gopls-pin-and-the-deferred-split-of-the-pinned-tools".
  - #45: the workflow's own chain (find, verify, write, four reviews,
    fix) stood in for the full ship gate on the documents, and the Go tool
    got a focused gate, a security auditor and a test engineer.
- Declined on 2026-10-13: the fix pass's suggestion to change T037's
  "the sitting that opens M3" to "sitting M2", because it contradicts
  OD-15.
- The output on the branch: the write pass and the ids, ac488e2; the tool,
  8cc30a8; the fix pass, b8439cb and 0e44a9e; `go run ./tools/ci all`
  exited 0. Gate round 1 at 0e44a9e: security auditor GO (3 Low, 2 Info),
  test engineer NO-GO (4 missing tests, 5 cheap ones). Fixes 0590ae8
  (CommonMark fences and backticks counted in one pass) and 8229928 (12
  12.11 states that the check cannot see a deleted highest id). Round 2 at
  8229928: both GO. Fixes bf98aea and 7db9744. Round 3, the targeted
  re-audit of the tests, GO at dece74f (Done record 1); the gaps it left
  are pending "tools-ci-reqs-test-gaps-left-at-the-round-cap".
- Closed since the items listed then:
  - the update of #43 and the answers to OD-14 and OD-7: #43 merged;
  - items 8 to 11 of the plan's decision block: T108 (#79, Done record
    11);
  - `internal/egress/**` on no surface: #48, on `catalog` (Done record 1);
  - the merges: #45 and #43 as stack #46, with #44.
- Covered elsewhere: the deletion limit of `tools/ci reqs` (a deletion of
  the highest ids passes the check) is stated in 12 12.11 and in pending
  "tools-ci-reqs-test-gaps-left-at-the-round-cap"; the closing of the
  spec sweep item of 2026-10-01 was done per OD-16 (Done record 1); the
  measurement of OD-9 is pending
  "od-9-measure-the-pending-host-list-clause-at-sitting-m0"; the widening
  of the checkpoint grep and the answered items of the old list are in
  pending "stage-b-docs-pass-leftovers" (the grep is moot once the
  headings keep their count: every migrated section keeps its heading).
- The sandbox-payload and external-repository handovers are tracked
  outside this repository.
