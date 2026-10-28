# 43. Keep the durable copy of memory in a private git repository per project

Date: 2026-10-28

## Status

Proposed

## Context

Memory was machine-local in v1 (Q14 of the index;
[ADR 0021](0021-keep-memory-as-plain-per-repo-files.md)), its backups
the owner's, and `romeu doctor` warned when the root was not excluded
from Time Machine without saying where the copy should be. With
[ADR 0042](0042-a-handoff-is-a-memory-entry-of-the-project-s-shared-directory.md)
the project's state (handoffs, decisions, pending notes) moves out of
the code repository and into memory, in every project that runs
julieta, and most of them have no `docs/handoff/` tree to fall back
on. Decided on 2026-10-28, with the question of whether to defer it
answered no: a disk lost after the move loses the state of every
project.

Scope: this repository, the product, and every project of every host
that runs romeu; written in [08 8.1](../spec/08-memory-handoff-salvage.md#81-memory-store)
(Durability), [03 3.2](../spec/03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1)
(`memory.remote`), [04 4.1 and 4.2](../spec/04-cli.md), [05 5.2](../spec/05-security.md#52-invariants)
(I18) and [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1), and this record.

### Alternatives considered

- **Time Machine or another host backup.** Not a copy romeu can
  verify, and a root excluded from Time Machine has none; the copy is
  on the same machine until the operator does more.
- **The config repository holds memory too.** One repository for every
  project's agent text, beside the specs the gates read; a wrong push
  would mix them, and the config repository is read by CI.
- **A memory server.** A non-goal of v1 ([00 0.3](../spec/00-scope.md#03-non-goals-v1)).
- **Committing the memory directory itself.** A `.git` inside a memory
  dir is what the layout allowlist refuses (I24), and a sandbox would
  see the remote and could reach for a credential.

## Decision

1. Each project spec names a private git repository, `memory.remote`,
   the durable copy of `<name>-env/memory/`. The host keeps its `.git`
   at `<name>-env/memory.git`, outside every mounted directory, with
   `git --separate-git-dir`; the sandbox sees plain files and never a
   remote, a credential or a push.
2. romeu commits `entries/` and `stamps/` of every memory dir, and
   nothing else, and pushes, as the last step of `sync`, `run`,
   `pull`, `salvage`, `rm`, `recreate` and `retire`, in gitsafe's host
   mode, signed with the project's signing key through the host's
   signing socket, as the author named by the project. A refused push
   and a history that cannot fast-forward are warnings; romeu never
   forces and never rebases agent text. The memory remote is the one
   push gitsafe builds (I18).
3. A spec without `memory.remote` is refused by `sync`; `memory:
   {remote: none}` opts out and is warned every time. On another
   machine, `sync` clones the remote into place.

## Consequences

- Q14 is answered: memory crosses machines through the remote, one
  writer at a time.
- [ADR 0021](0021-keep-memory-as-plain-per-repo-files.md) stands for
  the store (plain files, no `.git` inside, the allowlist), and its
  "machine-local in v1" consequence is replaced by this record.
- A push moves agent text, and only that, to a repository the operator
  owns; `salvage/` and `snapshot/` never leave the machine this way.
  romeu does not check the remote's visibility; the operator makes it
  private, and `doctor` shows the URL.
- The kit's pin moves to a julieta that files handoffs in memory only
  after the durable copy is in use on the host (T172 needs T175).
- Each project costs one private repository.
