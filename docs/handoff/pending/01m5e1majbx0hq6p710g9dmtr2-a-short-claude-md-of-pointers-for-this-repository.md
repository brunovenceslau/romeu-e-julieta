# A short CLAUDE.md of pointers for this repository

What is at stake: small. A session that starts here without the resume
prompt finds no pointer to this repository's own rules (the ask-first
surfaces, ADR 0038, ADR 0039, ADR 0040, `go run ./tools/ci all`); it
has only the sandbox payload's instructions, which state rules for every
repository and none of this one's.

Scope: this repository.

Measured on 2026-10-20: this repository tracks no `CLAUDE.md` and no
`AGENTS.md`. Of the repositories the sandbox kit runs, five track a
`CLAUDE.md` of about 960 to 3 180 tokens (a length over 4); none copies
a line of the payload's file, and some repeat one of its rules in other
words, so the same rule lives in more than one place.

Decided on 2026-10-20:

- No repository's instructions tell the agent to follow the payload's
  file: inside the sandbox the harness loads it in every session, and
  outside it (the host, CI, an adopter) the file does not exist; in a
  public repository the pointer would also tie public text to an
  interim, private payload.
- The payload holds the rules for every repository; a repository's file
  holds only its own facts and pointers, and states any exception to
  the payload as an exception.

Proposed, to build when reopened: a `CLAUDE.md` here of pointers only,
no rule restated: where the ask-first surfaces and grants live, the
three ADRs above, the local CI command and the gotchas it needs on a
recreated sandbox, and `docs/handoff/now.md` for the current state.
Whether the product's adopters get such a file is a separate question
(06 6.5 names only the personal kit's instructions file); not measured.

Reopen: a session here that misses one of these rules without the
resume prompt, or when the payload moves to the personal kit (T114).
