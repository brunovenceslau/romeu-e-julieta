---
id: T133
title: "The `invariant` kind, `tools/ci invariants` and `tools/ci mutate`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T105
  - T106
after: []
ask_first:
  - checks
sitting: none
---

# T133 - The `invariant` kind, `tools/ci invariants` and `tools/ci mutate`

Goal: The `invariant` kind of `tools/new` (12 12.3), the `invariants`
check that pairs each guard tag with a test tag, and `mutate`, which
runs `tools/mutants` over one committed entry per guard (10 10.2; 05
5.2).

Re-plan (2026-10-12, stage B): Deferred to M6 with the trigger of the
plan's row: the first invariant whose guard has code and whose test
could be green by omission, found in review; or the first
release-candidate tag. Carries T007's three "deferred with the tooling"
rows verbatim and the mutate sentence. T087 and T090 need it. Until it
reopens, `owns` is empty.

- Implements: REQ-0474 (05 5.2), REQ-0478 (05 5.2), REQ-0771 (10 10.1),
  REQ-1111 (12 12.3), REQ-1121 (12 12.3).
- Operator: no
- Acceptance:
  - T007's three deferred rows, verbatim: the `invariant` kind writes
    the guard tag and the paired test tag for an id of 05 5.2; `tools/ci
    invariants` fails an id whose guard tag has no test tag, or whose
    test tag has no guard tag, at every build-tag level the runner has;
    `tools/ci mutate` reports an invariant id whose guard, with its
    decision passed through, leaves every tagged test green.
  - A test tag sits on a `Test*` function; for each id `tools/ci mutate`
    runs the tagged functions by name under each level's build tag the
    runner has (`e2e` on the Linux runners), parsing `go test -json`,
    and reports the id red when one fails by its own assertion; a
    fixture package whose guard stub is a no-op makes `mutate` report
    the id as not red.
  - `tools/ci mutate` runs `tools/mutants` over one committed mutants
    entry per guard, whose `new` text is the pass-through of the guarded
    decision (item 13 of Decisions of the re-plan in `docs/plan.md`,
    decided on 2026-10-12).
- Verify: `go test ./tools/ci/... -run '^Test(Invariants|Mutate)'`.
