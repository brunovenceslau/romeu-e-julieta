# Finish T105 PR-B: the procgroup switch

Branch feat/t105b-procgroup-switch holds the builder's work as one WIP
commit, saved before a sandbox recreate; no gate has run. PR-A merged
(#52), so PR-B targets main with no stack.

To do: finish the setup's `mise install` move to the mise class with
its test (the edit may be partial); add T105's acceptance lines from
"t137-signal-spec-pr-items-left-at-the-round-cap"; re-run every gate;
rewrite the WIP commit into reviewable commits; ship gate (code-reviewer
opus, security-auditor opus, test-engineer sonnet); PR with approval
lines for each surface touched (procgroup, checks, and others measured
against .github/ask-first.yaml).

The PR body states these behaviour changes: a step that exits 0 with a
grandchild still on its pipe now passes; probes are bounded at 30s and
go env at 30m; error texts become "<prog> exited N" and "ran past the X
limit of class tool-*"; a second signal is absorbed; a SIGHUP to
tools/mutants ends it by default action; the pack/unpack pair runs as
two tool-git commands started together.

Open: Run hangs when a Stdout writer blocks (drain waits for the
process); same class as L6 in "t137-items-left-at-the-round-cap".

Reopen: the next session.
