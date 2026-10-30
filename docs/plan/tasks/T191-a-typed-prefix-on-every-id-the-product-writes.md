---
id: T191
title: "a typed prefix on every id the product writes: `<kind>_<ulid>` and the closed table of id kinds"
milestone: M2
module: canon
fate: new
owns:
  - internal/canon/id.go
  - internal/canon/id_test.go
  - internal/memstore/*.go
  - internal/cmd/julieta/memory/show*.go
  - internal/cmd/julieta/memory/edit*.go
  - internal/cmd/julieta/memory/list*.go
  - internal/state/candidate_dir.go
  - internal/state/candidate_dir_test.go
  - internal/state/records.go
  - internal/state/records_test.go
  - internal/state/validate.go
  - internal/state/validate_test.go
  - internal/render/types.go
  - internal/salvage/manifest_write.go
  - internal/salvage/capture.go
  - internal/salvage/capture_archive.go
  - internal/salvage/*_test.go
  - docs/spec/03-formats.md
  - docs/spec/08-memory-handoff-salvage.md
  - docs/spec/13-runtime-ledger.md
needs: []
after: []
ask_first:
  - digests
  - gates
  - formats
sitting: none
---

# T191 - a typed prefix on every id the product writes: `<kind>_<ulid>` and the closed table of id kinds

Goal: an id read outside its context says which kind it is. A bare
lowercase ULID of 26 characters names a memory entry, a handoff, a
generation, a salvage, a candidate and a spool event alike, and a
reader of a chat line, a pull request body or a note takes one kind
for another. Decided on 2026-10-28 (the pending note
`docs/handoff/pending/01m61zvp4t4vgbvp0wzw4t7xgr.md`, which T176 moves;
the examples in memory 01m62641frppzgbjd7w19ccxs1), its three open
points on 2026-10-30: the file name carries the prefix too, a bare
ULID is accepted in silence for good, and every ULID the product
writes gets a kind. The rule and the table of kinds are in 03 3.12
(REQ-1319, REQ-1320), written with this task; the file-name lines of
03 3.7, 3.8, 3.9, 3.10, 08 8.1 and 13 13.3 follow. This task is
ordered before T174, which writes the handoff type and the stamp files
(one format change now, no migration later); T174's one-time migration
renames the entry files of before.

1. `canon.ID`: one type in `internal/canon` next to the ULID, a kind
   of a closed Go table (`canon.Mem`, `canon.Hof`, `canon.Stp`,
   `canon.Gen`, `canon.Slv`, `canon.Cnd`, `canon.Spl`) and a ULID;
   `String` and `MarshalText` write `<kind>_<ulid>`; `ParseID(kind, s)`
   takes `<kind>_<ulid>` and the bare 26 characters alike, refuses
   another kind, uppercase, and a second underscore; a decode into a
   record member takes both forms by the member's declared kind. The
   ULID itself keeps its bare `MarshalText`, so a ULID among a digest's
   inputs is digested bare (REQ-1319) and no digest changes.
2. The memory store: an entry file is `entries/mem_<ulid>.md`, or
   `hof_<ulid>.md` for a `handoff` entry (REQ-0241); the reader admits
   the bare name of before; `memory check` warns on a bare name
   (julieta deletes nothing, 08 8.1) and refuses nothing; `add` prints
   the typed id, `show`, `edit` and `list` take both forms (REQ-0656
   stamps the typed `id`). The index line of `memory list` and the
   SessionStart index (T172) print the typed id through `String` and
   need no change of their own; `romeu memory show <id>...` (T070)
   parses through `canon.ID` the same way.
3. The host records: `candidateId` (03 3.7, REQ-0226), `generations[].id`
   and `salvageId`, and `salvage[].id` (REQ-0235) are typed members;
   the candidate dir under the state root is `cnd_<ulid>`, the reader
   and the cleanup of `candidate_dir.go` admit the bare dir name of
   before; the salvage manifest writes the typed id. The approval
   check of I7 (`a.CandidateID != c.id`) compares decoded values, so a
   record of before matches a candidate of now.
4. The stamp (`stamps/stp_<ulid>.yaml`, REQ-1300) and the spool
   (`spool/spl_<ulid>.json`, 13 13.3) are written by T174 and T064
   (left v1); this task fixes their kinds in the table only, and the
   spool layout admits the typed name alone (a hostile input, I32).

- Implements: REQ-1319 (03 3.12), REQ-1320 (03 3.12), REQ-0258
  (03 3.12), REQ-0241 (03 3.9), REQ-0226 (03 3.7), REQ-0235 (03 3.8),
  REQ-0656 (08 8.1), REQ-0333 (04 4.2), REQ-0411 (04 4.3).
- Operator: approves `digests` (the id type lands in `internal/canon`),
  `gates` (`internal/state` records and the candidate dir) and
  `formats` (the `id` and `candidateId` members change their written
  form; no schema pattern constrains them today, so the forecast may
  come to nothing).
- Acceptance:
  - `TestParseID`: a table over the seven kinds, each in its typed and
    its bare form, decodes to the same value; `TestParseIDRefuses`:
    another kind's prefix, uppercase, a second underscore, 25 or 27
    characters, each with the error quoting the input; a fuzz target
    over `ParseID` reaches no panic.
  - `TestIDTextRoundTrip`: `MarshalText` of a decoded bare id writes
    the typed form; a ULID's `MarshalText` stays bare.
  - A grep of the digest inputs of 03 3.12 (the widening set, the
    recreate fields, the normalized render, the fingerprint list of
    3.8) finds no `canon.ID`, recorded in the pull request; a golden
    digest of before the task equals one after it.
  - `TestEntryFileNameIsTyped`: `add` writes `entries/mem_<ulid>.md`;
    a store with one bare-named entry of before lists and shows it,
    `memory check` warns on it once and exits 0.
  - `TestCandidateDirIsTyped`: a new candidate dir is `cnd_<ulid>`; a
    bare-named dir of before is found and cleaned the same way.
  - `julieta memory show 01m6...` and `julieta memory show mem_01m6...`
    print the same entry; `julieta memory show gen_01m6...` exits 2
    with the error naming the kind.
  - `go run ./tools/ci reqs` and `go run ./tools/ci plan` pass.
- Verify: `go test -race ./internal/canon/ ./internal/memstore/...
  ./internal/state/ ./internal/salvage/ -run
  'TestParseID|TestParseIDRefuses|TestIDTextRoundTrip|TestEntryFileNameIsTyped|TestCandidateDirIsTyped'
  -v`.
