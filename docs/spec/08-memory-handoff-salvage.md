# 8. Memory, handoff, snapshot and salvage

Back to [index](../spec.md). Reader: implementers of `memstore`,
`handoff`, `salvage` and the `julieta-claude` kit. Type: reference.

Snapshot and salvage exist because nothing sandbox-only may be lost when
a sandbox is recreated, removed or dies; they are the deterministic half
of every destructive journey and are required to be complete, not
merely present.

## 8.1 Memory store

| Aspect | Decision |
|---|---|
| Unit | one memory directory per repo per project: `$ROMEU_ROOT/<name>-env/memory/<dir>/`; a repo URL belongs to one project (Q4) |
| Mount | direct rw into that project's sandbox only; never mounted by two sandboxes; never a VS Code folder |
| Layout allowlist | top level contains only `entries/`, `handoff/`, `snapshot/`, `salvage/`, `import/`; regular files only (checked with Lstat); no dotfiles, no `.git`, no symlinks, FIFOs or devices; caps: 1 MiB per entry or handoff file, 10 000 entries, snapshot and salvage payloads bounded by the salvage cap |
| Enforcement | `julieta memory check` fails on a violation; romeu `sync` and `run` refuse (exit 2) and name the offending path; romeu reads memory only through `memstore`'s `os.Root` readers and never extracts archives from it (I24) |
| Interface | two packages, split by what each side needs. `memstore`, linked by both binaries, holds the readers on `os.Root`, `List(filter)`, `Get(id)` and `Search(query)`, with the layout allowlist and the caps. `memstore/write`, linked by julieta only, holds `Store`, which adds `Put(entry)` and `Delete(id)`, the store lock, and import and verify. romeu links no code that can delete a memory entry, so the I16 scan needs no exception for one |
| v1 backend | `fs` on `os.Root`; atomic writes (temp + rename); a file lock per store for concurrent julieta processes |
| Stamping | julieta assigns the ULID `id` and the `created`/`updated` timestamps on `add` and `edit`; callers cannot supply them (principle: deterministic) |
| Portability | machine-local in v1 (Q14) |
| Agent neutrality | markdown + YAML front matter; no agent-specific fields |

*Why for us:* an external memory server has no per-project authorization
and must not have its data dir mounted over virtiofs; a plain per-repo
directory gives isolation by construction and keeps a server-per-repo
backend possible later.

### How julieta replaces the agent's built-in memory

- The `julieta-claude` kit makes the agent's own memory directory
  non-writable, and sets the disabling setting when the pinned agent
  version has one (Q8).
- The SessionStart hook (8.2) prints the open memory entries, so the
  agent consumes memory instead of remembering to read it.
- The skill rule (`skills/julieta/SKILL.md`) tells the agent to use
  `julieta memory` for anything worth keeping, and to tag an entry
  `lesson` when it records what a session taught.

### Import and verification

