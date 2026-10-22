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
written in this record, in REQ-0163 and the must-understand rule of 3.6
(`docs/spec/03-formats.md`), in the decoders listed in T162 and T163,
in `schemas/` (which keep their strict form, see Decision) and in the
decision note named above; no other home carries it.

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
what falls outside it. The decision note of 2026-10-21 listed it with the
strict readers, wrongly, since romeu writes it. The first version of
this record left it strict as an open question; the maintainer decided
it on 2026-10-22, see Decision.

### Alternatives considered

- **Bump the version on every added member.** The state before this
  record. Taken away: it forces both binaries to move in step for a
  change that breaks nobody.
- **Keep the manifest strict, and bump the protocol number for every
  added member.** Not taken: an optional member would force a paired
  upgrade of a romeu on the host and the julieta in a sandbox, which is
  built from a host bin dir and is not upgraded with it.
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

- The manifest (`manifest.v1`) follows the same rule: julieta's reader
  skips a member it does not know, as the other readers do. Decided on
  2026-10-22 by the maintainer, scope this repository, written in this
  record and in 03 3.6. One exception, the must-understand rule: a
  member that an older julieta could not safely ignore, one that
  restricts, denies or changes what julieta must do (for example a new
  limit, a narrower egress, a refusal), is not added as an optional
  member. It bumps the protocol number (REQ-0218), and the window of
  N and N-1 (REQ-0219) refuses a julieta that is too old with
  `manifest-protocol` (RJ-342). A member julieta may safely ignore (an
  informational one, or an addition that a newer julieta uses for a new
  feature) is added without a bump. The window protects what is
  security-relevant; optional members stop forcing a paired upgrade of a
  sandbox's julieta.

REQ-0159, REQ-0160, REQ-0163, REQ-0261 and REQ-0271 are amended to say
it, T162 changes the readers of the other formats, and T163 changes
the manifest's and states the must-understand rule.

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
- The manifest reader skips an unknown member, and the writer is
  responsible for the must-understand rule: a romeu that adds a member an
  older julieta must honor, and does not bump the protocol number, makes
  that julieta ignore a restriction without an error. Nothing in the code
  can tell the two kinds of member apart, so a review of a change to the
  manifest type asks which kind it adds. A skipped member is also lost
  where julieta caches the manifest (`cacheManifest` in
  `internal/cmd/julieta/setup/state.go` writes `manifest.Encode` of the
  decoded value), as in the rewrites above; a cached manifest therefore
  never holds a member this julieta does not know. Reopened when a
  second writer of the manifest exists, or when a restriction is found
  to have been ignored.
- The state files of julieta (`setup.json`, the hooks record, the hook
  failures and the system hooks path) are written and read by the same
  binary, and stay strict. Reopened if a second program reads one.
- The pending note "Readers of formats between binaries ignore unknown
  members" is closed by T162; the decision note of 2026-10-21 points
  here.
