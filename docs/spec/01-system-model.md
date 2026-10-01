# 1. System model

Back to [index](../spec.md). Reader: every implementer and reviewer;
read this page first. Type: explanation of the model, with reference
tables for the gates and state machines. Every journey in [09](09-journeys.md) is a
view of this model; a journey that needs something outside it means the
model is wrong.

## 1.1 Components (one job each)

| Component | Single responsibility | Runs where | Language |
|---|---|---|---|
| **romeu** | Turns reviewed specs into host state: materializes the tree, owns the gates, drives `sbx`, delivers julieta, verifies and imports salvage, ingests the runtime ledger. Deterministic; never an agent; never executes repository content. | macOS host | Go |
| **julieta** | Does chores inside a sandbox: secondary clones, tools (mise), git hooks, memory, handoff, snapshot/salvage, runtime events, run layout, pin bumps and checks, spec and catalog validation. Never widens what the sandbox can do. | sandbox (linux amd64/arm64); also the config repo's CI | Go |
| **product repo** (`romeu-e-julieta`, public) | Source of both binaries, product kits, egress catalog, skills, schemas, docs | GitHub | Go, YAML, Markdown |
| **config repo** (reference instance `verona`) | `projects/<name>.yaml` specs and personal kits. Agents author it; merges are reviewed. It is also a project like any other (the config project). | GitHub; cloned into the host tree | YAML |
| **worked-on repos** | The code being developed; each may carry `mise.toml` + `mise.lock` | GitHub; plain clones in the host tree (never submodules) | - |
| **egress catalog** | Maps `backend:tool` to domains, a fixed mise meta set, a base git set, and an upload flag per domain | embedded in romeu and julieta (`catalog/egress.yaml`) | YAML |
| **host tree** | `$ROMEU_ROOT`: per project the rendered env file, sidecar metadata, julieta binaries, host clones, review checkouts, memory dirs; two generated workspace files | host disk | - |
| **host settings** | Machine-local, operator-owned: root path, config repo, absolute tool paths, git hosts, workload repository allowlist, secret bindings `name@project -> argv`, signing agent socket | `$XDG_CONFIG_HOME/romeu/settings.yaml` | YAML |
| **host state** | Machine-local trust and lifecycle records: the toolchain acknowledgement and one record per project (approval, awaiting candidate, generations, applied egress), a descriptor cache, a lock | `$XDG_STATE_HOME/romeu/` | JSON |
| **sandbox** | One sbx microVM per project, named after the project, clone mode on the primary repo | sbx | - |
| **memory** | One directory per repo per project: memory entries, handoffs, snapshots and salvage payloads, in a fixed layout. Plain files, format owned by julieta, agent-neutral. | host disk, mounted read-write into its sandbox only | - |
| **runtime ledger** | The per-machine, add-only record of runtime events: julieta writes events to a per-project spool, romeu ingests them into the ledger and derives a per-project view ([13](13-runtime-ledger.md)) | spool and view: host tree, mounted into their own sandbox only (read-write and read-only); ledger: host state, never mounted | JSON |
| **kits** | v3 kit sources built locally by sbx at create: product mixins (embedded in romeu) and personal kits (from the config repo) plus a workload pinned by digest | host tree `.romeu/kits/` | v3 descriptors, sh |
| **run layout** | Multiplexer-neutral panes and command steps per project; rendered by julieta into herdr `layout.apply` | spec `run:` | YAML |

## 1.2 Sources of truth vs derived

