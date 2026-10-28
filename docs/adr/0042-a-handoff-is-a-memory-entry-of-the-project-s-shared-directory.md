# 42. A handoff is a memory entry of the project's shared directory

Date: 2026-10-28

## Status

Proposed

## Context

julieta carried what a session learns in two systems: memory entries
(`julieta memory`, one directory per repo, [08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store))
and handoffs, plain files of their own with a reader of their own
(`julieta handoff`, `romeu handoff`). The SessionStart hook printed the
latest narrative in full and then the bodies of the open entries under
one tag with behaviour, `lesson`, inside one bound of 8 KiB.

Measured on 2026-10-28 in the romeu-e-julieta sandbox: the hook output
was 7651 bytes of its 8 KiB, the narrative 3983 of them, and the memory
part showed 5 of 14 open entries before the cut. The narrative repeated
what the repository's `docs/handoff/` tree held, and that tree put the
operator's working state in a public repository. The agent's own memory
loads an index of up to 25 KB at a session's start and reads a note on
demand; its notes carry one of four kinds.

Scope: this repository, the product, and every sandbox that runs romeu
and julieta; written in [03 3.9](../spec/03-formats.md#39-memory-entry-schema-memory-entryv1),
[03 3.10](../spec/03-formats.md#310-stamp-schema-stampv1), [04](../spec/04-cli.md),
[08 8.1 to 8.3](../spec/08-memory-handoff-salvage.md) and this record.
The sandboxes of today keep their kit until its pin moves; nothing here
changes the kit they run.

### Alternatives considered

- **Two systems with two bounds.** A bound for the narrative and one for
  the index keeps the two readers, two schemas and the duplicate with
  `docs/handoff/`. Rejected: the duplication is the cost, not the bound.
- **A handoff type with a directory of its own.** The same entry rules,
  stored apart from the entries. Rejected: a second store for one type
  reintroduces a second reader.
- **The project's memory as a server.** A non-goal of v1
  ([00 0.3](../spec/00-scope.md#03-non-goals-v1)), and
  [ADR 0021](0021-keep-memory-as-plain-per-repo-files.md) gives the reason.

## Decision

1. A handoff is a memory entry of type `handoff`, the fifth type tag
   beside the four kinds of the agent's own memory (`user`, `feedback`,
   `project`, `reference`). Its body holds the headings the `handoff`
   skill names; the one written before a sandbox is destroyed carries
   the tag `tombstone` and one heading more. One is open at a time:
   adding one closes the open one. An open handoff is edited as any
   entry; a closed one is immutable.
2. Every project has a shared memory directory, `memory/_shared/`,
   beside the directories of its repos, for the entries that hold for
   the project as a whole (`memory add --shared`) and for every handoff.
   The manifest names it (`sharedMemory`), `romeu sync` creates it, and
   the sandbox mounts it as it mounts the others.
3. The stamp of [03 3.10](../spec/03-formats.md#310-stamp-schema-stampv1)
   replaces the handoff file's front matter and the facts file: where
   every manifest repo's tree was at one moment. julieta measures it
   into the handoff entry at `add`, under the `stamp` key, and writes it
   as a file at each session's end (`julieta stamp`); the resume line
   compares the two.
4. The SessionStart hook prints an index: the warnings, the fixed line,
   the open handoff (its line, the gap line, its body up to 4 KiB) and
   one line per other open entry up to 24 KiB (`julieta memory index
   --hook`). The host reads the same store with `romeu memory
   list|show`. `julieta handoff write|show|list` and `romeu handoff`
   leave; `julieta setup` migrates the handoff files of before, once.
5. The type-specific key `stamp` is admitted in the entry front matter
   for this type only, and the schema stays `memory-entry/v1`: the key
   is optional and no older entry changes.

## Consequences

- One store, one reader, one index; the bodies the agent reads are the
  ones it asks for. The decision "memory per repository and per
  project" of 2026-10-25 is taken by rule 2, the shared directory that
  note proposed.
- Each sandbox gains one mount at its next recreate, and the manifest
  one member; a manifest without it refuses `--shared` and a handoff
  with a usage error, so nothing is filed in the wrong place.
- The project's state leaves git when it moves into memory. Until
  memory has a durable home (the deferred row of the index, Q14), a
  project that moves its state keeps a versioned copy in its
  repository, as `docs/handoff/` of this repository is, and no task
  removes that copy before then.
- [ADR 0021](0021-keep-memory-as-plain-per-repo-files.md) stands: plain
  files, one directory per repo per project, plus the shared one; the
  entry format gains a key julieta writes, and still no field that
  names an agent.
- REQ-0245 and REQ-0415 are retired; the `handoff.v1` schema is
  replaced by `stamp.v1`.
