# Memory per repository and per project, and where it is kept durably

What is at stake: medium. The choice shapes
`docs/spec/08-memory-handoff-salvage.md` (REQ-0642, REQ-0677, REQ-0645)
and where the only copy of every memory lives; a late change moves data
on every machine.

Scope: this repository, the memory spec (08 8.1) and `romeu doctor`.

Two memories are proposed: one per repository, keyed by its URL
(`github.com/<owner>/<repo>`), and one per project, with two indexes and
the prompt deciding what goes where. Example: a build project of
romeu-e-julieta that holds romeu-e-julieta and verona.

Today the spec keeps one memory directory per repository per project
(REQ-0642). Storage stays per repository: snapshot and salvage are
per-repository git bundles (REQ-0688, REQ-0704) and the store lock is
per directory (REQ-0655). Reading is already project-wide (REQ-0666), so a
session sees the memory of every repository of its project.

Decided on 2026-10-28: option 2. A shared memory directory per project,
`memory/_shared/`, beside the repository directories, holds the entries
that hold for the project as a whole (`julieta memory add --shared`) and
every handoff, which is now a memory entry of type `handoff` (ADR 0042,
03 3.9, 08 8.1). The manifest names it as `sharedMemory`, `sync` creates
it, and the sandbox mounts it like the others; the lock is per
directory, as before. The skill carries the judgment: the repository
memory holds what stays true of that code in any project; the shared
memory holds cross-repository decisions, ordering and the handoff.

Q4 (one repository in two projects) stays as it was: forbidden in v1.

Durability, decided the same day (ADR 0043): a private git repository
per project, named by the project spec as `memory.remote`, its `.git`
kept outside every mount, committed (signed) and pushed by romeu on
the host after each of its commands (task T175, milestone M2). This
repository's state moves into memory only once that copy is in use
(T176, which needs T175).

Reopened by: the first project with two repositories, which measures
whether the shared directory is enough.
