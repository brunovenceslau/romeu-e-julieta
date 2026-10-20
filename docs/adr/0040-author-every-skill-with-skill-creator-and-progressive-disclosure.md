# 40. Author every skill with skill-creator and progressive disclosure

Date: 2026-10-20

## Status

Accepted

## Context

T078 wrote the julieta and handoff skills as one `SKILL.md` each, with
a short description and every detail in the body. Claude Code loads a
skill in three levels: the name and description are always in context
and decide whether the skill is used; the body is read when it is;
files the body links are read only when the body sends the agent to
them. The skill `anthropic-skills:skill-creator`, which the harness
ships, calls this progressive disclosure and gives the method that
goes with it: a description that says what the skill does and when to
use it, a body under about 500 lines that holds the workflow and the
reason for each rule, and detail in `references/`, `scripts/` or
`assets/`, each linked from the body with when to read it.

A short description makes an agent skip a skill it needs, and a body
that holds every case spends context on the cases that do not apply.
Both skills of T078 had the first defect, and the handoff skill held
both kinds' headings where an agent writes one kind at a time.

The same question comes up wherever a skill is written: in this
repository's `skills/`, which ship to every adopter, and in the skills
and kits of the maintainer's other repositories and sandboxes.

What is at stake: if the rule is wrong, skills gain an indirection they
do not need, undone by folding a reference back into its `SKILL.md`.
Without it, each skill is shaped by whoever writes it, and a skill
that is not used when it should be fails silently.

Scope: every adopter of the product, for the skills under `skills/`
(REQ-1255 in 12 12.3); every repository and sandbox of the maintainer,
for the use of skill-creator. The second lives today in the sandbox
payload, through a prompt for the payload's repository, and once romeu
and julieta replace the interim sandboxes in the personal kit of the
config repository (T114, REQ-0608), through the pending note "Rules to
move when the interim payload is retired".

### Alternatives considered

- **Keep each skill in one file.** The state before this record; it
  has the defects above.
- **A house style of this repository.** Not taken: the harness already
  ships the method and its tooling, and a house style would drift from
  it.
- **skill-creator, with the shape checked by a test.** Taken.

## Decision

Decided on 2026-10-20:

1. Creating, editing, reviewing or testing a skill uses the skill
   `anthropic-skills:skill-creator` and follows its progressive
   disclosure. **[review]**
2. A builder brief for work on a skill names that skill. **[review]**
3. Each skill under `skills/` meets REQ-1255: a front matter with the
   directory's name and a description, a body under 500 lines, every
   other file linked from `SKILL.md`, and every relative link naming a
   file of the skill. `TestSkillsNameRealCommands` checks it.

## Consequences

- T078 implements REQ-1255; its skills are rewritten to it, and its
  test reads every file of a skill.
- REQ-0595 now says the kit installs each skill's whole directory, so
  `references/` reaches the sandbox with `SKILL.md`; T079, which builds
  that kit, meets it.
- The pending note "Rules to move when the interim payload is retired"
  lists 1 and 2 for the payload and for T114.
