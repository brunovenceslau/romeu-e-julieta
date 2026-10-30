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
required to be complete, not merely present; what complete means is
the rule of 8.5.

## 8.1 Memory store

| Aspect | Decision |
|---|---|
| Unit | `REQ-0642` one memory directory per repo per project, `$ROMEU_ROOT/<name>-env/memory/<dir>/`, and one shared directory per project, `memory/_shared/`, for the entries that hold for the project as a whole (03 3.9, `--shared`; every `handoff`) and the stamps (03 3.10); the name cannot collide with a repo dir, whose pattern (REQ-0176) starts with a letter or a digit; `REQ-0643` a repo URL belongs to one project (Q4) |
| Mount | `REQ-0644` direct rw into that project's sandbox only; never mounted by two sandboxes; never a VS Code folder |
| Layout allowlist | the one home of this list (I24 and `memstore` cite it): `REQ-0645` the top level of a repo dir contains only `entries/`, `snapshot/`, `salvage/`, and of the shared dir only `entries/`, `stamps/`; a `handoff/` of before 03 3.10's stamp is admitted until `julieta setup` migrates it (04 4.3, REQ-1301), as the warning `handoff-legacy` in `memory check`, `sync` and `run`, never a refusal; regular files only (checked with Lstat); no dotfiles except julieta's temporary names (below), no `.git`, no symlinks, FIFOs or devices. `REQ-0646` `sync` creates each memory dir, the shared one too, 0700, writes the shared one into the manifest as `sharedMemory` (03 3.6) and renders its mount (02 2.3, REQ-0140); julieta creates no memory dir and writes its files 0600 |
| Moving a store | `REQ-0647` a memory entry file is self-contained: copying `memory/<dir>/entries/` whole is a complete move, and the store lock is recreated on first use (the J13 rule, whose home is this row) |
| Caps | `REQ-0648` the starting values of 8.6: per entry or stamp file, entries, stamp files; snapshot and salvage payloads are bounded by the salvage cap, which is per salvage (8.5 step 4). `REQ-0649` after each write, `julieta stamp` keeps `stamps/` under its cap by removing the files beyond the newest ones julieta keeps (8.6), never the newest; `REQ-0650` a `stamps/` over its cap warns `stamps-over-cap` in `sync`, `run` and `memory index --hook` and never refuses |
| Enforcement | `REQ-0651` `julieta memory check` fails on a violation; romeu `sync` and `run` refuse (exit 2) and name the offending path; `REQ-0652` romeu reads memory only through `memstore`'s `os.Root` readers and never extracts archives from it (I24) |
| Interface | `REQ-0653` two packages, split by what each side needs. `memstore`, linked by both binaries, holds the readers on `os.Root`, `List(filter)`, whose filter carries the narrowing of `memory list` (04 4.3: the query text, the status, the tags, the since bound, the limit and the sort), and `Get(id)`, with the layout allowlist and the caps. `memstore/write`, linked by julieta only, holds `Store`, which adds `Put(entry)` and the store lock. julieta never deletes a memory entry: `edit --status done` closes one, and backups of memory are the owner's |
| v1 backend | `fs` on `os.Root`; every write follows the atomic write rule of [03 3.8](03-formats.md#38-host-state-schemas-state-v1). `REQ-0654` A temporary file is named `.tmp-<ULID>` in the target's own directory; a name of that grammar is never read, is not an allowlist violation (`memory check` and `sync` warn, never refuse), and `julieta setup` removes any older than its own start. `REQ-0655` The store lock is a sandbox-local file, `$HOME/.local/state/julieta/locks/<dir>.lock`, never in the memory dir, because a memory dir is mounted into one sandbox only. `Put` takes it, and so do `memory add` of a `handoff` and `julieta stamp` for the whole command, git reads included, on the lock of the shared dir |
| Stamping | `REQ-0656` julieta assigns the typed `id` (`mem_<ulid>`, or `hof_<ulid>` for a `handoff`, [03 3.12](03-formats.md#312-digests-internalcanon)) and the `createdAt`/`updatedAt` timestamps on `add` and `edit`; callers cannot supply them (principle: deterministic) |
| Durability | `REQ-1303` The durable copy of `<name>-env/memory/` is a private git repository the project spec names (`memory.remote`, 03 3.2; Q14): the host keeps its `.git` at `<name>-env/memory.git`, outside every mounted directory (`git --separate-git-dir`), so the layout allowlist above still refuses a `.git` inside a memory dir and the sandbox sees plain files and never the remote, its credential or a push. `REQ-1304` romeu commits `entries/` and `stamps/` of every memory dir, and nothing else (`snapshot/` and `salvage/` are secret-bearing, 8.5, and their content is on origin or in the salvage refs), and pushes, as the last step of `sync`, `run`, `pull`, `salvage`, `rm`, `recreate` and `retire`, every git call in gitsafe's host mode with its keys passed per call and none written (I2), the commit signed with the project's signing key through `signing.agentSocket` of host settings (I34's one key, `SSH_AUTH_SOCK` set for that call only) and its author the project's name, and a project with no signing socket commits unsigned and is warned `memory-unsigned` by `sync`, `run` and `doctor` every time; the push is the one push romeu builds, to `memory.remote` and nothing else (I18); a fetch or a push the remote refuses, a project the step cannot resolve after the command's work (host settings, the live render, the config clone or the spec at its commit), and `romeu.lock` held by another romeu when the step takes it again are the warning `memory-push-failed`, never a refusal, since the command's work is done and the next command pushes again (decided on 2026-10-29); a remote that moved on (a second machine) is fetched and fast-forwarded before the commit, and a history that cannot fast-forward is the warning `memory-diverged`, left to the operator, since romeu never force-pushes and never rebases agent text. `REQ-1305` A project spec without `memory.remote` is refused by `sync` (`memory-remote-missing`, exit 2); `memory: {remote: none}` opts out, and `sync`, `run` and `doctor` then warn `memory-local` every time, so a machine-local store is a choice made each day, not a default; on a second machine, `sync` clones the remote into `<name>-env/memory/` with the separate git dir when the directory is absent |
| Agent neutrality | `REQ-0657` markdown + YAML front matter; no agent-specific fields |

*Why for us:* an external memory server has no per-project authorization
and must not have its data dir mounted over virtiofs; a plain per-repo
directory gives isolation by construction and keeps a server-per-repo
backend possible later.

### How julieta replaces the agent's built-in memory

- `REQ-0658` The `julieta-claude` kit makes the agent's own memory directory
  non-writable, and sets the disabling setting when the pinned agent
  version has one (Q8).
- `REQ-0659` The SessionStart hook (8.2) prints the index of the open memory
  entries, one line each, so the agent sees what is kept and reads an
  entry with `julieta memory show <id>` when the work touches it, and one
  fixed line every session: `built-in memory is disabled, use julieta
  memory add`. The index, not the bodies, is what the agent's own memory
  loads at a session's start, and it is what fits the bound of 8.6.
- `REQ-0660` The skill rule (`skills/julieta/SKILL.md`) tells the agent to use
  `julieta memory` for anything worth keeping, which type tag of 03 3.9
  each case takes (`user`, `feedback`, `project`, `reference`; `handoff`
  through the `handoff` skill, 8.3), that a session hitting friction
  worth fixing writes a `feedback` entry so that another session can
  triage it in parallel (03 3.9), which target each entry takes
  (`--repo <dir|url>`, `-C <path>` or `--shared`, exactly one, 03 3.9), and to
  keep out what the code, the git history or an instruction file
  (`CLAUDE.md`, `AGENTS.md`, `.claude/rules/`) already holds; `add`
  warns `memory-derivable` on the part of that rule a check can judge
  (04 4.3). `REQ-0661` Something worth
  keeping that no memory entry fits, and that a runtime event would
  carry once the ledger is built, goes into the handoff, and the agent
  opens an issue labelled `deferred` that names the free-text note row
  of the index's [Deferred decisions](../spec.md#in-the-product). `REQ-0662` When
  signing a commit fails, or waits on a confirmation that does not come,
  the rule tells the agent to ask the operator, and never to unset
  `commit.gpgsign` or sign another way.

## 8.2 Session hooks (user-side middleware)

The `julieta-claude` kit wires two Claude Code hooks. They are the
agent-facing counterpart of the repo's pre-push hook and CI: mechanical,
run every time, never remembered.

| Hook | Command | Prints or writes |
|---|---|---|
| `REQ-0663` SessionStart, for every source the pinned Claude Code fires (startup, resume, clear, compact) | `julieta memory index --hook` | `REQ-0664` in this order: (1) julieta's warnings, each by its slug of 04 4.1, one line each: `REQ-0665` `lock-stale`, `snapshots-disabled`, `agent-memory-writable`, `agent-path-missing` (8.5 step 5), `repo-setup-failed`, `memory-open-count`, `memory-stale`, `skills-stale`, `layout-stale`, `agent-hooks-missing`, and every recorded hook failure (one line per hook and repo, the newest failure of each, shown until a later run of the same hook succeeds, below; a `setup` that left the sandbox unusable is one, the hook `Setup`, whose line names `$HOME/.local/state/julieta/setup.log`, 06 6.4); (2) the fixed built-in memory line (8.1); (3) the open `handoff` entry of the project's shared dir (8.3): its index line, then the gap line of 8.3 when the newest stamp differs from its stamp, then its body, or the gap line of 8.3 alone when no `handoff` is open; (4) the index of the other open entries over every memory dir of the project, the line of `memory list` (04 4.3) per entry, newest `updatedAt` first, under a line that says how to read one: `read an entry: julieta memory show <id>`. `REQ-0666` It reads every manifest repo's memory dir and labels each item with its repo. `REQ-0667` The output is bounded by the two values of 8.6, each cut at a line: part (3) by the narrative bound, the body cut by whole lines with the last line `read the rest: julieta memory show <id>`, and part (4) by the index bound, the index holding every whole line that fits, as the agent's own memory loads its index, never a partial line, with the last line `<n> more entries: julieta memory list` when any is left out. The warnings are never cut. `REQ-1291` Two warnings keep the index small without a hand, both printed in part (1), both hints to close or update, since julieta deletes nothing (8.1): `memory-open-count`, one line, when the open entries of the project, the `handoff` apart, exceed the count of 8.6. `REQ-1294` `memory-stale`, one line naming the id and the escaped title of each open entry whose `updatedAt` is older than the days of 8.6, oldest first, at most ten lines and then `<n> more stale entries: julieta memory list --status open`, so the part that is never cut stays small |
| SessionEnd | `julieta stamp` | `REQ-0668` a stamp file (03 3.10), so a session that ends without `/handoff` still leaves where the trees were; whether it raises the gap line is decided by what it holds (8.3), not by its time |

`REQ-0669` julieta records in its state the time of the last SessionStart hook run
(`memory index --hook`) and of the last SessionEnd run (`stamp`); 8.5
step 1 reads them.

`REQ-0670` Memory and handoff text is written by agents. SessionStart
shows it to the next session as data, under a line that says so; the
bound limits its volume, not its content
([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).

`REQ-0671` Both hooks invoke julieta by the absolute path of the read-only
`.romeu/bin` mount ([06 6.4](06-kits.md#64-product-kits)). `REQ-0672` The
SessionStart and SessionEnd hooks always exit 0: on a failure each
prints one stdout line naming the failure and `julieta status`, and julieta records it as a hook failure.
`REQ-0673` When julieta itself cannot run (the mount is absent after an sbx
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
`REQ-0674` Which sources fire, that the output reaches the agent's context, its
size limit, the behaviour on a non-zero exit and the SessionEnd timeout
of the pinned version are recorded by one probe row of
[11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last),
which also checks that the stdout of a failing hook still reaches the
agent's context.

`REQ-0675` julieta records every failing git hook run (hook, repo, exit status,
time) in its state, and every `setup` run that leaves the sandbox
unusable as the hook `Setup` with no repo, naming
`$HOME/.local/state/julieta/setup.log` (06 6.4, REQ-1308); SessionStart,
`julieta status` and `romeu status` surface them until a later run of
the same hook succeeds, a clean `setup` for `Setup`.

## 8.3 Handoff

| Aspect | Decision |
|---|---|
| Narrative handoff | `REQ-0676` the operator types `/handoff` (before `/clear`) or `/handoff --final` (before recreate/rm); the skill has the agent write the required headings (03 3.9) and pipe them to `julieta memory add --tag handoff --repo <dir> --body-file -`, with `--tag tombstone` for `--final`; a handoff is a memory entry, and nothing else holds one |
| Stamp | measured by julieta into the entry at `add`, and written as a file by the SessionEnd hook (8.2; 03 3.10) |
| Location | `REQ-0677` the project's shared dir, with `repo` the url of the repo the narrative is about, named by `--repo` or `-C` (03 3.9, REQ-1296); the command prints `shared` and that repo. Every other entry is filed under the repo `add` names, never under the working directory, which the agent's shell does not keep between commands (03 3.9), and the command prints it, so work across repos is filed where the agent meant, and a repo the manifest does not hold is refused, never filed under another (decided on 2026-10-29, after a session's notes on a repo outside the manifest were filed under this one) |
| One open | `REQ-0678` `add` of a `handoff` closes the open one first (03 3.9), and the migration of 04 4.3 leaves one open, so the project has one open narrative; the reader takes the newest `createdAt` should a hand edit leave two |
| Stamping | `REQ-0679` julieta writes the `stamp` key of the entry at `add` and refuses one supplied by the caller (03 3.9) |
| Reader | `REQ-0680` one function in `memstore`, used by julieta and romeu: "the open handoff" = the open `handoff` entry of the shared dir; "newest stamp" = the greatest ULID among the files of `stamps/`; no file hides an entry |
| Resume | `REQ-0681` SessionStart prints the open handoff plus the difference between its stamp and the newest stamp. The gap line `the newest stamp differs from the handoff of <time>: <changes>`, where `<changes>` lists the count of new commits, a moved branch and a changed dirty count, prints only when the newest stamp file differs from the stamp of the open handoff; a newer stamp with the same positions, such as the one SessionEnd writes at `/clear` after `/handoff`, prints no gap line. The line names what is measured, not how the session ended. When no `handoff` is open, part (3) of 8.2 is the one line `no handoff is open; the newest stamp is of <time>`, or `no handoff and no stamp yet` before the first session end. `REQ-0682` The container e2e asserts both: `/handoff` then `/clear` with no commit (no line), and `/handoff`, a commit, then `/clear` (the line) |
| Before destruction | `REQ-0683` after the preflight and before the salvage half, `romeu rm` asks the reader whether the shared dir holds an open `handoff` tagged `tombstone` whose stamp names the open generation; on a TTY it asks whether to go on without one, and without a TTY it warns (`no-tombstone`) and goes on (04 4.2). The narrative handoff is the operator's step: romeu cannot ask a stopped agent for one (J6) |
| Host | `REQ-0684` `romeu memory list\|show <name>` prints the index, the open handoff first, and entries in full, escaped and labelled as text an agent wrote (04 4.2) |

Claude Code fires SessionEnd when `/clear` ends a session, then
SessionStart for the new one (Q22). `REQ-0685` With the rules above, the stamp file
written at `/clear` never displaces the handoff written by `/handoff`
just before it, whatever the order of the two hooks: a stamp is a file
and a handoff an entry.

*Why for us:* agent state kept in git notes is invisible to a heads-only
refspec; a plain file on a host mount survives the sandbox without git
plumbing.

## 8.4 Snapshot (after every commit)

- `REQ-0686` Trigger: julieta's hook dispatcher after `post-commit`,
  `post-rewrite`, `post-merge`, with no debounce (the bundle holds the
  objects not reachable from the last synced base, so it grows with the
  work since the last sync); `REQ-0687` the romeu commands that snapshot
  are named in their rows of [04 4.2](04-cli.md#42-romeu-host).
- `REQ-0688` Content: `git bundle create` of all local branches, tags, notes, the
  current generation's `refs/salvage/<generation>/*` refs (no earlier
  generation's), every worktree HEAD and, per branch, each reflog
  entry's commit not reachable from a bundled ref, as
  `refs/salvage-reflog/<branch>/<n>`, excluding objects reachable from
  the manifest's `repos[].base` (the origin SHAs the host already has,
  the same base salvage uses). One bundle is kept, not the last N.
- `REQ-0689` Write: `snapshot/heads.bundle` is renamed into place first and
  `snapshot/heads.json` (`schema: snapshot/v1`, ref -> SHA, `createdAt`,
  `disabled`) after it, each by the atomic write rule of 03 3.8. romeu
  trusts the bundle's own refs and reads `heads.json` for `status` only,
  so a kill between the two writes leaves nothing romeu acts on wrongly.
  `REQ-0690` A snapshot of a repo takes the store lock of that repo's memory dir
  (8.1, `locks/<dir>.lock`) for the two writes, so two snapshots of one
  repo run one after the other and the pair on disk is always one
  snapshot's.
- `REQ-0691` A snapshot that cannot be written never undoes the commit: it keeps
  the previous bundle, removes its own `.tmp-<ULID>` file before the
  hook failure is recorded, is recorded as a hook failure with its
  repo (8.2), and the repo's own hooks still run (04 4.3). A kill
  leaves the file, and `julieta setup` stays its cleanup (8.1).
- `REQ-0692` A repo whose local config sets `core.hooksPath` bypasses the
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

`REQ-0693` Salvage covers the generation's recorded repo set
([01 1.6](01-system-model.md#generation)), not the current spec. `REQ-0694` Every
git call of the sandbox half runs in gitsafe's sandbox mode
([05 5.1](05-security.md#51-hardened-git-internalgitsafe)).

### Sandbox half: `julieta salvage` (run by romeu via `sbx env exec`)

romeu passes `salvageRun.id` and, per repo, `repos[].base` in the
manifest.

1. `REQ-0695` `--stop-agents`: SIGTERM to the agent's processes (names from
   `internal/agent/<agent>`), wait up to 10 s, then SIGKILL; record it
   in the manifest's `agentStop` (3.11). `REQ-0696` Zero matched while a session is
   open, read from julieta state (a SessionStart recorded by `memory
   index --hook` with no later SessionEnd from `stamp`, 8.2), is the
   skipped item `agent-not-matched`; zero matched with no
   open session is recorded in the manifest's `agentStop` (3.11) and is
   no item. `REQ-0697` Then salvage waits on the store lock of every manifest
   memory dir, the shared one included (8.1), so a SessionEnd hook already running finishes
   first; `REQ-0698` while it waits more than 10 s on a lock it prints the lock's
   path on stderr and in its `--json` progress, and the wait has no
   bound, since a stamp must finish and the signal rule of 04 4.1 is
   the way out; `REQ-0699` a stamp written during salvage is allowed and
   not counted for completeness. `cmd/julieta` passes the agent
   profile (process names, state paths) to `salvage`; the `salvage`
   package, which romeu links too, does not import `agent/*`.
2. For each repo in the manifest, and for each nested repository found
   inside a worktree (a `.git` below the top level), for each worktree
   from `git worktree list --porcelain`:
   - `REQ-0700` uncommitted tracked and untracked (non-ignored) changes: tree
     through a temporary `GIT_INDEX_FILE`, `commit-tree` with parent HEAD,
     `-c commit.gpgsign=false`, author `julieta-salvage`, ref
     `refs/salvage/<generation>/<salvage-id>/<worktree-n>`, outside
     `refs/heads`, so `git push --all` or `--mirror` never publishes it;
     worktree and index untouched;
   - `REQ-0701` each stash becomes `refs/salvage/<generation>/<salvage-id>/stash-<n>`;
   - `REQ-0702` each worktree's HEAD and status are read again; if either moved,
     the capture repeats once, then the worktree is skipped as
     `worktree-changing`. A rebase, merge, cherry-pick or bisect in
     progress is skipped as `in-progress-operation`. Other processes
     are not stopped; this recheck is what catches them.

   `REQ-0703` Untracked files count against the salvage cap of step 4 and are
   listed `over-cap` beyond it. Salvage writes objects into the
   sandbox's repositories, so a full sandbox disk fails this step;
   freeing space inside the sandbox is the recovery before
   `--from-host`.
3. `REQ-0704` One bundle per repo with all local branches, tags, notes, the
   current generation's `refs/salvage/<generation>/*` refs, every
   worktree HEAD (detached ones included) and the reflog-only commits of
   8.4, excluding objects reachable from `repos[].base` (host-known
   SHAs, not the sandbox's remote-tracking refs). A nested repository
   gets its own bundle.
4. `REQ-0705` Ignored files, minus `salvage.excludeIgnored`, into `ignored.tar.gz`
   up to the salvage cap of 8.6, shared with the untracked files of step
   2; the rest listed as `over-cap`.
5. `REQ-0706` Agent state (from `internal/agent/claude`): project memory files and
   todos to `agent/`; transcripts excluded unless `--include-transcripts`
   (they may contain echoed tokens), listed as `transcript-excluded`;
   `REQ-0707` scratch dirs named by the agent profile within the same cap. The
   profile splits its paths into required ones, which exist at the
   pinned agent (`julieta setup` and probe C5 check them), and
   optional ones, such as a todos or scratch dir a session never
   created. Only a missing required path is the skipped item
   `agent-path-missing`; a missing optional path is no item.
6. `REQ-0708` Write `manifest.json` (3.11). A salvage dir without its
   `manifest.json`, which an interrupted salvage leaves, is ignored by
   verification and kept.

`REQ-0709` `julieta salvage --fingerprint` ([04 4.3](04-cli.md#43-julieta-sandbox))
runs only the reads of steps 2 and 4 and prints the sandbox-side
values of the fingerprint of
[03 3.8](03-formats.md#38-host-state-schemas-state-v1); it writes
nothing and stops no agent. romeu fetches the daemon heads itself, as
step 3 of its half does.

**Salvage payloads are secret-bearing.** `REQ-0710` `ignored.tar.gz`, `agent/` and
transcripts can hold credentials and code. Their dirs are 0700 and their
files 0600; they stay until the owner deletes them and are part of any
backup of `$ROMEU_ROOT`. `REQ-0711` `romeu salvage` prints `Salvage payloads are
secret-bearing: ignored files and transcripts can hold credentials and
code.` once when it stores ignored files or transcripts. Keeping them in
the mount is a risk accepted in
[05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1). `REQ-0712` A root
excluded from Time Machine, as the `root-indexed` check of
[04 4.2](04-cli.md#42-romeu-host) advises, is in no Time Machine backup,
so its memory dirs and unpushed work need another copy, which that
check's hint says. Once `romeu salvage` has recorded its result on the
generation, and the generation is not `removing`, the owner may delete
that salvage dir or any payload in it: `rm`, `recreate` and `retire`
read the record, `doctor` checks the salvage refs on the host and no
salvage dir, and `status` lists the records, so a deleted dir is no
finding. While the generation is `removing`, a rerun may salvage again
(01 1.6), so the dir stays. `REQ-0713` The refs under `refs/romeu/salvage/` stay in
every case, since `doctor` fails on a missing one.

### Host half: `romeu salvage`

1. `REQ-0714` Read the manifest via `os.Root`; `salvageId` must equal the id romeu
   passed; every repo in the generation's repo set must appear (a
   missing one is the skipped item `repo-missing`; a `salvageId` other
   than the id romeu passed is the error `salvage-id-mismatch`,
   `RJ-339`, exit 5, not a skipped item); verify every file's size and
   sha256 (a mismatch is the skipped item `payload-mismatch`), under the
   path and cap rules of
   [03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1).
2. `REQ-0715` For each bundle: validate every ref name, `git bundle verify`
   (hardened; fetch origin into `refs/romeu/origin/` first if
   prerequisites are missing), `git bundle unbundle`, then
   `git fsck --strict` on each unbundled head, as `pull` does (04 4.2),
   since `transfer.fsckObjects` does not cover an unbundle (05 5.1); a
   failure is the skipped item `fsck-failed`. Then create-only refs
   under `refs/romeu/salvage/<name>/<generation>/<salvage-id>/<dir>/...`.
3. `REQ-0716` Primary repo: hardened fetch from the live daemon; every daemon head
   must be present in the bundle refs with the same SHA; a head absent
   or different is the skipped item `daemon-heads-missing`. A daemon URL
   whose scheme or host gitsafe refuses is the skipped item
   `daemon-url-refused`, which names sbx, because the salvage
   cross-check assumes the daemon at `git://127.0.0.1`. Secondary repos
   have no daemon, so no cross-check (05 5.4).
4. `REQ-0717` Recompute completeness (ignoring the manifest's own `complete`):
   `result: complete` only when nothing was skipped except
   `excluded-by-spec` and `transcript-excluded`; otherwise `incomplete`
   with each skipped item as a reason; record it on the generation; exit
   5 when incomplete (01 1.6).
5. `REQ-0718` `--from-host` (sandbox dead or unreachable): skip the sandbox half;
   import `refs/sandboxes/<name>/*` and every `snapshot/heads.bundle`;
   record `result: lost, reasons: [sandbox-lost]`.

### Skipped reasons and their recovery

The reasons are those of
[03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1); `REQ-0719` this
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
| `daemon-url-refused`, `repo-missing`, `daemon-heads-missing` | rerun once the sandbox is reachable at its daemon; a persisting one names an sbx change |
| `payload-mismatch` | the payload is corrupt; rerun, or accept the loss of that payload |
| `salvage-id-mismatch` (`RJ-339`) | rerun; a mismatch that persists names a julieta that wrote another salvage's manifest, report it as an issue |

`REQ-0720` A rerun of `rm`, `recreate` or `retire` on a `removing` generation whose
fresh fingerprint
([03 3.8](03-formats.md#38-host-state-schemas-state-v1)) differs
salvages again and ends as `fingerprint-unstable` (`RJ-338`,
[04 4.4](04-cli.md#44-error-ids)) says; that error is not a skipped
reason, so `--accept-loss` does not cover it.

### Not salvaged

`REQ-0721` Salvage keeps only what the steps above name. It does not keep paths
outside the recorded repos and the agent profile's state paths, clones
that no spec records, `/tmp`, installed plugins, multiplexer scrollback,
or the workload's volumes ([06 6.2](06-kits.md#62-workload-choice)),
which `rm` and `recreate` list before salvage.

### Recovery

Salvage and preservation stop at the host. Recovery is the operator's
in v1: a branch or salvage commit comes back by the J10 step 3 commands
(a `recover/<b>` branch from the salvage ref, then a push to origin,
which the next sandbox fetches); `REQ-0722` `ignored.tar.gz` and `agent/` are
inspected on the host and copied in by hand, and an archive is
extracted inside the sandbox only (J6 step 6). Each preservation prints
the salvage ref prefix it wrote (04 4.2). A `romeu restore` command is
a row of [Deferred decisions](../spec.md#in-the-product).

*Why for us:* Codespaces keeps only `/workspaces` across a rebuild and
DevPod can orphan state on force delete; we prove preservation with a
verified manifest before any destructive step.

## 8.6 Starting values

These are starting values: the plan may tune them, and this table is the
one place they are written.

| Value | Default |
|---|---|
| size of one entry or stamp file | 1 MiB |
| entries per memory dir | 10 000 |
| stamp files julieta keeps in `stamps/` | the newest 50 |
| SessionStart handoff, part (3) of REQ-0664, and the `handoff-long` warning of `add` | 4 KiB |
| SessionStart index, part (4) of REQ-0664 | 24 KiB (about 130 lines of 180 bytes; the agent's own memory loads 25 KB). Both bounds hold within the size of hook output the pinned agent passes to its context, which probe C5 records (REQ-0674); a smaller measured size lowers the index bound first |
| open entries of a project before `memory-open-count` | 100, below what the index bound holds, so the warning comes before the cut |
| days without an update before an open entry is `memory-stale` | 90 |
| salvage cap, per salvage (the manifest's `salvage.capBytes`, 03 3.6) | 1 GiB |
