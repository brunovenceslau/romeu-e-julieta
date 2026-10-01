# 4. CLI surface

Back to [index](../spec.md). Reader: implementers of `romeu-cli` and
`julieta-core`. Type: reference. The generated `docs/reference/` pages
are derived from the same data this page specifies.

## 4.1 Conventions (both binaries)

| Topic | Rule |
|---|---|
| Parsing | stdlib `flag`, `<binary> <command> [subcommand] [flags] [args]`; `--help` on every level |
| Output | human text on stdout; `--json` on every read command emits one JSON document (schema in `schemas/`); errors on stderr, one line first, details after. All output goes through the CLI writer, which escapes with `termsafe` by default (I29); raw output is an explicit, named exception (only the final `exec` of `romeu run`, see 05 5.4) |
| Exit codes | `0` ok; `1` operation failed; `2` usage or precondition (bad flags, invalid settings/spec, missing or too-old sbx/git, missing secret binding, incompatible julieta, unknown sandbox, illegal state transition); `3` approval required (gate 1 or 2); `4` drift detected; `5` refused to protect data (incomplete salvage, unrecovered or unpushed work) |
| Error ids | every error has a stable id `RJ-<exit><nn>` (for example `RJ-301 approval-required`), a slug, a message and a fix hint, defined in one data table in `internal/cli` together with the exit codes; the text output prints `error RJ-301 approval-required: <message>` then `fix: <hint>`; `--json` errors carry `{"id", "slug", "exit", "message", "fix"}`; tests assert ids, never message text; `docs/reference/errors.md` and `exit-codes.md` are generated from the table |
| Idempotency | every mutating command converges; a second run with no input change performs no writes except the documented state timestamp `lastRun` (meta-test in 10) |
| Locking | mutating romeu commands take `$XDG_STATE_HOME/romeu/romeu.lock` (flock, non-blocking; `RJ-101 locked`, "another romeu is running"), then run promotion recovery (01 1.6) |
| Subprocesses | romeu: only `sbx` and `git` by absolute path from host settings, with a scrubbed env (`PATH=/usr/bin:/bin`, `HOME`, `LANG=C`, `TERM` for interactive calls, and `SSH_AUTH_SOCK=<signing.agentSocket>` for sbx calls of a project using `git-ssh-sign`). julieta: `git`, `mise`, `herdr` |
| Version | `romeu version` / `julieta version` print version, commit, build date, Go version, protocol; `--json` adds the binary sha256, `GOARCH`, and (julieta) whether its own directory is writable |

## 4.2 romeu (host)

