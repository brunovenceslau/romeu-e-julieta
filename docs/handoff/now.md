# Current state

## Where we stopped

Checkpoint of 2026-10-18, after the day run (Done record 37). Done
records are merged up to 36. The day run's stack, #149 to #168, GitHub
stack #167, is open and none of it is merged.

## Next steps

1. Merge the stack bottom to top, or the part of it that is ready; the
   next record pull request ticks each merged task (`fate: merged`,
   `merged:`, `pr:`): T132, T010, T123, T125, T139 and T012.
2. Every task the plan lists as ready after that waits on an operator
   step or an ask-first surface this run did not take (below).

## Open for the operator

One block, a recommendation each. Items answered on 2026-10-18 are in
the decision note "the-day-run-of-2026-10-18".

1. mise's minisign key: confirm out of band the key id and key line #164
   states, before its merge. Recommended: compare them with the key
   mise publishes on its own site and in its release notes.
2. The workload pin of T012: 06 6.2 names the repository and no tag,
   and `home` waits for sitting M2. Recommended: pick the tag at sitting
   M2, then `go run ./tools/kitpin workload <ref>`.
3. The nineteen Proposed records of T123 (#155), and the selection of
   the thirteen round-8 items its re-plan states (#154). Recommended:
   read both in the pull requests before merging them.
4. The forbidigo allow list of T139 (#161): 04 4.1 names no package
   set for the output patterns. Recommended: keep the list of
   composition roots it states.
5. How product types reach `schemagen.Register` under REQ-0971, before
   T135. Recommended: a sentence in REQ-0971 like the one for
   `tools/new`, so `tools/ci` imports `internal/manifest`.
6. Tasks this run skipped, each on an ask-first surface it did not
   take: T013 (`approvals`; it also needs the workload pin), T103
   (`release`), T135 (`contracts`). Recommended: a checkpoint grant for
   the three surfaces at the next sitting.
7. The sittings of T112 (M1), T113 (M0) and T126 (O8). Recommended:
   hold sitting M0 first, since T113's go/no-go gates the kit tasks.
8. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.
9. The behaviour of `romeu init` on a non-empty root
   ("init-with-a-non-empty-root-is-unspecified"), a product decision
   for T041. Recommended: decide it in T041's re-plan, before T041
   starts.

## In flight

- The day run's stack, #149 to #168.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
