---
id: T172
title: "`julieta memory index`, the SessionStart renderer and the memory maintenance warnings"
milestone: M2
module: handoff
fate: new
owns:
  - internal/handoff/index*.go
  - internal/handoff/print*.go
  - internal/handoff/testdata/**
  - internal/cmd/julieta/handoff/**
  - internal/cmd/julieta/memory/index*.go
  - internal/cmd/julieta/setup/memory_warnings*.go
  - kits/julieta-claude/managed-settings.json
  - skills/handoff/**
  - kits/julieta-claude/skills/handoff/**
  - e2e/julieta_index_test.go
needs:
  - T059
  - T171
  - T174
after: []
ask_first:
  - kits
sitting: none
---

# T172 - `julieta memory index`, the SessionStart renderer and the memory maintenance warnings

Earlier title: the SessionStart index and the memory maintenance
warnings.

Goal: `julieta memory index [--hook]` replaces `julieta handoff show`:
the SessionStart prints the open handoff and an index of the other open
entries, as the agent's own memory loads its index at a session's
start, and two warnings keep that index small without a hand. Decided
on 2026-10-28, with the specification amended in the same pull request
as T171: 08 8.1 (REQ-0659), 08 8.2 (REQ-0663, REQ-0664, REQ-0665,
REQ-0667, REQ-0669, REQ-1291, REQ-1294), 08 8.3 (REQ-0681, REQ-0682),
08 8.6 (four starting values), 04 4.1 (the slugs) and 04 4.3
(REQ-0414). The hook lines of the kit and the `handoff` skill (06 6.3,
REQ-0595; ask-first `kits`) change with the command, and
`internal/handoff` and `internal/cmd/julieta/handoff` are removed once
T174 and this task carry what they did. The generated pages
`docs/reference/julieta/memory/*.md` are regenerated (rule 1).

1. The output (REQ-0659, REQ-0664, REQ-0414). In order: (1) the
   warnings; (2) the fixed line; (3) the open `handoff` of the shared
   dir: its index line, the gap line of REQ-0681 when the newest stamp
   differs from its stamp, and its body, or the gap line alone when no
   handoff is open; (4) `read an entry: julieta memory show <id>` and
   one line per other open entry over every memory dir of the project,
   the line of `memory list` (T171), newest `updatedAt` first, `shared`
   in the repo column for the shared dir. Without `--hook`, parts (3)
   and (4) only.
2. The cut (REQ-0667). Two bounds of 8.6, each cut at a line: part (3)
   by the narrative bound (4 KiB), the body cut by whole lines with
   `read the rest: julieta memory show <id>` last, and part (4) by the
   index bound (24 KiB), every whole line that fits, with `<n> more
   entries: julieta memory list` last when any is left out. The
   renderer of `internal/handoff/print.go` cut by whole entry; the index
   is a new block cut by line in `internal/handoff/index.go`, with its
   own `reserve` arithmetic; `print.go` and `internal/cmd/julieta/handoff`
   leave with `handoff show`, and the package is renamed `internal/stamp`
   with the row of 12 12.2 and the tree of 02 2.1.
3. The warnings (REQ-1291, REQ-1294, REQ-0665, 04 4.1).
   `memory-open-count`, one line, when the open entries of the project,
   the handoff apart, exceed 100; `memory-stale`, one line naming id and
   escaped title for each open entry whose `updatedAt` is older than 90
   days, oldest first, at most ten lines and then `<n> more stale
   entries: julieta memory list --status open`. Computed from the
   entries the command already reads, printed in part (1), never cut;
   `julieta status` is not changed by this task. The values are
   constants with their 8.6 rows cited.
4. The state (REQ-0669): the time of the last `memory index --hook`
   run is recorded where `handoff show --hook` recorded it, so 8.5 step
   1 reads the same key.
5. The kit and the skill (REQ-0663, REQ-0595, REQ-0676): the SessionStart
   hook lines of `kits/julieta-claude/managed-settings.json` call
   `memory index --hook` and the SessionEnd line `stamp`; the `handoff`
   skill pipes the narrative to `julieta memory add --tag handoff
   --body-file -`, with `--tag tombstone` for `--final`, asks for a
   title that is the Goal in one line prefixed `handoff:`, and keeps the
   headings the store requires (the docs test of REQ-1115 ties them);
   `KitDigest` (T171) changes with them.

- Implements: REQ-0659 (08 8.1), REQ-0663 (08 8.2), REQ-0664 (08 8.2),
  REQ-0665 (08 8.2), REQ-0667 (08 8.2), REQ-0669 (08 8.2), REQ-1291
  (08 8.2), REQ-1294 (08 8.2), REQ-0681 (08 8.3), REQ-0682 (08 8.3),
  REQ-0414 (04 4.3), REQ-0595 (06 6.4).
- Operator: approves the `kits` surface (the hook lines and the
  `handoff` skill), and reads the skill once.
- Acceptance:
  - With one open handoff, three open entries of two repos and the
    shared dir, and one done entry, `memory index --hook` prints the
    warnings, the fixed line, the handoff's index line, no gap line
    when the newest stamp equals its stamp, its body, then the `read an
    entry` line and three index lines, newest `updatedAt` first, each
    labelled with its repo or `shared`, no body; the done entry is
    absent; a golden file pins the output. With a newer stamp whose
    head moved, the gap line names the commits, the branch and the
    dirty count (REQ-0681, REQ-0682); with no open handoff, the gap
    line stands alone in part (3).
  - A handoff body over 4 KiB is cut at a line and ends in `read the
    rest: julieta memory show <id>`; with enough entries that the index
    passes 24 KiB, the output ends in a whole index line and the footer
    with `n` equal to the lines left out; a title that would straddle
    the bound is absent, not cut; the warnings above are complete.
  - With 101 open entries besides the handoff, `memory-open-count` is
    printed once; with 100, it is not. An entry 91 days old by the
    injected clock prints one `memory-stale` line; 89 days prints
    none; a done entry prints none; eleven stale entries print ten
    lines, oldest first, and `1 more stale entries: julieta memory list
    --status open`.
  - The container e2e runs `/handoff` (the skill's command) then
    `/clear`'s two hooks in both orders and reads the handoff first,
    no gap line, then a gap line after a commit (J5; Q22).
  - The kit's hook lines name the two commands and `tools/ci kits` is
    green with the regenerated digest; the skill names no `julieta
    handoff` command.
  - The ids above say what the code does; `go run ./tools/ci reqs`
    passes, and `go generate` leaves `docs/reference` unchanged.
- Verify: `go test -race ./internal/handoff/... ./internal/cmd/julieta/...`,
  `go test -tags e2e ./e2e/... -run '^TestJulietaIndex|^TestJulietaHooks' -v`
  (10 10.8), `go run ./tools/ci kits` and `go run ./tools/ci reqs`.
