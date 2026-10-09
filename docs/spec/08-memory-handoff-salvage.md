# 8. Memory, handoff, snapshot and salvage

Back to [index](../spec.md). Reader: implementers of `memstore`,
`handoff`, `salvage` and the `julieta-claude` kit. Type: reference.

Salvage preserves everything the sandbox holds that 8.5 covers, before
a planned destruction (recreate, rm, retire). When a sandbox dies
unplanned, the snapshot (8.4) preserves what was committed, and
uncommitted work since the last snapshot is lost; a periodic snapshot
of uncommitted changes is a row of
[Deferred decisions](../spec.md#in-the-product). Snapshot and salvage
are the deterministic half of every destructive journey and are
required to be complete, not merely present.

## 8.1 Memory store

| Aspect | Decision |
|---|---|
| Unit | one memory directory per repo per project: `$ROMEU_ROOT/<name>-env/memory/<dir>/`; a repo URL belongs to one project (Q4) |
| Mount | direct rw into that project's sandbox only; never mounted by two sandboxes; never a VS Code folder |
| Layout allowlist | the one home of this list (I24 and `memstore` cite it): the top level contains only `entries/`, `handoff/`, `snapshot/`, `salvage/`; regular files only (checked with Lstat); no dotfiles except julieta's temporary names (below), no `.git`, no symlinks, FIFOs or devices. `sync` creates each memory dir 0700 and julieta writes its files 0600 |
| Caps | the starting values of 8.6: per entry or handoff file, entries, handoff files; snapshot and salvage payloads are bounded by the salvage cap, which is per salvage (8.5 step 4). julieta keeps `handoff/` under its cap by removing facts files beyond the newest ones it keeps, never the newest facts file and never a narrative file; a `handoff/` over its cap warns in `sync` and `run` and never refuses |
| Enforcement | `julieta memory check` fails on a violation; romeu `sync` and `run` refuse (exit 2) and name the offending path; romeu reads memory only through `memstore`'s `os.Root` readers and never extracts archives from it (I24) |
| Interface | two packages, split by what each side needs. `memstore`, linked by both binaries, holds the readers on `os.Root`, `List(filter)`, whose filter carries the text of `list --query`, and `Get(id)`, with the layout allowlist and the caps. `memstore/write`, linked by julieta only, holds `Store`, which adds `Put(entry)` and the store lock. julieta never deletes a memory entry: `edit --status done` closes one, and backups of memory are the owner's |
| v1 backend | `fs` on `os.Root`; every write follows the atomic write rule of [03 3.8](03-formats.md#38-host-state-schemas-state-v1). A temporary file is named `.tmp-<ULID>` in the target's own directory; a name of that grammar is never read, is not an allowlist violation (`memory check` and `sync` warn, never refuse), and `julieta setup` removes any older than its own start. The store lock is a sandbox-local file, `$HOME/.local/state/julieta/locks/<dir>.lock`, never in the memory dir, because a memory dir is mounted into one sandbox only. `Put` takes it, and so does `handoff write` for the whole command, git reads included, on the lock of its target dir |
| Stamping | julieta assigns the ULID `id` and the `createdAt`/`updatedAt` timestamps on `add` and `edit`; callers cannot supply them (principle: deterministic) |
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
  agent consumes memory instead of remembering to read it, and one
  fixed line every session: `built-in memory is disabled, use julieta
  memory add`.
- The skill rule (`skills/julieta/SKILL.md`) tells the agent to use
  `julieta memory` for anything worth keeping, and to tag an entry
  `lesson` when it records what a session taught. Something no event
  id expresses goes into the handoff, and the agent opens an issue
  labelled `deferred` that names the row. When signing a commit fails,
  or waits on a confirmation that does not come, the rule tells the
  agent to ask the operator, and never to unset `commit.gpgsign` or
  sign another way.

## 8.2 Session hooks (user-side middleware)

The `julieta-claude` kit wires two Claude Code hooks. They are the
agent-facing counterpart of the repo's pre-push hook and CI: mechanical,
run every time, never remembered.

| Hook | Command | Prints or writes |
|---|---|---|
| SessionStart, for every source the pinned Claude Code fires (startup, resume, clear, compact) | `julieta handoff show --hook` | in this order: (1) julieta's warnings, one line each: a stale `mise.lock`, snapshots disabled for a repo, the agent's own memory dir writable or not empty, a required agent-profile path that does not exist (8.5 step 5), a default branch `julieta setup` moved (`default-branch-moved`, `<repo> <old>..<new>`, shown by the first SessionStart after the move and then cleared), and every recorded hook failure (shown until a later run of the same hook succeeds, below); (2) the fixed built-in memory line (8.1); (3) the latest narrative handoff, or the gap line of 8.3; (4) the facts that changed since it (branch moved, new commits, dirty count); (5) open entries, newest first; (6) entries tagged `lesson` whose status is open, newest first. It reads every manifest repo's memory dir and labels each item with its repo. The whole output is bounded (8.6); when it is cut, the last line is `<n> more entries: julieta memory list`. Warnings come first, so the bound never cuts them |
| SessionEnd | `julieta handoff write --facts` | a `facts` handoff, so a session that ends without `/handoff` still leaves stamped git facts; whether it raises the gap line is decided by the facts it carries (8.3), not by its time |

Memory, handoff and lesson text is written by agents. SessionStart
shows it to the next session as data, under a line that says so; the
bound limits its volume, not its content
([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).

Both hooks invoke julieta by the absolute path of the read-only
`.romeu/bin` mount ([06 6.4](06-kits.md#64-product-kits)). The
SessionStart and SessionEnd hooks always exit 0: on a failure each
prints one stdout line naming the failure and `julieta status`, and julieta records it as a hook failure.
When julieta itself cannot run (the mount is absent after an sbx
change), nothing of julieta is there to print, so the kit renders each
hook command as a shell wrapper that prints the failure line and exits
0 in that case. The PreToolUse guard of 06 6.4 is excluded from this
rule, because it must fail closed: its wrapper exits 2 when its program
cannot run, which blocks the tool call. By the Claude Code hooks
documentation (code.claude.com/docs/en/hooks, read on 2026-10-09), exit
2 is the one exit code that blocks a PreToolUse call by itself; any
other code, a hook that cannot start and a hook that times out let the
call proceed, so a guard that hangs is not a gate. Probe C5 confirms
this for the pinned version.
Which sources fire, that the output reaches the agent's context, its
size limit, the behaviour on a non-zero exit and the SessionEnd timeout
of the pinned version are recorded by one probe row of
[11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last),
which also checks that the stdout of a failing hook still reaches the
agent's context.

julieta records every failing git hook run (hook, repo, exit status,
time) in its state; SessionStart, `julieta status` and `romeu status`
surface them until a later run of the same hook succeeds.

## 8.3 Handoff

| Aspect | Decision |
|---|---|
| Narrative handoff | the operator types `/handoff` (before `/clear`) or `/handoff --final` (before recreate/rm); the skill has the agent write the required headings and pipe them to `julieta handoff write [--final]` |
| Facts handoff | written by the SessionEnd hook (8.2) |
| Location | the memory dir of the repo containing the agent's cwd, or of `--repo`; never a question. `julieta memory add` and `julieta handoff write` print the repo they filed under, so work across repos is filed where the agent meant |
| File name | `handoff/<ULID>-<kind>.md` (03 3.10); ULIDs sort by time |
| Stamping | julieta writes all front matter (3.10) with git facts for every repo in the manifest |
| Handoff reader | one function in `internal/handoff`, used by julieta and romeu: "latest narrative" = the greatest ULID among `clear` and `final` files; "newest facts" = the greatest ULID among `facts` files; a facts file never hides a narrative |
| Resume | SessionStart prints the latest narrative plus the difference between its facts and the newest facts. The gap line `the newest facts differ from the latest narrative of <time>: <changes>`, where `<changes>` lists the count of new commits, a moved branch and a changed dirty count, prints first only when the newest facts file's git facts differ from the facts stamped in the latest narrative (new commits, a moved branch, a changed dirty count); a newer facts file with the same git facts, such as the one SessionEnd writes at `/clear` after `/handoff`, prints no gap line. The line names what is measured, not how the session ended: it also prints when `/handoff` ran and a commit followed before `/clear`. Three goldens pin it: `/handoff` then `/clear` (no line), a facts-only end with new commits (the line), and `/handoff`, a commit, then `/clear` (the line) |
| Before destruction | after the preflight and before the salvage half, `romeu rm` asks the handoff reader whether the open generation has a `final` handoff; on a TTY it asks whether to go on without one, and without a TTY it warns (`no-final-handoff`) and goes on (04 4.2). The narrative handoff is the operator's step: romeu cannot ask a stopped agent for one (J6) |
| Host | `romeu handoff <name>` prints the latest narrative and newest facts (escaped), labelled as text an agent wrote |

Claude Code fires SessionEnd when `/clear` ends a session, then
SessionStart for the new one (Q22). With the rules above, the facts file
written at `/clear` never displaces the narrative written by `/handoff`
just before it, whatever the order of the two hooks.

*Why for us:* agent state kept in git notes is invisible to a heads-only
refspec; a plain file on a host mount survives the sandbox without git
plumbing.

## 8.4 Snapshot (after every commit)

- Trigger: julieta's hook dispatcher after `post-commit`,
  `post-rewrite`, `post-merge`, with no debounce (the bundle holds the
  objects not reachable from the last synced base, so it grows with the
  work since the last sync); the romeu commands that snapshot are
  named in their rows of [04 4.2](04-cli.md#42-romeu-host).
- Content: `git bundle create` of all local branches, tags, notes, the
  current generation's `refs/salvage/<generation>/*` refs (no earlier
  generation's), every worktree HEAD and, per branch, each reflog
  entry's commit not reachable from a bundled ref, as
  `refs/salvage-reflog/<branch>/<n>`, excluding objects reachable from
  the manifest's `repos[].base` (the origin SHAs the host already has,
  the same base salvage uses). One bundle is kept, not the last N.
- Write: `snapshot/heads.bundle` is renamed into place first and
  `snapshot/heads.json` (`schema: snapshot/v1`, ref -> SHA, `createdAt`, `disabled`) after it,
  each by the atomic write rule of 03 3.8. romeu trusts the bundle's own
  refs and reads `heads.json` for `status` only, so a kill between the
  two writes leaves nothing romeu acts on wrongly.
- A snapshot that cannot be written never undoes the commit: it keeps
  the previous bundle, removes its own `.tmp-<ULID>` file before the
  hook failure is recorded, is recorded as a hook failure with its
  repo (8.2), and the repo's own hooks still run (04 4.3). A kill
  leaves the file, and `julieta setup` stays its cleanup (8.1).
- A repo whose local config sets `core.hooksPath` bypasses the
  dispatcher; `julieta setup` writes `disabled: true` into its
  `heads.json`, `julieta setup` and `julieta status` report "snapshots
  disabled for <dir>", SessionStart prints it, and `romeu status` shows
  it.
- Purpose: recovery when a sandbox dies without salvage (J10). The
  snapshot lives in the sandbox's memory mount until `romeu pull` or a
  preservation imports it. For the primary repo,
  `refs/sandboxes/<name>/*` is a second copy as of its last fetch; a
  secondary repo has no second copy, and a removal inside the sandbox
  loses its snapshot. Importing snapshots at every `sync` and `run` is a
  row of [Deferred decisions](../spec.md#in-the-product).

## 8.5 Salvage (complete, before destruction)

Salvage covers the generation's recorded repo set
([01 1.6](01-system-model.md#generation)), not the current spec. Every
git call of the sandbox half runs in gitsafe's sandbox mode
([05 5.1](05-security.md#51-hardened-git-internalgitsafe)).

### Sandbox half: `julieta salvage` (run by romeu via `sbx env exec`)

romeu passes `salvageRun.id` and, per repo, `repos[].base` in the
manifest.

1. `--stop-agents`: SIGTERM to the agent's processes (names from
   `internal/agent/<agent>`), wait up to 10 s, then SIGKILL; record it,
   with the number of processes matched. Zero matched while the agent
   pane runs is the skipped item `agent-not-matched`. Then salvage waits
   on the store lock of every manifest memory dir (8.1), so a
   SessionEnd hook already running finishes first; a facts handoff written during salvage is allowed
   and not counted for completeness. `cmd/julieta` passes the agent
   profile (process names, state paths) to `salvage`; the `salvage`
   package, which romeu links too, does not import `agent/*`.
2. For each repo in the manifest, and for each nested repository found
   inside a worktree (a `.git` below the top level), for each worktree
   from `git worktree list --porcelain`:
   - uncommitted tracked and untracked (non-ignored) changes: tree
     through a temporary `GIT_INDEX_FILE`, `commit-tree` with parent HEAD,
     `-c commit.gpgsign=false`, author `julieta-salvage`, ref
     `refs/salvage/<generation>/<salvage-id>/<worktree-n>`, outside
     `refs/heads`, so `git push --all` or `--mirror` never publishes it;
     worktree and index untouched;
   - each stash becomes `refs/salvage/<generation>/<salvage-id>/stash-<n>`;
   - each worktree's HEAD and status are read again; if either moved,
     the capture repeats once, then the worktree is skipped as
     `worktree-changing`. A rebase, merge, cherry-pick or bisect in
     progress is skipped as `in-progress-operation`. Other processes
     are not stopped; this recheck is what catches them.

   Untracked files count against the salvage cap of step 4 and are
   listed `over-cap` beyond it. Salvage writes objects into the
   sandbox's repositories, so a full sandbox disk fails this step;
   freeing space inside the sandbox is the recovery before
   `--from-host`.
3. One bundle per repo with all local branches, tags, notes, the
   current generation's `refs/salvage/<generation>/*` refs, every
   worktree HEAD (detached ones included) and the reflog-only commits of
   8.4, excluding objects reachable from `repos[].base` (host-known
   SHAs, not the sandbox's remote-tracking refs). A nested repository
   gets its own bundle.
4. Ignored files, minus `salvage.excludeIgnored`, into `ignored.tar.gz`
   up to the cap (default 1 GiB per salvage, shared with the untracked
   files of step 2); the rest listed as `over-cap`.
5. Agent state (from `internal/agent/claude`): project memory files and
   todos to `agent/`; transcripts excluded unless `--include-transcripts`
   (they may contain echoed tokens), listed as `transcript-excluded`;
   scratch dirs named by the agent profile within the same cap. The
   profile splits its paths into required ones, which exist at the
   pinned agent (`julieta setup` and probe C5 check them), and
   optional ones, such as a todos or scratch dir a session never
   created. Only a missing required path is the skipped item
   `agent-path-missing`; a missing optional path is no item.
6. Write `manifest.json` (3.11). A salvage dir without its
   `manifest.json`, which an interrupted salvage leaves, is ignored by
   verification and kept.

**Salvage payloads are secret-bearing.** `ignored.tar.gz`, `agent/` and
transcripts can hold credentials and code. Their dirs are 0700 and
their files 0600; they stay until the owner deletes them and are part
of any backup of `$ROMEU_ROOT`. `romeu salvage` prints that sentence
once when it stores ignored files or transcripts. Keeping them in the
mount is a risk accepted in
[05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1). A
root excluded from Time Machine, as the `root-indexed` check of
[04 4.2](04-cli.md#42-romeu-host) advises, is in no Time Machine
backup, so its memory dirs and unpushed work need another copy, which
that check's hint says. Once `romeu salvage` has recorded its result on
the generation, and the generation is not `removing`, the owner may
delete that salvage dir or any payload in it; while the generation is
`removing`, a rerun may salvage again (01 1.6), so the dir stays: `rm`, `recreate` and `retire` read the record, `doctor` checks the
salvage refs on the host and no salvage dir, and `status` lists the
records, so a deleted dir is no finding. The refs under
`refs/romeu/salvage/` stay, since `doctor` fails on a missing one.

### Host half: `romeu salvage`

1. Read the manifest via `os.Root`; `salvageId` must equal the id romeu
   passed; every repo in the generation's repo set must appear (a
   missing repo is incomplete); verify every file's size and sha256,
   under the path and cap rules of
   [03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1).
2. For each bundle: validate every ref name, `git bundle verify`
   (hardened; fetch origin into `refs/romeu/origin/` first if
   prerequisites are missing), `git bundle unbundle`, then
   `git fsck --strict` on each unbundled head, as `pull` does (04 4.2),
   since `transfer.fsckObjects` does not cover an unbundle (05 5.1); a
   failure is the skipped item `fsck-failed`. Then create-only refs
   under `refs/romeu/salvage/<name>/<generation>/<salvage-id>/<dir>/...`.
3. Primary repo: hardened fetch from the live daemon; every daemon head
   must be present in the bundle refs with the same SHA. A daemon URL
   whose scheme or host gitsafe refuses is the skipped item
   `daemon-url-refused`, which names sbx, because the salvage
   cross-check assumes the daemon at `git://127.0.0.1`. Secondary repos
   have no daemon, so no cross-check (05 5.4).
4. Recompute completeness (ignoring the manifest's own `complete`):
   `result: complete` only when nothing was skipped except
   `excluded-by-spec` and `transcript-excluded`; otherwise `incomplete`
   with each skipped item as a reason; record it on the generation; exit
   5 when incomplete (01 1.6).
5. `--from-host` (sandbox dead or unreachable): skip the sandbox half;
   import `refs/sandboxes/<name>/*` and every `snapshot/heads.bundle`;
   record `result: lost, reasons: [sandbox-lost]`.

### Skipped reasons and their recovery

The reasons are those of
[03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1); this
table is their one recovery guide. `--accept-loss=<reason>` is the last
resort for each.

| Reason | Recovery |
|---|---|
| `excluded-by-spec`, `transcript-excluded` | none needed: chosen by the spec or by the missing flag |
| `over-cap` | add the path to `salvage.excludeIgnored` (gate 2), or copy it out of the sandbox, then rerun |
| `unreadable` | the path and the git error are in the manifest; fix it and rerun |
| `disk-full` | free space inside the sandbox, then rerun |
| `worktree-changing`, `in-progress-operation` | let the process or the operation finish (or abort it), then rerun |
| `agent-not-matched`, `agent-path-missing` | the agent profile no longer matches the pinned agent; rerun once the agent is stopped by hand, and report the drift as an issue |
| `fsck-failed` | the bundle holds a malformed object; inspect the repo in the sandbox, then rerun |
| `daemon-url-refused`, a missing repo, daemon heads not in the bundle | rerun once the sandbox is reachable at its daemon; a persisting one names an sbx change |

A rerun of `rm`, `recreate` or `retire` on a `removing` generation whose
worktrees keep changing is not a skipped reason: the third salvage in a
row that a changed fingerprint starts exits 5 with the error
`fingerprint-unstable` (`RJ-338`, [04 4.4](04-cli.md#44-error-ids)),
which `--accept-loss` does not cover, and its hint is the recovery.

### Not salvaged

Salvage keeps only what the steps above name. It does not keep paths
outside the recorded repos and the agent profile's state paths, clones
no spec records, `/tmp`, installed plugins, multiplexer scrollback, or
the workload's volumes ([06 6.2](06-kits.md#62-workload-choice)), which
`rm` and `recreate` list before salvage.

### Recovery

Salvage and preservation stop at the host. Recovery is the operator's
in v1: a branch or salvage commit comes back by the J10 step 3 commands
(a `recover/<b>` branch from the salvage ref, then a push to origin,
which the next sandbox fetches); `ignored.tar.gz` and `agent/` are
inspected on the host and copied in by hand, and an archive is
extracted inside the sandbox only (J6 step 6). Each preservation prints
the salvage ref prefix it wrote (04 4.2). A `romeu restore` command is
a row of [Deferred decisions](../spec.md#in-the-product).

*Why for us:* Codespaces keeps only `/workspaces` across a rebuild and
DevPod can orphan state on force delete; we prove preservation with a
verified manifest before any destructive step.

## 8.6 Starting values

These are starting values: the plan may tune them, and this table is
the one place they are written.

| Value | Default |
|---|---|
| size of one entry or handoff file | 1 MiB |
| entries per memory dir | 10 000 |
| files in `handoff/` | 1 000 |
| facts files julieta keeps in `handoff/` | the newest 50 |
| SessionStart output | 8 KiB |
| salvage cap, per salvage (the manifest's `salvage.capBytes`, 03 3.6) | 1 GiB |
