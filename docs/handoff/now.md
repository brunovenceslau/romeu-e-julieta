# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T078 merged: "Session of 2026-10-20, the
skills of T078 and the CI plan" (01m5drzqrqhekxjja8dkz6njf2) ticks T078
and records ADR 0040 and the CI plan.

## Next steps

1. #218 (skill validation findings): CI green, then the operator's merge.
2. Ask the operator before planning one `gitsafe` task for "gitsafe verbs
   for the snapshot bundle" (01m5d7v42kv79hf42g15ffgk59) and "gitsafe
   verbs for the global hooks path and the dirty count"
   (01m5d7v43hd4hsw60e4bhek3pw); then re-plan and build T060.
3. Plan "One race run, test tags and parallel CI jobs"
   (01m5drzqsp6zg34xzmywckwb7b), measuring the slow race packages first.
4. A `julieta status` task for "julieta status reads the hook failure
   store" (0000000000782g86tsyzte1t16).
5. T150, the imports check that refuses `encoding/json` v1.
6. T077, then T081 (the kits of M2).
7. The e2e cache annotation of "The spec catches up with the hardening
   of T127, T143 and T144" (0000000000pz8zh0x006xkqa4w).
8. T111, the cut-over of M1, once T060 lands; T112 through the sessions
   of the config and kit repositories ("T112 in the config and kit
   repositories", 00000000008h7a0ey78frxzgfr).

## Open for the operator

One block, a recommendation each.

1. Merge #218 once green, then this checkpoint. Recommended: today.
2. Paste the prompt of "The skill-creator rule in the sandbox payload"
   (01m5drzqycwwtat9czcf4egdsr) into the payload's repository.
   Recommended: with item 3, one session there.
3. Say whether the prompt for the payload's handoff tools was pasted:
   `handoff_check.py` passes here at this checkpoint, so the ULID
   reading looks fixed; the migrator's relative links are unmeasured.
   Recommended: close the item if that session reported its PR.
4. Answer the three questions of "Prefer the harness's defaults and
   publish what is left" (01m5drzqvjrf3e7p7b2v68temw). Recommended:
   before the CI task, which adds tooling.
5. Approve or decline the two proposals of "Validate Mermaid diagrams"
   (01m5drzqwhebfeqegfbe7gmte9). Recommended: the payload rule now, the
   CI check with the first Mermaid block.
6. Approve the `gitsafe` task of next step 2 (ask first). Recommended:
   one task.
7. Report the pull requests of the prompts pasted on 2026-10-20 (T112 in
   the config and kit repositories, ADR 0039 in the payload), and say
   when the GitHub App of ADR 0011 changes the sandbox's authentication.

## In flight

- `fix/skills-validation-findings`, #218.
- `docs/checkpoint-after-t078`, this checkpoint.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
