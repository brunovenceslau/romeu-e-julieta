# Rules to move when the interim payload is retired

What is at stake: a rule that lives only in the interim sandbox payload
is lost when romeu and julieta replace it, and builders in every
repository go back to the old behavior.

Scope: every repository and sandbox of the maintainer.

The rules this repository's records mean for the payload, each with
whether it was sent and its later home, the personal kit in the config
repository (REQ-0608) unless named otherwise:

- Search for a maintained library before writing one; import, copy a
  small proven piece with its license and origin, or write by hand with
  a stated reason (ADR 0038, and the pending note
  "library-first-in-the-sandbox-payload"). Sent: the payload's #269,
  merged on 2026-10-20. Open: the personal kit, or a product skill
  under `skills/` if adopters get it too.
- State the scope of every decision record (ADR 0039, accepted on
  2026-10-20), as a sentence in the payload's "How to work". Sent: the
  sandbox payload of 2026-10-20 holds the sentence in "How to work",
  measured in this sandbox's `~/.claude/CLAUDE.md`, by the payload's #270,
  merged on 2026-10-20. Open: the personal kit.
- Use the skill `anthropic-skills:skill-creator` and its progressive
  disclosure to create, edit, review or test a skill, and name it in a
  builder brief for such work (ADR 0040, decided on 2026-10-20). Sent:
  the payload's #278, merged on 2026-10-20, with a brief guard row as its
  check. Open: the personal kit, T114.
- A brief names its skills, or is refused (the sandbox definitions
  repository's ADR-0035, its brief guard hook). Sent: in the payload
  since 2026-10-13. Open: `julieta hooks guard` (REQ-0597 to REQ-0599),
  which carries no brief rule today.
- An edit waits for its skill: a Go file for
  `cc-skills-golang:golang-how-to`, a spec for `agent-skills:spec`, a
  plan for `agent-skills:plan`, a skill for
  `anthropic-skills:skill-creator` (the skill-file row is this
  repository's ADR 0040 made a check). Sent: the sandbox definitions
  repository's ADR-0040 and section 15 of its
  `docs/intent/brief-guard-spec.md`, merged on 2026-10-28 in that
  repository's pull requests #307 (Go) and #308 (the table of rows).
  Open, split as decided on 2026-10-28: the mechanism goes to the
  product, `julieta hooks guard` (it reads a table of path globs and the
  skills that satisfy each, refuses an edit a row names until one of its
  skills is loaded, and passes every other edit when it cannot tell, while
  its Bash refusals of REQ-0598 stay fail-closed as REQ-0599 says); the
  rows go to the personal kit (REQ-0608, T114), because the skills they
  name are the maintainer's choice and an adopter may not have those
  plugins. The spec must grow first: REQ-0410 and REQ-0598 read only a
  Bash call's `tool_input.command`, and the rule needs the file tools'
  inputs and the Skill calls. Losing either rule
  is silent: no check fails when the payload's guard goes.
- Never weaken a check to get green: not the gate (disable, skip, lower
  a threshold), and not the code it measures; removing or loosening a
  defensive check, an assertion or a validation in the code under test
  so that a coverage, lint or mutation gate passes is a weakening too,
  and ask-first, and the branch gets a test instead (decided on
  2026-10-29, after two checks of T039 were removed to meet the
  coverage floor and then restored with tests in #315). Sent: the
  sandbox definitions repository's issue #309, opened on 2026-10-29.
  Open: the personal kit in the config repository (REQ-0608, T114).

The payload holds other rules that came from elsewhere; listing them
belongs to the payload's own repository.

Reopen: when the migration of the payload to the config repository is
planned (T114), when a record means another rule for the payload, or when a
payload session reports its pull request.
