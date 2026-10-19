# What this repo is

romeu-e-julieta is the host-side orchestrator for sandboxed agent work
(the host binary, its gates and its spec). The plan is docs/plan.md,
the spec is docs/spec.md and docs/spec/, the decisions are docs/adr/.
This tree holds the session handoff notes: now.md is the current state,
done/ the numbered records, decisions/ the standing decisions, pending/
the open items. The house rules live in the sandbox payload
(~/.claude/CLAUDE.md), not here.

A decision note says in its first paragraph what is at stake: what
happens if the decision is wrong or fails, and how hard that is to undo
(rule 9 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
as
[ADR 0037, state what is at stake in every decision record](../adr/0037-state-what-is-at-stake-in-every-decision-record.md)
amends it).
