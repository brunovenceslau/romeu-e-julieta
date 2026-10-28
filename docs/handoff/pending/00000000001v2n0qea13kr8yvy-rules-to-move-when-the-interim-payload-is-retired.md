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
  which carries no brief rule today; the destination is the product's
  guard, not the personal kit.
- An edit waits for its skill: a Go file for
  `cc-skills-golang:golang-how-to`, and, in a follow-up pull request of
  that repository, a spec, plan or skill file for `agent-skills:spec`,
  `agent-skills:plan` or `anthropic-skills:skill-creator` (the skill row
  is this repository's ADR 0040 made a check). Recorded in that
  repository's ADR-0040 and section 15 of its
  `docs/intent/brief-guard-spec.md`, decided on 2026-10-28 in its pull
  request #307. Open: `julieta hooks guard`, which carries no edit rule
  today. Losing either rule is silent: no check fails when the payload's
  guard goes.

The payload holds other rules that came from elsewhere; listing them
belongs to the payload's own repository.

Reopen: when the migration of the payload to the config repository is
planned (T114), when a record means another rule for the payload, or when a
payload session reports its pull request.
