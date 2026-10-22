# 41. Readers of formats between binaries ignore unknown members

Date: 2026-10-22

## Status

Proposed

## Context

REQ-0163 said that any change to a format's fields bumps its version,
and the readers of the formats the product owns decoded strictly, an
unknown member being an error. On 2026-10-21, while T154 added a member
to `julieta-status/v1`, the maintainer decided that formats evolve by
the rules of protobuf (the note "Formats evolve by the rules of
protobuf" under `docs/handoff/decisions/`): adding a member is backward
compatible and keeps the version.

What is at stake: medium. A reader that refuses an unknown member makes
every added member a break between two binaries that ship apart. A
romeu on the host and a julieta in a sandbox are upgraded at different
times, and a newer julieta that adds a member to `julieta-status/v1`,
`snapshot/heads.json`, a memory entry or a handoff file makes an older
reader fail on a file that is valid for its version. Undoing the rule is
a spec edit and the return of the strict option on each reader.

Scope: this repository, for every format the product writes. It is
written in this record, in REQ-0163 (`docs/spec/03-formats.md`), in the
decoders listed in T162, in `schemas/` (which keep their strict form,
see Decision) and in the decision note named above; no other home
carries it.

The readers fall in two groups:

- A format one binary writes and another reads: the `--json` documents
  of `julieta status`, `setup`, `handoff` and `memory`,
  `snapshot/heads.json`, the memory entry and the handoff file. Their
  writer is a program of a release that can be newer than the reader's.
- A file a person writes: the project spec, host settings, the catalog
  and the configuration files of `tools/ci`. An unknown key there is most
  often a typing error, and the person is present to fix it.

The manifest is written by romeu and read by julieta, and it has its own
window of the protocol number N and N-1 (REQ-0219) with an error for
what falls outside it. The decision note lists it with the strict
readers, and this record leaves it strict; see Consequences.

### Alternatives considered

- **Bump the version on every added member.** The state before this
  record. Taken away: it forces both binaries to move in step for a
  change that breaks nobody.
- **Ignore unknown members in every reader.** Not taken: a person's typo
  (`pulll: true`) would be accepted and ignored with no message.
- **Keep the strict readers and warn on an unknown member.** Not taken:
  a warning in a status document is an error for the program that reads
  it, and none of these readers has a channel for it.
- **Decide by the writer: ignore unknown members in what a program
  writes, refuse them in what a person writes.** Taken.

## Decision

Decided on 2026-10-22, from the decision of 2026-10-21:

- Adding a member to a format keeps its version. Removing or renaming a
  member, or changing its type or meaning, is a new version, and the
  name of a removed member is never used again.
- A reader of a format one binary writes and another reads skips a
  member it does not know, at any depth. It keeps every other rule of
  strict decoding: a duplicate member, the size cap, UTF-8, a missing
  member, and the check of the schema field, where a newer version is
  `format-newer` (RJ-311).
- A reader of a file a person writes refuses an unknown key.
- The JSON Schema of a format in `schemas/` keeps `additionalProperties:
  false`: it describes the document the version of this build writes,
  and the examples that CI validates against it.

REQ-0159, REQ-0160, REQ-0163, REQ-0261 and REQ-0271 are amended to say
it, and T162 changes the readers.

## Consequences

- A newer writer can add a member without a new version. A reader that
  skips a member and then rewrites the file drops it, and this is
  accepted at two places: `julieta memory edit` rewrites an entry
  whole, so an older julieta that edits an entry a newer julieta wrote
  loses the member; and the hooks that record a disabled snapshot
  (`internal/hooks/actions/disabled.go`) read `snapshot/heads.json` and
  write it back without the members they do not know. A handoff file is
  never rewritten: `julieta handoff write` only creates a new one. It is the cost of
  skipping rather than keeping unknown members, as a protobuf runtime
  that does not keep unknown fields has it. Reopened when a member of
  such a format holds data that must survive a rewrite.
- The YAML depth cap of the memory entry (`internal/memstore/entry.go`,
  `maxDepth`, 4) and of the handoff file (`internal/handoff/decode.go`,
  5) also bounds what a newer writer may add: a member nested deeper is
  refused, as it is today.
- The manifest is still strict, and a member added to `manifest.v1` is
  still a new protocol number. This is an open question: the decision
  note of 2026-10-21 lists the manifest among the files a person writes,
  which it is not, and the choice between the protocol window and the
  rule above is the maintainer's. The spec says so (03, opening and
  3.6). Reopened when the maintainer decides.
- The state files of julieta (`setup.json`, the hooks record, the hook
  failures and the system hooks path) are written and read by the same
  binary, and stay strict. Reopened if a second program reads one.
- The pending note "Readers of formats between binaries ignore unknown
  members" is closed by T162; the decision note of 2026-10-21 points
  here.
