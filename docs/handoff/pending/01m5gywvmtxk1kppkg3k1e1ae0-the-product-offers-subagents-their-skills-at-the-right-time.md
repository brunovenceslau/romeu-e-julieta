# The product offers subagents their skills at the right time

What is at stake: medium. Without it, the skills a subagent needs are
loaded by luck, and an agent works outside its domain rules.

Scope: this repository (the product); the rules belong to the personal
kit of the config repository.

Analysis of 2026-10-22. In the sandbox, 25 subagent briefs that only
named skills on a `SKILLS:` line led to 1 Skill call (after a follow-up
message); 4 briefs that said "invoke each with the Skill tool first"
loaded all at call 1, before orientation; briefs that asked for each
skill at a moment were followed out of order (a router or checklist
loaded early or after the code it should guide), and one builder
misreported its own load position.

In the product, the agent-aware kit is `julieta-claude` (06 6.4,
REQ-0595 to REQ-0599, task T079, not built). Its only PreToolUse guard
(`julieta hooks guard`, REQ-0410) reads Bash commands; the spec has
nothing on subagent briefs or SubagentStop. Personal plugins and their
skills live in a personal kit in the config repository (06 6.5,
REQ-0608, T114).

Proposed, not decided:

- The mechanism in the product: a PreToolUse guard of Agent briefs, a
  SubagentStop check of the order of Skill calls against the first touch
  of the domain, and a julieta command that reports each subagent's
  skill calls from its transcript.
- The rules (which skill, which class: process at the start, domain
  routers after orientation and before the first domain edit, checklists
  before the step they check) in the personal kit.
- Each harness fact the mechanism relies on as a C-row probe of 11 for
  the agent version the product fixes.

The timing classes are a hypothesis being measured in the sandbox kit's
repository.

Reopen: that measurement's result, or the build of T079.
