# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T059 merged: "Session of 2026-10-20,
json v2, gitsafe, the hooks and go fix" (01m5d7v40m0axrfnby7097da4e)
ticks T147, T148, T149, T151, T054, T152 and T059. The handoff notes
were renamed to `<id>-<slug>.md` in the pull request before this one.

## Next steps

1. T078 (skills): built and pushed, not reviewed; code review, then its
   pull request with the `kits` approval line.
2. Plan one `gitsafe` task for "gitsafe verbs for the snapshot bundle"
   (01m5d7v42kv79hf42g15ffgk59) and "gitsafe verbs for the global hooks
   path and the dirty count" (01m5d7v43hd4hsw60e4bhek3pw); then re-plan
   and build T060.
3. A `julieta status` task for
   "julieta status reads the hook failure store"
   (0000000000782g86tsyzte1t16).
4. T150, the imports check that refuses `encoding/json` v1.
5. T077, then T081 (the kits of M2).
6. The removal of a corrupted e2e cache entry writes a job annotation,
   as decided ("The spec catches up with the hardening of T127, T143 and
   T144", 0000000000pz8zh0x006xkqa4w).
7. T111, the cut-over of M1, once T060 and T078 land; T112 through the
   sessions of the config and kit repositories ("T112 in the config and
   kit repositories", 00000000008h7a0ey78frxzgfr).

## Open for the operator

One block, a recommendation each. Decisions of 2026-10-20 are in
"The operator items of 2026-10-20" (0000000000abv84dqf1csebd9c) and in
the Done record above.

1. Merge the migration pull request, then this checkpoint. Recommended:
   in that order, today.
2. Paste the prompt for the payload's handoff tools, in the payload's
   source repository (the checker reads julieta's ULIDs as item ids;
   the migrator misses relative links). Recommended: today; until then `handoff_check.py`
   cannot pass here.
3. Approve the `gitsafe` task of next step 2 (ask first). Recommended:
   one task, after T078.
4. Report the pull requests of the three prompts pasted on 2026-10-20
   (T112 in the config and kit repositories, ADR 0039 in the payload).
   Recommended: when those sessions finish, so their notes close.
5. The GitHub App of ADR 0011, decided for the start of M3 (T126 waits
   for it), is being set up early in another session. Recommended: say
   when the sandbox's authentication changes, so no push is in flight.

## In flight

- `feat/t078-skills` (T078), pushed, no pull request yet.
- The migration pull request and this checkpoint.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
