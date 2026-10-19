# Current state

## Where we stopped

Checkpoint of 2026-10-19, after GitHub stack #175 and #177 merged
(Done record 39). Done records are merged up to 38.

## Next steps

1. Every task the plan lists as ready waits on an operator step or an
   ask-first surface (below).

## Open for the operator

One block, a recommendation each. Items answered on 2026-10-18 are in
the decision note "the-day-run-of-2026-10-18".

1. The workload pin of T012: 06 6.2 names the repository and no tag,
   and `home` waits for sitting M2. Recommended: pick the tag at sitting
   M2, then `go run ./tools/kitpin workload <ref>`.
2. How product types reach `schemagen.Register` under REQ-0971, before
   T135. Recommended: a sentence in REQ-0971 like the one for
   `tools/new`, so `tools/ci` imports `internal/manifest`.
3. Tasks this run skipped, each on an ask-first surface it did not
   take: T013 (`approvals`; it also needs the workload pin), T103
   (`release`), T135 (`contracts`). Recommended: a checkpoint grant for
   the three surfaces at the next sitting.
4. The sittings of T112 (M1), T113 (M0) and T126 (O8). Recommended:
   hold sitting M0 first, since T113's go/no-go gates the kit tasks.
5. The kit prompts ("prompts-for-the-sandbox-kit-and-the-house-kit").
   Recommended: ask for both final prompts in one message.
6. The behaviour of `romeu init` on a non-empty root
   ("init-with-a-non-empty-root-is-unspecified"), a product decision
   for T041. Recommended: decide it in T041's re-plan, before T041
   starts.

## In flight

- Nothing.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
