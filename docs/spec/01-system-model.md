# 1. System model

Back to [index](../spec.md). Reader: every implementer and reviewer;
read this page first. Type: explanation of the model, with reference
tables for the gates and state machines. Every journey in [09](09-journeys.md) is a
view of this model; a journey that needs something outside it means the
model is wrong.

## 1.1 Components (one job each)

| Component | Single responsibility | Runs where | Language |
|---|---|---|---|
| **romeu** | Turns reviewed specs into host state: materializes the tree, owns the gates, drives `sbx`, delivers julieta, verifies and imports salvage. Deterministic; never an agent; never executes repository content. | macOS host | Go |
| **julieta** | Does chores inside a sandbox: secondary clones, tools (mise), git hooks, memory, handoff, snapshot/salvage, run layout, pin bumps and checks, spec and catalog validation. Never widens what the sandbox can do. | sandbox (linux amd64/arm64); also the config repo's CI | Go |
| **product repo** (`romeu-e-julieta`, public) | Source of both binaries, product kits, egress catalog, skills, schemas, docs | GitHub | Go, YAML, Markdown |
| **config repo** (for example `my-config`) | `projects/<name>.yaml` specs and personal kits. Agents author it; merges are reviewed. It is also a project like any other (the config project). | GitHub; cloned into the host tree | YAML |
| **worked-on repos** | The code being developed; each may carry `mise.toml` + `mise.lock` | GitHub; plain clones in the host tree (never submodules). v1 supports GitHub-shaped URLs (one owner segment) on the hosts in `gitHosts`; repos, CI and releases assume GitHub ([03 3.1](03-formats.md#31-name-and-path-rules-shared)) | - |
| **egress catalog** | Maps `backend:tool` to domains, a fixed mise meta set, a base git set, and an upload flag per domain | embedded in romeu and julieta (`catalog/egress.yaml`) | YAML |
| **host tree** | `$ROMEU_ROOT`: per project the rendered env file, sidecar metadata, julieta binaries, host clones, review checkouts, memory dirs; two generated workspace files | host disk | - |
| **host settings** | Machine-local, operator-owned: root path, config repo, absolute tool paths, git hosts, workload repository allowlist, secret bindings `name@project -> argv`, signing agent socket | `$XDG_CONFIG_HOME/romeu/settings.yaml` | YAML |
| **host state** | Machine-local trust and lifecycle records: the toolchain acknowledgement and one record per project (approval, generations, applied egress), a descriptor cache, a lock | `$XDG_STATE_HOME/romeu/` | JSON |
| **sandbox** | One sbx microVM per project, named after the project, clone mode on the primary repo | sbx | - |
| **memory** | One directory per repo per project: memory entries, handoffs, snapshots and salvage payloads, in a fixed layout. Plain files, format owned by julieta, agent-neutral. | host disk, mounted read-write into its sandbox only | - |
| **kits** | v3 kit sources built locally by sbx at create: product kits (embedded in romeu) and personal kits (from the config repo) plus a workload pinned by digest | host tree `.romeu/kits/` | v3 descriptors, sh |
| **run layout** | Panes and command steps per project; rendered by julieta into herdr `layout.apply` | spec `run:` | YAML |

The runtime ledger of [13](13-runtime-ledger.md) is designed and not
built in v1; it waits for its row in
[Deferred decisions](../spec.md#in-the-product).

## 1.2 Sources of truth vs derived

| Class | Items | Who writes | Reviewed how |
|---|---|---|---|
| Truth, in git | config repo `projects/*.yaml`, `kits/*`; each repo's `mise.toml`/`mise.lock`; product kits, catalog, skills, `.github/ask-first.yaml`, the Go spec types with their tags and `internal/spec/rules.go` | agents (sandbox) and humans | PR review, and what that review is worth: [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1); romeu reads config and repo content only at a named commit |
| Truth, machine-local | host settings; host state | operator (settings); romeu (state) | never mounted into any sandbox |
| Generated, in git | every file of the inventory in [12 12.3](12-engineering.md#123-generators), the `docs/reference/*` pages that replace the field and rule tables of [03](03-formats.md) once each format's generator lands among them (03, opening) | `go generate ./...` | `tools/ci generated` fails on any diff; never hand-edited |
| Candidate (unapproved) | `<name>-env/.romeu/candidates/<candidate-id>/` (rendered files and materialized kits) | romeu `sync` | the gate; promoted only on approval (1.6) |
| Derived, live | `<name>-env/sbxenv.yaml`, `.romeu/render.json`, `.romeu/kits/*`, `.romeu/bin/*` | romeu (promotion) | drift-checked by hash before every overwrite and before every sbx call of a **P** command (1.5) |
| Derived from state | `$ROMEU_ROOT/dev.code-workspace`, `$ROMEU_ROOT/review.code-workspace` | romeu (each promotion, `retire` and `pull`) | written again whole from host state each time; no record holds their hash, so they are not drift-checked; `doctor` derives them again from host state and reports a difference, and the next promotion, `retire` or `pull` overwrites a hand edit |
| Derived, in sandbox | secondary clones, installed tools, julieta manifest cache, herdr layout, the `julieta` link on `PATH` | julieta | disposable; recreated by `julieta setup` |
| sbx-owned facts romeu reads | sandbox existence, state and workspace path (`sbx ls --json`); the `remote.sandbox-<name>.*` keys sbx writes into the primary host clone's `.git/config`, of which romeu reads the URL; sbx version | sbx | read only |
| Agent output that reaches the host | memory dir files, `snapshot/heads.json` among them; git objects via fetch or bundle; julieta's `--json` stdout read over `sbx env exec` (`version`, `setup`, `status`, `salvage`) | agents / julieta | data only: validated, escaped for display, stored in romeu namespaces; never executed, never checked out outside a hardened review checkout |
| Host facts the agent reads | the project's live spec commit: `specCommit` in the manifest ([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)), fresh at each exec | romeu | the manifest schema. The candidate state is host-only: the agent compares `specCommit` with the commit it pushed to learn whether its spec change is live |

*Why for us (drift rule):* chezmoi refuses to overwrite a target that
changed since it last wrote it; we keep that because a hand edit of
`sbxenv.yaml` is exactly the unreviewed widening the gate exists to stop.

### Where data lives

Every store the product writes or causes to exist, in one place; the
layouts are in [02 2.3](02-layouts.md#23-host-tree) and
[02 2.4](02-layouts.md#24-host-settings-and-state).

| Store | Path | Whose data | Who reads it | Lifetime | `rm` / `retire` |
|---|---|---|---|---|---|
| project tree | `$ROMEU_ROOT/<name>-env/` (`sbxenv.yaml`, `.romeu/`) | the operator's | the host user; the sandbox sees `.romeu/bin` read-only | until `retire` | kept / moved to `.attic` |
| host clones | `<name>-env/<dir>/` | the repository owner's; origin content agents can push | the host user, in Restricted Mode; sbx clones the primary | until `retire`; never moved, re-cloned or deleted by `sync`, `run` or `rm` | kept / moved |
| refs in romeu namespaces | `refs/romeu/*` and `refs/sandboxes/<name>/*` in each host clone | the operator's; salvage commits can hold untracked files | the host user | the clone's: deleting or re-cloning it loses them | kept / moved |
| review checkouts | `<name>-env/review/<dir>/` | sandbox work | the host user, in Restricted Mode | until `retire` | kept / moved |
| memory dirs | `<name>-env/memory/<dir>/` (entries, handoffs, snapshots, salvage payloads) | agent-written | the host user; the project's sandbox, read-write | until `retire` | kept / moved |
| workspace files | `$ROMEU_ROOT/dev.code-workspace`, `review.code-workspace` | derived | the host user, in VS Code | rewritten from host state | rewritten |
| `.attic` | `$ROMEU_ROOT/.attic/<name>/<UTC-ts>/` | the operator's | the host user | until the owner deletes it (J7 step 4) | - / created |
| host settings | `$XDG_CONFIG_HOME/romeu/settings.yaml` | the operator's | romeu; never mounted | until the operator deletes it | kept / kept |
| host state | `$XDG_STATE_HOME/romeu/` (gate 1 record, project records, descriptor cache, lock) | romeu's records | romeu; never mounted | the machine's | generation closed / record moved to the state attic |
| state attic | `$XDG_STATE_HOME/romeu/attic/<name>/<UTC-ts>/` | the operator's | nobody; romeu writes it once | until the owner deletes it (J7 step 4) | - / created |
| sbx sandbox and its per-name volumes | sbx's storage | agent work inside the sandbox | sbx and the sandbox | one generation | removed after salvage; volumes as probe A12 records / the same |
| romeu-applied egress rules | sbx policy, per sandbox | the operator's approvals | sbx | while the generation is open | removed / removed |
| keychain entries | named by each secret argv in host settings | the operator's | the secret argv on the host; sbx injects the value | the operator's | kept / kept |
| signing agent socket | `signing.agentSocket` | the operator's signing key | sbx forwards it into a `git-ssh-sign` sandbox | the operator's | kept / kept |

The table is written by hand; generating it is a row of
[Deferred decisions](../spec.md#in-the-product).

## 1.3 Trust boundaries

| Boundary | Rule | Invariants |
|---|---|---|
| **A. sandbox -> host** | Anything an agent wrote reaches the host only as data. romeu never runs mise, hooks, filters, tasks or scripts from repo content; never checks out agent-sourced content except into a hardened review checkout; escapes every agent-originated string before printing it | I1-I6, I12, I17, I24, I27, I28, I29 |
| **B. spec -> host commands** | A spec names secrets; the command resolving a name lives in host settings, keyed `name@project`. romeu never handles a secret value. | I8, I25 |
| **C. spec -> sandbox capability** | Every widening goes through the per-project gate; toolchain changes go through the per-machine acknowledgement. Nothing unapproved is ever at a live path, except an adopted generation, which is accepted as it stands until it is recreated (1.6). julieta is delivered read-only into a sandbox and checked for compatibility there ([06 6.3](06-kits.md#63-julieta-delivery)); it also runs in the config repo's CI from a release pinned by version and checked by `julieta pin check --workflows` ([02 2.2](02-layouts.md#22-config-repo)), where no romeu exists. | I7, I15, I28, I30 |
| **D. host UI -> agent trees** | No host tool romeu configures is pointed at an agent-writable tree. Workspace files list host clones and review checkouts only, both to be opened in Restricted Mode (host clones carry origin content that agents can push); memory dirs are never workspace folders. Host indexers are kept off by a `.metadata_never_index` marker `sync` writes in each `<name>-env/` ([02 2.3](02-layouts.md#23-host-tree)), whose effect probe A17 measures; `doctor` warns when the marker is missing or `$ROMEU_ROOT` is in a Time Machine-included or cloud-synced path, and what an indexer still reaches is a residual risk of [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1). Only VS Code's trust settings are checked ([04 4.2](04-cli.md#checks-run-by-romeu-doctor)); another editor's trust model is the operator's to verify. | I13, I14, I23 |
| **E. sandbox -> network** | Egress = catalog-derived domains from pinned commits plus gated extras; upload-capable domains are always gated. Defense in depth, not containment. | I9, I10, I22 |
| **F. sandbox identity** | romeu acts only on sandboxes with an open generation (1.7) it recorded (by create or `adopt`), whose workspace path matches the project's primary clone | I19, I26 |

## 1.4 The gates

There are two romeu-level approvals, both TTY-only, plus sbx's own
native approval of host commands, which romeu never bypasses (never
`--auto-approve`; `romeu doctor` recommends
`sbx settings set env.rememberHostCommands true`). `sbx env plan` output
is not part of romeu's gates: sbx shows its own plan and prompts during
`sbx env run`. *Why for us:* sbx already re-asks when a host command
changes; romeu gates only what sbx cannot see.

### Gate 1: toolchain acknowledgement (per machine)

Record `toolchain.json` ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)):
the sbx version, romeu's version and sha256, the embedded julieta's
version, protocol and sha256, and the acknowledged catalog itself. Approved once per change with `romeu approve --toolchain`, which
prints, before the prompt, the julieta version change, the catalog diff
(domains added or removed per `backend:tool`, upload flags changed) with
the projects each change affects, the live sbx version beside the
newest recorded one, and `downgrade` beside any version lower than the
recorded one. Every command that invokes sbx compares the live
`sbx version`, its own version and sha256 and its embedded catalog
digest to the record and exits 3 on a mismatch, except
`romeu approve --toolchain`, which is gate 1 itself, `romeu status` and
`romeu doctor`, which report the mismatch, and the protective commands
of 1.5, which print it and go on
([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)). This
keeps an sbx or romeu upgrade from turning into a gate on every
project.

The tested window is the set of sbx versions with a recording set under
`e2e/testdata/sbx/`. A live version outside it is reported by `status`
and `doctor` as the warning `sbx-untested`, naming the version and the
recorded range; **P** commands print it and go on. An sbx output
romeu cannot parse, or one that lacks a field romeu reads, is
`upstream-shape` (exit 2) by the upstream rule of [03](03-formats.md)
(opening), and its details name the recorded versions. A dev build
carries the version `v0.0.0-dev+<commit>` and is acknowledged like any
other version, once per build per machine; when julieta and the catalog
are unchanged the prompt is a single line.

*Why for us:* sbx updates itself, and romeu or sbx can be replaced by a
package manager run or by another process or person on the machine; a
catalog or julieta change that arrives that way can widen egress for
every project. Gate 1 makes it visible before any project uses it;
`doctor` reports, and does not stop.

### Gate 2: project widening (per project)

The widening set is canonical JSON digested with `canon.Digest("widening", set)`
([03 3.12](03-formats.md#312-digests-internalcanon)).

| Field | Content |
|---|---|
| `project` | project name |
| `repos` | sorted `{dir, url, primary, ref}` |
| `secrets` | sorted `{name, service, argvDigest}` (argv from host settings) |
| `workload` | `{repository, digest, capabilities}`; `capabilities` = the normalized capability set read from the workload descriptor (below) |
| `kits` | ordered `{id, source: product|personal, contentDigest, args, capabilities}` |
| `egressGated` | sorted gated domains, each with its reason (upload, non-catalog lock host, spec extra, spec tool override, kit-declared) |
| `ports` | sorted port specs |
| `agent` | the agent handle |
| `salvageExcludes` | `salvage.excludeIgnored` (it defines acknowledged loss) |
| `renderDigest` | `canon.Digest("render", r)` where `r` is the rendered `sbxenv.yaml` decoded and stripped of every key whose spec field carries no `gate:"widening"` tag (today `sandboxOptions`) |

Which spec fields are gated and which apply only at create is declared
once, as Go struct tags on the spec types (`gate:"widening"`,
`apply:"recreate|live"`); `go generate` derives the widening set, the
recreate digest, the I7 flip-table test and the field table of
[03 3.2](03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1),
which `docs/reference/project.md` replaces once its generator lands.
Fields deliberately left ungated are listed in one committed list
beside the tags; `go generate` fails on a field that is on neither, so
a forgotten tag is a generator failure, not a silent strip (I7). Today
the list holds `sandboxOptions` and `run`. *Why for us:* resource limits
change how much of the host the sandbox uses, not what it can reach or
read, and the run layout's pane commands run only inside the sandbox.

Normalized capability set (from v3 descriptors, parsed by one grammar
in `internal/oci`, which `internal/render` calls for local kits): network
domains per phase, mounts and volumes, credentials/secrets, ssh-agent
use, privileges (user, capabilities), lifecycle hooks. Descriptors are
parsed with the strict subset grammar of
[03](03-formats.md) (opening), and any key or capability type the model
does not know is a sync error (fail closed); a capability type sbx adds
after v1 is a row of [Deferred decisions](../spec.md#in-the-product). Kit-declared network domains go through `egress.Split`,
so upload-capable ones appear in `egressGated`. A personal kit that
declares a network, mount, volume, credential or ssh-agent capability
is refused (sync error). Its other capabilities (install steps,
privileges, lifecycle hooks) are not refused: they are shown in the
gate 2 diff.

The gate diff is never truncated. It starts with a summary that lists
first, marked, the changed capabilities, gated domains, secret argv and
the workload repository, then content-only kit changes with file and
line counts (a golden holds that order); then it shows every changed
field, a file-level diff of kit sources (added, removed, changed paths
with modes; full text diffs; a binary or non-UTF-8 file as size and
sha256, flagged `binary`), the workload repository highlighted if it
changed, and the host-settings argv behind each secret name; the summary
is repeated right above the prompt, so a long diff cannot push it out of
view. All text passes through `termsafe`.

Approvals are per machine by design. *Why for us:* Codespaces
authorizes extra repo access at create, per user; ours is per machine
because secrets resolve on that machine.

## 1.5 Preflight before every mutating sbx call

Every romeu command marked **P** in [04](04-cli.md#42-romeu-host) runs,
in this order, before any mutating sbx call, and stops at the first
failure. Steps 1, 5 and 6 read from sbx (`sbx version`, its settings
and policies, `sbx ls --json`) and change nothing; every sbx output they
read is decoded by the upstream rule of [03](03-formats.md) (opening).
`sync` runs step 1 before its first sbx read and steps 2 to 6 before its
first mutating call.

| Step | Check | Exit |
|---|---|---|
| 1 | toolchain (gate 1) | 3 |
| 2 | an interrupted promotion is finished (1.6) or reported | 4 |
| 3 | drift of every live derived file against `render.json` | 4 |
| 4 | widening digest recomputed from the live files equals the approval (gate 2) | 3 |
| 5 | host checks shared with `doctor` whose failure widens a sandbox or makes agent output executable on the host: sbx global secrets and broad allow rules (I22), host tools that auto-trust `$ROMEU_ROOT` (I23), and, for a project using `git-ssh-sign`, the signing socket rule of [06 6.4](06-kits.md#64-product-kits). A listing whose shape is not a recorded shape, or that cannot be parsed, fails the step with `upstream-shape` ([03](03-formats.md), opening) | 2 |
| 6 | sandbox identity (I26): an absent sandbox passes; a present one must have an open generation (1.7) romeu recorded (`RJ-203 unknown-sandbox` otherwise) and the workspace path `$ROMEU_ROOT/<name>-env/<primary>`; a present sandbox whose generation is `removing` passes for `rm`, `recreate` and `retire`, and any other mutating command, `salvage --from-host` included, exits with `removal-pending` (`RJ-336`) | 2 |

The preflight never touches the network: descriptors come from the
content-addressed cache in host state, filled only by `sync`.

The protective commands `salvage`, `salvage --from-host`, `stop` and
`pull` widen nothing and forward no socket: they print a failure of
steps 1, 4 or 5 and go on, and stop at step 2, 3 or 6. `rm`, `retire`
and `recreate` run their salvage half the same way and pass every step
before their removal half. *Why for us:* a check that cannot widen a
sandbox must not keep the operator from saving work; drift and identity
still stop every command, because acting on an unknown file or sandbox
is the harm itself.

## 1.6 State machines

The candidate and generation lifecycles live in `internal/state` as
transition tables (from, event, to, effect); an event with no row is an
illegal transition and exits 2 with its error id. Tests and diagrams are
generated from the tables: every row is exercised and every missing
pair is asserted illegal. Each row's Effect is asserted by a
hand-written case (the file deleted, the record cleared, the exit
code). The promotion commit and the retire move are ordered step lists
with a recovery rule per step, tested by fault injection
([10 10.1](10-testing-style.md#101-test-levels)); the probe lifecycle
is the table of [11 11.4](11-host-probes.md#114-probe-lifecycle).

The generation states persist in the project record
`projects/<name>.json`
([03 3.8](03-formats.md#38-host-state-schemas-state-v1)). The candidate
states are transient within one command: a command killed in one of
them leaves a candidate dir that the next `sync` deletes and renders
again, so an approval given before step 1 of the promotion commit is
given again on that `sync`. Every sbx output a transition reads is
decoded by the upstream rule of [03](03-formats.md) (opening), so a
shape romeu does not recognize is never read as an absent sandbox.

### Candidate

At most one candidate per project exists at any time.

| From | Event | To | Effect |
|---|---|---|---|
| (none) | `sync` renders | rendered | candidate dir written under `.romeu/candidates/<candidate-id>/` |
| rendered | widening digest equals the approval | promoted | promotion commit (below) |
| rendered | digest differs, TTY, `y` | approved | approval recorded |
| rendered | digest differs, TTY, `N` | declined | candidate dir deleted |
| rendered | digest differs, no TTY | declined | candidate dir deleted; the widening digest printed; exit 3 |
| approved | promotion commit completes | promoted | candidate dir deleted |

A candidate dir left by a killed command is deleted by the next `sync`.
Approving a candidate rendered without a terminal is a row of
[Deferred decisions](../spec.md#in-the-product).

### Promotion commit

Promotion is crash-safe without a rollback copy: nothing gate 2
covers is modified before the commit point. `.romeu/bin` is not in
gate 2 (it is a fixed mount whose content may change, 06 6.3); step 2
rewrites it, under a running sandbox too, and a crash there is rolled
forward by the recovery below.

1. Write `render.json` (atomic rename) with
   `promoting: <candidateId>` and the previous file digests.
2. Copy kits into content-addressed, immutable dirs
   `.romeu/kits/<id>-<digest12>/` (a dir that already exists is verified,
   never rewritten); write `.romeu/bin/*` and the workspace files.
3. Rename `sbxenv.yaml` into place: **the commit point**.
4. Rewrite `render.json` without `promoting`; delete the candidate dir
   and every kit dir no longer referenced.

Recovery runs after taking the lock in every mutating command: a
`render.json` with `promoting` whose candidate dir still verifies
against its `filesDigest` is finished by repeating steps 2-4 (each is
idempotent). If the candidate no longer verifies, romeu writes
`render.json` back with the previous digests, plus the digests of the
files step 2 wrote, and reports `interrupted-promotion-abandoned`
(exit 4), never drift; `romeu sync` renders a fresh candidate. The old
live `sbxenv.yaml` and its kit dirs are intact, so nothing unapproved
is ever live. `status` reports an interrupted promotion without the
lock.

### Generation

A generation is one sandbox lifetime, identified by a ULID.

| From | Event | To | Effect |
|---|---|---|---|
| (none) | `run` finds the sandbox absent and is about to create it | open | record id, recreate digest, workspace path, the repo set, written before `sbx env run` |
| open | the `sbx env run` of that same command returns non-zero, and `sbx ls --json` then shows no sandbox | (none) | the record just written is removed; exit 1 |
| open | the `sbx env run` of that same command returns non-zero, and `sbx ls --json` then shows the sandbox | open | the record is kept; exit 1 |
| (none) | `adopt` on TTY, workspace path matches | open | record as above with `adopted: true` and an unknown recreate digest; the generation is accepted as it stands until it is recreated, and `status` shows `adopted, not gated` |
| open | `salvage`, `rm`, `recreate` or `retire` starts the sandbox half | salvaging | new salvage id |
| salvaging | `salvage`, `rm`, `recreate` or `retire` starts the sandbox half again (the earlier command was interrupted or failed) | salvaging | new salvage id |
| salvaging | the sandbox half fails (julieta exits 1, a full disk) or its exec is interrupted | open | salvage record `result: incomplete`, `reasons: [sandbox-half-failed]`; exit 1 |
| open | `salvage --from-host`, the sandbox absent | open | refs and snapshot bundles imported under a new salvage id; salvage record `result: lost`, `reasons: [sandbox-lost]` |
| salvaging | `salvage --from-host`, the sandbox absent | salvaging | as the open row |
| salvaging | salvage verified; command was `salvage` | open | salvage record `result: complete` |
| salvaging | salvage incomplete, no `--accept-loss` | open | salvage record `result: incomplete` with reasons; exit 5 |
| salvaging | salvage complete or `--accept-loss`; command removes the sandbox | removing | state written, with the salvage entry's `fingerprint` ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)), before `sbx env rm` |
| removing | `sbx env rm` succeeds | closed-removed | egress rules removed |
| removing | `sbx env rm` fails, or the sandbox found present (a command killed before `sbx env rm`) | removing | the existing salvage record kept; exit 1; a rerun of `rm`, `recreate` or `retire` resumes at `sbx env rm` only when a fresh fingerprint of the daemon heads and each worktree's HEAD and full status equals the `fingerprint` of the state's salvage entry ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)); otherwise `removing -> salvaging` with a new salvage id |
| removing | the sandbox found absent (crash recovery) | closed-removed | the existing salvage record kept; egress rules removed |
| open | `run`, `rm`, `stop`, `pull`, `salvage` without `--from-host`, or `retire` finds the sandbox absent | closed-lost | keeps a lost record `salvage --from-host` wrote for this generation, else writes one: refs and snapshots preserved as a salvage record `result: lost` |
| salvaging | the sandbox found absent (crash recovery) | closed-lost | as above |

Absent means `sbx ls --json` exited 0, parsed, and did not list the
sandbox. A non-zero exit is `sbx-unknown` (exit 1), and an output that
does not parse is `upstream-shape` (exit 2) by the upstream rule of
[03](03-formats.md) (opening); either changes nothing. A `run` that
closes an open generation as lost does not create in the same command:
it prints the lost record and exits 1, and the next `run` creates.

A romeu killed between the first row's record and the end of
`sbx env run` leaves an open generation: the next `run` finds the
sandbox absent and closes it as lost, or finds it present and goes on.
`salvage --from-host` refuses when `sbx ls --json` shows the sandbox
present (exit 2, fix hint `romeu salvage`) and does not close a
generation; closing stays with the row that finds the sandbox absent.

Destructive commands write in this order: the refs per repo
(create-only, so a rerun verifies them), the salvage record, the state
`removing` with the salvage entry's `fingerprint`, `sbx env rm`,
`closed-removed`. Recovery: `removing` with the sandbox absent becomes
`closed-removed`, and with it present the next `rm` resumes at
`sbx env rm` when a fresh fingerprint of the daemon heads and each
worktree's HEAD and full status equals that `fingerprint`, and
otherwise salvages again under a new salvage id; any other mutating
command exits 2 with `removal-pending`, and `status` and `doctor`
report it as `removal-pending`
([04 4.4](04-cli.md#44-error-ids)); `salvaging` with no
salvage record is resumed by the next salvage, which verifies the refs
already created.

Salvage always uses the generation's recorded repo set, not the current
spec, so a repo removed from the spec (J13) is still salvaged. Whether
a generation has a final handoff is computed when `rm` runs, by the
handoff reader ([08 8.3](08-memory-handoff-salvage.md#83-handoff)), not
stored. `status` is a projection of these records.

### Retire

`retire` runs only with no sandbox (after its `rm` half) and changes two
places, the tree under `$ROMEU_ROOT` and the record under
`$XDG_STATE_HOME`, in this order:

1. Write `retiring` (with the attic timestamp) to the project record.
2. Move `<name>-env/` to `$ROMEU_ROOT/.attic/<name>/<UTC-ts>/`.
3. Move the record to `$XDG_STATE_HOME/romeu/attic/<name>/<UTC-ts>/`.

Any command that finds a record with `retiring` finishes the remaining
steps before anything else, and `sync` never recreates a tree for a
retiring record.

## 1.7 Vocabulary

One word per concept. `go generate` writes this table's "Not" column
to `docs/reference/vocabulary.yaml`, outside the `checks` globs of
`.github/ask-first.yaml`, and `tools/ci vocabulary` reads that file and
fails when a phrase from it appears in docs (except `docs/reviews/` and
the "Not" column of this table), help text or error messages. A
generated glossary page is a row of
[Deferred decisions](../spec.md#in-the-product).

| Word | Meaning | Not |
|---|---|---|
| operator | the person who runs romeu on a host, owns the host settings and answers the gates; today the same person as the maintainer | - |
| maintainer | the person who merges and releases the product repository; today the same person as the operator | - |
| developer | the operator as a user of the product ([00 0.2](00-scope.md#02-users)) | - |
| this specification | the document under `docs/spec.md` and `docs/spec/` | this spec |
| project | A named unit: 1..n repos, one sandbox, one `<name>-env/` directory. `name` is stable and unique. | - |
| primary repo | The repo sbx clones into the sandbox (clone mode); exactly one per project | - |
| secondary repo | Any other repo of the project; cloned inside the sandbox by julieta from origin | - |
| spec | A project's YAML in the config repo | project config |
| config repo | The repo holding specs and personal kits; the only meaning of "config" on its own | - |
| config project | The project whose primary repo is the config repo | - |
| sandbox | an sbx sandbox romeu created or adopted for a project | - |
| development sandbox | the sbx sandbox this repository is developed in, provisioned outside romeu with the operator's account and keys; [ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md) and the token rows of [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1) mean this | - |
| host settings | The operator's machine-local `settings.yaml` | host config |
| personal kit | A kit from the config repo's `kits/` (`personal:` in a spec) | config kit |
| product kit | A kit from the product repo's `kits/`, embedded in romeu (`product:` in a spec); sbx's field value `kind: mixin` stays in code font | mixin |
| run layout | A spec's `run:` section: tabs, panes and steps | run-config |
| sync | spec at a named commit -> candidate -> gate -> live tree | - |
| approve | gate 1 on its own (`romeu approve --toolchain`); gate 2 is answered inside `sync` | - |
| candidate | an unapproved render under `.romeu/candidates/` | pending candidate |
| promotion | the ordered steps that make an approved candidate live (1.6) | - |
| interrupted promotion | a `render.json` that still carries `promoting` (1.6) | - |
| drift | a live derived file whose hash differs from `render.json` (1.2) | - |
| preflight | the ordered checks of 1.5 before a mutating sbx call | - |
| run | preflight, ensure sandbox, egress and tools, then attach to the run layout | - |
| generation | one sandbox lifetime, identified by a ULID recorded by romeu | - |
| open generation | a generation in state `open` or `salvaging`; `removing` counts as open only for `rm`, `recreate` and `retire`; any mutating command but `rm`, `recreate` and `retire` that finds a `removing` generation with its sandbox present exits 2 with `removal-pending` ([04 4.4](04-cli.md#44-error-ids)); `status` and `doctor` report it as `removal-pending` | - |
| adopt | record an existing sandbox of the right name and workspace as a generation | - |
| snapshot | julieta's bundle of unpushed work into the memory dir's `snapshot/` after each commit | live salvage |
| salvage | stop agents, capture everything sandbox-only into the memory dir's `salvage/`, verify and import it on the host | - |
| rm | salvage, then remove the sandbox; the tree stays | - |
| recreate | rm, then run | - |
| retire | the inverse of sync: rm, then move `<name>-env/` to `$ROMEU_ROOT/.attic/`; never deletes. What it moves is the owner's to delete by hand (J7 step 4) | - |
| orphaned | a project or repo dir in the host tree that no spec names (J7, J13) | - |
| handoff | agent narrative (`clear`, `final`) or julieta-stamped git facts (`facts`), a plain file in the memory dir | - |
| lesson | in a user's project, a memory entry tagged `lesson` ([08 8.1](08-memory-handoff-salvage.md#81-memory-store)), shown at SessionStart; in this repository, a file under `docs/lessons/` ([12 12.7](12-engineering.md#127-text-standard-and-lessons)), read by contributors and review. The two share the word only | - |
| widening set | gate 2's digested content | - |
| recreate-class | fields tagged `apply:"recreate"`: repos, kits, workload, ports, sandbox options, agent, and (until probe A10 says otherwise) secrets | - |
| recreate needed | the live sandbox's recreate digest differs from the approved one; `status` reports it, and `run` raises `recreate-needed` ([04 4.2](04-cli.md#how-romeu-run-reaches-the-run-layout), step 3) | stale (for the recreate digest) |
| review checkout | a hardened checkout of sandbox work under `<name>-env/review/<dir>/` | - |
| sandbox probe | a block B check that the probe harness runs inside a sandbox, ids C2..C6 (C1 is retired, 11 11.2) | - |
| runtime ledger | the per-machine, add-only store of runtime events under host state ([13](13-runtime-ledger.md), designed, not built in v1); "the ledger" on its own means this | - |
| spool | a project's agent-writable directory where julieta writes events until romeu ingests them | - |
| ingest | romeu reads one project's spool and creates ledger entries | - |
| ledger view | a project's read-only directory that romeu derives from the runtime ledger | - |
| pin freshness | whether each tool pin of [12 12.1](12-engineering.md#121-tech-stack) is the newest published version of its tool; `tools/ci pins` reports it. Not the freshness of a `mise.lock`, which `julieta lock --check` verifies against `mise.toml` (07 7.3) | - |
