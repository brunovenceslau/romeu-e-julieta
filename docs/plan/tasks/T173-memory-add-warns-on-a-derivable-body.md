---
id: T173
title: "memory add warns on a derivable body"
milestone: M2
module: memstore
fate: new
owns:
  - internal/cmd/julieta/memory/derivable*.go
needs:
  - T035
after:
  - T171
ask_first: []
sitting: none
---

# T173 - memory add warns on a derivable body

Goal: the part of "keep out what an instruction file already holds"
(REQ-0660) that a check can judge, as a warning on `memory add`, a
string comparison that costs no model call on a command the agent runs
often. Decided on 2026-10-28, with the specification amended in the
same pull request as T171: 04 4.3 (REQ-1290) and 04 4.1 (the slug).
`after: T171` for the file overlap under `internal/cmd/julieta/memory/`.

1. The rule (REQ-1290). `add` warns `memory-derivable`, and writes the
   entry, when a line of the body of at least 40 bytes, after trimming
   leading and trailing white space, equals byte for byte a line,
   trimmed the same way, of the repo's `CLAUDE.md`, `AGENTS.md`,
   `CLAUDE.local.md` or `.claude/rules/*.md` at the working tree, or of
   a file one of them `@`-imports from inside the repo (the import line
   is `@<path>` alone on a line, as the pinned agent reads it; an import
   outside the repo, or one that does not exist, is skipped). The
   warning names the file and the line number of the match, escaped,
   one line per match, at most ten. An instruction file over 1 MiB is
   skipped and named in the warning's place.
2. The read is on the repo's working tree through `os.Root`, as the
   store reads are, and never follows a symlink out of it; a repo with
   none of the files warns nothing and reads nothing else. The slug is a
   row of 04 4.1 and changes no exit code (REQ-0276).

- Implements: REQ-1290 (04 4.3).
- Operator: no.
- Acceptance:
  - A body whose one line equals a `CLAUDE.md` line of 40 bytes, with
    different surrounding white space, warns `memory-derivable` naming
    `CLAUDE.md` and the line, and the entry is written and listed; a
    39-byte match warns nothing; a match in a file `AGENTS.md`
    `@`-imports from `docs/` warns naming that file; an `@` import to
    `/etc/passwd` or to a missing file is skipped; a symlink from
    `.claude/rules/x.md` out of the repo is skipped; eleven matches
    print ten lines and `1 more`.
  - `add` exits 0 with the warning on stderr and the `--json` document
    unchanged but for the warning list, as the other warnings of 04 4.1
    are carried.
  - REQ-1290 says what the code does; `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/cmd/julieta/memory/...` and
  `go run ./tools/ci reqs`.
