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

Options:

1. Keep per-repository storage only; knowledge across repositories lives
   in the primary repository.
2. Add a project directory beside the repository ones. A name such as
   `_project` cannot collide with a repository directory, whose pattern
   (REQ-0176) starts with `[a-z0-9]`. This changes REQ-0642, REQ-0677
   and the allowlist of REQ-0645.
3. Project only. Rejected: snapshot and salvage are per repository.

Recommendation: keep per-repository storage now and decide between 1 and
2 at the first multi-repository project. With two memories, the default
destination lives in the command (`memory add` in a repository files
there, `--project` files to the project) and the skill carries only the
judgment: the repository memory holds what stays true of that code in
any project; the project memory holds cross-repository decisions,
ordering, the session's plan and handoffs.

Interactions:

- Q4 (the same repository in two projects, forbidden in v1, REQ-0643;
  `docs/spec.md` Q4). A repository memory keyed by URL is what would let
  one repository sit in two projects. That also reopens REQ-0644 (a
  memory directory is mounted into one sandbox only) and the
  per-directory lock of REQ-0655. In the example, verona could not keep
  its own project while it is in the build project.
- Durability. The spec keeps memory machine-local (Q14, open before the
  first release-candidate tag, `docs/spec.md`), outside git (REQ-0645
  refuses a `.git`), and backups are the owner's (REQ-0653). `romeu
  doctor` warns `root-indexed` when the root is not excluded from Time
  Machine (REQ-0376), while REQ-0712 says a root excluded from Time
  Machine needs another copy of its memory directories, handoffs and
  unpushed work. The doctor's hint says so, but no mechanism of the
  spec makes that copy.

Open questions for the operator:

- In the example, does verona leave its own project, or must both exist
  (which reopens Q4)?
- Where is memory kept durably?

Reopen: the first multi-repository project or Q14, whichever comes
first.
