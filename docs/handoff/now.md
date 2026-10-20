# Current state

## Where we stopped

Checkpoint of 2026-10-20, after T035 merged (Done record 46, which
ticks T035, T145 and T146). Later that day ADR 0039 and ADR 0021 were
accepted and the operator items answered
("the-operator-items-of-2026-10-20").

## Next steps

1. The next swap of ADR 0038, the ssh-agent listing of `internal/signing`
   (signing surface, ask first), per
   "hand-written-code-a-maintained-library-could-replace".
2. The rest of the ready list (`go run ./tools/ci plan --ready`); T111
   waits for T054, T059, T060 and T078.
3. T112 lands through the sessions of the config and kit repositories
   ("t112-in-the-config-and-kit-repositories").
4. A task on the `gitsafe` surface hardens the fast-forward before
   `julieta setup` runs in daily use, and one gives the removal of a
   corrupted e2e cache entry a job annotation
   ("the-operator-items-of-2026-10-20").

## Open for the operator

One block. Decisions of 2026-10-19 are in
"the-grants-of-2026-10-19", and those of 2026-10-20 in
"the-operator-items-of-2026-10-20".

1. Report the pull requests of the three prompts pasted on 2026-10-20
   (T112 in the config and sandbox kit repositories, ADR 0039 in the
   payload), so their pending notes close.
2. Create the GitHub App of
   [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md)
   when M3 starts; the sitting of T126 (O8) waits for it.

## In flight

- Nothing.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