Every command marked **P** runs the preflight of
[01 1.5](01-system-model.md#15-preflight-on-every-sbx-path) before invoking
sbx.

| Command | Purpose | Reads | Writes | Runs | Exit codes |
|---|---|---|---|---|---|
| `romeu init --config-url <url> [--config-project <name>] [--root <abs>] [--config-ref <ref>]` | create host settings; clone the config repo into `<root>/<cfg>-env/<dir>` | nothing prior | host settings (refuses to overwrite), one clone | host: git | 0, 1, 2 |
| `romeu sync [<name>...\|--all] [--from origin\|sandbox/<branch>\|<sha>] [--overwrite-drift]` | render specs at a named commit into candidates; run gate 2; promote; apply live changes | host settings, config repo objects, repo objects, registries (descriptors not cached), state | clones (missing only), `refs/romeu/origin/*`, candidates, live derived files on promotion, memory dirs and ledger dirs (create only), ledger entries and views (ingest), workspace files, project records, descriptor cache, live egress | host: git, sbx (P for `--from sandbox/...` and live changes) | 0, 1, 2, 3, 4 |
| `romeu approve <name>` / `romeu approve --toolchain` | gate 2 for the awaiting candidate / gate 1; TTY required | project record, candidate, live `sbx version` | project record, promotion / `toolchain.json` | host | 0, 1, 2 (no TTY or nothing awaiting), 4 (candidate changed) |
| `romeu run <name> [--no-attach] [--timings [--json]]` **P** | ensure sandbox, egress, julieta, setup; attach to the run layout | everything above | project record (generation, lastRun, egressApplied), salvage refs when preserving a lost generation, ledger entries and the view (ingest) | host: sbx, git; sandbox: julieta | 0-5 |
| `romeu adopt <name>` | record an existing sandbox named `<name>` as an open generation; TTY required | `sbx ls --json`, project record | project record | host: sbx | 0, 1, 2 (no TTY, open generation exists, workspace path differs, no such sandbox) |
| `romeu stop <name>` **P** | `julieta snapshot --all`, then `sbx stop` | project record | memory (via julieta) | host: sbx; sandbox: julieta | 0, 1, 2, 3, 4 |
| `romeu salvage <name> [--from-host] [--include-transcripts]` **P** | capture everything sandbox-only; verify and import on host | project record, memory dir | memory (via julieta), `refs/romeu/salvage/...`, project record, ledger entries and the view (ingest) | host: sbx, git; sandbox: julieta | 0-5 |
| `romeu rm <name> [--accept-loss]` **P** | salvage, then remove the sandbox; tree kept; warns when the generation has no `final` handoff | project record, memory handoff dir | as salvage; sbx removal; egress rules removed; generation closed | host: sbx, git | 0-5 |
| `romeu recreate <name> [--accept-loss]` **P** | rm then run | as rm + run | as rm + run | as rm + run | 0-5 |
| `romeu retire <name> [--force]` **P** | rm if needed; report unpushed/salvage-only work; move `<name>-env/` to `.attic/`; drop from workspace files | spec (project must be absent unless `--force`), clones | `.attic/`, workspace files, state attic | host: git, sbx | 0-5 (5: TTY confirm declined, or non-TTY without `--force`) |
| `romeu status [<name>] [--json]` | per project: sandbox state, spec vs synced commit, drift, interrupted promotion, awaiting candidate, toolchain acknowledgement state, recreate needed, generations and their states, salvage records, unimported snapshots, orphaned projects and repo dirs; for a running sandbox also julieta's warnings and recorded hook failures (via `julieta status --json`) | read only | nothing | host: sbx, git (read); sandbox: julieta (read) | 0, 1 (never 3) |
| `romeu doctor [--json]` | host health and security checks (below) | host settings, git config, sbx settings and policies, `$HOME` symlinks, tool configs, the signing socket, the ledger and each spool and view | nothing | host | 0 (all pass), 1 (any fail) |
| `romeu pull [-C <dir>] [--branch <b>]` **P** | fetch the sandbox's current work for the repo containing `<dir>` and update its review checkout | project record, remote URL, `julieta status --json` | `refs/sandboxes/<name>/*` (primary), `refs/romeu/snapshots/<name>/*` (secondary), `review/<dir>/` | host: git, sbx; sandbox: julieta | 0-4 |
| `romeu handoff <name> [--repo <dir>] [--json]` | print the latest narrative handoff and the newest facts; read through the handoff reader (I24) | memory dir via `os.Root` | nothing | host | 0, 1 |
| `romeu ledger ingest [<name>...\|--all]` | ingest the spools of the named projects into the runtime ledger and derive their views ([13 13.4](13-runtime-ledger.md#134-ingest)); the same ingest runs inside `sync`, `run` and `salvage` | spools, the ledger | ledger entries (create only), views | host | 0, 1 (`ledger-incomplete`), 2 |
| `romeu ledger query [--project <name>] [--type <t>] [--tool <key>] [--ref <id>] [--rejected] [--since <time>] [--until <time>] [--json]` | print ledger entries, each field of each project (the full view, host only), ordered by `ingested`, project, key; `--since` and `--until` compare `ingested`; `--json` prints one object per line | the ledger | nothing | host | 0, 1, 2 |
| `romeu version` | | | | | 0 |

Removed projects: `sync` never deletes, so there is no `--prune` flag;
a project absent from the spec but present in the tree is reported as
`orphaned` by `sync` and `status`, and `romeu retire` is the explicit
verb. The same applies to a repo removed from a spec (`orphaned repo
dir`, see J13).

### How `romeu sync` renders, gates and promotes

1. Take the lock (runs promotion recovery). Load and validate host
   settings (exit 2). Gate 1 check when a live change will need sbx.
2. Hardened fetch of the config repo's origin into
   `refs/romeu/origin/<cfg-dir>/*`; with `--from sandbox/<b>`: preflight,
   start the config project's sandbox if stopped (`sbx env exec <dir> --
   true`), confirm it is running and resolve the live daemon URL, hardened
   fetch into `refs/sandboxes/<cfg>/*` (Q17). `--from <sha>` accepts only
   a commit reachable from `refs/romeu/origin/*` or `refs/sandboxes/*`
   (exit 2 otherwise).
3. `git fsck --strict` on the spec commit; read `projects/*.yaml` via
   `git cat-file`; strict decode and validate all specs (cross-project
   rules: unique names, unique URLs); exit 2 listing every error.
4. For each target project: create `<name>-env/`, `memory/<dir>/`
   (create only, with the fixed subdirectories) and `ledger/spool/`
   and `ledger/view/` (create only); hardened clone of
   missing repos; hardened fetch of each repo's origin into
   `refs/romeu/origin/<dir>/*`; resolve `egressCommit` from
   `refs/romeu/origin/<dir>/<ref>`.
5. Refuse (exit 2) if any memory dir violates the layout allowlist (I24).
   Then ingest each target project's spool
   ([13 13.4](13-runtime-ledger.md#134-ingest)); a failed ingest is
   reported and does not stop `sync`.
6. Read workload and kit descriptors: the cache in host state first;
   `internal/oci` by digest only on a cache miss; local kits from the
   config commit's tree via `git fsck --strict` and the validated walk
   (I27). Derive egress ([07](07-mise-egress.md)). Delete any candidate
   dir the project record does not name. Render the candidate
   (`sbxenv.yaml`, `render.json`, `kits/`) under `.romeu/candidates/`;
   an awaiting candidate is superseded.
7. Drift check of live files against the current `render.json`
   (exit 4 naming the file, unless `--overwrite-drift`).
8. Gate 2 and the candidate state machine
   ([01 1.6](01-system-model.md#candidate)); promotion commit on
   approval or when unchanged.
9. If the sandbox is running: reconcile egress live; if the `secrets`
   field is tagged `apply:"live"` (A10, Q18), `sbx secret set <svc>
   --sandbox <name> --command <rendered command>` (never a value); report
   "recreate needed" when the recreate digest differs from the running
   generation.
10. Report orphaned projects and repo dirs; remove nothing.

### How `romeu run` reaches the run layout

1. Preflight (01 1.5). Then ingest the project's spool (13 13.4); a
   failed ingest is reported and `run` continues.
2. `sbx ls --json`:
   - a sandbox named `<name>` that romeu has no open generation for:
     `RJ-203 unknown-sandbox`, fix hint "check its workspace, then
     `romeu adopt <name>`";
   - present but its workspace path differs from
     `$ROMEU_ROOT/<name>-env/<primary>`: exit 2;
   - absent with an open generation: preserve it (import
     `refs/sandboxes/<name>/*` and every `snapshot/` bundle into
     `refs/romeu/salvage/<name>/<gen>/<new salvage id>/`, record
     `result: lost`), move it to `closed-lost`, continue.
3. `sbx env run -d --clone <dir>` (never `--auto-approve`; sbx's own
   prompt appears in this terminal when needed). On create, record a new
   generation (01 1.6).
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
7. If the recreate digest differs: warn "stale: <fields> changed ->
   romeu recreate <name>".
8. Unless `--no-attach`: `exec` into
   `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- <abs julieta path> layout up`.

### How `romeu pull` updates a review checkout

Resolve the project and repo from `<dir>` (must be inside
`$ROMEU_ROOT/<name>-env/`); preflight; `julieta snapshot --all` and
`julieta status --json` via exec; primary: confirm running, re-read the
daemon URL, hardened fetch into `refs/sandboxes/<name>/*`, then compare
fetched heads with `julieta status` (mismatch: exit 1, nothing updated);
secondary: verify and unbundle `snapshot/heads.bundle` into
`refs/romeu/snapshots/<name>/<dir>/*`; update `review/<dir>/` (a
standalone repo with the host clone as alternate and the hardening keys
in its local config) to the sandbox's current branch head, detached.

### Checks run by `romeu doctor`

Checks marked **pre** also run in the preflight (01 1.5, step 5); one
implementation serves both.

| Check | Result |
|---|---|
| sbx version below the floor (v0.46.0) or not at the configured absolute path | fail |
| git version below the floor ([05 5.1](05-security.md#51-hardened-git-internalgitsafe)) | fail |
| toolchain acknowledgement missing or stale | fail |
| host global git config: `core.hooksPath`, `core.fsmonitor`, `filter.*` (including a global git-lfs filter), `include.*`, `includeIf.*`, `fetch.prune=true`, `diff.external`, `core.pager`, `alias.*`, `url.*.insteadOf` | warn, with the review-checkout risk |
| sbx: global (not per-sandbox) secrets for a service any project uses | fail (I22) **pre** |
| sbx: a global allow rule of `**` or other broad wildcard, or a permissive default network policy | fail (I22) **pre** |
| sbx: `env.rememberHostCommands` not true | warn |
| sbx: `kit.allowedSources` does not admit the workload registries | fail |
| root inside a git repo, equal to `$HOME`, not absolute, not owned by the user, or containing host settings/state | fail |
| a symlink under `$HOME` (depth 1) or `$HOME/.config` (depth 2) resolving into `$ROMEU_ROOT` | fail (I23) **pre** |
| mise `trusted_config_paths` or a direnv allow list covering `$ROMEU_ROOT` | fail (I23) **pre** |
| VS Code: `security.workspace.trust.enabled=false`, or `$ROMEU_ROOT` or a parent trusted | fail (I14, I23) **pre** |
| host settings not 0600, state dir not 0700 | fail |
| secret `argv[0]` missing or not absolute; entries for unknown projects | fail / warn |
| signing: a project uses `git-ssh-sign` and `signing.agentSocket` is unset, unreachable, or holds a number of keys other than one, or its one key differs from the kit's `signingKey` arg | fail **pre** |
| tree: live derived files drifted; interrupted promotion; memory dir violating the layout allowlist | fail |
| ledger: directory not 0700 or an entry not 0600; an ingested entry whose event region does not hash to the key in its name, or an entry that does not parse (`ledger-entry-mismatch`); a project's view that differs from the one derived again | fail |
| ledger: a spool root missing, a symlink or not a directory | fail |
| ledger: spool files ingest skips, as counts per reason with the first paths (13 13.3); a leftover temporary name; the ledger's size past the warning size (13 13.8) | warn |

## 4.3 julieta (sandbox)

| Command | Purpose | Reads | Writes | Exit codes |
|---|---|---|---|---|
| `julieta setup [--json]` | idempotent: point `$HOME/.local/bin/julieta` at the running binary; install the hook dispatcher (global `core.hooksPath`); clone missing secondary repos from origin; fetch origin in every repo and fast-forward the local default branch when it is checked out, clean and fast-forwardable (otherwise report); `install`; `memory check`; agent memory dir check; the ledger view must not be writable (`ledger-view-writable`, exit 2); drain the spool (13 13.3) | manifest, the ledger view | `$HOME/.local/bin/julieta`, `$HOME/src/...`, repos (ff only), git global config (sandbox), dispatcher dir, mise data, julieta state, the spool (drain) | 0, 1, 2 |
| `julieta install [--repo <dir>] [--force]` | `mise install` (locked) in every repo; skip when the `lock-set` digest equals the last success | manifest, locks | mise data, julieta state | 0, 1 (names the tool and the blocked host) |
| `julieta lock [--repo <dir>] [--check]` | `mise lock --platform linux-x64,linux-arm64,macos-x64,macos-arm64`; `--check` verifies freshness without writing (every tool in `mise.toml` has a lock entry and every lockable entry covers the 4 platforms) | `mise.toml`, `mise.lock` | `mise.lock` (not with `--check`) | 0, 1 |
| `julieta hooks run <hook> [args]` | dispatcher: julieta's own action (`pre-commit`: `lock --check` when `mise.toml` or `mise.lock` is staged; `post-commit`, `post-rewrite`, `post-merge`: `snapshot`), then the repo's tracked `.githooks/<hook>` if executable, then `$GIT_DIR/hooks/<hook>` if executable; the hook's arguments go to each, and its stdin to the first of the two that exists; the first non-zero status stops; a failure is recorded in julieta state and emitted as a `hook-failed` event (13 13.2) | repo | as the hooks do; julieta state; the spool | hook's status |
| `julieta memory add\|list\|show\|edit\|rm\|search [--repo <dir>] [--json]` | memory entries of the repo containing cwd (or `--repo`); `edit --status done` closes an entry; julieta stamps id and timestamps | memory dir | memory dir | 0, 1, 2 |
| `julieta memory import --format jsonl <file>` | import entries, idempotent by content digest; report `read`, `imported`, `skipped-duplicate`, `invalid` | file | memory dir | 0, 1 (`invalid > 0`), 2 |
| `julieta memory verify --against <file>` | set equality between the JSONL file's content digests and the store's, then equality of `status` for every matched entry | file, memory | nothing | 0, 1 |
| `julieta memory check` | every manifest memory dir is mounted, writable, and satisfies the layout allowlist | manifest | nothing | 0, 1 |
| `julieta handoff write [--final\|--facts]` | read the narrative on stdin (not for `--facts`), validate headings, stamp facts, write `handoff/<ULID>-<kind>.md` | stdin, repos | memory handoff dir | 0, 1, 2 |
| `julieta handoff show [--hook] [--repo <dir>]` | print the latest narrative handoff, the facts that changed since, and open memory entries; `--hook` (SessionStart) also prints `lesson` entries and julieta's warnings (08 8.2) | memory, repos, julieta state | nothing | 0, 1 |
| `julieta handoff list [--json]` | list handoffs | memory | nothing | 0 |
| `julieta event add [--tool <key>] [--ref <id>] [--path <path>]` | write a `note` event to the spool; each value is validated as 13 13.2 says, and at least one is given; no flag takes free text | manifest | the spool | 0, 1, 2 (a rejection reason of 4.4) |
| `julieta event list [--others] [--json]` | print this project's entries from the ledger view, rejected ones included; `--others` prints the other projects' entries, with the fields the view policy allows | the ledger view | nothing | 0, 1 |
| `julieta snapshot [--repo <dir>\|--all]` | bundle unpushed work into `memory/<dir>/snapshot/` (no debounce) | repos, manifest | memory | 0, 1 |
| `julieta salvage [--stop-agents] [--include-transcripts] [--json]` | full salvage for the `salvageRun` id in the manifest | manifest, repos, agent dirs | memory dirs, `salvage/*` branches (create only) | 0, 1, 5 |
| `julieta layout up [--dry-run [--json]]` | start the herdr server if absent, apply missing tabs, report "layout stale" when the live layout's `run` digest differs from `runDigest`, attach; `--dry-run` prints the `layout.apply` requests without applying (golden tests) | manifest | herdr state (not with `--dry-run`) | 0, 1, 2 |
| `julieta pane run <id>` | internal: the herdr leaf argv; runs the pane's steps | manifest | - | last step's status |
| `julieta pin workload [--tag <t>] <spec-file>` | resolve the workload tag to a digest, verify the manifest list covers linux/amd64 and linux/arm64, rewrite the spec field | spec, registry | spec | 0, 1, 2 |
| `julieta pin check [--workflows] <path>...` | report pins that are not digests or are behind: spec workloads and, with `--workflows`, the julieta release pinned in the config repo's CI workflows | files, registry, GitHub releases | nothing | 0, 1 |
| `julieta spec validate [--catalog] <file>...` | strict decode + schema + name/path rules, per file and across files; `--catalog` also fetches each repo's `mise.lock` at its `ref` (hardened, shallow) and reports every `backend:tool` and lock host missing from the embedded catalog | files; origins with `--catalog` | nothing | 0, 2 |
| `julieta status [--json]` | per repo: branch, HEAD, all local branch heads, ahead/behind, dirty, stashes, worktrees, last snapshot, snapshots disabled (repo-local `core.hooksPath`), tools installed; agent memory dir state; warnings; recorded hook failures | manifest, repos, julieta state | nothing | 0 |
| `julieta version [--json]` | | | | 0 |

julieta refuses (exit 2) any command that needs the manifest when none
is cached and `JULIETA_MANIFEST` is unset, except `spec validate`,
`version`, `lock`, and `memory` with an explicit `--memory-dir`.

Environment julieta sets for mise calls: `MISE_LOCKED=1`,
`MISE_DISABLE_UPDATE_WARNING=1`, `MISE_USE_VERSIONS_HOST_TRACK=false`,
`MISE_AUTO_INSTALL=false`, `MISE_YES=1`, and
`MISE_TRUSTED_CONFIG_PATHS` set to the manifest's repo paths (nothing
else is trusted).

A julieta command that exits non-zero also writes a `command-failed`
event to the spool when the manifest names one; which commands do not,
and why emitting never changes a command's exit status, is in
[13 13.2](13-runtime-ledger.md#132-events).

## 4.4 Error ids of the runtime ledger

The ledger has no message catalog of its own. Its errors are rows of
the one table of 4.1, named here by slug; the table assigns the
numbers.

| Slug | Exit | Raised by |
|---|---|---|
| `event-syntax`, `event-version`, `event-keys`, `event-id`, `event-domain`, `secret-pattern` | 2 | `julieta event add`, for its own input. romeu's ingest records the same id as the reason of a rejected entry and does not exit on it ([13 13.4](13-runtime-ledger.md#134-ingest)) |
| `ledger-view-writable` | 2 | `julieta setup` |
| `ledger-incomplete` | 1 | `romeu ledger ingest`. Inside `sync`, `run` and `salvage` it is reported and the exit status does not change |
| `ledger-entry-mismatch` | 1 | ingest, when an entry's name exists with other content; `romeu doctor` |
