# 4. CLI surface

Back to [index](../spec.md). Reader: implementers of `romeu-cli` and
`julieta-core`. Type: reference. The generated `docs/reference/` pages
are derived from the same data this page specifies.

## 4.1 Conventions (both binaries)

| Topic | Rule |
|---|---|
| Parsing | stdlib `flag`, `<binary> <command> [subcommand] [flags] [args]`; flags precede positional arguments, because `flag` stops at the first argument that is not a flag; `--help` on every level |
| Output | human text on stdout; `--json` on every command a program calls, named in its row, emits one JSON document, or one object per line where the row says so, with a schema in `schemas/` named after the command (`<binary>-<command>.v1`, [02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)); errors on stderr, one line first, details after. All output goes through the CLI writer, which escapes with `termsafe` by default (I29); raw output is an explicit, named exception (only the final `exec` of `romeu run`, see 05 5.4). Every document romeu decodes from a subprocess in a sandbox or from a mount is decoded strictly with a size cap, and no string from it becomes a path, a ref or an argv element without the validators of [03 3.1](03-formats.md#31-name-and-path-rules-shared) |
| Exit codes | `0` ok; `1` operation failed; `2` usage or precondition (bad flags, invalid settings/spec, missing or too-old sbx/git, missing secret binding, incompatible julieta, unknown sandbox, illegal state transition); `3` approval required (gate 1 or 2); `4` drift detected, or an interrupted promotion abandoned; `5` refused to protect data (incomplete salvage, a fingerprint that keeps changing during a removal, unrecovered or unpushed work, a stopped sandbox that needs a recreate); `6` the check ran and found what it checks for (`romeu doctor`, `julieta lock --check`, `julieta pin check`, `julieta spec validate`, `julieta memory check`), so `1` keeps the one meaning "the operation failed" |
| Error ids | every error has a stable id `RJ-<nnn>`, a slug, an exit code, a message, a fix hint and a "recovered in" cell, defined in one data table in `internal/cli`; the cell holds the `docs/guide/` anchor of the error's recovery or `none: <reason>` ([12 12.3](12-engineering.md#123-generators), 12 12.8). The spec names an error by both, as "the error `format-newer` (`RJ-311`)": the id is the stable key, the slug its readable name. The number is one sequence and carries no meaning: the exit code is a field of the row, so an error moved to another exit keeps its id. An id is never reused or renumbered; a retired id stays in the table, marked retired, with its last text. The ids written before this rule keep their numbers, and a new id takes the next number after the highest, and a pull request that adds one is rebased on the current base, with its checks run again, before the merge ([12 12.3](12-engineering.md#123-generators)). The text output prints `error RJ-301 project-approval-required: <message>` then `fix: <hint>`; `--json` errors carry `{"id", "slug", "exit", "message", "fix"}` and not the "recovered in" cell, which a reader reaches by the id in `docs/reference/errors.md`; tests assert ids, never message text; `docs/reference/errors.md`, with each id's recovery link, and `exit-codes.md` are generated from the table. A fix hint begins with a command line the user can run, its placeholders filled from context, or with `decide:` and the choice; a test over the generated command definitions parses every command line in a fix hint and in a `--help` example into its command, subcommand and flags, and fails on one the definitions do not have, as 12 12.3 does for a `SKILL.md`; a hint that names `--overwrite-drift`, `--accept-loss` or `--force` says what that flag discards. The details of an error a failed subprocess raised carry its escaped argv, its exit status, the first 4 KiB of its stderr, with the home path, user name, host name and root path replaced as the recorder of [10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract) does, the numbered step of 4.2 it ran in, and the romeu, julieta and sbx versions. The argv, the status, the step and the versions hold no secret value, since romeu never handles one (I25), so they can be pasted into an issue. A token in a remote URL or an sbx error body is not removed, so the reader looks the stderr over before pasting it. A subsystem's reasons are rows of the same table, each row saying who raises it. The ids this specification names are listed in 4.4 |
| Warnings | a warning is printed, escaped, and changes no exit code. Each is a row of the same table, by slug and with no id. Each check of `romeu doctor` (4.2) is a row by its slug too: a check that stops a command, as each **pre** check does in the preflight, is an error row with an id (4.4), and a check that only warns stays id-less. The warnings: `sbx-untested`, `spec-moved`, `recreate-needed`, `layout-stale`, `skills-stale`, `snapshots-disabled`, `no-final-handoff`, `version-only-tool`, `default-branch-moved`, `lock-no-checksum`, `lock-unknown-tool`, `agent-hooks-missing`, `julieta-pin-mismatch`. `recreate-needed` names one condition, the recreate digest differing from the generation's: a warning on a running sandbox and the error `RJ-305` on a stopped one, so the two rows share the slug |
| Idempotency | a mutating command converges: a second run with no input change leaves every file's content and mode and every ref as they were, except the documented state timestamp `lastRunAt`. The appending commands add one thing per run by design, and this is the list: `romeu salvage`, and `rm`, `recreate` and `retire` through it (a salvage id with its record and refs); `romeu stop`, `romeu pull` and `julieta snapshot` (a rewritten `snapshot/heads.json`, whose `createdAt` moves); `julieta memory add` (an entry, unless an open entry with the same content digest exists: then it prints that entry's id and writes nothing); `julieta handoff write` (a handoff file). Each appending julieta command prints what it wrote, its id and path, in text and with `--json`, so a caller whose call timed out can tell whether the write landed. This list, the **P** marker of 4.2 and the lock of the next row are fields of each command definition, with no default, so a definition without them fails generation ([12 12.3](12-engineering.md#123-generators)); the meta-test of 10 10.1 reads the same fields. This list and the **P** marker of 4.2 are design input: the generated `docs/reference/<command>.md` pages replace them once the command definitions land, the rule [03](03-formats.md) (opening) states for its tables |
| Locking | mutating romeu commands take `$XDG_STATE_HOME/romeu/romeu.lock` (flock, non-blocking; `RJ-101 locked`, "another romeu is running"), then run promotion recovery (01 1.6). The lock is held for the whole command, sbx calls included, and released before the final `exec` of `run`, so an attached `run` blocks no other command. One lock per machine is a chosen cost: commands on different projects wait for each other, a create that builds kits included; a lock per project is a row of [Deferred decisions](../spec.md#in-the-product) |
| Subprocesses | romeu: only `sbx` and `git` by absolute path from host settings, with a scrubbed env (`PATH=/usr/bin:/bin`, `HOME`, `LANG=C`, `TERM` for interactive calls, and `SSH_AUTH_SOCK=<signing.agentSocket>` for sbx calls of a project using `git-ssh-sign`). julieta: `git`, `mise`, `herdr`. `sbx version`, `sbx ls --json` and the settings and policy listings are reads; a mutating sbx call changes a sandbox, a policy or a secret. Every subprocess has a timeout (10 10.6), past which it fails with `subprocess-timeout`, except the salvage exec, which moves up to the salvage cap over the mount and stops only on an interrupt; an interrupted salvage leaves its dir without a `manifest.json`, which verification ignores and keeps (08 8.5) |
| Version | `romeu version` / `julieta version` print version, commit, build date, Go version, protocol; `--json` adds the binary sha256, `GOARCH`, and (julieta) whether its own directory is writable. `romeu version --json` also carries a `versions` object: sbx, git, macOS and, per project, the workload digest and the kit pins of its live render, so a report can paste one document |

## 4.2 romeu (host)

Every command marked **P** runs the preflight of
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call) before any
mutating sbx call. Every command whose Runs cell names julieta runs the
compatibility check of [06 6.3](06-kits.md#63-julieta-delivery) before
its first julieta call.

| Command | Purpose | Reads | Writes | Runs | Exit codes |
|---|---|---|---|---|---|
| `romeu init --config-url <url> [--config-project <name>] [--root <abs>] [--config-ref <ref>]` | create host settings; clone the config repo into `<root>/<cfg>-env/<dir>`. It needs an existing config repo whose ref holds `projects/<cfg>.yaml` (J1 step 0), and applies `doctor`'s root check before it writes | nothing prior | host settings, its `root`, `config` and `tools` keys ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1); refuses to overwrite), one clone | host: git | 0, 1, 2 |
| `romeu sync [--all] [--from origin\|<sha>] [--overwrite-drift] [<name>...]` **P** | render specs at a named commit into candidates; run gate 2; promote; apply live changes | host settings, config repo objects, repo objects, registries (descriptors not cached), state | clones (missing only), `refs/romeu/origin/*`, candidates, live derived files on promotion, `<name>-env/` and memory dirs (create only), the `.metadata_never_index` marker, workspace files, project records, descriptor cache, live egress | host: git, sbx (preflight step 1 before its first sbx read and steps 2 to 6 before its first mutating call, step 9 below) | 0, 1, 2, 3, 4 |
| `romeu approve --toolchain` | gate 1 (01 1.4); TTY required; makes no mutating sbx call (it reads `sbx version`) | `toolchain.json`, live `sbx version` | `toolchain.json` | host: sbx (read) | 0, 1, 2 (no TTY) |
| `romeu run [--no-attach] <name>` **P** | ensure sandbox, egress, julieta, setup; attach to the run layout | everything above | project record (generation, `lastRunAt`, `egressApplied`), salvage refs and record when preserving a lost generation | host: sbx, git; sandbox: julieta | 0-5 (5: step 3, a stopped sandbox that needs a recreate) |
| `romeu adopt <name>` | record an existing sandbox named `<name>` as an open generation (01 1.7); before the prompt it prints, escaped, the mounts, policies, secrets and image sbx reports for it; TTY required; `status` shows the generation as `adopted, not gated` until it is recreated ([01 1.6](01-system-model.md#generation)) | `sbx ls --json`, `sbx policy ls`, project record | project record | host: sbx | 0, 1, 2 (no TTY, open generation exists, workspace path differs, no such sandbox), 3 (gate 1) |
| `romeu stop <name>` **P** | `julieta snapshot --all`, then `sbx stop`. A failed snapshot is reported and the stop goes on, since the sandbox's disk persists; the command then exits 1. The next `run` starts the sandbox (run step 3) | project record | memory (via julieta) | host: sbx; sandbox: julieta | 0, 1, 2, 4 |
| `romeu salvage [--from-host] [--include-transcripts] <name>` **P** | capture everything sandbox-only; verify and import on host ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)). It stops agents (`julieta salvage --stop-agents`) only when `rm`, `recreate` or `retire` calls it; a standalone salvage leaves agents running and says so in its report. A stopped sandbox is started for the sandbox half and stopped again. When the start or the sandbox half fails, salvage exits 1 naming `--from-host` and what it does not recover: uncommitted changes, stashes, ignored files and agent state. Each preservation prints the salvage ref prefix it wrote | project record, memory dir | memory (via julieta), `refs/romeu/salvage/...`, project record | host: sbx, git; sandbox: julieta | 0, 1, 2, 4, 5 |
| `romeu rm [--accept-loss=<reason>[,<reason>]] <name>` **P** | after the preflight and before the salvage half, the handoff reader checks for a `final` handoff of the generation: on a TTY `rm` asks whether to go on without one, and without a TTY it warns (`no-final-handoff`) and goes on. Then it prints the workload volumes that salvage does not cover ([06 6.2](06-kits.md#62-workload-choice)), salvages, and removes the sandbox; the tree is kept. `--accept-loss` accepts only the skipped reasons it names ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)), and the salvage record keeps them. A rerun on a `removing` generation whose fingerprint keeps changing exits 5 with `fingerprint-unstable` (4.4), which `--accept-loss` does not cover | project record, memory handoff dir | as salvage; sbx removal; egress rules removed; generation closed | host: sbx, git | 0-5 |
| `romeu recreate [--accept-loss=<reason>[,<reason>]] <name>` **P** | `rm` then `run`. A create that fails after the removal, because a workload, frontend or kit download is gone, leaves the generation closed, the salvage refs and memory intact and no sandbox; the error is `kit-build-failed`, and the recovery is a pin or catalog fix, then `romeu run` (J6) | as rm + run | as rm + run | as rm + run | 0-5 |
| `romeu retire [--force] [--accept-loss=<reason>[,<reason>]] <name>` **P** | `rm` if needed; an incomplete inner salvage exits 5 before any move unless `--accept-loss` names its reasons. Report unpushed and salvage-only work and confirm on a TTY; off a TTY, `--accept-loss` stands for the confirmation. Say whether the workload volumes are left (06 6.2); move `<name>-env/` to `.attic/`; drop it from the workspace files. `--force` only allows a project the spec still lists | spec (the project must be absent unless `--force`), clones | `.attic/`, workspace files, state attic | host: git, sbx | 0-5 (5: an incomplete salvage, `fingerprint-unstable` (4.4), a declined confirmation, or no TTY without `--accept-loss`) |
| `romeu status [--json] [<name>]` | per project: sandbox state, spec vs synced commit, drift, interrupted promotion, toolchain acknowledgement state, recreate needed, generations and their states (`adopted, not gated` for an adopted one), salvage records, unimported snapshots, orphaned projects and repo dirs; per repo, the `createdAt` of the last `snapshot/heads.json`, `disabled` when it says so ([08 8.4](08-memory-handoff-salvage.md#84-snapshot-after-every-commit)), and the newest facts' dirty and stash counts; the warnings of 4.1; for a running sandbox also julieta's warnings and recorded hook failures (via `julieta status --json`). `status` runs no preflight, so it skips that exec, and says so, when the project's drift or identity report is not clean | read only | nothing | host: sbx, git (read); sandbox: julieta (read) | 0, 1 (never 3) |
| `romeu doctor [--json]` | host health and security checks (below) | host settings, git config, sbx settings, policies and `sbx ls --json`, `$HOME` symlinks, tool configs, the signing socket, the tree, the clones' refs, each secret argv's output (never kept) | nothing | host: sbx, git, each secret argv | 0 (all pass), 1 (a check could not run), 6 (any fail) |
| `romeu pull [-C <dir>] [--branch <b>]` **P** | fetch the sandbox's current work for the repo containing `<dir>` and update its review checkout | project record, remote URL, `julieta status --json` | `refs/sandboxes/<name>/*` (primary), `refs/romeu/snapshots/<name>/*` (secondary), `review/<dir>/`, `review.code-workspace` | host: git, sbx; sandbox: julieta | 0, 1, 2, 4 |
| `romeu handoff [--repo <dir>] [--json] <name>` | print the latest narrative handoff and the newest facts, labelled as text an agent wrote ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)); read through the handoff reader (I24). Other memory, lessons included, is read with a pager that shows bytes and runs nothing (05 5.4) | memory dir via `os.Root` | nothing | host | 0, 1 |
| `romeu version [--json]` | | | | | 0 |

Removed projects: `sync` never deletes, so there is no `--prune` flag;
a project absent from the spec but present in the tree is reported as
`orphaned` by `sync` and `status`, and `romeu retire` is the explicit
verb. The same applies to a repo removed from a spec (`orphaned repo
dir`, see J13).

### How `romeu sync` renders, gates and promotes

1. Take the lock (runs promotion recovery). Load and validate host
   settings (exit 2; a missing secret binding is `secret-unbound`,
   whose fix hint prints the `<name>@<project>` entry to paste). Check
   gate 1 (01 1.4; exit 3): `sbx version` is a read, and 01 1.5 holds
   back only a mutating sbx call, so a stale toolchain stops `sync`
   before it promotes.
2. Hardened fetch of the config repo's origin into
   `refs/romeu/origin/<cfg-dir>/*`. The spec commit is the head of the
   config ref, or with `--from <sha>` that commit, which must be
   reachable from `refs/romeu/origin/*` (exit 2 otherwise): a spec on
   a branch is read after the branch is pushed.
3. `git fsck --strict` on the spec commit; read `projects/*.yaml` via
   `git cat-file`; strict decode and validate all specs (cross-project
   rules: unique names, unique URLs); exit 2 listing every error.
4. For each target project: create `<name>-env/` (mode 0700, with its
   `.metadata_never_index` marker, [02 2.3](02-layouts.md#23-host-tree))
   and `memory/<dir>/` (create only, with the fixed subdirectories);
   hardened clone of missing repos; hardened fetch of each repo's
   origin into `refs/romeu/origin/<dir>/*`; resolve `egressCommit` from
   `refs/romeu/origin/<dir>/<ref>`.
5. Refuse (exit 2) if any memory dir violates the layout allowlist (I24).
6. Read workload and kit descriptors: the cache in host state first;
   `internal/oci` by digest only on a cache miss (`registry-unreachable`
   when it cannot); local kits from the config commit's tree via
   `git fsck --strict` and the validated walk (I27). Derive egress
   ([07](07-mise-egress.md)). Delete any candidate dir the project
   record does not name. Render the candidate (`sbxenv.yaml`,
   `render.json`, `kits/`) under `.romeu/candidates/`.
7. Drift check of live files against the current `render.json`
   (exit 4, `drift-detected`, naming the file; `--overwrite-drift`
   replaces the edited live file, and its edit is lost).
8. Gate 2 and the candidate state machine
   ([01 1.6](01-system-model.md#candidate)); promotion commit on
   approval or when unchanged.
9. `sbx ls --json`. If the sandbox is running:
   the rest of the preflight (01 1.5), then reconcile egress live; if the `secrets`
   field is tagged `apply:"live"` (A10, Q18), `sbx secret set <svc>
   --sandbox <name> --command <rendered command>` (never a value); report
   "recreate needed" when the recreate digest differs from the running
   generation.
10. Report orphaned projects and repo dirs; remove nothing. Warn
    `julieta-pin-mismatch` when the julieta release the config repo's
    `validate.yml` pins is not the version romeu embeds.

### How `romeu run` reaches the run layout

1. Preflight (01 1.5). When `refs/romeu/origin/<cfg-dir>/<ref>`, as last
   fetched and with no network call, is ahead of the synced spec
   commit, print the warning `spec-moved: romeu sync <name>`.
2. The identity step of the preflight (01 1.5, step 6) has read
   `sbx ls --json` and refused the two cases that are not this
   project's sandbox: one romeu has no open generation for
   (`RJ-203 unknown-sandbox`, fix hint "check its workspace, then
   `romeu adopt <name>`") and one whose workspace path differs. Absent,
   `sbx-unknown` and `upstream-shape` are as
   [01 1.6](01-system-model.md#generation) defines them. An absent
   sandbox with an open generation is preserved as the closed-lost row
   of 01 1.6 says, keeping a lost record `salvage --from-host` already
   wrote for the generation; otherwise romeu imports
   `refs/sandboxes/<name>/*` and every `snapshot/` bundle into
   `refs/romeu/salvage/<name>/<gen>/<new salvage id>/` and records
   `result: lost` with the newest facts' counts as the reasons
   `dirty-at-last-facts:<n>` and `stashes-at-last-facts:<n>`. Either
   way the generation moves to `closed-lost`. `run` then prints a fixed
   block (the lost generation, the time of its last snapshot, the
   branches and the salvage id preserved with the salvage ref prefix it
   wrote, and the dirty and stash counts as not preserved, with
   `romeu handoff <name>` to read them, J10 step 3 to recover a
   branch, and `romeu run <name>` to create the new sandbox) and exits
   1 with `generation-lost` (`RJ-335`); that next `run` creates
   ([01 1.6](01-system-model.md#generation)).
3. Compare the recreate digest of the live render with the open
   generation's, then act on the sandbox's state:
   - absent: record a new generation first (01 1.6), then
     `sbx env run -d --clone <dir>` (never `--auto-approve`; sbx's own
     prompt appears in this terminal when needed). If sbx returns
     non-zero on that create, read `sbx ls --json`: with the sandbox
     absent, remove the record just written; with it present, half
     built, keep the open record. Exit 1 in both cases (a kit that does
     not build is `kit-build-failed`), so only a crash leaves a record
     without a sandbox, and the next `run` closes it as lost.
   - present and stopped: when the digests differ, exit 5 with
     `recreate-needed` (fix: `romeu recreate <name>`), since starting
     it would run an env file that no longer matches the sandbox;
     otherwise `sbx env run -d <dir>` starts it, with no provisioning
     (what sbx does with an env file that changed under a stopped
     sandbox is recorded by A15).
   - running: no `sbx env run`; when the digests differ, warn
     `recreate-needed` ("recreate needed: <fields> changed -> romeu
     recreate <name>").

   romeu names the env file by path; whether sbx also reads a sandbox
   configuration from the workspace tree is recorded by probe A5, and
   if it does, the drift check refuses one
   ([03 3.3](03-formats.md#33-rendered-sbxenvyaml-sbx-env-file-schemaversion-1)).
4. Reconcile egress: `sbx policy allow network --sandbox <name> <d>` for
   missing domains; `sbx policy rm network --sandbox <name> --resource
   <d>` for domains romeu applied that left the set; verify with
   `sbx policy check network --sandbox <name> <d>`. (Exact argv comes
   from the block A recordings.)
5. Compatibility check (06 6.3): `sbx env exec <dir> --
   <abs .romeu/bin>/julieta-linux-<GOARCH> version --json`.
6. `sbx env exec <dir> --env JULIETA_MANIFEST=... --
   <abs julieta path> setup --json`; on failure print julieta's report
   (escaped) and exit 1 without attaching.
7. Unless `--no-attach`: `exec` into
   `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- <abs julieta path> layout up`.
   When the run layout fails, `layout up` exits with `layout-failed`,
   whose fix hint is the host command that opens a shell in the
   sandbox without it (`sbx env exec -it <dir> -- /bin/bash -l`, with
   `<dir>` filled in; julieta reads the manifest it cached).

### How `romeu pull` updates a review checkout

Resolve the project and repo from `<dir>` (must be inside
`$ROMEU_ROOT/<name>-env/`); preflight; compatibility check (06 6.3);
`julieta snapshot --all` and
`julieta status --json` via exec; primary: confirm running, re-read the
daemon URL, hardened fetch into `refs/sandboxes/<name>/*`, then compare
fetched heads with `julieta status` (mismatch: exit 1, nothing updated);
secondary: verify and unbundle `snapshot/heads.bundle` into
`refs/romeu/snapshots/<name>/<dir>/*`, then run `git fsck --strict` on
the unbundled head, the call kit materialization uses (I27), since
`transfer.fsckObjects` does not cover an unbundle (05 5.1); update `review/<dir>/` (a
standalone repo with the host clone as alternate and the hardening keys
in its local config) to the sandbox's current branch head, detached;
write `review.code-workspace` again from the review checkouts.

### Checks run by `romeu doctor`

Checks marked **pre** also run in the preflight (01 1.5, step 5); one
implementation serves both. Each check is a row of the table of 4.1 by
its slug, which `--json` reports with its result; a check whose
condition is also an error elsewhere (`drift-detected`,
`signing-socket`) shares that error's slug. A **pre** check stops a
mutating command with exit 2, except the protective commands of
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call),
which print it and go on; so each has an id in 4.4.

| Check | Slug | Result |
|---|---|---|
| sbx version below the floor (v0.46.0) or not at the configured absolute path | `sbx-floor` | fail |
| sbx version outside the tested window ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)) | `sbx-untested` | warn |
| git version below the floor ([05 5.1](05-security.md#51-hardened-git-internalgitsafe)) | `git-floor` | fail |
| toolchain acknowledgement missing or stale | `toolchain-stale` | fail |
| host global git config sets a key that [05 5.1](05-security.md#51-hardened-git-internalgitsafe) neutralizes | `host-git-config` | warn, with the review-checkout risk |
| sbx: global (not per-sandbox) secrets for a service any project uses | `sbx-global-secret` | fail (I22) **pre** |
| sbx: a global allow rule of `**` or other broad wildcard, or a permissive default network policy | `sbx-broad-allow` | fail (I22) **pre** |
| sbx: `env.rememberHostCommands` not true | `sbx-remember-commands` | warn |
| sbx: `kit.allowedSources` does not admit the workload registries | `sbx-allowed-sources` | fail |
| root inside a git repo, equal to `$HOME`, not absolute, not owned by the user, or containing host settings/state | `root-unsafe` | fail |
| root under `~/Library/Mobile Documents`, `~/Library/CloudStorage/` or `~/Dropbox`, or not excluded from Time Machine; a `<name>-env/` without its `.metadata_never_index` marker, the form probe A17 measures ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts-first-in-parallel-with-the-first-build-layer)) | `root-indexed` | warn; the hint adds that a root excluded from Time Machine needs another copy of its memory dirs, handoffs and unpushed work; tested by the case "doctor warns root-indexed when the marker is missing" |
| `$ROMEU_ROOT/.attic` or a `<name>-env/` that its group or others can access | `tree-mode` | fail |
| a symlink under `$HOME` (depth 1) or `$HOME/.config` (depth 2) resolving into `$ROMEU_ROOT` | `home-symlink` | fail (I23) **pre** |
| mise `trusted_config_paths` or a direnv allow list covering `$ROMEU_ROOT` | `auto-trust` | fail (I23) **pre** |
| VS Code: `security.workspace.trust.enabled=false`, or `$ROMEU_ROOT`, a parent of it or any folder under it in the trusted folders. Only VS Code's trust settings are checked; another editor's trust model is the operator's to verify | `vscode-trust` | fail (I14, I23) **pre** |
| host settings not 0600, state dir not 0700 | `settings-mode` | fail |
| secret `argv[0]` missing or not absolute; a secret argv that, run once with exactly `HOME` and `PATH=/usr/bin:/bin` ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)), exits non-zero or prints nothing (the output is tested for emptiness only, never printed or kept, I25) | `secret-argv` | fail |
| secret entries for unknown projects | `secret-unknown-project` | warn |
| signing: a project uses `git-ssh-sign` and `signing.agentSocket` is unset, unreachable, or holds a number of keys other than one, or its one key differs from the kit's `signingKey` arg | `signing-socket` | fail **pre** |
| tree: live derived files drifted | `drift-detected` | fail |
| tree: an interrupted promotion | `interrupted-promotion` | fail |
| tree: a memory dir violating the layout allowlist | `memory-layout` | fail |
| tree: a workspace file that differs from the one derived again from host state ([01 1.2](01-system-model.md#12-sources-of-truth-vs-derived)) | `workspace-files` | fail |
| an open generation with no sandbox in `sbx ls --json`, or a sandbox a project names with no generation in `open`, `salvaging` or `removing` | `generation-sandbox` | fail |
| a `removing` generation with its sandbox present ([01 1.6](01-system-model.md#generation)); the hint is `romeu rm <name>` | `removal-pending` | fail |
| a record of a format romeu knows that does not decode ([03](03-formats.md), opening); the hint is as in `RJ-334` | `record-unreadable` | fail |
| a project's `egressApplied` differs from the per-sandbox rules `sbx policy ls` reports | `egress-applied` | fail |
| a ref under a salvage record's `refsPrefix` missing from its clone, naming the record | `salvage-ref-missing` | fail |
| a project record over 256 KiB ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)) | `record-size` | warn |

Each check that fails prints a fix hint (4.1) naming the change that
clears it, because several common setups fail a **pre** check:
`sbx-global-secret` names the per-sandbox form of the sbx secret for
that service and the global one to remove; `sbx-broad-allow` names the
rule to remove; `vscode-trust` names the setting to turn back on or the
trusted folder to remove; `home-symlink` and `auto-trust` name the link
or the trust entry; `signing-socket` points at J1 step 5.

## 4.3 julieta (sandbox)

| Command | Purpose | Reads | Writes | Exit codes |
|---|---|---|---|---|
| `julieta setup [--json]` | idempotent; every git call runs in gitsafe's sandbox mode ([05 5.1](05-security.md#51-hardened-git-internalgitsafe)), as every julieta git call does: point `$HOME/.local/bin/julieta` at the running binary; install the hook dispatcher (global `core.hooksPath`); clone missing secondary repos from origin; fetch origin in every repo and fast-forward the local default branch when it is checked out, clean and fast-forwardable (otherwise report), recording a move as the warning `default-branch-moved` (`<repo> <old>..<new>`), which `julieta status` shows and the first SessionStart after the move shows once, then clears ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)); `install`; `memory check`; agent memory dir check; the required agent-profile paths of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction) step 5 exist; compare the installed skills and hook lines with the digest julieta embeds (06 6.3, `skills-stale`) | manifest | `$HOME/.local/bin/julieta`, `$HOME/src/...`, repos (ff only), git global config (sandbox), dispatcher dir, mise data, julieta state | 0, 1, 2 |
| `julieta install [--repo <dir>] [--force]` | `mise install` (locked) in every repo; skip when the `lock-set` digest equals the last success. A download blocked by egress fails naming the tool, the host, the sbx command that approves it once for this sandbox (lost at recreate), and the durable fix, a spec edit and gate 2; egress follows the spec's `ref`, so a tool added on a branch installs once it merges; the error says whether the host is in the egress set the manifest carries ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress)) | manifest, locks | mise data, julieta state | 0, 1 |
| `julieta lock [--repo <dir>] [--check]` | `mise lock --platform linux-x64,linux-arm64`, the platforms a sandbox runs; a repo whose CI runs on macOS adds those platforms in its own mise settings. `--check` verifies freshness without writing: the lock parses with the pinned mise, every tool in `mise.toml` has a lock entry, and every lockable entry covers `linux-x64` and `linux-arm64`; a lockable entry without a checksum is the warning `lock-no-checksum`, and a `backend:tool` the embedded catalog does not know is the warning `lock-unknown-tool`, naming the two fixes of 07 7.5 step 2 | `mise.toml`, `mise.lock` | `mise.lock` (not with `--check`) | 0, 1, 6 (`--check` found a missing entry or platform) |
| `julieta hooks run <hook> [args]` | dispatcher: julieta's own action (`pre-commit`: `lock --check` when `mise.toml` or `mise.lock` is staged; `post-commit`, `post-rewrite`, `post-merge`: `snapshot`), then the repo's tracked `.githooks/<hook>` if executable, then `$GIT_DIR/hooks/<hook>` if executable; the hook's arguments go to each, and its stdin to the first of the two that exists; a failure of julieta's own `snapshot` is recorded and does not stop the chain (08 8.4); otherwise the first non-zero status stops, and before exiting the dispatcher prints on stderr `julieta hooks: <hook> stage <julieta\|tracked\|local> (<path>) exited <n>`, plus the error id and fix when the stage is julieta's own; a failure is recorded in julieta state until the same hook next succeeds | repo | as the hooks do; julieta state | hook's status |
| `julieta memory add\|list\|show\|edit [--repo <dir>] [--json]` | memory entries of the repo containing cwd (or `--repo`); `list --query <text>` filters; `edit --status done` closes an entry; julieta stamps id and timestamps; `add` prints the id, the path it wrote and the repo it filed the entry under | manifest, memory dir | memory dir | 0, 1, 2 |
| `julieta memory check` | every manifest memory dir is mounted, writable, and satisfies the layout allowlist | manifest | nothing | 0, 1, 6 (a violation of the layout) |
| `julieta handoff write [--final\|--facts]` | read the narrative on stdin (not for `--facts`), validate headings, stamp facts, write `handoff/<ULID>-<kind>.md`, print its id and path | stdin, manifest, repos | memory handoff dir | 0, 1, 2 |
| `julieta handoff show [--hook] [--repo <dir>]` | print the latest narrative handoff, the facts that changed since, and open memory entries; `--hook` (SessionStart) also prints `lesson` entries and julieta's warnings (08 8.2) | manifest, memory, repos, julieta state | nothing | 0, 1 |
| `julieta handoff list [--json]` | list handoffs | manifest, memory | nothing | 0 |
| `julieta snapshot [--repo <dir>\|--all]` | bundle unpushed work into `memory/<dir>/snapshot/` (no debounce) | repos, manifest | memory | 0, 1 |
| `julieta salvage [--stop-agents] [--include-transcripts] [--json]` | full salvage for the `salvageRun` id in the manifest | manifest, repos, agent dirs | memory dirs, `salvage/*` branches (create only) | 0, 1, 5 |
| `julieta layout up [--dry-run [--json]]` | start the herdr server if absent, apply missing tabs, report "layout stale" when the live layout's `run` digest differs from `runDigest`, attach; `--dry-run` prints the `layout.apply` requests without applying (golden tests). A herdr whose protocol differs from the one A13 recorded is `herdr-protocol`; any other failure is `layout-failed` (4.2, run step 7) | manifest | herdr state (not with `--dry-run`) | 0, 1, 2 |
| `julieta pane run <id>` | internal: the herdr leaf argv; runs the pane's steps | manifest | - | last step's status |
| `julieta pin workload [--tag <t>] <spec-file>` | resolve the workload tag to a digest, verify the manifest list covers linux/amd64 and linux/arm64, rewrite the spec field; the registry host is in the config project's `egress.extra` (07 7.5) | spec, registry | spec | 0, 1, 2 |
| `julieta pin check [--workflows] [--json] <path>...` | report pins that are not digests or are behind: spec workloads and, with `--workflows`, the julieta release pinned in the config repo's CI workflows, whose pinned archive sha256 it also compares with that release's `checksums.txt` | files, registry, GitHub releases | nothing | 0, 1, 6 (a pin not a digest, behind, or not matching) |
| `julieta spec validate [--catalog] [--json] <file>...` | strict decode + schema + name/path rules, per file and across files, and one line naming the host-only rules of 03 3.1 it did not check; `--catalog` also fetches each repo's `mise.lock` at its `ref` (hardened, shallow) and reports every `backend:tool` and lock host missing from the embedded catalog: an unknown `backend:tool` is a finding, the case 07 7.5 step 2 makes a sync error, and a missing lock host is a warning, since 07 7.5 step 3 gates it | files; origins with `--catalog` | nothing | 0, 1, 2 (a bad flag or an unreadable file), 6 (any finding in the files given: a decode, schema or rule error, or with `--catalog` an unknown `backend:tool`) |
| `julieta status [--json]` | per repo: branch, HEAD, all local branch heads, ahead/behind, dirty, stashes, worktrees, last snapshot, snapshots disabled (repo-local `core.hooksPath`), tools installed; the live spec commit (`specCommit`, 03 3.6); the egress set romeu applied, from the manifest; agent memory dir state; whether the agent's settings carry the two hooks with julieta's exact commands and both skills are installed (`agent-hooks-missing` otherwise, 06 6.5); warnings; recorded hook failures | manifest, repos, julieta state, agent settings | nothing | 0 |
| `julieta version [--json]` | | | | 0 |

julieta refuses (exit 2) a command whose Reads cell names the manifest
when none is cached and `JULIETA_MANIFEST` is unset.

Environment julieta sets for mise calls, the one list of it, whose
reasons are in [07 7.3](07-mise-egress.md#73-lockfile-rules) and
[07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress):
`MISE_LOCKED=1`, `MISE_DISABLE_UPDATE_WARNING=1`,
`MISE_USE_VERSIONS_HOST_TRACK=false`, `MISE_AUTO_INSTALL=false`,
`MISE_YES=1`, and `MISE_TRUSTED_CONFIG_PATHS` set to the manifest's repo
paths (nothing else is trusted).

The runtime ledger's commands (`romeu ledger ingest` and `query`,
`julieta event add` and `list`), its ingest steps and the events julieta
would write are designed in [13](13-runtime-ledger.md) and are not in
v1; they wait for their row in
[Deferred decisions](../spec.md#in-the-product).

## 4.4 Error ids

The ids this specification names, as rows of the one table of 4.1,
which holds these and every other error. `RJ-101`, `RJ-203`, `RJ-204`
and `RJ-301` were written before the sequence rule of 4.1 and keep
their numbers; the sequence goes on from `RJ-302`. This list stands
until the error table of 4.1 lands in `internal/cli`; from then the
table is the source and this section is a link to
`docs/reference/errors.md`.

| Id | Slug | Exit | Raised by |
|---|---|---|---|
| `RJ-101` | `locked` | 1 | every mutating romeu command (4.1, Locking) |
| `RJ-203` | `unknown-sandbox` | 2 | preflight step 6 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)) |
| `RJ-204` | `signing-socket` | 2 | `sync`, `doctor` and the preflight of a project using `git-ssh-sign` ([06 6.4](06-kits.md#64-product-kits)) |
| `RJ-301` | `project-approval-required` | 3 | gate 2: preflight step 4, and a `sync` without a TTY whose widening digest differs ([01 1.6](01-system-model.md#candidate)) |
| `RJ-302` | `toolchain-approval-required` | 3 | gate 1 ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)); fix `romeu approve --toolchain` |
| `RJ-303` | `drift-detected` | 4 | preflight step 3 and `sync` step 7 |
| `RJ-304` | `interrupted-promotion-abandoned` | 4 | promotion recovery ([01 1.6](01-system-model.md#promotion-commit)); fix `romeu sync <name>` |
| `RJ-305` | `recreate-needed` | 5 | `run` step 3, on a stopped sandbox; on a running one the same slug is a warning (4.1, Warnings) |
| `RJ-306` | `salvage-incomplete` | 5 | `salvage`, `rm`, `recreate`, `retire`; the details name each reason, and the hint the `--accept-loss=<reason>` that accepts it |
| `RJ-307` | `salvage-path` | 5 | salvage verification ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)) |
| `RJ-308` | `sbx-unknown` | 1 | any command that needs to know whether a sandbox is absent, when `sbx ls --json` exits non-zero ([01 1.6](01-system-model.md#generation)) |
| `RJ-309` | `sbx-output-unparsed` | 2 | retired: merged into `RJ-310`; its last text was "gate 1 and preflight step 5" |
| `RJ-310` | `upstream-shape` | 2 | every decoder of an output the product does not own, sbx outputs included: gate 1, preflight step 5 and the generation machine ([03](03-formats.md), opening) |
| `RJ-311` | `format-newer` | 2 | every reader of a versioned format ([03](03-formats.md), opening) |
| `RJ-312` | `julieta-not-embedded` | 2 | `run`, `sync` and promotion in a build without julieta ([02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)) |
| `RJ-313` | `manifest-size` | 2 | `sync` ([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)); `julieta spec validate` reports the same rule as a finding of `RJ-328` |
| `RJ-314` | `secret-in-argv` | 2 | host settings load ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)) |
| `RJ-315` | `secret-unbound` | 2 | `sync` step 1; the hint prints the entry to paste |
| `RJ-316` | `julieta-protocol` | 2 | the compatibility check ([06 6.3](06-kits.md#63-julieta-delivery)) |
| `RJ-317` | `julieta-digest` | 2 | the compatibility check |
| `RJ-318` | `julieta-bin-writable` | 2 | the compatibility check |
| `RJ-319` | `herdr-protocol` | 2 | `julieta layout up` |
| `RJ-320` | `layout-failed` | 1 | `julieta layout up`, the last step of `run` |
| `RJ-321` | `kit-build-failed` | 1 | `run` step 3 and `recreate` |
| `RJ-322` | `registry-unreachable` | 1 | `sync` step 6, `julieta pin workload`, `julieta pin check` |
| `RJ-323` | `subprocess-timeout` | 1 | any subprocess past its timeout (4.1, Subprocesses) |
| `RJ-324` | `disk-full` | 1 | any write; inside a salvage it is a skipped reason instead (03 3.11) |
| `RJ-325` | `lock-unparsed` | 2 | `sync` ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress), step 1) |
| `RJ-326` | `lock-host-invalid` | 2 | `sync` (07 7.5, step 3) |
| `RJ-327` | `catalog-unknown-tool` | 2 | `sync` (07 7.5, step 2) |
| `RJ-328` | `check-findings` | 6 | `romeu doctor`, `julieta lock --check`, `julieta pin check`, `julieta spec validate` (every finding in the files it was given), `julieta memory check`; the details list each finding by its slug |
| `RJ-329` | `sbx-global-secret` | 2 | `doctor` and preflight step 5 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)); the hint names the per-sandbox secret and the global one to remove |
| `RJ-330` | `sbx-broad-allow` | 2 | `doctor` and preflight step 5; the hint names the rule to remove |
| `RJ-331` | `home-symlink` | 2 | `doctor` and preflight step 5; the hint names the link |
| `RJ-332` | `auto-trust` | 2 | `doctor` and preflight step 5; the hint names the trust entry |
| `RJ-333` | `vscode-trust` | 2 | `doctor` and preflight step 5; the hint names the setting to turn back on or the trusted folder to remove |
| `RJ-334` | `record-unreadable` | 2 | every reader of a versioned format, for a file of a format it knows that does not decode ([03](03-formats.md), opening); the file is never overwritten, the details name it, and the hint is to restore it from a backup, with `romeu salvage --from-host <name>` to save the work first |
| `RJ-335` | `generation-lost` | 1 | `run` step 2, after it preserves a lost generation; the hint is `romeu run <name>`, which creates the new sandbox |
| `RJ-336` | `removal-pending` | 2 | preflight step 6 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)), for any mutating command but `rm`, `recreate` and `retire` that finds a `removing` generation with its sandbox present; `status` and `doctor` report it as `removal-pending` ([01 1.7](01-system-model.md#17-vocabulary)); the hint is `romeu rm <name>`, which finishes the removal |
| `RJ-337` | `format-unsupported` | 2 | every reader of `project.v1` or `host-settings.v1`, for a file whose version is older than the window of the current and the previous version ([03](03-formats.md), opening); the hint is to apply the edit in the release notes of each skipped release |
| `RJ-338` | `fingerprint-unstable` | 5 | `rm`, `recreate` and `retire`, on a generation `removing` with its sandbox present, on the third or a later salvage in a row that a changed fingerprint started, counted from `salvage[].cause` as [03 3.8](03-formats.md#38-host-state-schemas-state-v1) reads it: that salvage runs and records its entry and `fingerprint` with the state `removing`, and the command then exits instead of running `sbx env rm`, so the generation stays `removing`. The details name the first path whose entry changed; the hint names the two recoveries, stop the process that writes it or add an ignored path to `salvage.excludeIgnored` (gate 2), then rerun; says that the first rerun after a recovery may salvage once more and exit 5 again, when its fresh fingerprint still differs from the one written (always so after an edit of `salvage.excludeIgnored`, an input of the fingerprint), and that the rerun after it resumes at `sbx env rm`; and says `--accept-loss` does not cover this error, since it is not a skipped reason ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)). A rerun whose fresh fingerprint equals the one written resumes at `sbx env rm`. This row is the one home of the rule; 01 1.6, 03 3.8, 08 8.5 and I17 cite it |
