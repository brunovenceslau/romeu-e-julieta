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
  - internal/cmd/julieta/memory/edit*.go
  - internal/cmd/julieta/memory/list*.go
  - internal/cmd/julieta/memory/memory*.go
  - internal/cmd/julieta/memory/show*.go
  - internal/cmd/julieta/memory/target*.go
  - internal/spec/rules*.go
  - skills/julieta/**
  - kits/julieta-claude/skills/julieta/**
  - internal/cmd/julieta/setup/migrate*.go
  - internal/manifest/**
  - internal/render/env.go
  - internal/render/env_test.go
  - internal/cmd/romeu/sync/step4_tree.go
  - internal/cmd/romeu/sync/step5_memory.go
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
  - formats
  - gates
  - kits
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
03 3.6 saying it bumps no protocol. The mount of the shared dir,
`./memory/_shared` beside the repo dirs of `sbxenv.yaml`
(`internal/render/env.go`), and the manifest of `RJ-313` sized with the
member are this task's too: T043's re-plan of 2026-10-28 gave the member
and the mount to T043, which cannot write a member `internal/manifest`
does not have before this task (decided on 2026-10-29).

1. The shared directory (REQ-0642, REQ-0645, REQ-0646, REQ-1295).
   `memstore` knows two layouts, the repo dir (`entries/`, `snapshot/`,
   `salvage/`) and the shared dir (`entries/`, `stamps/`); the manifest
   names the shared dir as `sharedMemory`, an optional member (03 3.6),
   and a manifest without it makes `--shared` and a `handoff` a usage
   error naming the member, and `stamp` and `memory index --hook` warn
   `shared-memory-missing` and exit 0 (REQ-1302). A `handoff/` of
   before is admitted as the warning `handoff-legacy` until migrated
   (REQ-0645), never refused. `memory add --shared` files in it and
   `list --shared` reads it (`show` and `edit` take no target, item 6);
   the index line shows `shared`.
2. The `handoff` type (REQ-1296 to REQ-1299, REQ-0676 to REQ-0679).
   Always shared; its target, `--repo` or `-C`, names the repo the
   narrative is about, and `--shared` with it is a usage error (item
   6). `memstore/write`
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
6. The explicit target (REQ-1318 of 03 3.9, REQ-0411, REQ-1290,
   REQ-0677, REQ-0660; decided on 2026-10-29, after a session's notes
   on a repo outside the manifest were filed under this one, since the
   agent's shell does not keep the working directory between commands).
   `add` takes exactly one of `--repo <dir|url>`, `-C <path>` and
   `--shared`, through one `pflag.Value` per flag that refuses a second
   `Set`, so a repeated flag is the parser's usage error (`RJ-349`);
   none or two is the same usage error, its hint the `dir` and `url` of
   every manifest repo. `--repo` matches a `dir`, or a `url` after the
   normalization of REQ-0177; `-C` is made absolute against the working
   directory, cleaned, and matched against each repo's `path` by whole
   components (`filepath.Rel` with no `..`, never a string prefix), the
   deepest match winning; a value in no repo is the usage error
   `memory-target` (exit 2) with the same hint, a new id declared in
   the package's `errors.go` with its row of 04 4.4 (the `contracts`
   tick), which also replaces the `RJ-348` path `resolve` takes today
   for a working directory in no repo. `edit`, `show` and `list` locate
   by id over every memory dir of the project, an id in two dirs
   failing (exit 1) with both paths, `list` narrowed by `--repo` or
   `--shared`, and the `--repo` of `show` and `edit` is removed. A
   `handoff` takes `--repo` or `-C` and never `--shared`, and its front
   matter gains `repo`, the matched repo's `url`, refused on any other
   type (REQ-1296), with `schemas/memory-entry.v1.json` regenerated for
   the key (the `formats` tick); the reader shows a `repo` the manifest
   no longer holds as text. The spec rule `repo-url-unique` (REQ-0177)
   joins `repo-dir-unique`, and both manifest readers refuse a repeated
   `dir` or `url` (REQ-1317 of 03 3.6). `memory-derivable` reads the
   instruction files of the target repo, the primary's for `--shared`.
   The `julieta` skill (`skills/julieta/SKILL.md`, its
   `references/memory.md`, and the copy under
   `kits/julieta-claude/skills/julieta/`) names the target in every
   example and drops the "wrong repo" advice that rests on the working
   directory, following skill-creator (ADR 0040); the `handoff` skill is
   T172's, which gains the `--repo <dir>` example (decided on
   2026-10-29), and either task may land first, since `--repo` is a
   flag of `add` today.

Re-plan (2026-10-28): after T178, and through it T177, for the file
overlap of `internal/cmd/julieta/setup/migrate*.go` with their
`internal/cmd/julieta/setup/*_test.go`; that order held through T172
until T177 stopped waiting on it. It costs nothing, since this task
already waits on T043.

- Implements: REQ-0140 (02 2.3), REQ-0177 (03 3.1), REQ-1317 (03 3.6),
  REQ-1295 (03 3.9), REQ-1318 (03 3.9), REQ-1296 (03 3.9), REQ-1297 (03 3.9),
  REQ-1298 (03 3.9), REQ-1299 (03 3.9), REQ-1302 (03 3.9), REQ-0237 (03 3.8), REQ-0244 (03 3.10), REQ-1300
  (03 3.10), REQ-0259 (03 3.12), REQ-0411 (04 4.3), REQ-0413 (04 4.3),
  REQ-1290 (04 4.3), REQ-1301 (04 4.3), REQ-0642 (08 8.1), REQ-0645
  (08 8.1), REQ-0648 (08 8.1), REQ-0649 (08 8.1), REQ-0650 (08 8.1),
  REQ-0660 (08 8.1), REQ-0668 (08 8.2),
  REQ-0676 (08 8.3), REQ-0677 (08 8.3), REQ-0678 (08 8.3), REQ-0679
  (08 8.3), REQ-0680 (08 8.3), REQ-0685 (08 8.3).
- Operator: approves the `contracts` surface for the error id of a
  target in no repo of the manifest (item 6), and for a new id for the
  manifest without `sharedMemory` if one is needed; the `gates` surface
  for the rule `repo-url-unique` in `internal/spec/rules.go`; and the
  `kits` surface for the `julieta` skill and its kit copy.
- Acceptance:
  - `romeu sync` step 4 creates `memory/_shared/stamps/` beside
    `entries/`, and step 5 warns when `stamps/` is over its cap: T043
    makes `_shared/` with `entries/` only, since memstore's allowlist
    has no `stamps/` before this task (decided on 2026-10-29).
  - After `sync`, the manifest holds `sharedMemory`, the render golden
    lists `./memory/_shared` beside the repo dirs, and the manifest
    sized for `RJ-313` counts the member.
  - A manifest without `sharedMemory`: `add --shared` and `add --tag
    handoff` exit 2 naming the member; with it, `add --shared --tag
    project` writes under `_shared/entries/` and `list` shows `shared`
    in the repo column; `memory check` accepts a repo dir with
    `entries/`, `snapshot/`, `salvage/` and a shared dir with
    `entries/`, `stamps/`, and refuses `stamps/` in a repo dir and
    `snapshot/` in the shared one; a repo dir with a `handoff/` of
    before warns `handoff-legacy` in `memory check` and is not refused;
    `--shared --repo a` exits 2 (two targets, item 6).
  - `add --tag handoff --repo a --body-file -` with the five headings in order
    writes the entry in the shared dir, with a `stamp` whose repos are
    the manifest's and whose generation is the manifest's; a body
    missing a heading, or with the headings out of order, exits 2
    naming the first wrong one; `--tag tombstone` requires the sixth;
    `--tag project --tag tombstone` exits 2; `--shared` with `handoff`
    exits 2, and `--repo x` with `x` in no manifest repo exits 2 as
    `memory-target`; a front matter `stamp` in the body file is refused.
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
  - The explicit target (item 6), with a manifest of two repos, `a` at
    `/w/a` and `b` at `/w/a/b`, run from `/w/a`: `add` with no target,
    with `--repo a --shared`, with `--repo a --repo b`, with `-C . --repo
    a`, with `--repo c` and with `-C /w/x` each exit 2, and the hint of
    each names `a`, `b` and their urls; `--repo a`, `--repo
    https://github.com/o/a` and `--repo https://github.com/O/A.git` file
    under `a`; `-C /w/a/b/src` files under `b` and `-C /w/ab` exits 2;
    `add --tag handoff --repo b --body-file -` writes in `_shared/` with
    `repo: https://github.com/o/b`, `--tag handoff --shared` exits 2,
    and `--tag project --repo a` with a body front matter `repo` exits
    2; `edit <id>` and `show <id>` find an entry of `b` run from `/w/a`
    with no target, and `list` prints the entries of both repos and the
    shared dir, `list --repo b` only `b`'s; a handoff whose `repo` is in
    no manifest repo is listed with the url as text; a manifest with
    two repos of `dir` `a` and `A`, or of urls `https://github.com/o/a`
    and `https://github.com/O/A.git`, is refused by julieta and by
    `sync` naming the repeated value, and `julieta spec validate`
    reports `repo-url-unique`; `add --shared` warns `memory-derivable`
    against the primary repo's instruction files, and `add --repo b`
    against `b`'s and not `a`'s; `--shared --shared` and `-C x -C y`
    exit 2 as `RJ-349`; `-C b/src` run from `/w/a` and `-C /w/a` file
    under `b` and `a`; `edit <id> --title x` of a handoff leaves `repo`
    as it was, the url as the manifest holds it (`https://github.com/O/B`
    when the manifest says so, not lowercased); `list --shared` prints
    the shared dir's entries only; `show` and `edit` of an unknown id
    exit 1, and of an id present in two dirs exit 1 naming both paths;
    the `julieta` skill's examples carry a target.
  - The ids above say what the code does; `go run ./tools/ci reqs`
    passes, and `go generate` leaves `schemas/` and `docs/reference`
    unchanged.
- Verify: `go test -race ./internal/memstore/... ./internal/handoff/...
  ./internal/manifest/... ./internal/cmd/julieta/...`, `go test -tags
  e2e ./e2e/... -run '^TestJulietaStamp|^TestJulietaMemory' -v` (10
  10.8), `go run ./tools/ci reqs`.
