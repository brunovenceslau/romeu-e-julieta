# Current state

## Where we stopped

Checkpoint of 2026-10-21: "Session of 2026-10-21, T153, the Go bump and
the work queue" (01m5eg0qdj584kyp8qjf74bq9w). The queue below runs
under "Work authorized while the operator is away on 2026-10-21"
(01m5eg0qefcvwdx05q4vnqcwfe).

## Next steps

1. T060 (`julieta snapshot`): its builder's report, then a code review
   and a security audit, then its pull request.
2. The status task for "julieta status reads the hook failure store"
   (0000000000782g86tsyzte1t16): its builder's report, a review, its
   pull request, and the Done record that closes the note.
3. "One race run, test tags and parallel CI jobs"
   (01m5drzqsp6zg34xzmywckwb7b): measure the race packages, plan, build.
4. T150, the imports check that refuses `encoding/json` v1.
5. The e2e cache annotation of 0000000000pz8zh0x006xkqa4w.
6. The `AGENTS.md` and `CLAUDE.md` of 01m5e1majbx0hq6p710g9dmtr2.
7. A task for T060's `disabled` item outside its owns.
8. T077, then T081 (the kits of M2).
9. The plan of "Dependency bumps run as a script, not by hand"
   (01m5e14094yhqhsxq2g941pwrf), docs only.
10. A checkpoint; then T111, the cut-over of M1, once T060 lands.

## Open for the operator

One block, a recommendation each.

1. Merge the queue's pull requests in their order. Recommended: as each
   goes green.
2. Answer the three questions of "Prefer the harness's defaults and
   publish what is left" (01m5drzqvjrf3e7p7b2v68temw). Recommended:
   before the CI task's build adds tooling.
3. Approve or decline the two proposals of "Validate Mermaid diagrams"
   (01m5drzqwhebfeqegfbe7gmte9). Recommended: the payload rule now, the
   CI check with the first Mermaid block.
4. ADR 0040 was merged as Accepted in #217 without a `decisions`
   approval line. Recommended: say whether that merge approves it, or
   it goes back to Proposed in a pull request of its own.
5. Paste the three payload prompts of this session's Done record, and
   report the sandbox kit's pull request for T112
   (00000000008h7a0ey78frxzgfr), and say when the GitHub App of ADR 0011
   changes the sandbox's authentication. Recommended: one session each.
6. Decide the scheduled pin-freshness run, whose deferral fired with
   GO-2026-6809 (01m5e14094yhqhsxq2g941pwrf). Recommended: with the
   bump plan of next step 9.

## In flight

- `feat/t060-julieta-snapshot`, T060, a builder in its own worktree.
- `feat/julieta-status-hook-failures`, the status task, a builder in its
  own worktree.
- `docs/checkpoint-2026-10-21`, this checkpoint.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
