# Warning slugs temp-file and handoff-cap in 04 4.1

What is at stake: small. Without slugs, the two warnings of `julieta
memory check` print as `note` lines that no test of 04 4.1, Warnings
reads, so a reader or a script cannot match them by slug; a slug added
later renames one line of output.

Scope: this repository.

Deferred on 2026-10-20, while building T035:

- REQ-0654 (08 8.1) says `memory check` and `sync` warn on a leftover
  temporary file `.tmp-<ULID>`, and REQ-0650 says a `handoff/` over its
  cap warns in `sync` and `run`.
- REQ-0276 (04 4.1, Warnings) lists every warning slug, and `cli.Warn`
  takes only a slug of that list; neither warning has one.
- Until they do, `memory check` prints `note temp-file <path>: ...` and
  `note handoff-cap <path>: ...` on stderr, with the rule names of
  `memstore` (`RuleTemp`, `RuleHandoffCap`), and changes no exit code.
  The T035 task file records this.

Reopen: the next spec pull request on 04 4.1.
