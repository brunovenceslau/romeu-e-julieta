# 4. CLI surface

Back to [index](../spec.md). Reader: implementers of `romeu-cli` and
`julieta-core`. Type: reference. The generated `docs/reference/` pages
are derived from the same data this page specifies.

## 4.1 Conventions (both binaries)

| Topic | Rule |
|---|---|
| Parsing | `REQ-0267` `github.com/spf13/cobra` with its flag package `pflag` ([ADR 0015, adopt spf13/cobra for the command line of romeu and julieta](../adr/0015-adopt-spf13-cobra-for-the-command-line-of-romeu-and-julieta.md)): `<binary> <command> [subcommand] [flags] [args]`, with GNU-style flags (`--name`, `--name=value`, `--name value`, a one-letter short form only where a row names one, flags and arguments in any order, `--` ends the flags); `--help` and `-h` on every level, and `help <command>`, except `julieta hooks run`, which passes its arguments on to git's hook unparsed and validates its hook name (03 3.1) before it builds an argv. Every command declares the arguments it takes, and a command with subcommands takes none. Every error of the parser (an unknown command, flag or subcommand, an argument count the command refuses) is the error `usage` (`RJ-349`) |
| Output | `REQ-0268` human text on stdout; `--json` on every command a program calls, named in its row, emits one JSON document, or one object per line where the row says so, with a schema in `schemas/` named after the command (`<binary>-<command>.v1`, [02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)); errors on stderr, one line first, details after. `REQ-0269` All output goes through the CLI writer, which escapes with `termsafe` by default (I29); `REQ-0270` raw output is an explicit, named exception: the interactive calls, which are the final `exec` of `romeu run` and `sbx env run -d [--clone] <dir>` (a create or a start), whose stdin, stdout and stderr are the terminal's because sbx prompts there (01 1.4; 05 5.4), and the shell completion script and the output of the hidden `__complete` commands (4.2, 4.3), which need real tabs and line feeds and hold only the binary's static text and names that pass the validators of [03 3.1](03-formats.md#31-name-and-path-rules-shared); the stderr text of those commands goes through the CLI writer, and they write no debug file (`BASH_COMP_DEBUG_FILE` is unset before cobra runs); every other subprocess has its stdin closed and its output captured, and the first 4 KiB of its stderr enter the error details (Error ids). `REQ-0271` Every document romeu decodes from a subprocess in a sandbox or from a mount is decoded strictly with a size cap (an unknown member is skipped, as 03 (opening) says of versions), and no string from it becomes a path, a ref or an argv element without the validators of [03 3.1](03-formats.md#31-name-and-path-rules-shared) |
| Completion | `REQ-1245` A completion function of either binary reads records only: it starts no subprocess, opens no network connection, takes no lock, writes nothing, returns at most 1000 names, each passing the validators of [03 3.1](03-formats.md#31-name-and-path-rules-shared), and stops after one second. A completion description is the binary's static text. The hidden `__complete` commands take no `--json` and do nothing a typed command cannot (ADR 0015) |
| Terminal | `REQ-0272` a command is on a TTY when its stdin is a terminal (`isatty`); `REQ-1274` gate 1 (`romeu approve --toolchain`) needs a terminal on stdin and on stderr, since its text is written to stderr, so the operator always sees what is approved, and it takes the no-TTY path when either is not one. Prompts, the whole gate 2 diff with the summary that 01 1.4 repeats above the prompt, and the confirmations of `rm` and `retire` are written to stderr, so a piped stdout keeps them visible; a command run with `--json` never prompts and takes the no-TTY path. The `/dev/ptmx` tests of 10 10.1 drive stdin and read stderr |
| Exit codes | `REQ-0273` `0` ok; `1` operation failed; `2` usage or precondition (bad flags, invalid settings/spec, missing or too-old sbx/git, missing secret binding, incompatible julieta, unknown sandbox, illegal state transition); `3` approval required (gate 1 or 2); `4` drift detected, or an interrupted promotion abandoned; `5` refused to protect data (incomplete salvage, a fresh fingerprint ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)) that differs from the newest one written with `removing`, unrecovered or unpushed work, a stopped sandbox that needs a recreate); `6` the check ran and found what it checks for (`romeu doctor`, `julieta lock --check`, `julieta pin check`, `julieta spec validate`, `julieta memory check`), so `1` keeps the one meaning "the operation failed"; `REQ-0274` `130` and `143`: the command was ended by SIGINT or SIGTERM, 128 plus the signal number, for both binaries ([Signals](#signals) below) |
| Error ids | `REQ-0275` every error has a stable id `RJ-<nnn>`, a slug, an exit code, a message, a fix hint and a "recovered in" cell, declared in the `errors.go` of the package that returns it; one test collects the union, which is the error table of this specification (4.4); the rules are [Error ids](#error-ids) below |
| Warnings | `REQ-0276` a warning is printed, escaped, and changes no exit code. Until T178 merges, this list lacks `repo-setup-failed` (REQ-1307): the list follows the code, which `TestWarningSlugs` holds it to. Each is a row of the same table, by slug and with no id. Each check of `romeu doctor` (4.2) is a row by its slug too: a check that stops a command, as each **pre** check does in the preflight, is an error row with an id (4.4), and a check that only warns stays id-less. The warnings are the id-less rows of this list, and this list is their one home; 04 4.2, 04 4.3 and 08 8.2 cite it: `sbx-untested`, `spec-moved`, `recreate-needed` (as a warning, `run` step 3), `no-tombstone`, `lock-unknown-tool`, `julieta-pin-mismatch`, `snapshots-disabled`, `skills-stale`, `layout-stale`, `agent-hooks-missing`, `version-only-tool`, `host-git-config`, `sbx-remember-commands`, `root-indexed`, `secret-unknown-project`, `record-size`, `lock-stale` (a `mise.lock` behind its `mise.toml`), `agent-memory-writable` (the agent's own memory dir writable or not empty), `agent-path-missing` (a required agent-profile path absent, 08 8.5 step 5), `fetch-failed` (a `setup` fetch that failed, naming the repo and the git error, 04 4.3), `memory-derivable` (an `add` whose body repeats a line of the repo's instruction files, 04 4.3), `memory-open-count` and `memory-stale` (the open entries of a project above the count of 08 8.6, and an open entry not updated for the days of 08 8.6; both printed by `memory index --hook`, 08 8.2), `handoff-long` (a `handoff` body over the narrative bound of 08 8.6, so the index shows it cut), `stamps-over-cap` (08 8.1), `handoff-legacy` (a `handoff/` directory of before 03 3.10 that `setup` has not migrated yet, or a file it could not migrate, 08 8.1), `shared-memory-missing` (a manifest without `sharedMemory`, 03 3.9), `memory-push-failed`, `memory-diverged`, `memory-unsigned` and `memory-local` (the durable copy of memory, 08 8.1). `recreate-needed` names one condition, the recreate digest differing from the generation's: a warning on a running sandbox and the error `RJ-305` on a stopped one, so the two rows share the slug |
| Idempotency | `REQ-0277` a mutating command converges: a second run with no input change leaves every file's content and mode and every ref as they were, except the documented state timestamp `lastRunAt`. `REQ-0278` The appending commands add one thing per run by design, and this is the list: `romeu salvage`, and `rm`, `recreate` and `retire` through it (a salvage id with its record and refs); `REQ-0279` `romeu stop` on a running sandbox, `romeu pull` and `julieta snapshot` (a rewritten `snapshot/heads.json`, whose `createdAt` moves); `julieta memory add` (an entry, unless an open entry with the same `memory-content` digest (03 3.12) exists: then it prints that entry's id and writes nothing); `julieta stamp` (a stamp file, 03 3.10). `REQ-0280` Each appending julieta command prints what it wrote, its id and path, in text and with `--json`, so a caller that runs the command again after a timeout can compare what the rerun prints with what `list` shows and tell whether the first write landed (`memory add` prints the existing entry's id and writes nothing when the content is the same). `REQ-0281` This list, the **P** marker of 4.2 and the lock of the next row are fields of each command definition, with no default, so a definition without them fails generation ([12 12.3](12-engineering.md#123-generators)); the meta-test of 10 10.1 reads the same fields. `REQ-0282` This list and the **P** marker of 4.2 are design input: the generated `docs/reference/<binary>/<command path>.md` pages replace them once the command definitions land, the rule [03](03-formats.md) (opening) states for its tables |
| Locking | `REQ-0283` mutating romeu commands take `$XDG_STATE_HOME/romeu/romeu.lock` (flock, non-blocking; the error `locked` (`RJ-101`), "another romeu is running"), then run promotion recovery (01 1.6). `REQ-0284` The commands that take it are the ones whose Writes cell in 4.2 is not `nothing`: `init`, `approve`, `sync`, `run`, `adopt`, `stop`, `salvage`, `rm`, `recreate`, `retire` and `pull`; `status`, `doctor`, `memory`, `version`, `completion` and `help` run without it and read records as the atomic write rule of 03 3.8 leaves them. This list is design input like the appending list: the lock field of each command definition replaces it. The lock is held for the whole command, sbx calls included, and released before the final `exec` of `run`, so an attached `run` blocks no other command. One lock per machine is a chosen cost: commands on different projects wait for each other, a create that builds kits included; a lock per project is a row of [Deferred decisions](../spec.md#in-the-product) |
| Subprocesses | every subprocess of either binary starts through `internal/procgroup`, in a process group of its own, with a scrubbed environment and a timeout of the class the Starting values table gives; the programs each binary runs and the rules are the list [Subprocesses](#subprocesses) below |
| Version | `REQ-0285` `romeu version` / `julieta version` print version, commit, build date, Go version, protocol; `--json` adds the binary sha256, `GOARCH`, and (julieta) whether its own directory is writable. `REQ-0286` `romeu version --json` also carries a `versions` object: sbx, git, macOS and, per project, the workload digest and the kit pins of its live render, so a report can paste one document |

### Error ids

1. `REQ-0287` The "recovered in" cell holds the `docs/guide/` anchor of the
   error's recovery or `none: <reason>` ([12 12.3](12-engineering.md#123-generators), 12 12.8);
   a test fails an id without one, and `docs/reference/errors.md` links it.
2. `REQ-0288` This specification names an error by both, as "the error
   `format-newer` (`RJ-311`)": the id is the stable key, the slug its
   readable name.
3. `REQ-0289` The number is one sequence and carries no meaning: the exit code is
   a field of the row, so an error moved to another exit keeps its id.
   An id is never reused or renumbered; a retired id stays in the table,
   marked retired, with its last text. The ids written before this rule
   keep their numbers; a new id is allocated by
   `go run ./tools/new error <package> <slug>`, which takes the next
   number after the highest, and a pull request that adds one is rebased
   on the current base, with its checks run again, before the merge
   (12 12.3).
4. `REQ-0290` The text output prints `error RJ-301 project-approval-required:
   <message>` then `fix: <hint>`; `--json` errors carry
   `{"id", "slug", "exit", "message", "fix", "details"}`, where `details`
   is the object the text form prints after the first line (the escaped
   argv, the status, the stderr head, the step, the versions; item 7),
   and not the "recovered in" cell, which a reader reaches by the id in `docs/reference/errors.md`;
   tests assert ids, never message text; `docs/reference/errors.md`,
   with each id's recovery link, and `exit-codes.md` are generated from
   the table.
5. `REQ-0291` A fix hint begins with a command line the user can run, its
   placeholders filled from context, or with `decide:` and the choice,
   except the one hint that a row of 4.4 quotes as one fixed text, the
   hint of `RJ-338`, which a test holds byte for byte and which item 6
   and item 7 skip; `REQ-0292` a hint that names `--overwrite-drift`,
   `--accept-loss` or `--force` says what that flag discards.
6. `REQ-0293` A test over the generated command definitions parses every command
   line in every other fix hint and in a `--help` example into its
   command, subcommand and flags, and fails on one the definitions do
   not have, as 12 12.3 does for a `SKILL.md`.
7. `REQ-0294` The details of an error a failed subprocess raised carry its escaped
   argv, its exit status, the first 4 KiB of its stderr, with the home
   path, user name, host name and root path replaced as the recorder of
   [10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract) does,
   the numbered step of 4.2 it ran in, and the romeu, julieta and sbx
   versions.
8. The argv, the status, the step and the versions hold no secret
   value, since romeu never handles one (I25), so they can be pasted
   into an issue. A token in a remote URL or an sbx error body is not
   removed, so the reader looks the stderr over before pasting it.
9. `REQ-0295` A subsystem's reasons are rows of the same table, each row saying
   who raises it. The ids this specification names are listed in 4.4.

### Subprocesses

1. `REQ-0296` romeu runs `sbx` and `git` by absolute path from host settings, with
   a scrubbed environment (`PATH=/usr/bin:/bin`, `HOME`, `LANG=C`,
   `TERM` for the interactive calls named under Output, and
   `SSH_AUTH_SOCK=<signing.agentSocket>` for the sbx calls of a project
   using `git-ssh-sign`), and, in `doctor` only, each secret argv of
   host settings, run once with exactly `HOME` and `PATH=/usr/bin:/bin`
   (03 3.4). The admitted-call-site test of I16 (05 5.2) holds
   `os/exec` to `internal/procgroup` and its runner to `sbxdrv`,
   `gitsafe` and that one row of `doctor`, so romeu never starts `gh`
   or any other program.
2. `REQ-0297` julieta runs `git`, `mise`, `herdr`, the hook stages of `hooks run`
   (the repo's tracked `.githooks/<hook>` and `$GIT_DIR/hooks/<hook>`,
   4.3), the pane steps of `pane run` through `mise exec` (03 3.2) and a
   repo's `setup` task under `julieta setup` (4.3); the last three run
   repository content on purpose, inside the sandbox only (I21).
3. `sbx version`, `sbx ls --json` and the settings and policy listings
   are reads; a mutating sbx call changes a sandbox, a policy or a
   secret.
4. `REQ-0298` Every subprocess of either binary starts through one package,
   `internal/procgroup`, in a process group of its own; a timeout, a
   cancellation and the signal rule below kill the whole group, and
   the wait after the kill is bounded: the output pipes drain until no
   output has moved for 2 s, so what a command wrote before it ended
   is read whole however late the reader is, and a holder of a pipe
   that left the group ends the drain after 2 s of silence. The drain
   lasts at most 30 s after the command ends, and less when the
   command's timeout comes first; a drain those end is an error of the
   command, never a success, since its output may be cut. `REQ-0299` The same package starts every subprocess of `tools/ci`
   and `tools/mutants`, so the group rule, the timeout and the
   scrubbed environment have one implementation, and a call-site scan
   fails an `os/exec` call outside it: a test in `internal/procgroup`
   until it moves beside the I16 scan (05 5.2).
5. `REQ-0300` Every subprocess has a timeout of the class the Starting values
   table below gives, past which it fails with `subprocess-timeout`,
   except the salvage exec, which moves up to the salvage cap over the
   mount and stops only on an interrupt (an interrupted salvage leaves
   its dir without a `manifest.json`, which verification ignores and
   keeps, 08 8.5), the hook stages and pane steps of julieta, which
   are the user's own programs, and the `go` build check and test runs
   of `tools/mutants`, which run with no timeout (the last row of the
   table below). `REQ-0301` The details of `subprocess-timeout`
   name the class and its limit, and its fix hint says the sandbox or
   the daemon may still be busy and names `sbx ls --json`.

### Starting values

Every subprocess has a timeout and fails with `subprocess-timeout`
(Subprocesses, item 5). `REQ-1220` These are starting values; the first B5
measurement confirms or resets them, and a change is a pull request
that edits this table alone. `REQ-0302` The table is one Go table in
`internal/procgroup`, keyed by class, with a seam that only tests set,
and each caller names its class.

| Class | Timeout |
|---|---|
| an sbx read (`sbx version`, `sbx ls --json`, `sbx env plan`, `sbx policy ls`, the secret and settings listings) | 30 s |
| a mutating policy or secret call (`sbx policy allow`, `rm` and `check`, `sbx secret set`, `sbx settings set`), `sbx stop`, `sbx env rm` | 2 min |
| `sbx env run -d <dir>` (a start) | 5 min |
| `sbx env run -d --clone <dir>` (a create: image pulls and kit builds) | 30 min |
| `sbx env exec` of `julieta setup` (mise downloads inside it) | 30 min |
| every other `sbx env exec` of julieta (`version`, `status`, `snapshot`), the salvage exec excepted | 5 min |
| `git clone` and `git fetch` on the host | 15 min |
| each secret argv and the credential helper | 30 s |
| julieta's own `git` | 5 min |
| `mise install` (also the one in the setup of `tools/ci all`), `mise lock`, and a repo's `mise run setup` under `julieta setup`; `go env` in `tools/mutants`, whose mise shim may install go on a cold cache | 30 min |
| `herdr` | 30 s |
| a command step of `tools/ci fast` or `tools/ci all`, the setup steps `mise trust` and `go mod download` included, `mise install` and the race step excepted | 15 min |
| the race step of `tools/ci all` | 40 min |
| a git command of `tools/ci` or `tools/mutants`; the `git pack-objects` and `git unpack-objects` pair of `tools/mutants` start together, and each runs under this limit, with no shared deadline | 2 min |
| a short probe of `tools/ci` or `tools/mutants` (`uname -m`, `sysctl -n sysctl.proc_translated`, `mise which`, `mise --version`, `go list -find`) | 30 s |
| the salvage exec, the hook stages and the pane steps | none (Subprocesses, item 5) |
| the `go` build check and test runs of `tools/mutants` (`go test -exec=true`, the baseline `go test`, each mutant's `go test`) | none: the signal handler kills the group (Signals), and `go test -timeout` bounds each test binary that runs |

### Signals

Both binaries install one handler, from `internal/procgroup`, in
`main`, and nothing else maps a signal:

1. `REQ-0303` The first SIGINT or SIGTERM cancels the command's context: every
   subprocess, which runs in a process group of its own (Subprocesses,
   item 4), is killed with its group, the lock is released, and the
   files already written stay as the recovery rules of 01 1.6 read
   them.
2. `REQ-0304` The exit status is 128 plus the signal number (130, 143). The
   signal wins over a failure the command had already found (1 to 6);
   a command that had finished its work keeps 0.
3. `REQ-0305` On the first SIGINT or SIGTERM, after the cancel of item
   1, the handler kills the group of every subprocess still running
   under the command's context, itself and before it returns, and
   refuses every later start under that context. `REQ-1234` A second
   SIGINT or SIGTERM that lands before the handler steps aside is
   absorbed: it does not end the process, and the status stays the one
   item 2 gives for the first signal. `REQ-1235` The handler then steps
   aside, and a later SIGINT or SIGTERM takes its default action,
   except a SIGINT that was ignored when the process started, which
   stays ignored (Go's `os/signal`). This order closes the window,
   measured on `tools/ci` on 2026-10-12, in which a second signal ended
   the process between the cancel and the group kill and left the
   group running (decided on 2026-10-13). What the handler does not
   cover, and where every group it would kill keeps running: a SIGKILL
   to the process, which no handler sees; a SIGHUP or a SIGQUIT, which
   it does not take: a SIGHUP ends the process by its default action, a
   SIGQUIT by the Go runtime's goroutine dump and exit status 2, while
   the race step of `tools/ci`, for example, runs on with no parent
   left to enforce its 40 minutes; and a panic, a fatal error or an
   `os.Exit` from another goroutine. A grandchild that calls `setsid`
   leaves its group, and the group kill does not reach it. A
   subprocess whose context does not derive from the one the handler
   gives the command is neither refused nor killed by the handler,
   only by its own cancellation or timeout.
4. `REQ-0306` A hook stage or a pane step ended by a signal is reported as
   `exited 128+<signal>` (4.3, `hooks run`); the `exit_code` a ledger
   event would carry stays as [13 13.2](13-runtime-ledger.md) defines
   it.
5. `REQ-0307` `tools/ci` and `tools/mutants` take the same rule from the same
   package.

## 4.2 romeu (host)

`REQ-0308` Every command marked **P** runs the preflight of
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call) before any
mutating sbx call. Every command whose Runs cell names julieta runs the
compatibility check of [06 6.3](06-kits.md#63-julieta-delivery) before
its first julieta call.

| Command | Purpose | Reads | Writes | Runs | Exit codes |
|---|---|---|---|---|---|
| `romeu init --config-url <url> [--config-project <name>] [--root <abs>] [--config-ref <ref>] [--git-credential-helper <name-or-path>]` | `REQ-0309` create host settings; clone the config repo into `<root>/<cfg>-env/<dir>`. It needs an existing config repo whose ref holds `projects/<cfg>.yaml` (J1 step 0), and applies `doctor`'s root check before it writes. `REQ-1275` In v1 `--config-ref` must be the remote's default branch: init asks the remote for its default branch and refuses, with exit 2 and the rule slug `config-ref-not-default` (no error id of its own), a ref that is another branch or a tag, and refuses with the slug `config-no-default-branch` a remote that has no default branch. `REQ-1283` `--git-credential-helper` takes the value `tools.gitCredentialHelper` takes ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1), REQ-1267: a `git-credential-<name>` helper's name or an absolute, clean path with no shell character), at most 1024 bytes; an absolute path must name a regular, executable file that the user or root owns and that neither the group nor others can write, in directories of the same kind up to `/` (a root-owned sticky directory such as `/tmp` is admitted above the parent; the path is checked both as given and with its links resolved), while a helper name is exempt, since git finds it in its exec-path; a value either rule refuses is a usage error, exit 2, naming the rule, before anything is written. Git's `!` shell form is refused, so an operator who uses `gh` passes a script at an absolute path that runs `/opt/homebrew/bin/gh auth git-credential "$@"` (git runs with `PATH=/usr/bin:/bin`, so the script names `gh` by its absolute path). Git may send the helper `erase` when the origin refuses the credential, which makes a storing helper such as `osxkeychain` or `store` delete what it stored. `REQ-1284` init records the value in the settings it writes, and the clone of the config repo, and no other git call of init, uses it as `credential.https://<host>.helper` for the host git connects to, after the empty `credential.helper` of [05 5.1](05-security.md#51-hardened-git-internalgitsafe), so no other host is handed the helper, as every later command does with the recorded helper (REQ-0464); `GIT_TERMINAL_PROMPT=0` stays. `REQ-1285` A clone that git fails is `config-clone-failed` (`RJ-356`), exit 1, with git's output, escaped and cut to its first 4 KiB, in the details, and so is what gitsafe refuses in what the remote sent (its HEAD, a ref, a file of the clone), with the reason in the details; a URL or a helper gitsafe refuses before git runs is a usage error, exit 2. A clone that fails leaves no settings and no clone, and removes the env dir and the last element of the root when this run made them and they are still empty directories; levels above the root that this run made stay | nothing prior | host settings, its `root`, `config` and `tools` keys ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1); refuses to overwrite), one clone | host: git | 0, 1, 2 |
| `romeu sync [--all] [--from origin\|<sha>] [--overwrite-drift] [<name>...]` **P** | `REQ-1221` render specs at a named commit into candidates; run gate 2; promote; apply live changes The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | host settings, config repo objects, repo objects, registries (descriptors not cached), state | clones (missing only), `refs/romeu/origin/*`, candidates, live derived files on promotion, `<name>-env/` and memory dirs (create only), the `.metadata_never_index` marker, workspace files, project records, descriptor cache, live egress | host: git, sbx (preflight step 1 before its first sbx read and steps 2 to 6 before its first mutating call, step 9 below) | 0, 1, 2, 3, 4 |
| `romeu approve --toolchain` | `REQ-0310` gate 1 (01 1.4); TTY required; `REQ-1276` a declined prompt exits 1 and writes nothing; makes no mutating sbx call (it reads `sbx version`) | `toolchain.json`, live `sbx version` | `toolchain.json` | host: sbx (read) | 0, 1 (declined), 2 (no TTY) |
| `romeu run [--no-attach] <name>` **P** | `REQ-0311` ensure sandbox, egress, julieta, setup; attach to the run layout The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | everything above | project record (generation, `lastRunAt`, `egressApplied`), salvage refs and record when preserving a lost generation | host: sbx, git; sandbox: julieta | 0-5 (5: step 3, a stopped sandbox that needs a recreate) |
| `romeu adopt <name>` | `REQ-0312` record an existing sandbox named `<name>` as an open generation (01 1.7); before the prompt it prints, escaped, the mounts, policies, secrets and image sbx reports for it; TTY required; `status` shows the generation as `adopted, not gated` until it is recreated ([01 1.6](01-system-model.md#generation)) | `sbx ls --json`, `sbx policy ls`, project record | project record | host: sbx | 0, 1, 2 (no TTY, open generation exists, workspace path differs, no such sandbox), 3 (gate 1) |
| `romeu stop <name>` **P** | `REQ-0313` `julieta snapshot --all`, then `sbx stop`. A failed snapshot is reported and the stop goes on, since the sandbox's disk persists; the command then exits 1. The next `run` starts the sandbox (run step 3). `REQ-0314` On a sandbox `sbx ls --json` shows stopped, `stop` runs no exec and no `sbx stop`, prints `already stopped` and exits 0 | project record | memory (via julieta) | host: sbx; sandbox: julieta | 0, 1, 2, 4 |
| `romeu salvage [--from-host] [--include-transcripts] <name>` **P** | capture everything sandbox-only; verify and import on host ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)). `REQ-0315` It stops agents (`julieta salvage --stop-agents`) only when `rm`, `recreate` or `retire` calls it; a standalone salvage leaves agents running and says so in its report. A stopped sandbox is started for the sandbox half and stopped again. `REQ-0316` When the start or the sandbox half fails, salvage exits 1 naming `--from-host` and what it does not recover: uncommitted changes, stashes, ignored files and agent state. Each preservation prints the salvage ref prefix it wrote The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | project record, memory dir | memory (via julieta), `refs/romeu/salvage/...`, project record | host: sbx, git; sandbox: julieta | 0, 1, 2, 4, 5 |
| `romeu rm [--accept-loss=<reason>[,<reason>]] <name>` **P** | `REQ-0317` after the preflight and before the salvage half, the memory reader checks for an open `handoff` entry tagged `tombstone` whose stamp names the open generation (08 8.3): on a TTY `rm` asks whether to go on without one, and without a TTY it warns (`no-tombstone`) and goes on. `REQ-0318` Then it prints the workload volumes that salvage does not cover ([06 6.2](06-kits.md#62-workload-choice)), salvages, and removes the sandbox; the tree is kept. `REQ-0319` `--accept-loss` accepts only the skipped reasons it names ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)), and the salvage record keeps them. `REQ-0320` A rerun on a `removing` generation whose fresh fingerprint differs ends as `fingerprint-unstable` (4.4) says The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | project record, memory handoff dir | as salvage; sbx removal; egress rules removed; generation closed | host: sbx, git; sandbox: julieta (the salvage half) | 0-5 |
| `romeu recreate [--accept-loss=<reason>[,<reason>]] <name>` **P** | `rm` then `run`. `REQ-0321` A create that fails after the removal, because a workload, frontend or kit download is gone, leaves the generation closed, the salvage refs and memory intact and no sandbox; the error is `kit-build-failed`, and the recovery is a pin or catalog fix, then `romeu run` (J6) The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | as rm + run | as rm + run | as rm + run | 0-5 |
| `romeu retire [--force] [--accept-loss=<reason>[,<reason>]] <name>` **P** | `REQ-0322` `rm` if needed; an incomplete inner salvage exits 5 before any move unless `--accept-loss` names its reasons. `REQ-0323` Report unpushed and salvage-only work and confirm on a TTY; off a TTY, `--accept-loss` stands for the confirmation. `REQ-0324` Say whether the workload volumes are left (06 6.2); move `<name>-env/` to `.attic/`; drop it from the workspace files. `REQ-0325` `--force` only allows a project the spec still lists. `REQ-0326` `status` and `doctor` never read `.attic/` or the state attic The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | spec (the project must be absent unless `--force`), clones | `.attic/`, workspace files, state attic | host: git, sbx; sandbox: julieta (the salvage half) | 0-5 (5: an incomplete salvage, `fingerprint-unstable` (4.4), a declined confirmation, or no TTY without `--accept-loss`) |
| `romeu status [--json] [<name>]` | `REQ-0327` per project: sandbox state, spec vs synced commit, drift, interrupted promotion, toolchain acknowledgement state, recreate needed, generations and their states (`adopted, not gated` for an adopted one), salvage records, unimported snapshots, orphaned projects and repo dirs; per repo, the `createdAt` of the last `snapshot/heads.json`, `disabled` when it says so ([08 8.4](08-memory-handoff-salvage.md#84-snapshot-after-every-commit)), and the newest stamp's dirty and stash counts (03 3.10); the warnings of 4.1; for a running sandbox also julieta's warnings and recorded hook failures (via `julieta status --json`). `REQ-0328` `status` runs no preflight, so it skips that exec, and says so, when the project's drift or identity report is not clean. `REQ-0329` A read that fails (an sbx error, a julieta exec that fails or times out, an unreadable record) is printed in that project's block with its error id and the output goes on; `--json` lists such reads under `errors` per project; the exit is 1 when any read failed and 0 otherwise, never 3 | read only | nothing | host: sbx, git (read); sandbox: julieta (read) | 0, 1 (never 3) |
| `romeu doctor [--json]` | `REQ-0330` host health and security checks (below) | host settings, git config, sbx settings, policies and `sbx ls --json`, `$HOME` symlinks, tool configs, the signing socket, the tree, the clones' refs, each secret argv's output (never kept) | nothing | host: sbx, git, each secret argv | `REQ-0331` 0 (all pass); 1 when any check could not run, the report being incomplete, with every failed check still printed; 6 when every check ran and any failed. `--json` carries one object per check with `result` in `pass`, `warn`, `fail` or `error`, and the error id for `error` |
| `romeu pull [-C <dir>] [--branch <b>]` **P** | `REQ-0332` fetch the sandbox's current work for the repo containing `<dir>` and update its review checkout The last step is the memory commit and push of 08 8.1, whose warnings close the command's output | project record, remote URL, `julieta status --json` | `refs/sandboxes/<name>/*` (primary), `refs/romeu/snapshots/<name>/*` (secondary), `review/<dir>/`, `review.code-workspace` | host: git, sbx; sandbox: julieta | 0, 1, 2, 4 |
| `romeu memory list [--repo <dir>\|--shared] [--json] <name>`, `romeu memory show [--json] <name> <id>...` | `REQ-0333` the host read of a project's memory: `list` prints the index of 08 8.2 over every memory dir of the project, the open `handoff` first with the gap line of 08 8.3, and takes the filters of `julieta memory list` (4.3); `show` prints the named entries in full, in the order given; every line is escaped (I29) and the output is labelled as text an agent wrote ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)); both read through `memstore`'s readers (I24) and write nothing | memory dirs via `os.Root` | nothing | host | 0, 1 |
| `romeu version [--json]` | | | | | 0 |
| `romeu completion bash\|zsh\|fish` | `REQ-1242` print the shell completion script of the command tree (ADR 0015); `romeu help [<command>]` prints the help of a command | | nothing | | 0, 2 |

`REQ-0334` Removed projects: `sync` never deletes, so there is no `--prune` flag;
a project absent from the spec but present in the tree is reported as
`orphaned` by `sync` and `status`, and `romeu retire` is the explicit
verb. The same applies to a repo removed from a spec (`orphaned repo
dir`, see J13).

### How `romeu sync` renders, gates and promotes

1. `REQ-0335` Take the lock (runs promotion recovery). Load and validate host
   settings (exit 2; a missing secret binding is `secret-unbound`,
   whose fix hint prints the `<name>@<project>` entry to paste). Check
   gate 1 (01 1.4; exit 3): `sbx version` is a read, and 01 1.5 holds
   back only a mutating sbx call, so a stale toolchain stops `sync`
   before it promotes.
2. `REQ-0336` Hardened fetch of the config repo's origin into
   `refs/romeu/origin/<cfg-dir>/*`. The spec commit is the head of the
   config ref, or with `--from <sha>` that commit, which must be
   reachable from `refs/romeu/origin/*` (exit 2 otherwise): a spec on
   a branch is read after the branch is pushed.
3. `REQ-0337` `git fsck --strict` on the spec commit; read `projects/*.yaml` via
   `git cat-file`; strict decode and validate all specs (cross-project
   rules: unique names, unique URLs); exit 2 listing every error. `REQ-0338` A file
   under `projects/` that is not `*.yaml` is an error naming the path
   (`projects-stray-file`, `RJ-345`, exit 2), and no spec whose primary
   repo URL equals `config.url` is an error naming `config.url`
   (`config-project-missing`, `RJ-346`, exit 2); 03 3.1 holds the two
   rules.
4. `REQ-0339` For each target project: create `<name>-env/` (mode 0700, with its
   `.metadata_never_index` marker, [02 2.3](02-layouts.md#23-host-tree))
   and `memory/<dir>/` (create only, with the fixed subdirectories);
   hardened clone of missing repos; hardened fetch of each repo's
   origin into `refs/romeu/origin/<dir>/*`; resolve `egressCommit` from
   `refs/romeu/origin/<dir>/<ref>`.
5. `REQ-0340` Refuse (exit 2) if any memory dir violates the layout allowlist (I24).
6. `REQ-0341` Read workload and kit descriptors: the cache in host state first;
   `internal/oci` by digest only on a cache miss (`registry-unreachable`
   when it cannot); local kits from the config commit's tree via
   `git fsck --strict` and the validated walk (I27). Derive egress
   ([07](07-mise-egress.md)). Delete any candidate dir the project
   record does not name. Render the candidate (`sbxenv.yaml`,
   `render.json`, `kits/`) under `.romeu/candidates/`.
7. `REQ-0342` Drift check of live files against the current `render.json`
   (exit 4, `drift-detected`, naming the file; `--overwrite-drift`
   replaces the edited live file, and its edit is lost).
8. `REQ-0343` Gate 2 and the candidate state machine
   ([01 1.6](01-system-model.md#candidate)); promotion commit on
   approval or when unchanged.
9. `REQ-0344` `sbx ls --json`. If the sandbox is running:
   the rest of the preflight (01 1.5), then reconcile egress live; if the `secrets`
   field is tagged `apply:"live"` (A10, Q18), `sbx secret set <svc>
   --sandbox <name> --command <rendered command>` (never a value); report
   "recreate needed" when the recreate digest differs from the running
   generation.
10. `REQ-0345` Report orphaned projects and repo dirs; remove nothing. Warn
    `julieta-pin-mismatch` when the julieta release the config repo's
    `validate.yml` pins is not the version romeu embeds.

### How `romeu run` reaches the run layout

1. `REQ-0346` Preflight (01 1.5). When `refs/romeu/origin/<cfg-dir>/<ref>`, as last
   fetched and with no network call, is ahead of the synced spec
   commit, print the warning `spec-moved: romeu sync <name>`.
2. `REQ-0347` The identity step of the preflight (01 1.5, step 6) has read `sbx ls
   --json` and refused the two cases that are not this project's
   sandbox: one romeu has no open generation for (the error
   `unknown-sandbox`, `RJ-203`, fix hint "decide: confirm that `<name>`
   is this project's sandbox (its workspace is
   `$ROMEU_ROOT/<name>-env/<primary>`), then `romeu adopt <name>`") and
   one whose workspace path differs. Absent, `sbx-unknown` and
   `upstream-shape` are as [01 1.6](01-system-model.md#generation)
   defines them. `REQ-0348` An absent sandbox with an open generation is preserved
   as the closed-lost row of 01 1.6 says, keeping a lost record `salvage
   --from-host` already wrote for the generation; otherwise romeu
   imports `refs/sandboxes/<name>/*` and every `snapshot/` bundle into
   `refs/romeu/salvage/<name>/<gen>/<new salvage id>/` and records
   `result: lost` with the newest facts' counts as the reasons
   `dirty-at-last-facts:<n>` and `stashes-at-last-facts:<n>`. `REQ-0349` Either way
   the generation moves to `closed-lost`. `run` then prints a fixed
   block (the lost generation, the time of its last snapshot, the
   branches and the salvage id preserved with the salvage ref prefix it
   wrote, and the dirty and stash counts as not preserved, with `romeu
   memory show <name>` to read them, J10 step 3 to recover a branch, and
   `romeu run <name>` to create the new sandbox) and exits 1 with
   `generation-lost` (`RJ-335`); that next `run` creates
   ([01 1.6](01-system-model.md#generation)).
3. Compare the recreate digest of the live render with the open
   generation's, then act on the sandbox's state:
   - `REQ-0350` absent: record a new generation first (01 1.6), then `sbx env run
     -d --clone <dir>` (never the flag that applies a plan without asking (`--auto-approve` or `-y`; `-f`/`--force` on `env run`, `env create`, `run` or `create`); the call is interactive
     and raw, as 4.1 Output says, so sbx's own prompt appears in this
     terminal when needed). If sbx returns non-zero on that create, read
     `sbx ls --json`: with the sandbox absent, remove the record just
     written; with it present, half built, keep the open record. Exit 1
     in both cases (a kit that does not build is `kit-build-failed`), so
     only a crash leaves a record without a sandbox, and the next `run`
     closes it as lost. `REQ-0351` A create that times out (`subprocess-timeout`)
     keeps the record whatever `sbx ls --json` shows, because sbx's
     daemon may finish the create after romeu stopped waiting; the next
     `run` finds the sandbox present and goes on, or absent and closes
     the generation as lost (01 1.6).
   - `REQ-0352` present and stopped: when the digests differ, exit 5 with
     `recreate-needed` (fix: `romeu recreate <name>`), since starting
     it would run an env file that no longer matches the sandbox;
     otherwise `sbx env run -d <dir>` starts it, with no provisioning
     (what sbx does with an env file that changed under a stopped
     sandbox is recorded by A15).
   - `REQ-0353` running: no `sbx env run`; when the digests differ, warn
     `recreate-needed` ("recreate needed: <fields> changed -> romeu
     recreate <name>").

   romeu names the env file by path; whether sbx also reads a sandbox
   configuration from the workspace tree is recorded by probe A5, and
   if it does, the drift check refuses one
   ([03 3.3](03-formats.md#33-rendered-sbxenvyaml-sbx-env-file-schemaversion-1)).

   `sbx env rm` carries `--force`, because romeu's calls have stdin
   closed and sbx fails a confirmation without a terminal (exit 1);
   romeu's own confirmation and salvage come before the call, and this
   `--force` approves no plan (I4 is about the plan of `env run` and
   `env create`).
4. `REQ-0354` Reconcile egress: `sbx policy allow network --sandbox <name> <d>` for
   missing domains; `sbx policy rm network --sandbox <name> --resource
   <d>` for domains romeu applied that left the set; verify with `sbx
   policy check network --sandbox <name> <d>`. (Exact argv comes from
   the block A recordings.) `REQ-0355` romeu writes each domain into
   `egressApplied` before its `allow` call and removes it from the
   record after its `rm` call succeeds, so the record always names every
   rule romeu may have applied; a reconcile cut short is finished by the
   next `run`, which reconciles from the record. The record write is the
   atomic write of 03 3.8, once before the first call and once after the
   last; `sbx policy check` runs after the writes.
5. `REQ-0356` Compatibility check (06 6.3): `sbx env exec <dir> --
   <abs .romeu/bin>/julieta-linux-<GOARCH> version --json`.
6. `REQ-0357` `sbx env exec <dir> --env JULIETA_MANIFEST=... --
   <abs julieta path> setup --json`; on failure print julieta's report
   (escaped) and exit 1 without attaching.
7. `REQ-0358` Unless `--no-attach`: `exec` into
   `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- <abs julieta path> layout up`.
   When the run layout fails, `layout up` exits with `layout-failed`,
   whose fix hint is the host command that opens a shell in the
   sandbox without it (`sbx env exec -it <dir> -- /bin/bash -l`, with
   `<dir>` filled in; julieta reads the manifest it cached).

### How `romeu pull` updates a review checkout

`REQ-0359` Resolve the project and repo from `<dir>` (must be inside
`$ROMEU_ROOT/<name>-env/`); preflight; compatibility check (06 6.3);
`julieta snapshot --all` and `julieta status --json` via exec; `REQ-0360` primary:
the sandbox must be running, else exit 2 with the error
`sandbox-stopped` (`RJ-347`), whose hint is `romeu run --no-attach
<name>`; `REQ-0361` re-read the daemon URL, hardened fetch into
`refs/sandboxes/<name>/*`, then compare fetched heads with `julieta
status` (mismatch: exit 1, nothing updated); `REQ-0362` secondary (its bundle is
read from the mount, so it needs no running sandbox): verify and
unbundle `snapshot/heads.bundle` into
`refs/romeu/snapshots/<name>/<dir>/*`, then run `git fsck --strict` on
the unbundled head, the call kit materialization uses (I27), since
`transfer.fsckObjects` does not cover an unbundle (05 5.1); `REQ-0363` update
`review/<dir>/` (a standalone repo with the host clone as alternate and
the hardening keys in its local config) to the sandbox's current branch
head, detached; write `review.code-workspace` again from the review
checkouts.

`REQ-0364` Every command that starts sbx or git checks its version against the
floor of 12 12.1 (sbx) or 05 5.1 (git) before gate 1 and exits 2 with
`sbx-floor` (`RJ-343`) or `git-floor` (`RJ-344`); `doctor` reports the
same check by its slug.

### Checks run by `romeu doctor`

`REQ-0365` Checks marked **pre** also run in the preflight (01 1.5, step 5); one
implementation serves both. Each check is a row of the table of 4.1 by
its slug, which `--json` reports with its result; a check whose
condition is also an error elsewhere (`drift-detected`,
`signing-socket`, `toolchain-approval-required`) shares that error's
slug. A **pre** check stops a mutating command with exit 2, except the
protective commands of
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call),
which print it and go on; so each has an id in 4.4.

| Check | Slug | Result |
|---|---|---|
| `REQ-0366` sbx version below the floor of 12 12.1 or not at the configured absolute path | `sbx-floor` | fail |
| `REQ-0367` sbx version outside the tested window ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)) | `sbx-untested` | warn |
| `REQ-0368` git version below the floor ([05 5.1](05-security.md#51-hardened-git-internalgitsafe)) | `git-floor` | fail |
| `REQ-0369` toolchain acknowledgement missing or stale | `toolchain-approval-required` | fail |
| `REQ-0370` host global git config sets a key that [05 5.1](05-security.md#51-hardened-git-internalgitsafe) neutralizes | `host-git-config` | warn, with the review-checkout risk |
| `REQ-0371` sbx: global (not per-sandbox) secrets for a service any project uses | `sbx-global-secret` | fail (I22) **pre** |
| `REQ-0372` sbx: a global allow rule of `**` or other broad wildcard, or a permissive default network policy | `sbx-broad-allow` | fail (I22) **pre** |
| `REQ-0373` sbx: `env.rememberHostCommands` not true | `sbx-remember-commands` | warn |
| `REQ-0374` sbx: `kit.allowedSources` does not admit the workload registries | `sbx-allowed-sources` | fail |
| `REQ-0375` root inside a git repo, equal to `$HOME`, not absolute, not owned by the user, or containing host settings/state; `REQ-1277` also a root inside the host settings directory or the host state directory, and a root, or `<root>/<cfg>-env`, that is a symlink, not owned by the user, or writable by its group or others | `root-unsafe` | fail |
| `REQ-0376` root under `~/Library/Mobile Documents`, `~/Library/CloudStorage/` or `~/Dropbox`, or not excluded from Time Machine; a `<name>-env/` without its `.metadata_never_index` marker, the form probe A17 measures ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts)) | `root-indexed` | warn; the hint adds that a root excluded from Time Machine needs another copy of its memory dirs, handoffs and unpushed work; tested by the case "doctor warns root-indexed when the marker is missing" |
| `REQ-0377` `$ROMEU_ROOT/.attic` or a `<name>-env/` that its group or others can access | `tree-mode` | fail |
| `REQ-0378` a symlink under `$HOME` (depth 1) or `$HOME/.config` (depth 2) resolving into `$ROMEU_ROOT` | `home-symlink` | fail (I23) **pre** |
| `REQ-0379` mise `trusted_config_paths` or a direnv allow list covering `$ROMEU_ROOT` | `auto-trust` | fail (I23) **pre** |
| `REQ-0380` VS Code: `security.workspace.trust.enabled=false`, or `$ROMEU_ROOT`, a parent of it or any folder under it in the trusted folders. Only VS Code's trust settings are checked; another editor's trust model is the operator's to verify | `vscode-trust` | fail (I14, I23) **pre** |
| `REQ-0381` host settings not 0600, state dir not 0700 | `settings-mode` | fail |
| `REQ-0382` secret `argv[0]` missing or not absolute; a secret argv that, run once with exactly `HOME` and `PATH=/usr/bin:/bin` ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)), exits non-zero or prints nothing (the output is tested for emptiness only, never printed or kept, I25) | `secret-argv` | fail |
| `REQ-0383` secret entries for unknown projects | `secret-unknown-project` | warn |
| `REQ-0384` signing: a project uses `git-ssh-sign` and `signing.agentSocket` is unset, unreachable, or holds a number of keys other than one, or its one key differs from the kit's `signingKey` arg | `signing-socket` | fail **pre** |
| `REQ-0385` tree: live derived files drifted | `drift-detected` | fail |
| `REQ-0386` tree: an interrupted promotion | `interrupted-promotion` | fail |
| `REQ-0387` tree: a memory dir violating the layout allowlist | `memory-layout` | fail |
| `REQ-0388` tree: a workspace file that differs from the one derived again from host state ([01 1.2](01-system-model.md#12-sources-of-truth-vs-derived)) | `workspace-files` | fail |
| `REQ-0389` an open generation with no sandbox in `sbx ls --json`, or a sandbox a project names with no generation in `open`, `salvaging` or `removing` | `generation-sandbox` | fail |
| `REQ-0390` a `removing` generation with its sandbox present ([01 1.6](01-system-model.md#generation)); the hint is `romeu rm <name>` | `removal-pending` | fail |
| `REQ-0391` a record of a format romeu knows that does not decode ([03](03-formats.md), opening); the hint is as in `RJ-334` | `record-unreadable` | fail |
| `REQ-0392` a project's `egressApplied` differs from the per-sandbox rules `sbx policy ls` reports | `egress-applied` | fail |
| `REQ-0393` a ref under a salvage record's `refsPrefix` missing from its clone, naming the record | `salvage-ref-missing` | fail |
| `REQ-0394` a project record over 256 KiB ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)) | `record-size` | warn |

`REQ-0395` Each check that fails prints a fix hint (4.1) naming the change that
clears it, because several common setups fail a **pre** check:
`sbx-global-secret` names the per-sandbox form of the sbx secret for
that service and the global one to remove; `sbx-broad-allow` names the
rule to remove; `vscode-trust` names the setting to turn back on or the
trusted folder to remove; `home-symlink` and `auto-trust` name the link
or the trust entry; `signing-socket` points at J1 step 5.

## 4.3 julieta (sandbox)

| Command | Purpose | Reads | Writes | Exit codes |
|---|---|---|---|---|
| `julieta setup [--json]` | `REQ-0396` idempotent; `REQ-1309` one run at a time: `setup` holds a sandbox-local lock (`$HOME/.local/state/julieta/locks/setup.lock`) for its whole run, and a second run waits for it, so the boot hook (06 6.4) and `romeu run` step 6 never clone, install or run a repo task at once; every git call runs in gitsafe's sandbox mode ([05 5.1](05-security.md#51-hardened-git-internalgitsafe)), as every julieta git call does: `REQ-0397` point `$HOME/.local/bin/julieta` at the running binary; install the hook dispatcher: write its scripts, record the system `core.hooksPath` in julieta state, then point the global `core.hooksPath` at the dispatcher dir, leaving the key alone when it already names it, and reporting and leaving it as it is when it names another; `REQ-0398` clone missing secondary repos from origin; `REQ-0399` fetch origin in every repo (a fetch that fails is the warning `fetch-failed`, naming the repo and the git error, shown by `status` and the next SessionStart, and `setup` goes on) and move no branch: every checkout stays at the commit the clone or the operator left it, so a bug is investigated on the exact code and locks it was cloned at, and `setup` may run at every boot (06 6.4) without changing the code under the session (decided on 2026-10-28; until then `setup` fast-forwarded the local default branch, and its warning slug `default-branch-moved` is never used again); the `--json` report keeps the format `julieta-setup/v1` (03, REQ-1310): each repo's new member `branchState` names the state of its local default branch against origin's, whatever the work tree holds or HEAD names (`behind`, `current`, `ahead`, `diverged`, or why it was not compared: `not-fetched`, `no-default`, `no-branch`, `failed`), and the deprecated `fastForward`, `from` and `to` stay, always empty; `REQ-1301` before `memory check`, migrate once a `handoff/` directory left in a memory dir by the handoff files of before 03 3.10's stamp, over every repo dir of the manifest: each `clear` or `final` file becomes a `handoff` entry of the shared dir with the file's ULID as its id, its `createdAt` the file's, its title `handoff of <createdAt>`, the `final` ones tagged `tombstone`, its front matter the entry's `stamp`, and only the newest of all migrated narratives is `open`, the rest `done`; each `facts` file becomes a stamp file of the same ULID (the first `julieta stamp` prunes beyond the cap of 08 8.6); an entry or stamp whose ULID already exists is skipped, so a run cut short resumes; a file that does not parse, or whose body fails the heading rule of 03 3.9, stays where it is and is warned `handoff-legacy`, never deleted (08 8.1), and the `handoff/` directory is removed only once it is empty; a second run on a migrated dir finds nothing to do; `REQ-1306` record each repo's mise configuration as trusted inside the sandbox (`mise trust` on each configuration file `install` reads), run with `MISE_TRUSTED_CONFIG_PATHS` and `MISE_PARANOID` unset as the agent's shell has them, so `mise` in that shell reads the files without asking; the host keeps no auto-trust (I23); `REQ-0400` `install`; `REQ-1307` then, in each repo whose mise configuration defines the task `setup` (as `mise tasks ls --json` lists it), `mise run setup` with the mise environment of `install` and the budget of 4.1, so a repo prepares what its own checks need (romeu-e-julieta's task fills the Go module cache); a task that fails is the warning `repo-setup-failed` (`<repo> exit <n>`), and `setup` goes on; `memory check`; agent memory dir check; `REQ-0401` the required agent-profile paths of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction) step 5 exist; `REQ-0402` compare the installed skills and hook lines with the digest julieta embeds (06 6.3, `skills-stale`). Exit 1 only when the sandbox is left unusable: the `PATH` link, the dispatcher or a secondary clone could not be made, the system `core.hooksPath` cannot be read or recorded or is not an absolute path, the global `core.hooksPath` names another dir, or `memory check` fails; a failed fetch, a failed repo `setup` task, `skills-stale` and a stale lock are warnings | manifest | `$HOME/.local/bin/julieta`, `$HOME/src/...`, repos (fetch only), git global config (sandbox), dispatcher dir, mise data and trust, julieta state, `$HOME/.local/state/julieta/setup.log` | 0, 1, 2 |
| `julieta install [--repo <dir>] [--force]` | `REQ-0403` `mise install` (locked) in every repo, after it has run the check of `lock --check` on each repo's files and found nothing (any finding is the error `check-findings`, exit 6, and nothing is installed; a file that does not parse is `lock-unparsed`, `RJ-325`); skip when the `lock-set` digest equals the last success, a run over every repo only (`--repo` neither skips nor records). `REQ-0404` A download mise reports as refused, by an HTTP 403 or by a proxy that refuses the tunnel, fails with the error `install-blocked` (`RJ-341`), whose message says what mise saw and that the refusal may be the egress policy, since a 403 does not prove it, and whose details name the tool, the host, the sbx command that approves it once for this sandbox (lost at recreate), and the durable fix, a spec edit and gate 2; the command is offered only for a host that is a plain domain name or an IPv4 address; any other failure of mise is `mise-failed` (`RJ-351`); egress follows the spec's `ref`, so a tool added on a branch installs once it merges; the error says whether the host is in the egress set the manifest carries ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress)) | manifest, locks, `.julieta/lock-exceptions` | mise data, julieta state | 0, 1, 2, 6 (a finding of the check) |
| `julieta lock [--repo <dir>] [--check]` | `REQ-0405` `mise lock --platform` with `linux-x64`, `linux-arm64` and the platforms the repo's `lockfile_platforms` setting lists (07 7.3). `REQ-0406` `--check` verifies freshness without writing: the lock and `mise.toml` parse with the pinned layout (`lockfile_version` 3, nesting of arrays and inline tables of at most 32 levels, keys and table headers of at most 8 parts, at most 1 MiB; a file that does not is the finding `lock-unparsed`, exit 6), every tool in `mise.toml` has a lock entry, and every lockable entry, judged by the key of `mise.toml` and never by the backend the lock claims, covers `linux-x64` and `linux-arm64`; `REQ-0407` every platform of a lockable entry holds a checksum of the form `sha256:` and 64 lowercase hexadecimal digits: one without is the finding `lock-no-checksum`, one of another form the finding `lock-bad-checksum`, each exit 6 (decided on 2026-10-22; a missing checksum may be excused tool by tool in `.julieta/lock-exceptions`, [07 7.3](07-mise-egress.md#73-lockfile-rules)); a `backend:tool` the embedded catalog does not know is the warning `lock-unknown-tool`, naming the two fixes of 07 7.5 step 2. A `mise.lock` that is not a regular file is refused before `lock` writes it, as `lock-unparsed` | `mise.toml`, `mise.lock`, `.julieta/lock-exceptions` | `mise.lock` (not with `--check`) | 0, 1, 2, 6 (`--check` found a missing entry, platform or checksum, or a file that does not parse) |
| `julieta hooks run <hook> [args]` | `REQ-0408` dispatcher: the `<hook>` of the system `core.hooksPath` that `setup` recorded, if executable, when the record is an absolute path other than the dispatcher dir; then julieta's own action (`pre-commit`: `lock --check`, on the files the commit records, read from the index the hook is given (`GIT_INDEX_FILE`, which `git commit -a` and `git commit <path>` set), when `mise.toml`, `mise.lock` or `.julieta/lock-exceptions` is staged; `post-commit`, `post-rewrite`, `post-merge`: `snapshot`), then the repo's tracked `.githooks/<hook>` if executable, then `$GIT_DIR/hooks/<hook>` if executable; the hook's arguments go to each, and so does a copy of its stdin, which the dispatcher copies once, with no bound, to a temporary file outside the repo that it removes before the stages run, so a hook that reads none leaves all of it to the next and no stdin size fails the hook (the stdin of `proc-receive`, a protocol with git, and a stdin that is a terminal or another character device, which never ends, go to the first hook that runs); the system hook runs with `JULIETA_HOOKS_SYSTEM=1`, and a dispatcher started with it set skips the system stage, so a system hook that leads back to the dispatcher ends; `REQ-0409` a failure of julieta's own `snapshot` is recorded and does not stop the chain (08 8.4); otherwise the first non-zero status stops, and before exiting the dispatcher prints on stderr `julieta hooks: <hook> stage <system\|julieta\|tracked\|local> (<path>) exited <n>` (or `exited 128+<signal>` for a stage a signal ended, 4.1 Signals), plus the error id and fix when the stage is julieta's own; a failure is recorded in julieta state until the same hook next succeeds | repo | as the hooks do; julieta state | hook's status |
| `julieta hooks guard [--json]` | `REQ-0410` reads one PreToolUse JSON document on stdin (`tool_name` and `tool_input.command`, the shape C5 records as a fixture under `e2e/testdata/agent/<version>/`); exits 2 with the refusal text of 06 6.4 on its patterns (REQ-0598: a merge, a `v*` tag push, a release, `gh api` changes to rulesets or repository settings) and 0 otherwise; exits 2 on a document it cannot decode or a command it cannot parse (fail closed) | stdin | nothing | 0, 2 |
| `julieta memory add\|list\|show\|edit [--repo <dir>\|--shared] [--json]` | `REQ-0411` memory entries of the repo containing cwd (or `--repo`), or of the project's shared dir with `--shared` (03 3.9); `add --body-file <path\|->` reads the body from a file or stdin, the way the `handoff` skill pipes a narrative; `list --query <text>` filters; `edit --status done` closes an entry; julieta stamps id and timestamps, and the `stamp` of a `handoff`; `add` prints the id, the path it wrote and the repo, or `shared`, it filed the entry under. `REQ-1288` `list` prints one line per entry, `<id> <repo> <status> <type> <updatedAt date> <title>`, the title escaped as every agent-written text printed to a terminal is (REQ-0670), so a newline in it never breaks the line; newest `updatedAt` first, and `--sort created` orders by `createdAt` instead. `REQ-1293` `list` narrows with `--query <text>`, `--status open\|done`, `--tag <t>` (repeatable; every given tag must hold), `--since <RFC 3339 date or timestamp>` (a date is read as midnight UTC; the entries whose `updatedAt` is at or after it; another form is a usage error, exit 2) and `--limit <n>`; `--json` carries the same entries in full, in the same order. `REQ-1289` `show` takes one or more ids and prints each entry in full, in the order given; an id with no entry fails the command with exit 1 after the known ones are printed. `REQ-1290` `add` warns `memory-derivable`, and writes the entry, when a line of the body of at least 40 bytes, after trimming, equals byte for byte a line, trimmed the same way, of the repo's `CLAUDE.md`, `AGENTS.md`, `CLAUDE.local.md` or `.claude/rules/*.md` at the working tree, or of a file one of them `@`-imports from inside the repo (the import is `@<path>` alone on a line); a string comparison, no model call, so it costs no tokens on a command the agent runs often. The warning names the file and line of each match, escaped, at most ten lines and then `<n> more`; an instruction file over 1 MiB, an import that leaves the repo or names no file, and a symlink out of the repo are skipped, the oversized file named in the warning's place; the warning is printed whether the entry is new or the duplicate of REQ-0279, on stderr, the `--json` document unchanged. The type tag rule is REQ-0242 and REQ-1287 (03 3.9) | manifest, memory dir | memory dir | 0, 1, 2 |
| `julieta memory check` | `REQ-0412` every manifest memory dir is mounted, writable, and satisfies the layout allowlist | manifest | nothing | 0, 1, 2, 6 (a violation of the layout) |
| `julieta stamp [--json]` | `REQ-0413` measure the stamp of 03 3.10 for every manifest repo and write `_shared/stamps/<ULID>.yaml`, keeping the newest files of 08 8.6 and removing the older; the SessionEnd hook (08 8.2); prints the path it wrote | manifest, repos | `_shared/stamps/` | 0, 1, 2 |
| `julieta memory index [--hook]` | `REQ-0414` print the open `handoff` entry (its index line, the gap line of 08 8.3 when the newest stamp differs from its stamp, and its body up to the narrative bound) and the index of the other open entries of the project (one line each, the line of `memory list`, up to the index bound); `--hook` (SessionStart) prints julieta's warnings and the fixed memory line first (08 8.2) | manifest, memory, julieta state | nothing | 0, 1, 2 |
| `julieta snapshot [--repo <dir>\|--all]` | `REQ-0416` bundle unpushed work into `memory/<dir>/snapshot/` (no debounce) | repos, manifest | memory | 0, 1, 2 |
| `julieta salvage [--stop-agents] [--include-transcripts] [--fingerprint] [--json]` | `REQ-0417` full salvage for the `salvageRun` id in the manifest. Its `--json` document carries the fingerprint object: the sandbox-side values of the fingerprint of [03 3.8](03-formats.md#38-host-state-schemas-state-v1), as the reads of steps 2 and 4 of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction) saw them (romeu fetches the daemon heads itself), which romeu digests at the `removing` write; completeness is the host's verdict (08 8.5), and julieta exits 1 only when it cannot write the salvage. With `--fingerprint`, no salvage: it reads the same values, writes nothing, stops no agent and prints that object and nothing else; it is the read a rerun on a `removing` generation makes ([01 1.6](01-system-model.md#generation)) | manifest, repos, agent dirs | memory dirs, refs under `refs/salvage/<generation>/` (create only, outside `refs/heads`, 08 8.5); nothing with `--fingerprint` | 0, 1, 2 (with and without `--fingerprint`) |
| `julieta layout up [--dry-run [--json]]` | `REQ-0418` start the herdr server if absent, apply missing tabs, report `layout-stale` when the live layout's `run` digest differs from `runDigest`, attach; `--dry-run` prints the `layout.apply` requests without applying (golden tests). A herdr whose protocol differs from the one A13 recorded is `herdr-protocol`; any other failure is `layout-failed` (4.2, run step 7) | manifest | herdr state (not with `--dry-run`) | 0, 1, 2 |
| `julieta pane run <id>` | `REQ-0419` internal: the herdr leaf argv; runs the pane's steps | manifest | - | last step's status |
| `julieta pin workload [--tag <t>] <spec-file>` | `REQ-0420` resolve the workload tag to a digest, verify the manifest list covers linux/amd64 and linux/arm64, rewrite the spec field; the registry host is in the config project's `egress.extra` (07 7.5) | spec, registry | spec | 0, 1, 2 |
| `julieta pin check [--workflows] [--json] <path>...` | `REQ-0421` report pins that are not digests or are behind: spec workloads and, with `--workflows`, the julieta release pinned in the config repo's CI workflows, whose pinned archive sha256 it also compares with that release's `checksums.txt` | files, registry, GitHub releases | nothing | 0, 1, 6 (a pin not a digest, behind, or not matching) |
| `julieta spec validate [--catalog] [--json] <file>...` | `REQ-0422` strict decode + schema + name/path rules, per file and across files, and one line naming the host-only rules of 03 3.1 it did not check; `--catalog` also fetches each repo's `mise.lock` at its `ref` (hardened, shallow) and reports every `backend:tool` and lock host missing from the embedded catalog: an unknown `backend:tool` is a finding, the case 07 7.5 step 2 makes a sync error, and a missing lock host is a warning, since 07 7.5 step 3 gates it. `REQ-0423` A file under `projects/` that is not `*.yaml` is a finding naming the path (03 3.1) | files; origins with `--catalog` | nothing | 0, 1, 2 (a bad flag or an unreadable file), 6 (any finding in the files given: a decode, schema or rule error, or with `--catalog` an unknown `backend:tool`) |
| `julieta status [--json]` | `REQ-0424` per repo: branch, HEAD, all local branch heads, ahead/behind, dirty, stashes, worktrees, last snapshot, snapshots disabled (repo-local `core.hooksPath`), tools installed; the live spec commit (`specCommit`, 03 3.6); the egress set romeu applied, from the manifest; agent memory dir state; whether the agent's settings carry the two hooks with julieta's exact commands and both skills are installed (`agent-hooks-missing` otherwise, 06 6.5); warnings; recorded hook failures | manifest, repos, julieta state, agent settings | nothing | 0, 2 |
| `julieta version [--json]` | | | | 0 |
| `julieta completion bash\|zsh\|fish` | `REQ-1243` print the shell completion script of the command tree (ADR 0015); `julieta help [<command>]` prints the help of a command | | nothing | 0, 2 |

`REQ-0425` julieta refuses (exit 2) a command whose Reads cell names the manifest
when none is cached and `JULIETA_MANIFEST` is unset; exit 2 on a missing
manifest is listed in each such row.

Environment julieta sets for mise calls, the one list of it, whose
reasons are in [07 7.3](07-mise-egress.md#73-lockfile-rules) and
[07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress):
`REQ-0426` `MISE_LOCKED=1`, `MISE_DISABLE_UPDATE_WARNING=1`,
`MISE_USE_VERSIONS_HOST_TRACK=false`, `MISE_AUTO_INSTALL=false`,
`MISE_YES=1`, `MISE_PARANOID=1`, `MISE_GLOBAL_CONFIG_FILE` and
`MISE_SYSTEM_CONFIG_FILE` set to paths below `/dev/null`, so that no
global or system configuration is read, `MISE_DATA_DIR`,
`MISE_CACHE_DIR` and `MISE_STATE_DIR` set to the julieta user's own
(`$HOME/.local/share/mise`, `$HOME/.cache/mise`,
`$HOME/.local/state/mise`), and `MISE_TRUSTED_CONFIG_PATHS` set to the
manifest's repo paths; a repo path that holds a colon, is relative or
holds a control character is refused before it reaches the variable. Of
the caller's environment a mise call keeps the search path, the home,
the temporary dir, the locale, the Go cache locations, the proxy
variables and the certificate files, and nothing else: no token (the
proxy of the sandbox authenticates the requests to GitHub), no XDG
variable and no `MISE_` or `GO` variable. Decided on 2026-10-22:
`MISE_YES=1` alone makes mise trust every configuration file it reads
(measured with mise 2026.10.3: a `mise.toml` outside the trusted paths
was read and its `exec()` command ran), and `MISE_PARANOID=1` beside it
refuses such a file as not trusted. `MISE_PARANOID=1` also makes mise
verify the attestation of a download, which contacts
`tuf-repo-cdn.sigstore.dev` ([07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress)).

The runtime ledger's commands (`romeu ledger ingest` and `query`,
`julieta event add` and `list`), its ingest steps and the events julieta
would write are designed in [13](13-runtime-ledger.md) and are not in
v1; they wait for their row in
[Deferred decisions](../spec.md#in-the-product).

## 4.4 Error ids

`REQ-0427` The ids this specification names, as rows of the one table of 4.1, which
holds these and every other error. `REQ-0428` `RJ-101`, `RJ-203`, `RJ-204` and
`RJ-301` were written before the sequence rule of 4.1 and keep their
numbers; the sequence goes on from `RJ-302`. This list stands until the
errors are declared in code (4.1, Error ids); from then the union one
test collects is the source, and this section is a link to
`docs/reference/errors.md`. `REQ-1258` A finding of a plain validation rule of
`internal/spec/rules.go`, one with no row in this table, has no id of
its own: it is an invalid document, exit 2, named by the rule's slug
(decided on 2026-10-22).

| Id | Slug | Exit | Raised by |
|---|---|---|---|
| `RJ-101` | `locked` | 1 | every mutating romeu command (4.1, Locking) |
| `RJ-203` | `unknown-sandbox` | 2 | preflight step 6 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)) |
| `RJ-204` | `signing-socket` | 2 | `sync` and the preflight of a project using `git-ssh-sign` ([06 6.4](06-kits.md#64-product-kits)); `doctor` reports the same check as a finding of `RJ-328` |
| `RJ-301` | `project-approval-required` | 3 | gate 2: preflight step 4, and a `sync` without a TTY whose widening digest differs ([01 1.6](01-system-model.md#candidate)) |
| `RJ-302` | `toolchain-approval-required` | 3 | gate 1 ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)); fix `romeu approve --toolchain` |
| `RJ-303` | `drift-detected` | 4 | preflight step 3 and `sync` step 7 |
| `RJ-304` | `interrupted-promotion-abandoned` | 4 | `REQ-0429` promotion recovery ([01 1.6](01-system-model.md#promotion-commit)); fix `romeu sync <name>` |
| `RJ-305` | `recreate-needed` | 5 | `run` step 3, on a stopped sandbox; on a running one the same slug is a warning (4.1, Warnings) |
| `RJ-306` | `salvage-incomplete` | 5 | `REQ-0430` `salvage`, `rm`, `recreate`, `retire`; the details name each reason, and the hint the `--accept-loss=<reason>` that accepts it |
| `RJ-307` | `salvage-path` | 5 | salvage verification ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)) |
| `RJ-308` | `sbx-unknown` | 1 | any command that needs to know whether a sandbox is absent, when `sbx ls --json` exits non-zero, and the rerun of `rm`, `recreate` or `retire` on a `removing` generation whose fresh fingerprint cannot be read ([01 1.6](01-system-model.md#generation)) |
| `RJ-309` | `sbx-output-unparsed` | 2 | `REQ-0431` retired: merged into `RJ-310`; its last text was "gate 1 and preflight step 5" |
| `RJ-310` | `upstream-shape` | 2 | every decoder of an output the product does not own, sbx outputs included: gate 1, preflight step 5 and the generation machine ([03](03-formats.md), opening); `doctor` reports it on the two I22 rows and exits 1, a check that could not run |
| `RJ-311` | `format-newer` | 2 | every reader of a versioned format ([03](03-formats.md), opening) |
| `RJ-312` | `julieta-not-embedded` | 2 | `REQ-0432` `run`, `sync` and promotion in a build without julieta ([02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)) |
| `RJ-313` | `manifest-size` | 2 | `sync` ([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)); `julieta spec validate` reports the same rule as a finding of `RJ-328` |
| `RJ-314` | `secret-in-argv` | 2 | host settings load ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)) |
| `RJ-315` | `secret-unbound` | 2 | `sync` step 1; the hint prints the entry to paste |
| `RJ-316` | `julieta-protocol` | 2 | the compatibility check ([06 6.3](06-kits.md#63-julieta-delivery)) |
| `RJ-317` | `julieta-digest` | 2 | the compatibility check |
| `RJ-318` | `julieta-bin-writable` | 2 | the compatibility check |
| `RJ-319` | `herdr-protocol` | 2 | `julieta layout up` |
| `RJ-320` | `layout-failed` | 1 | `julieta layout up`, the last step of `run` |
| `RJ-321` | `kit-build-failed` | 1 | `run` step 3 and `recreate` |
| `RJ-322` | `registry-unreachable` | 1 | `REQ-0433` `sync` step 6, `julieta pin workload`, `julieta pin check` |
| `RJ-323` | `subprocess-timeout` | 1 | any subprocess past its timeout (4.1, Subprocesses) |
| `RJ-324` | `disk-full` | 1 | `REQ-0434` any write; inside a salvage it is a skipped reason instead (03 3.11) |
| `RJ-325` | `lock-unparsed` | 2 | `sync` ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress), step 1); `julieta install` and `julieta lock`, for a `mise.lock`, a `mise.toml` or an exceptions file of a repo that cannot be read as one (`julieta lock --check` reports a file that does not parse as a finding of `RJ-328`); the hint is `julieta lock --repo <dir>` |
| `RJ-326` | `lock-host-invalid` | 2 | `sync` (07 7.5, step 3) |
| `RJ-327` | `catalog-unknown-tool` | 2 | `sync` (07 7.5, step 2) |
| `RJ-328` | `check-findings` | 6 | `REQ-0435` `romeu doctor`, `julieta lock --check`, `julieta install` (the same check of the repo's mise files, before it installs), `julieta pin check`, `julieta spec validate` (every finding in the files it was given), `julieta memory check`; the details list each finding by its slug |
| `RJ-329` | `sbx-global-secret` | 2 | preflight step 5 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)); `doctor` reports the same check by its slug as a finding of `RJ-328`; the hint names the per-sandbox secret and the global one to remove |
| `RJ-330` | `sbx-broad-allow` | 2 | preflight step 5 (01 1.5); `doctor` reports the same check by its slug as a finding of `RJ-328`; the hint names the rule to remove |
| `RJ-331` | `home-symlink` | 2 | preflight step 5 (01 1.5); `doctor` reports the same check by its slug as a finding of `RJ-328`; the hint names the link |
| `RJ-332` | `auto-trust` | 2 | preflight step 5 (01 1.5); `doctor` reports the same check by its slug as a finding of `RJ-328`; the hint names the trust entry |
| `RJ-333` | `vscode-trust` | 2 | preflight step 5 (01 1.5); `doctor` reports the same check by its slug as a finding of `RJ-328`; the hint names the setting to turn back on or the trusted folder to remove |
| `RJ-334` | `record-unreadable` | 2 | every reader of a versioned format, for a file of a format it knows that does not decode ([03](03-formats.md), opening); the file is never overwritten, the details name it, and the hint is to restore it from a backup, with `romeu salvage --from-host <name>` to save the work first; `status` and `doctor` report it without this exit (03, opening) |
| `RJ-335` | `generation-lost` | 1 | `run` step 2, after it preserves a lost generation; the hint is `romeu run <name>`, which creates the new sandbox |
| `RJ-336` | `removal-pending` | 2 | `REQ-0436` preflight step 6 ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)), for any mutating command but `rm`, `recreate` and `retire` that finds a `removing` generation with its sandbox present; `status` and `doctor` report it as `removal-pending` ([01 1.7](01-system-model.md#17-vocabulary)); the hint is `romeu rm <name>`, which finishes the removal |
| `RJ-337` | `format-unsupported` | 2 | every reader of `project.v1` or `host-settings.v1`, for a file whose version is older than the window of the current and the previous version ([03](03-formats.md), opening); the hint is `decide: apply the edit in the release notes of each skipped release` |
| `RJ-338` | `fingerprint-unstable` | 5 | `rm`, `recreate` and `retire`, on a generation `removing` with its sandbox present, when a fresh fingerprint ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)) differs from the newest `fingerprint` written with `removing`; the rule, the hint and the end states are [below](#fingerprint-unstable-rj-338) |
| `RJ-339` | `salvage-id-mismatch` | 5 | `REQ-0437` the host half of `salvage`, `rm`, `recreate` and `retire`, step 1 of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction), when the manifest's `salvageId` is not the id romeu passed; not a skipped reason, so `--accept-loss` does not cover it; hint: `decide: rerun; a mismatch that persists names a julieta that wrote another salvage's manifest, report it as an issue` |
| `RJ-340` | `lock-missing` | 2 | `REQ-0438` `sync` (07 7.5 step 1) and `julieta install` (07 7.3) on a `mise.toml` without `mise.lock`; hint: `julieta lock --repo <dir>` |
| `RJ-341` | `install-blocked` | 1 | `REQ-0439` `julieta install` on a download mise reports as refused (an HTTP 403 or a refused tunnel), with the message and the details 04 4.3 lists; hint: `decide: approve the host once for this sandbox with the sbx command named, or add the host to the spec's egress and run gate 2` |
| `RJ-342` | `manifest-protocol` | 2 | `REQ-0440` every julieta command that reads a manifest whose `protocol` is outside N and N-1 (03 3.6); hint: `romeu run <name>` |
| `RJ-343` | `sbx-floor` | 2 | `REQ-0441` every romeu command that starts sbx, before gate 1, when its version is below the floor of 12 12.1; `doctor` reports the same slug; hint: `decide: install sbx at or above the floor of 12 12.1` |
| `RJ-344` | `git-floor` | 2 | `REQ-0442` every romeu command that starts git, before gate 1, when its version is below the floor of 05 5.1; `doctor` reports the same slug; hint: `decide: install git at or above the floor of 05 5.1` |
| `RJ-345` | `projects-stray-file` | 2 | `REQ-0443` `sync` step 3 (4.2) on a file under `projects/` that is not `*.yaml`; `julieta spec validate` reports it as a finding of `RJ-328`; hint: `decide: move or remove <path>` |
| `RJ-346` | `config-project-missing` | 2 | `REQ-0444` `sync` step 3 (4.2) when no spec's primary repo URL equals `config.url` of host settings; hint: `decide: add the config repository's own spec under projects/, or fix config.url in settings.yaml` |
| `RJ-347` | `sandbox-stopped` | 2 | `REQ-0445` `pull` on a primary whose sandbox is stopped; the hint is `romeu run --no-attach <name>` |
| `RJ-348` | `internal` | 1 | `REQ-0446` `main` of either binary, for an error a command returns whose chain holds no `*cli.Error` (10 10.1, Error ids): a defect of the code; an error of the command-line parser is `usage` (`RJ-349`) instead; the hint is `decide: open an issue with the details` |
| `RJ-349` | `usage` | 2 | `REQ-1244` `cli.Main` of either binary, for every error the command-line parser returns: a command, subcommand or flag the definitions do not have, or an argument count a command refuses (4.1, Parsing); the hint is the `--help` line of the binary or of the command named |
| `RJ-350` | `illegal-transition` | 2 | every command that drives a state machine of [01 1.6](01-system-model.md#16-state-machines), for a `(from, event)` pair no row of its transition table has; the record is left as it was; the hint is `decide: run romeu status to read the state the record is in, and report the command that raised the event as an issue` |
| `RJ-351` | `mise-failed` | 1 | `REQ-1269` `julieta install` and `julieta lock`, for a mise that exits non-zero or is ended by a signal and is neither a refused download (`install-blocked`) nor past its timeout (`subprocess-timeout`); the details hold the argv, the status and the first 4 KiB of its output (4.1, Error ids, item 7); the hint is `decide: read the stderr in the details, fix what mise reports, then run the command again` |
| `RJ-352` | `root-unsafe` | 2 | `REQ-1278` `romeu init`, for a root `doctor`'s check `root-unsafe` fails (04 4.2, the root checks; I20), the message naming which rule; hint: `decide: pass another root with --root` |
| `RJ-353` | `settings-exist` | 2 | `REQ-1279` `romeu init`, when host settings already exist at the settings path, or appear there while init runs; init never overwrites them; hint: `decide: keep the settings that exist, or move them aside and run init again` |
| `RJ-354` | `settings-unwritable` | 2 | `REQ-1280` `romeu init`, when the settings directory or file cannot be made or written; the details hold the system's message; hint: `decide: make the settings directory writable, or point ROMEU_SETTINGS at a path that is, then run init again` |
| `RJ-355` | `toolchain-declined` | 1 | `romeu approve --toolchain` on a terminal, when the answer to gate 1 is anything but `y` ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)); `toolchain.json` is unchanged, so a command chained after it does not run; hint: `romeu approve --toolchain`. `RJ-352` to `RJ-354` are kept for the rows of `romeu init` decided on 2026-10-23 |
| `RJ-356` | `config-clone-failed` | 1 | `REQ-1286` `romeu init`, when git fails the clone of the config repo, or gitsafe refuses what the remote sent: a URL that names no repository, an access the origin refuses, or a private repository and no `--git-credential-helper`; the details hold `config.url` and git's output (4.1, Error ids, item 7): git itself prints no credential, but what a helper writes to its standard error is part of that output and is not filtered; nothing of the run is left; hint: `decide: check the URL and your access; a private config repo needs --git-credential-helper` |

### `fingerprint-unstable` (`RJ-338`)

1. `REQ-0447` The command salvages again under a new salvage id, and the
   `salvaging` rows of [01 1.6](01-system-model.md#generation) decide
   how that salvage ends.
2. `REQ-0448` When it completes, or `--accept-loss` names its skipped reasons, the
   command records its entry and `fingerprint` with the state
   `removing`, and then exits 5 with this error instead of running
   `sbx env rm`, so the generation stays `removing`.
3. `REQ-0449` When it is verified incomplete without them: `RJ-306`, exit 5, back
   to `open`.
4. `REQ-0450` When its sandbox half fails: exit 1, back to `open`, naming
   `--from-host` as the `salvage` row of 4.2 says.
5. `REQ-0451` When the sandbox is found absent: `closed-lost`.
6. `REQ-0452` A rerun whose fresh fingerprint equals the one written resumes at
   `sbx env rm`; `REQ-0453` one whose fresh fingerprint cannot be read exits 1 and
   changes nothing (01 1.6).
7. `REQ-0454` A salvage restarted from `salvaging`, left by a romeu that ended,
   killed or failed, while it salvaged, is the command's own salvage
   and not one a changed fingerprint started, so it goes on to
   `sbx env rm` when it is complete or accepted; a write made inside the
   sandbox after its read is the risk
   [05 5.4](05-security.md#risks-of-running-romeu) accepts.
8. `REQ-0455` The details name the kind of entry that changed, as 03 3.8 defines
   it.
9. `REQ-0456` The hint is one fixed text: "If nothing writes in the sandbox any
   more, rerun; the first rerun may exit 5 once more, and the one after
   it resumes at `sbx env rm`. If something still writes, stop it, or,
   for an ignored path, add it to `salvage.excludeIgnored` (gate 2);
   then rerun, with the same result. `--accept-loss` does not cover
   this error."
10. `REQ-0457` This error is not a skipped reason
    ([03 3.11](03-formats.md#311-salvage-manifest-schema-salvagev1)).
    The hint does not say that the `romeu sync` of its second route
    exits 2 with `removal-pending` once it has promoted the edit, when
    the sandbox is running (4.2, `sync` step 9); the first report that
    the exit confused an operator reopens the hint's text.
11. This list is the one home of the rule (decided on 2026-10-12);
    01 1.6, 03 3.8, 08 8.5 and I17 cite it.
