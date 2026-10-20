# supersede and link in tools/new adr

- `go run ./tools/new adr` writes a new record in the adr-tools layout
  (rule 6 of
  [ADR 0001, adopt a documentation standard with checkable rules and a voice](../../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)), and `tools/ci sequences` checks the links. What
  adr-tools has and it lacks: `-s`, which writes the `Superseded by`
  line into the old record, `-l` for a named link, and the link graph.
  Decided on 2026-10-19: keep `tools/new` rather than vendor adr-tools
  (a shell tool with no commit upstream since 2020, whose index would
  differ from the generated one).
- What is at stake: a supersede written by hand fails `tools/ci
  sequences` until both links match, a few minutes lost.

Reopen: the next record that supersedes or amends another by hand.
