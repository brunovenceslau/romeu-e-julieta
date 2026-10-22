---
id: T159
title: "An AGENTS.md of pointers and a CLAUDE.md that imports it"
milestone: none
module: none
fate: new
owns:
  - AGENTS.md
  - CLAUDE.md
  - REUSE.toml
needs: []
after: []
ask_first: []
sitting: none
---

# T159 - An AGENTS.md of pointers and a CLAUDE.md that imports it

Goal: the file of "A short CLAUDE.md of pointers for this repository"
(01m5e1majbx0hq6p710g9dmtr2), with the decision of 2026-10-21 ("Work
authorized while the operator is away on 2026-10-21",
01m5eg0qefcvwdx05q4vnqcwfe): the instructions file is an `AGENTS.md` of
pointers, and a one-line `CLAUDE.md` imports it; no rule is restated.

A session here that starts without the resume prompt finds a pointer to
this repository's own rules. The file states no rule: it names where
each lives, so a rule has one home. Claude Code reads a `CLAUDE.md`
before an `AGENTS.md` in the same directory and expands an
`@AGENTS.md` line in it (the memory page of the Claude Code
documentation, "Import additional files" and "AGENTS.md", read on
2026-10-22), so other agents read `AGENTS.md` and Claude Code reads the
same text through the import.

1. `AGENTS.md` at the root: pointers to `docs/handoff/now.md`, the
   spec and the plan, `.github/ask-first.yaml` and 12 12.4, ADR 0038,
   0039 and 0040 with one line each, the local CI command and its
   parts, and `docs/handoff/gotchas/`. It has no pointer to the sandbox
   payload: inside the sandbox the harness loads the payload in every
   session, and outside it the file does not exist.
2. `CLAUDE.md` at the root: the line `@AGENTS.md`.
3. `REUSE.toml` gains both files in its first annotation, as for
   `README.md`; the tree of 02 2.1 lists both (the file is owned by
   other open tasks, so this task amends two lines without owning it,
   as T157 did for 05).

- Implements: none (a file for agents; no requirement row).
- Operator: no action at merge. Whether the product's adopters get such
  a file is a separate question (06 6.5), not measured.
- Acceptance:
  - `AGENTS.md` names only paths that exist, and holds no rule restated
    from the sandbox payload or from an ADR.
  - `CLAUDE.md` is the one line `@AGENTS.md`.
  - `go run ./tools/ci all` passes with both files tracked.
- Verify: `go run ./tools/ci all`; in a Claude Code session started at
  the root, `/context` lists `CLAUDE.md` under Memory files.