| Class | Items | Who writes | Reviewed how |
|---|---|---|---|
| Truth, in git | config repo `projects/*.yaml`, `kits/*`; each repo's `mise.toml`/`mise.lock`; product kits, catalog, skills, `.github/ask-first.yaml`, the Go spec types with their tags and `internal/spec/rules.go` | agents (sandbox) and humans | PR review; romeu reads config and repo content only at a named commit |
| Truth, machine-local | host settings; host state, the runtime ledger included | operator (settings); romeu (state) | never mounted into any sandbox |
| Generated, in git | `schemas/*.json`, `docs/reference/*`, `.github/CODEOWNERS`, the field and rule tables of this spec's implementation | `go generate ./...` | `tools/ci generated` fails on any diff; never hand-edited |
| Candidate (unapproved) | `<name>-env/.romeu/candidates/<candidate-id>/` (rendered files and materialized kits) | romeu `sync` | the gate; promoted only on approval (1.6) |
| Derived, live | `<name>-env/sbxenv.yaml`, `.romeu/render.json`, `.romeu/kits/*`, `.romeu/bin/*` | romeu (promotion) | drift-checked by hash before every overwrite and before every sbx call of a **P** command (1.5) |
| Derived from state | `$ROMEU_ROOT/dev.code-workspace`, `$ROMEU_ROOT/review.code-workspace` | romeu (each promotion and `retire`) | written again whole from host state each time; no record holds their hash, so they are not drift-checked and a hand edit is overwritten |
| Derived from the ledger | `<name>-env/ledger/view/*` | romeu (ingest) | `doctor` derives it again and compares byte for byte ([13 13.6](13-runtime-ledger.md#136-view)) |
| Derived, in sandbox | secondary clones, installed tools, julieta manifest cache, herdr layout, the `julieta` link on `PATH` | julieta | disposable; recreated by `julieta setup` |
| sbx-owned facts romeu reads | sandbox existence, state and workspace path (`sbx ls --json`); the `remote.sandbox-<name>.*` keys sbx writes into the primary host clone's `.git/config`, of which romeu reads the URL; sbx version | sbx | read only |
| Agent output that reaches the host | memory dir files; git objects via fetch or bundle; spool files of the runtime ledger | agents / julieta | data only: validated, escaped for display, stored in romeu namespaces or as ledger entries; never executed, never checked out outside a hardened review checkout |

*Why for us (drift rule):* chezmoi refuses to overwrite a target that
changed since it last wrote it; we keep that because a hand edit of
`sbxenv.yaml` is exactly the unreviewed widening the gate exists to stop.

## 1.3 Trust boundaries

| Boundary | Rule | Invariants |
|---|---|---|
| **A. sandbox -> host** | Anything an agent wrote reaches the host only as data. romeu never runs mise, hooks, filters, tasks or scripts from repo content; never checks out agent-sourced content except into a hardened review checkout; escapes every agent-originated string before printing it | I1-I6, I24, I27, I29, I31-I33 |
| **B. spec -> host commands** | A spec names secrets; the command resolving a name lives in host settings, keyed `name@project`. romeu never handles a secret value. | I8, I25 |
| **C. spec -> sandbox capability** | Every widening goes through the per-project gate; toolchain changes go through the per-machine acknowledgement. Nothing unapproved is ever at a live path. julieta runs only as delivered. | I7, I15, I28, I30 |
| **D. host UI -> agent trees** | No host tool is pointed at an agent-writable tree. Workspace files list host clones and review checkouts only, both to be opened in Restricted Mode (host clones carry origin content that agents can push); memory dirs are never workspace folders. | I13, I14, I23 |
| **E. sandbox -> network** | Egress = catalog-derived domains from pinned commits plus gated extras; upload-capable domains are always gated. Defense in depth, not containment. | I9, I10, I22 |
| **F. sandbox identity** | romeu acts only on sandboxes with an open generation it recorded (by create or `adopt`), whose workspace path matches the project's primary clone | I19, I26 |

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
the sbx version, romeu version, julieta version and protocol, and the
acknowledged catalog itself. Approved once per change with
`romeu approve --toolchain`, which prints, before the prompt, the julieta
version change and the catalog diff (domains added or removed per
`backend:tool`, upload flags changed) with the projects each change
affects. Every command that invokes sbx compares the live
`sbx version`, its own version and its embedded catalog digest to the
record and exits 3 on a mismatch; `romeu status` and `romeu doctor`
report the mismatch instead of exiting 3. This keeps an sbx or romeu upgrade from turning
into a gate on every project.

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
recreate digest, the I7 flip-table test and the field table in
[03 3.2](03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1).

Normalized capability set (from v3 descriptors, parsed by `internal/oci`
for images and by `internal/render` for local kits): network domains per
phase, mounts and volumes, credentials/secrets, ssh-agent use, privileges
(user, capabilities), lifecycle hooks. Descriptors are parsed with a
strict subset grammar: no YAML anchors, aliases, merge keys or includes,
and any key or capability type the model does not know is a sync error
(fail closed). Kit-declared network domains go through `egress.Split`,
so upload-capable ones appear in `egressGated`. A personal kit that
declares a network, mount, volume, credential or ssh-agent capability
is refused (sync error). Its other capabilities (install steps,
privileges, lifecycle hooks) are not refused: they are shown in the
gate 2 diff.

The gate diff is never truncated. It starts with a summary (each changed
field, file counts and line counts per kit), then shows every changed
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
and policies, `sbx ls --json`) and change nothing:

| Step | Check | Exit |
|---|---|---|
| 1 | toolchain (gate 1) | 3 |
| 2 | an interrupted promotion is finished (1.6) or reported | 4 |
| 3 | drift of every live derived file against `render.json` | 4 |
| 4 | widening digest recomputed from the live files equals the approval (gate 2) | 3 |
| 5 | host checks shared with `doctor` that can widen a sandbox behind romeu's back: sbx global secrets and broad allow rules (I22), host tools that auto-trust `$ROMEU_ROOT` (I23), and, for a project using `git-ssh-sign`, the signing socket rule of [06 6.4](06-kits.md#64-product-kits) | 2 |
| 6 | sandbox identity (I26): an absent sandbox passes; a present one must have an open generation romeu recorded (`RJ-203 unknown-sandbox` otherwise) and the workspace path `$ROMEU_ROOT/<name>-env/<primary>` | 2 |

The preflight never touches the network: descriptors come from the
content-addressed cache in host state, filled only by `sync`. Destructive
commands (`salvage`, `rm`, `retire`) may run when only the spec moved on,
but never on drift or a failed identity check.

## 1.6 State machines

Each machine lives in `internal/state` as a transition table (from,
event, to, effect); an event with no row is an illegal transition and
exits 2 with its error id. Tests are generated from the tables: every
row is exercised and every missing pair is asserted illegal. State is
persisted in the project record `projects/<name>.json`
([03 3.8](03-formats.md#38-host-state-schemas-state-v1)).

### Candidate

At most one candidate per project exists at any time.

| From | Event | To | Effect |
|---|---|---|---|
| (none) | `sync` renders | rendered | candidate dir written under `.romeu/candidates/<candidate-id>/` |
| rendered | widening digest equals the approval | promoted | promotion commit (below) |
| rendered | digest differs, TTY, `y` | approved | approval recorded |
| rendered | digest differs, TTY, `N` | declined | candidate dir deleted |
| rendered | digest differs, no TTY | awaiting | record `{candidateId, wideningDigest, filesDigest}`; exit 3 |
| awaiting | `approve <name>` on TTY; digests recomputed from the candidate files equal the record; `y` | approved | approval recorded |
| awaiting | `approve <name>`; `N` | declined | candidate dir deleted, record cleared |
| awaiting | `approve <name>`; recomputed digests differ | declined | candidate dir deleted, record cleared; exit 4 "candidate changed; run romeu sync" |
| awaiting | `sync` renders a new candidate | superseded | old dir deleted; the new one enters `rendered` |
| approved | promotion commit completes | promoted | candidate dir deleted, record cleared |

`approve` binds to the awaiting record's `candidateId`, never to "the
latest render". A candidate dir not named by the project record (left by
a crashed `sync`) is deleted by the next `sync`.

### Promotion commit

Promotion is crash-safe without a rollback copy: nothing gate 2
covers is modified before the commit point. `.romeu/bin` is not in
gate 2 (it is a fixed mount whose content may change, 06 6.3); step 2
rewrites it, under a running sandbox too, and a crash there is rolled
forward by the recovery below.

1. Write `render.json` (atomic rename) with `promoting: <candidateId>`.
2. Copy kits into content-addressed, immutable dirs
   `.romeu/kits/<id>-<digest12>/` (a dir that already exists is verified,
   never rewritten); write `.romeu/bin/*` and the workspace files.
3. Rename `sbxenv.yaml` into place: **the commit point**.
4. Rewrite `render.json` without `promoting`; delete the candidate dir
   and every kit dir no longer referenced; clear the candidate record.

Recovery runs after taking the lock in every mutating command: a
`render.json` with `promoting` whose candidate dir still verifies
against its `filesDigest` is finished by repeating steps 2-4 (each is
idempotent). If the candidate no longer verifies, the command exits 4 and
`romeu sync --overwrite-drift` renders a fresh candidate; the old live
`sbxenv.yaml` and its kit dirs are intact, so nothing unapproved is ever
live. `status` reports an interrupted promotion without the lock.

### Generation

A generation is one sandbox lifetime, identified by a ULID.

| From | Event | To | Effect |
|---|---|---|---|
| (none) | `run` finds the sandbox absent and is about to create it | open | record id, recreate digest, workspace path, the repo set, written before `sbx env run` |
| open | the `sbx env run` of that same command returns non-zero, and `sbx ls --json` then shows no sandbox | (none) | the record just written is removed; exit 1 |
| open | the `sbx env run` of that same command returns non-zero, and `sbx ls --json` then shows the sandbox | open | the record is kept; exit 1 |
| (none) | `adopt` on TTY, workspace path matches | open | record as above with `adopted: true` and an unknown recreate digest |
| open | `salvage`, `rm`, `recreate` or `retire` starts the sandbox half | salvaging | new salvage id |
| salvaging | `salvage`, `rm`, `recreate` or `retire` starts the sandbox half again (the earlier command was interrupted or failed) | salvaging | new salvage id |
| open | `salvage --from-host` | open | refs and snapshot bundles imported under a new salvage id; salvage record `result: lost`, `reasons: [sandbox-lost]` |
| salvaging | `salvage --from-host` | salvaging | as the open row |
| salvaging | salvage verified; command was `salvage` | open | salvage record `result: complete` |
| salvaging | salvage incomplete, no `--accept-loss` | open | salvage record `result: incomplete` with reasons; exit 5 |
| salvaging | salvage complete or `--accept-loss`; command removes the sandbox | closed-removed | `sbx env rm`, egress rules removed |
| open | `run` or `rm` finds the sandbox absent | closed-lost | refs and snapshots preserved as a salvage record `result: lost` |
| salvaging | sandbox found absent (crash recovery) | closed-lost | as above |

A romeu killed between the first row's record and the end of
`sbx env run` leaves an open generation: the next `run` finds the
sandbox absent and closes it as lost, or finds it present and goes on.
`salvage --from-host` does not close a generation; closing stays with
the row that finds the sandbox absent.

Salvage always uses the generation's recorded repo set, not the current
spec, so a repo removed from the spec (J13) is still salvaged. Whether
a generation has a final handoff is computed when `rm` runs, by the
handoff reader ([08 8.3](08-memory-handoff-salvage.md#83-handoff)), not
stored. `status` is a projection of these records.

## 1.7 Vocabulary

One word per concept. `tools/ci vocabulary` is generated from this
table and fails when a phrase from the "Not" column appears in docs
(except `docs/reviews/` and the "Not" column of this table), help text
or error messages.

| Word | Meaning | Not |
|---|---|---|
| project | A named unit: 1..n repos, one sandbox, one `<name>-env/` directory. `name` is stable and unique. | - |
| primary repo | The repo sbx clones into the sandbox (clone mode); exactly one per project | - |
| secondary repo | Any other repo of the project; cloned inside the sandbox by julieta from origin | - |
| spec | A project's YAML in the config repo | project config |
| config repo | The repo holding specs and personal kits; the only meaning of "config" on its own | - |
| config project | The project whose primary repo is the config repo | - |
| host settings | The operator's machine-local `settings.yaml` | host config |
| personal kit | A kit from the config repo's `kits/` (`personal:` in a spec) | config kit |
| run layout | A spec's `run:` section: tabs, panes and steps | run-config |
| sync | spec at a named commit -> candidate -> gate -> live tree | - |
| approve | the gates on their own (`--toolchain` for gate 1) | - |
| candidate | an unapproved render under `.romeu/candidates/` | pending candidate |
| awaiting | the candidate state recorded when no TTY was available | pending approval |
| run | preflight, ensure sandbox, egress and tools, then attach to the run layout | - |
| generation | one sandbox lifetime, identified by a ULID recorded by romeu | - |
| adopt | record an existing sandbox of the right name and workspace as a generation | - |
| snapshot | julieta's bundle of unpushed work into the memory dir's `snapshot/` after each commit | live salvage |
| salvage | stop agents, capture everything sandbox-only into the memory dir's `salvage/`, verify and import it on the host | - |
| rm | salvage, then remove the sandbox; the tree stays | - |
| recreate | rm, then run | - |
| retire | the inverse of sync: rm, then move `<name>-env/` to `$ROMEU_ROOT/.attic/`; never deletes | - |
| handoff | agent narrative (`clear`, `final`) or julieta-stamped git facts (`facts`), a plain file in the memory dir | - |
| widening set | gate 2's digested content | - |
| recreate-class | fields tagged `apply:"recreate"`: repos, kits, workload, ports, sandbox options, agent, and (until probe A10 says otherwise) secrets | - |
| review checkout | a hardened checkout of sandbox work under `<name>-env/review/<dir>/` | - |
| sandbox probe | a block B check that the probe harness runs inside a sandbox, ids `C1..C5` | - |
| runtime ledger | the per-machine, add-only store of runtime events under host state; "the ledger" on its own means this | - |
| spool | a project's agent-writable directory where julieta writes events until romeu ingests them | - |
| ingest | romeu reads one project's spool and creates ledger entries | - |
| ledger view | a project's read-only directory that romeu derives from the runtime ledger | - |
