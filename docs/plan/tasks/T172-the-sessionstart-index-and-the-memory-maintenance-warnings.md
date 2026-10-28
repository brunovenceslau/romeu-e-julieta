---
id: T172
title: "the SessionStart index and the memory maintenance warnings"
milestone: M2
module: handoff
fate: new
owns:
  - internal/handoff/print*.go
  - internal/cmd/julieta/handoff/show*.go
  - internal/cmd/julieta/handoff/hook.go
  - internal/cmd/julieta/setup/memory_warnings*.go
  - e2e/julieta_handoff_test.go
needs:
  - T059
  - T171
after: []
ask_first: []
sitting: none
---

# T172 - the SessionStart index and the memory maintenance warnings

Goal: the SessionStart hook prints an index of the open memory entries,
one line each, as the agent's own memory loads its index at a session's
start, and two warnings keep that index small without a hand. Decided
on 2026-10-28, with the specification amended in the same pull request
as T171: 08 8.1 (REQ-0659), 08 8.2 (REQ-0664, REQ-0665, REQ-0667,
REQ-1291, REQ-1294), 08 8.6 (two starting values), 04 4.1 (two slugs)
and 04 4.3 (REQ-0414). The generated pages
`docs/reference/julieta/handoff/*.md` are regenerated (rule 1).

1. The index (REQ-0659, REQ-0664, REQ-0414). Parts (5) and (6) of the
   hook output become one part: the line of `memory list` (T171,
   REQ-1288) per open entry, over every manifest repo's memory dir,
   newest `updatedAt` first, under the line
   `read an entry: julieta memory show <id>`. No body is printed and no
   tag is printed apart. `handoff show` without `--hook` prints the same
   index in place of the open entries.
2. The cut (REQ-0667). Two bounds of 8.6, each cut at a line: parts (2)
   to (4) by the narrative bound (4 KiB), the narrative cut by whole
   lines with `read the rest: julieta handoff show` last, and the index
   by its own bound (24 KiB), every whole line that fits, never a
   partial one, with `<n> more entries: julieta memory list` last when
   any is left out. `internal/handoff/print.go` has the footer and cuts the
   entries today by whole entry and the narrative by line; the index is
   a new block cut by line, and its `reserve` arithmetic changes with
   it.
3. The warnings (REQ-1291, REQ-1294, REQ-0665, 04 4.1).
   `memory-open-count`, one line, when the open entries of the project
   exceed the count of 8.6 (100, below what the index bound holds), and
   `memory-stale`, one line naming id
   and escaped title for each open entry whose `updatedAt` is older than
   the days of 8.6 (90), oldest first, at most ten lines and then `<n>
   more stale entries: julieta memory list --status open`. Both are
   computed from the entries the hook already reads and printed in part
   (1) with julieta's other warnings, never cut; `julieta status` is not
   changed by this task. The four values are constants with their 8.6
   rows cited.

- Implements: REQ-0659 (08 8.1), REQ-0664 (08 8.2), REQ-0665 (08 8.2),
  REQ-0667 (08 8.2), REQ-1291 (08 8.2), REQ-1294 (08 8.2), REQ-0414
  (04 4.3).
- Operator: no.
- Acceptance:
  - With three open entries of two repos and one done entry, `handoff
    show --hook` prints the warnings, the fixed line, the narrative,
    the facts and then the `read an entry` line followed by three index
    lines, newest `updatedAt` first, each labelled with its repo, and
    no body; the done entry is absent; a golden file pins the output.
  - With enough entries that the index passes 24 KiB, the output ends in
    a whole index line and the footer `<n> more entries: julieta memory
    list`, `n` equal to the lines left out, and the warnings above are
    complete; the test plants a title that would straddle the bound and
    reads it absent, not cut. A narrative over 4 KiB is cut at a line
    and ends in `read the rest: julieta handoff show`, and the index
    after it is whole.
  - With 101 open entries, `memory-open-count` is printed, once; with
    100, it is not. An open entry whose `updatedAt` is 91 days before
    the clock the test injects prints one `memory-stale` line with its
    id and title; one of 89 days prints none; a done entry prints none;
    eleven stale entries print ten lines, oldest first, and `1 more
    stale entries: julieta memory list --status open`.
  - The container e2e of the hooks reads the `read an entry` line and
    an index line after `/clear`'s SessionStart (J, 09 step 3).
  - REQ-0659, REQ-0664, REQ-0665, REQ-0667, REQ-1291, REQ-1294 and
    REQ-0414 say what the code does; `go run ./tools/ci reqs` passes,
    and `go generate` leaves `docs/reference` unchanged.
- Verify: `go test -race ./internal/handoff/... ./internal/cmd/julieta/handoff/... ./internal/cmd/julieta/setup/...`,
  `go test -tags e2e ./e2e/... -run '^TestJulietaHandoff|^TestJulietaHooks' -v`
  (10 10.8), and `go run ./tools/ci reqs`.
