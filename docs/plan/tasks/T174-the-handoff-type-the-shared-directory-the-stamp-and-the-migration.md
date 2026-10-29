---
id: T174
title: "the handoff type, the shared directory, the stamp and the migration"
milestone: M2
module: memstore
fate: new
owns:
  - internal/memstore/shared*.go
  - internal/memstore/handoff*.go
  - internal/memstore/write/**
  - internal/handoff/stamp*.go
  - internal/handoff/measure*.go
  - internal/cmd/julieta/stamp/**
  - internal/cmd/julieta/all/import_stamp.go
  - internal/cmd/julieta/memory/add*.go
  - internal/cmd/julieta/setup/migrate*.go
  - internal/manifest/**
  - e2e/julieta_stamp_test.go
needs:
  - T035
  - T171
after:
  - T166
  - T043
  - T178
ask_first:
  - contracts
sitting: none
---

# T174 - the handoff type, the shared directory, the stamp and the migration

Goal: the store side of ADR 0042: a handoff is a `handoff` entry of the
project's shared directory, the stamp of 03 3.10 replaces the handoff
file, and the handoff files of before are migrated once. Decided on
2026-10-28, with the specification amended in the same pull request as
T171: 03 3.6 (`sharedMemory`), 03 3.9 (REQ-1295 to REQ-1299), 03 3.10
(REQ-0244, REQ-1300), 03 3.12 (REQ-0259), 04 4.3 (REQ-0411, REQ-0413,
REQ-1301), 08 8.1 (REQ-0642, REQ-0645, REQ-0646, REQ-0648 to REQ-0650)
and 08 8.3 (REQ-0676 to REQ-0680, REQ-0683, REQ-0685). `schemas/` is
owned by T166 and the `_shared/` creation of `sync` by T043; this task
amends `schemas/stamp.v1.json` and waits for both (`after`), and T043
gains the one sentence of REQ-0646 below. `internal/manifest` (T135,
merged) gains the optional member `sharedMemory`, with the shape test of
03 3.6 saying it bumps no protocol.

1. The shared directory (REQ-0642, REQ-0645, REQ-0646, REQ-1295).
   `memstore` knows two layouts, the repo dir (`entries/`, `snapshot/`,
   `salvage/`) and the shared dir (`entries/`, `stamps/`); the manifest
   names the shared dir as `sharedMemory`, an optional member (03 3.6),
   and a manifest without it makes `--shared` and a `handoff` a usage
   error naming the member, and `stamp` and `memory index --hook` warn
   `shared-memory-missing` and exit 0 (REQ-1302). A `handoff/` of
   before is admitted as the warning `handoff-legacy` until migrated
   (REQ-0645), never refused. `memory add|list|show|edit --shared` act on
   it; the index line shows `shared`.
2. The `handoff` type (REQ-1296 to REQ-1299, REQ-0676 to REQ-0679).
   Always shared; `--repo` with it is a usage error. `memstore/write`
   validates the headings, and the sixth for `tombstone`, and refuses
   `tombstone` on another type; it refuses a supplied `stamp`, measures
   one at `add`, closes the open `handoff` of the dir under the store
   lock before it writes, and refuses every `edit` of a done `handoff`.
   `add --body-file <path|->` reads the body from a file or stdin
   (REQ-0411). `add` warns `handoff-long` above the narrative bound of
   8.6 (04 4.1); the duplicate rule runs first, and a duplicate closes
   nothing.
3. The stamp (REQ-0244, REQ-1300, REQ-0413, REQ-0668, REQ-0648 to
   REQ-0650). One type in `internal/handoff` (renamed `internal/stamp`
   with the module row of 12 12.2 and the tree of 02 2.1 when the
   old reader leaves, in T172), measured by one function
   over every manifest repo through `gitsafe` (the dirty count of 03
   3.10 as `Status.Changed`, T153), written under the `stamp` key of a
   handoff and as `_shared/stamps/<ULID>.yaml` by `julieta stamp`,
   which keeps the newest 50 and removes the older; `stamps-over-cap`
   warns and never refuses. The `memory-content` digest leaves the
   stamp out (REQ-0259). `schemas/stamp.v1.json` replaces
   `handoff.v1.json`.
4. The reader (REQ-0680, REQ-0683, REQ-0685): one function in
   `memstore`, used by julieta and romeu, that returns the open handoff
   of the shared dir (the newest `createdAt` when a migration left two)
   and the newest stamp; `rm`'s question reads the `tombstone` tag and
   the stamp's generation through it (T068 owns the question).
5. The migration (REQ-1301): `setup`, before `memory check`, turns each
   `handoff/<ULID>-clear|final.md` of a memory dir into a `handoff`
   entry of the shared dir with the file's ULID, tagged `tombstone` for
   `final`, its front matter the `stamp`; each `facts` file into
   `stamps/<ULID>.yaml`, over every repo dir of the manifest; only the
   newest migrated narrative is open; a ULID that already exists is
   skipped, so a run cut short resumes; a file that does not parse or
   fails the heading rule stays and is warned `handoff-legacy`; the
   directory is removed only once empty. A second run finds nothing.
   Under the store lock of both dirs.

Re-plan (2026-10-28): after T178, and through it T177, for the file
overlap of `internal/cmd/julieta/setup/migrate*.go` with their
`internal/cmd/julieta/setup/*_test.go`; that order held through T172
until T177 stopped waiting on it. It costs nothing, since this task
already waits on T043.

- Implements: REQ-1295 (03 3.9), REQ-1296 (03 3.9), REQ-1297 (03 3.9),
  REQ-1298 (03 3.9), REQ-1299 (03 3.9), REQ-1302 (03 3.9), REQ-0237 (03 3.8), REQ-0244 (03 3.10), REQ-1300
  (03 3.10), REQ-0259 (03 3.12), REQ-0411 (04 4.3), REQ-0413 (04 4.3),
  REQ-1301 (04 4.3), REQ-0642 (08 8.1), REQ-0645 (08 8.1), REQ-0648
  (08 8.1), REQ-0649 (08 8.1), REQ-0650 (08 8.1), REQ-0668 (08 8.2),
  REQ-0676 (08 8.3), REQ-0677 (08 8.3), REQ-0678 (08 8.3), REQ-0679
  (08 8.3), REQ-0680 (08 8.3), REQ-0685 (08 8.3).
- Operator: approves the `contracts` surface if a new error id is
  needed for the manifest without `sharedMemory`; otherwise no.
- Acceptance:
  - A manifest without `sharedMemory`: `add --shared` and `add --tag
    handoff` exit 2 naming the member; with it, `add --shared --tag
    project` writes under `_shared/entries/` and `list` shows `shared`
    in the repo column; `memory check` accepts a repo dir with
    `entries/`, `snapshot/`, `salvage/` and a shared dir with
    `entries/`, `stamps/`, and refuses `stamps/` in a repo dir and
    `snapshot/` in the shared one; a repo dir with a `handoff/` of
    before warns `handoff-legacy` in `memory check` and is not refused;
    `--shared --repo x` exits 2.
  - `add --tag handoff --body-file -` with the five headings in order
    writes the entry in the shared dir, with a `stamp` whose repos are
    the manifest's and whose generation is the manifest's; a body
    missing a heading, or with the headings out of order, exits 2
    naming the first wrong one; `--tag tombstone` requires the sixth;
    `--tag project --tag tombstone` exits 2; `--repo x` with `handoff`
    exits 2; a front matter `stamp` in the body file is refused.
  - A second `add --tag handoff` leaves the first `done` and the second
    `open`, under the lock (two concurrent adds leave exactly one
    open); a duplicate of the open handoff prints its id and closes
    nothing; `edit` of the open one changes its body; `edit` of a done
    one exits 2. With a manifest lacking `sharedMemory`, `stamp` exits 0
    and warns `shared-memory-missing`.
  - A body one byte over 4 KiB warns `handoff-long` and is written.
  - `julieta stamp` writes `_shared/stamps/<ULID>.yaml` matching the
    schema, with the dirty count of T153; 51 stamps leave the newest
    50; a `stamps/` planted over the cap warns `stamps-over-cap` in
    `stamp` and in `memory check`, and refuses nothing. Two entries
    equal but for the stamp are duplicates (REQ-0259).
  - The reader returns the open handoff and the newest stamp; a stamp
    file newer than the handoff never hides it (REQ-0685).
  - `setup` on two repo dirs holding, between them, one `clear`, one
    newer `final`, three `facts` files and one file that does not parse
    leaves two `handoff` entries in `_shared/entries/` with the files'
    ULIDs and titles `handoff of <createdAt>`, the `final` one tagged
    `tombstone` and `open`, the older `clear` one `done`, three stamps,
    the unparsed file in place with `handoff-legacy` warned, and the
    other `handoff/` dir gone; a `setup` killed after the entries are
    written and run again finishes with the same result; a second
    `setup` changes nothing (compared byte for byte); the salvage
    reasons read `dirty-at-last-stamp:<n>` (REQ-0237; T061 owns the
    writer and reads this field).
  - The container e2e of `julieta stamp` and of `memory add --tag
    handoff` run the SessionEnd command and a handoff add in a real
    sandbox.
  - The ids above say what the code does; `go run ./tools/ci reqs`
    passes, and `go generate` leaves `schemas/` and `docs/reference`
    unchanged.
- Verify: `go test -race ./internal/memstore/... ./internal/handoff/...
  ./internal/manifest/... ./internal/cmd/julieta/...`, `go test -tags
  e2e ./e2e/... -run '^TestJulietaStamp|^TestJulietaMemory' -v` (10
  10.8), `go run ./tools/ci reqs`.
