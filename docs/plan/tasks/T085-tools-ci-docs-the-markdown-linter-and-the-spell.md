---
id: T085
title: "`tools/ci docs`: the Markdown linter and the spell checker"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T084
  - T075
after: []
ask_first:
  - checks
  - ask-first
  - dependencies
sitting: none
---

# T085 - `tools/ci docs`: the Markdown linter and the spell checker

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 10 10.2 (the `docs` step), ADR 0001 rule 5, index
  ("Deferred decisions": the linter and spell checker rows), S10.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`, `ask-first`,
  `dependencies` (if a tool enters `mise.lock`).
- Acceptance:
  - The pull request picks the Markdown linter and the spell checker,
    puts their configuration files on the `checks` globs of
    `.github/ask-first.yaml`, and fixes the format and path of the
    accepted-words list. The Deferred decisions table says this pull
    request is the one that picks them, so the choice is not a task of
    its own; the two rows leave the table in the same change.
  - A fixture fails each tool: one lint violation and one misspelled
    word.
  - A tool that enters `mise.lock` through an `npm:` or `pipx:` backend
    has a checksum-locked entry and runs no post-install script; the
    front-matter and link rules landed in T006.
- Verify: `go test ./tools/ci/... -run '^TestDocs'`, which runs the
  fixture lines; `go run ./tools/ci docs`.
