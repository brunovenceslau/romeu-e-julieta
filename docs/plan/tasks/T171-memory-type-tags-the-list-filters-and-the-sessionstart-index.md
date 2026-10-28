---
id: T171
title: "memory type tags, the list filters and the SessionStart index"
milestone: M2
module: memstore
fate: merged
owns:
  - internal/memstore/entry*.go
  - internal/memstore/memstore.go
  - internal/memstore/reader*.go
  - internal/cmd/julieta/memory/**
  - skills/julieta/**
  - kits/julieta-claude/skills/julieta/**
  - internal/agent/claude/digest*.go
  - e2e/julieta_memory_test.go
needs:
  - T035
after: []
ask_first:
  - kits
sitting: none
merged: 2026-10-28
pr: 292
---

# T171 - memory type tags, the list filters and the SessionStart index

Goal: the memory store speaks the four kinds of note the agent's own
memory records, and `memory list` is the index line the SessionStart
hook of T172 prints. Decided on 2026-10-28, with the specification
amended in the same pull request: 03 3.9 (REQ-0242, REQ-1287,
REQ-1292), 04 4.3 (REQ-0411, REQ-1288, REQ-1289, REQ-1293) and 08 8.1
(REQ-0653, REQ-0660). 03 3.9 and 12 12.7 are owned by T166; this task
amends them without owning the files, as T153 did with 05 5.1, and does
not wait for T166. The generated pages `docs/reference/julieta/memory/*.md`
are regenerated (rule 1).

1. Type tags (REQ-0242, REQ-1287, REQ-1292). `user`, `feedback`,
   `project`, `reference` and `handoff` are the closed set of tags with
   behaviour; `lesson` has none from now on. Every entry carries exactly
   one: `add` without one, `add` or `edit` with two, and an `edit` that
   would leave none, is a usage error (exit 2) that names the five. What
   a `handoff` entry requires beyond that (its headings, its `stamp`,
   its shared location, one open at a time) is T174's. An entry file
   whose tags hold none is read as it is, listed with the type `-`, and
   matched by `list --tag` on its free tags, never on a type tag; the first `edit` that gives it a type
   tag is its migration, so no entry file is rewritten by this task. The `memory-content` digest (03 3.12) is
   unchanged: the tags are already in it.
2. The index line (REQ-1288, REQ-1293, REQ-0653). `list` prints one
   line per entry: `<id> <repo> <status> <type> <updatedAt date>
   <title>`, newest `updatedAt` first, the title escaped through
   `termsafe` as today, so a newline in it never breaks the line;
   `--sort created` orders by `createdAt`. `memstore.Filter` gains
   `Tags` (every given tag must hold), `Since` (RFC 3339 date or
   timestamp, a date read as midnight UTC; `updatedAt` at or after it),
   `Limit` and `Sort`, beside `Query` and `Status`; the readers apply
   them on `os.Root` as the two filters of today are applied. `--json`
   carries the same entries in full, in the same order.
3. `show <id>...` (REQ-1289) takes one or more ids and prints each in
   full, in the order given; an id with no entry fails the command with
   exit 1 after the known ones are printed.
4. The skill (REQ-0660; `skills/julieta/SKILL.md` and
   `references/memory.md`, ask-first `kits`, authored with
   skill-creator, ADR 0040): which type each case takes, in the words
   of 03 3.9; the rule to keep out what the code, the git history or an
   instruction file already holds; that the SessionStart hook prints the
   index and `memory show <id>` reads an entry; and no `lesson` tag. The
   copy under `kits/julieta-claude/skills/julieta/` is kept equal to
   `skills/julieta/` by hand (it was written in the same commit, and no
   tool copies it), so this task changes both; `KitDigest`
   (`internal/agent/claude/digest.go`, 06 6.3, REQ-0587), which a test
   of that package holds to the digest of the kit's files, changes with
   them, and the sandboxes report `skills-stale` until their kit pin
   moves to the merge.

- Implements: REQ-0242 (03 3.9), REQ-1287 (03 3.9), REQ-1292 (03 3.9),
  REQ-0411 (04 4.3), REQ-1288 (04 4.3), REQ-1289 (04 4.3), REQ-1293
  (04 4.3), REQ-0653 (08 8.1), REQ-0660 (08 8.1).
- Operator: approves the `kits` surface (the skill), and reads the
  skill text once, since it is what every session follows.
- Acceptance:
  - `add` with no type tag, `add` or `edit` with two, and an `edit`
    that would leave none, exit 2 and name the five tags; `add --tag
    feedback --tag ci` writes an entry
    whose tags are `[ci, feedback]`; an entry file without a type tag,
    planted in `entries/`, is listed with `-` and is left byte for byte
    as it was by `list` and `show`.
  - `list` prints the line of REQ-1288 per entry, newest `updatedAt`
    first, a title holding a newline and an escape sequence printed on
    one line in `termsafe`'s form, and `--sort created` reorders;
    `--tag a --tag b` matches an
    entry only when both hold; `--since` takes `2026-10-28` and
    `2026-10-28T10:00:00Z` and refuses another form with exit 2;
    `--limit 1` prints one line; the table-driven test runs each filter
    alone and all together, and the `--json` document holds the full
    entries in the same order.
  - `show a b` prints both in order; `show a unknown` prints `a` and
    exits 1 naming `unknown`.
  - The container e2e of `julieta memory` covers `add` with each type,
    the usage errors, and `list --tag user`.
  - The skill names no `lesson` tag and `go run ./tools/ci kits` is
    green with the regenerated digest.
  - REQ-0242, REQ-1287, REQ-1292, REQ-1288, REQ-1293, REQ-1289,
    REQ-0653 and REQ-0660 say what the code does; `go run ./tools/ci
    reqs` passes, and `go generate` leaves `docs/reference` unchanged.
- Verify: `go test -race ./internal/memstore/... ./internal/cmd/julieta/memory/...`,
  `go test -tags e2e ./e2e/... -run '^TestJulietaMemory' -v` (10 10.8),
  `go run ./tools/ci kits` and `go run ./tools/ci reqs`.
