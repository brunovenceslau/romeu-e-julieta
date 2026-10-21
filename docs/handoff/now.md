# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T078 merged: "Session of 2026-10-20, the
skills of T078 and the CI plan" (01m5drzqrqhekxjja8dkz6njf2) ticks T078
and records ADR 0040 and the CI plan. Since then, on 2026-10-21, T153 merged
(#224), ticked by "T153, the gitsafe verbs of the snapshot bundle and
the system hooks stage" (01m5ed3ahrd8b8e0p20606r97g).

## Next steps

1. Build T060 (`julieta snapshot`), re-planned on T153 on 2026-10-21.
2. Plan "One race run, test tags and parallel CI jobs"
   (01m5drzqsp6zg34xzmywckwb7b), measuring the slow race packages first.
3. A `julieta status` task for "julieta status reads the hook failure
   store" (0000000000782g86tsyzte1t16).
4. T150, the imports check that refuses `encoding/json` v1.
5. T077, then T081 (the kits of M2).
6. The e2e cache annotation of "The spec catches up with the hardening
   of T127, T143 and T144" (0000000000pz8zh0x006xkqa4w).
7. T111, the cut-over of M1, once T060 lands; T112 through the sessions
   of the config and kit repositories ("T112 in the config and kit
   repositories", 00000000008h7a0ey78frxzgfr).
8. Plan the bump tool of "Dependency bumps run as a script, not by hand"
   (01m5e14094yhqhsxq2g941pwrf); the trigger of its deferred scheduled
   run fired with GO-2026-6809.
9. Build the file of "A short CLAUDE.md of pointers for this
   repository" (01m5e1majbx0hq6p710g9dmtr2) at its trigger.

## Open for the operator

One block, a recommendation each.

1. Answer the three questions of "Prefer the harness's defaults and
   publish what is left" (01m5drzqvjrf3e7p7b2v68temw). Recommended:
   before the CI task, which adds tooling.
2. Approve or decline the two proposals of "Validate Mermaid diagrams"
   (01m5drzqwhebfeqegfbe7gmte9). Recommended: the payload rule now, the
   CI check with the first Mermaid block.
3. ADR 0040 was merged as Accepted in #217 without a `decisions`
   approval line. Recommended: say whether that merge approves it, or
   it goes back to Proposed in a pull request of its own.
4. Report the sandbox kit repository's pull request for T112 ("T112 in
   the config and kit repositories", 00000000008h7a0ey78frxzgfr; the
   config repository's part is done), and say when the GitHub App of
   ADR 0011 changes the sandbox's authentication.
5. Decide the scheduled run of the pin-freshness check, whose deferral
   fired with GO-2026-6809 (01m5e14094yhqhsxq2g941pwrf). Recommended:
   with the bump tool, one plan.

## In flight

- `docs/tick-t153-replan-t060`, T153's tick and T060's re-plan.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
