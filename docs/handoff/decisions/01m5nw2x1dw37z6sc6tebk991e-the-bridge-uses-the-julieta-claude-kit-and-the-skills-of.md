# The bridge uses the julieta-claude kit and the skills of this repository

What is at stake: medium. If a daily sandbox's definition cannot load
the `julieta-claude` kit, sitting M1 stops at its first wave until a
fallback is decided; undoing it is a revert of the kit entry in each
definition.

Decided on 2026-10-23. Scope: every repository and sandbox of the
maintainer; written in T111, T079, T115, rule 9 and a deferred row of
`docs/plan.md`, and in `docs/notes/julieta-by-hand.md`. When romeu
renders every definition (sitting M3), the rule lives in the rendered
definitions and this record closes.

- The skills of `skills/` are the official ones. The bridge of T111
  carries them, and the session hook lines, through the `julieta-claude`
  kit of T079, pinned at a SHA of this repository; there is no interim
  kit in the config repository.
- The sandbox agent payload's own `handoff` skill is not kept in step
  with `skills/handoff`; in a bridged sandbox the kit is listed after
  the payload's kit, so this repository's skill is the one installed.
- `julieta setup` runs once per created sandbox as a runbook step, since
  the kit's SessionStart hook runs `handoff show --hook` only.
- The first wave is romeu-e-julieta and verona; T112's sandbox gate
  lands during the week of daily use.

Reopen: the kit frontend of a daily sandbox rejects the `julieta-claude`
entry, or T114 merges.
