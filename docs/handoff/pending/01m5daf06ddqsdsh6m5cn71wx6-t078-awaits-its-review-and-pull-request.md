# T078 awaits its review and pull request

What is at stake: small; the skills of M1 wait, and T111 needs them.

Scope: this repository.

T078 (the julieta and handoff skills, and `TestSkills*`) was built on
2026-10-20 on branch `feat/t078-skills`, rebased on main after T059, with
`go run ./tools/ci all` green; no code review has run. Its pull request
carries the `kits` approval line (`skills/**`) and this Prior art
section:

The skill test reads a SKILL.md for julieta command lines and for fenced
heading blocks with a line scan (code fences by a toggle, code spans by
backtick pairs), the level `handoff.CheckHeadings` already uses. Command
and flag existence is checked against the real cobra tree (`cli.Tree`,
`Command.Flag`, `ShorthandLookup`), and the headings come from
`handoff.Headings`, so nothing is copied. Not taken: yuin/goldmark, a
full CommonMark parser and a new dependency (ask first). No dependency
added.

Open from the build: no Claude Code instance loaded the skills; the SPDX
header is an HTML comment after the front matter; the check does not
follow a julieta command inside a longer pipeline.

Reopen: when the code review runs; the note goes with T078's Done record.