`julieta memory import --format jsonl <file>` imports entries in the
format of [03 3.9](03-formats.md#39-memory-entry-schema-memory-entryv1),
idempotent by content digest, and reports `read`, `imported`,
`skipped-duplicate`, `invalid` (exit 1 if `invalid > 0`); an import is
correct when `imported + skipped-duplicate == read`. `julieta memory
verify --against <file>` checks set equality of content digests between
the file and the store, then that every matched entry has the same
`status` (status is not part of the content digest, so a closed entry
still deduplicates).

## 8.2 Session hooks (user-side middleware)

The `julieta-claude` kit wires two Claude Code hooks. They are the
agent-facing counterpart of the repo's pre-push hook and CI: mechanical,
run every time, never remembered.

| Hook | Command | Prints or writes |
|---|---|---|
| SessionStart | `julieta handoff show --hook` | the latest narrative handoff and the facts that changed since it (branch moved, new commits, dirty count); open memory entries; entries tagged `lesson`; julieta's warnings: a stale `mise.lock`, snapshots disabled for a repo, the agent's own memory dir writable or not empty, and every recorded hook failure (shown until a later run of the same hook succeeds, below) |
| SessionEnd | `julieta handoff write --facts` | a `facts` handoff, so a session that ends without `/handoff` still leaves stamped git facts |

julieta records every failing git hook run (hook, repo, exit status,
time) in its state; SessionStart, `julieta status` and `romeu status`
surface them until a later run of the same hook succeeds. The
dispatcher also emits each failure as a `hook-failed` event of the
runtime ledger ([13 13.2](13-runtime-ledger.md#132-events)); julieta
state stays the record a session reads.

## 8.3 Handoff

| Aspect | Decision |
|---|---|
| Narrative handoff | the operator types `/handoff` (before `/clear`) or `/handoff --final` (before recreate/rm); the skill has the agent write the required headings and pipe them to `julieta handoff write [--final]` |
| Facts handoff | written by the SessionEnd hook (8.2) |
| Location | the memory dir of the repo containing the agent's cwd; never a question |
| File name | `handoff/<ULID>-<kind>.md` (03 3.10); ULIDs sort by time |
| Stamping | julieta writes all front matter (3.10) with git facts for every repo in the manifest |
| Handoff reader | one function in `internal/handoff`, used by julieta and romeu: "latest narrative" = the greatest ULID among `clear` and `final` files; "newest facts" = the greatest ULID among `facts` files; a facts file never hides a narrative |
| Resume | SessionStart prints the latest narrative plus the difference between its facts and the newest facts |
| Before destruction | `romeu rm` asks the handoff reader, at that moment, whether the open generation has a `final` handoff and warns when it has none (the deterministic salvage still runs) |
| Host | `romeu handoff <name>` prints the latest narrative and newest facts (escaped) |

Claude Code fires SessionEnd when `/clear` ends a session, then
SessionStart for the new one (Q22). With the rules above, the facts file
written at `/clear` never displaces the narrative written by `/handoff`
just before it, whatever the order of the two hooks.

*Why for us:* agent state kept in git notes is invisible to a heads-only
refspec; a plain file on a host mount survives the sandbox without git
plumbing.

## 8.4 Snapshot (after every commit)

- Trigger: julieta's hook dispatcher after `post-commit`,
  `post-rewrite`, `post-merge`, with no debounce (the bundle holds only
  unpushed objects, so it is small); `romeu stop` and `romeu pull` call
  `julieta snapshot --all`.
- Content: `git bundle create` of all local branches, tags, notes,
  `salvage/*` refs and every worktree HEAD, excluding objects reachable
  from the manifest's `repos[].base` (the origin SHAs the host already
  has, the same base salvage uses); written atomically to
  `memory/<dir>/snapshot/heads.bundle` with `snapshot/heads.json`
  (ref -> SHA, created).
- A repo whose local config sets `core.hooksPath` bypasses the
  dispatcher; `julieta setup` and `julieta status` report "snapshots
  disabled for <dir>", SessionStart prints it, and `romeu status` shows
  it.
- Purpose: recovery when a sandbox dies without salvage (J10).

## 8.5 Salvage (complete, before destruction)

Salvage covers the generation's recorded repo set
([01 1.6](01-system-model.md#generation)), not the current spec.

Before the sandbox half, romeu ingests the project's spool into the
runtime ledger ([13 13.4](13-runtime-ledger.md#134-ingest)). An ingest
that does not finish is recorded in the salvage record's `reasons` as
`ledger-incomplete`. It is not a skipped item: it changes no `result`
and no exit status, so it never makes `rm` exit 5. Events julieta
writes during the sandbox half stay in the spool, which is a host
directory and outlives the sandbox, for the next ingest. `retire` has
no next ingest, because it moves the spool away, so it ingests once
more after the sandbox half and before the move.

### Sandbox half: `julieta salvage` (run by romeu via `sbx env exec`)

romeu passes `salvageRun.id` and, per repo, `repos[].base` in the
manifest.

1. `--stop-agents`: SIGTERM to the agent's processes (names from
   `internal/agent/<agent>`), wait up to 10 s, then SIGKILL; record it.
   `cmd/julieta` passes the agent profile (process names, state paths)
   to `salvage`; the `salvage` package, which romeu links too, does
   not import `agent/*`.
2. For each repo in the manifest, for each worktree from
   `git worktree list --porcelain`:
   - uncommitted tracked and untracked (non-ignored) changes: tree
     through a temporary `GIT_INDEX_FILE`, `commit-tree` with parent HEAD,
     `-c commit.gpgsign=false`, author `julieta-salvage`, ref
     `refs/heads/salvage/<generation>/<salvage-id>/<worktree-n>`;
     worktree and index untouched;
   - each stash becomes `refs/heads/salvage/<generation>/<salvage-id>/stash-<n>`.
3. One bundle per repo with all local branches, tags, notes, salvage
   refs and every worktree HEAD (detached ones included), excluding
   objects reachable from `repos[].base` (host-known SHAs, not the
   sandbox's remote-tracking refs). Nested repositories found inside a
   worktree (a `.git` below the top level) get their own bundle.
4. Ignored files, minus `salvage.excludeIgnored`, into `ignored.tar.gz`
   up to the cap (default 1 GiB per salvage); the rest listed as
   `over-cap`.
5. Agent state (from `internal/agent/claude`): project memory files and
   todos to `agent/`; transcripts excluded unless `--include-transcripts`
   (they may contain echoed tokens), listed as `transcript-excluded`;
   scratch dirs named by the agent profile within the same cap.
6. Write `manifest.json` (3.11).

### Host half: `romeu salvage`

1. Read the manifest via `os.Root`; `salvageId` must equal the id romeu
   passed; every repo in the generation's repo set must appear (a
   missing repo is incomplete); verify every file's size and sha256.
2. For each bundle: validate every ref name, `git bundle verify`
   (hardened; fetch origin into `refs/romeu/origin/` first if
   prerequisites are missing), `git bundle unbundle`, then create-only
   refs under `refs/romeu/salvage/<name>/<generation>/<salvage-id>/<dir>/...`.
3. Primary repo: hardened fetch from the live daemon; every daemon head
   must be present in the bundle refs with the same SHA. Secondary repos
   have no daemon, so no cross-check (05 5.4).
4. Recompute completeness (ignoring the manifest's own `complete`):
   `result: complete` only when nothing was skipped except
   `excluded-by-spec` and `transcript-excluded`; otherwise `incomplete`
   with each skipped item as a reason; record it on the generation; exit
   5 when incomplete (01 1.6).
5. `--from-host` (sandbox dead or unreachable): skip the sandbox half;
   import `refs/sandboxes/<name>/*` and every `snapshot/heads.bundle`;
   record `result: lost, reasons: [sandbox-lost]`.

*Why for us:* Codespaces keeps only `/workspaces` across a rebuild and
DevPod can orphan state on force delete; we prove preservation with a
verified manifest before any destructive step.
