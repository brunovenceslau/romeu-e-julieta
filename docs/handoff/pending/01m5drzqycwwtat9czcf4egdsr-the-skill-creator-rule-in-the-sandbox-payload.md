# The skill-creator rule in the sandbox payload

What is at stake: small. Until the payload holds it, ADR 0040's rule
reaches only this repository, and a builder elsewhere writes skills its
own way.

Scope: every repository and sandbox of the maintainer, through the
payload's repository (`kits/claude-home`); later the personal kit
(T114), through "Rules to move when the interim payload is retired"
(00000000001v2n0qea13kr8yvy).

On 2026-10-20 this prompt went to the maintainer, to paste into a
session of the payload's repository; its pull request is not yet
reported:

````text
Payload rule: every skill change goes through anthropic-skills:skill-creator.
Written by the romeu-e-julieta session of 2026-10-20; it carries no approval.
A merge, or a step a rule marks ask-first, needs my ok in this session.

Decided on 2026-10-20: creating, editing, reviewing or testing a skill (a
SKILL.md and the files it bundles) uses the skill
`anthropic-skills:skill-creator` and follows its progressive disclosure:
the name and description, always in context, say what the skill does and
when to use it; the SKILL.md body stays under about 500 lines and holds
the workflow; details live in `references/`, `scripts/` or `assets/`,
each named from the body with when to read it.

Scope: every repository and sandbox of the maintainer. Home today:
kits/claude-home of this repository (the interim payload). Home once
romeu-e-julieta and verona replace it: the personal kit in the config
repository (REQ-0608 of romeu-e-julieta); romeu-e-julieta's note "Rules
to move when the interim payload is retired" lists it.

1. The rule text: one item in "How to work" of the payload's CLAUDE.md
   (kits/claude-home/spec.yaml), stating the scope as ADR 0039 of
   romeu-e-julieta asks.
2. The check: an active row of hooks/brief-guard-rules.json, id
   "skill-authoring", requires_all ["anthropic-skills:skill-creator"],
   for a brief that creates, edits, reviews or tests a skill. Draft the
   trigger on paths and verbs (SKILL.md, a skills/ directory, "write /
   edit / recreate / review a skill"), and make sure it does not match
   the `SKILLS: ...` line every brief carries or a mere mention of
   "skills" in prose. Add positive and negative fixtures to the brief
   guard's tests, the SKILLS line among the negatives.
3. Decide late, and record the trigger that reopens it: whether a
   PreToolUse hook on Edit and Write of a SKILL.md path should also
   check the skill was loaded in the session (the brief guard covers
   subagents only, not the orchestrator's own edits).

Library first, review before the PR, local CI, signed commits, and this
repository's own gates (the forbidden-names gate where it is public)
apply as its CLAUDE.md says. Report the pull request back so the
romeu-e-julieta note can close; end your report with what you did not
verify.
````

Reopen: when the payload session reports its pull request; then the
note of rules to move records it as sent and this file goes.
