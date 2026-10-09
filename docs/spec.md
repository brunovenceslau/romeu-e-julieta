# Spec: romeu e julieta v1

Reader: the planner and implementers of romeu e julieta v1, and
reviewers of the design. Type: reference index; start here, then read
[01](spec/01-system-model.md).

Status: **draft v1 specification**; its review history is in
[reviews/](reviews/), its decisions in [adr/](adr/). It is the source
of truth for the implementation plan and describes the current state
only; a spec change that reverses a decision adds an ADR that
supersedes it ([12 12.5](spec/12-engineering.md#125-decisions-and-history)).
Every document follows the documentation standard of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md).

romeu e julieta organizes development environments built on Docker
Sandboxes (`sbx`). **romeu** runs on the host: a deterministic,
auditable binary that turns reviewed project specs into one directory
tree and drives `sbx`. **julieta** runs inside each sandbox: tools (via
mise), git hooks, memory, handoff, salvage and the in-sandbox terminal
layout. No agent and no LLM ever runs on the host.

## Index

| # | File | Covers |
|---|---|---|
| - | this file | principles, success criteria, boundaries, open questions, deferred decisions |
| 0 | [spec/00-scope.md](spec/00-scope.md) | objective, users, non-goals, license, scope justification, teams adopting AI-assisted development |
| 1 | [spec/01-system-model.md](spec/01-system-model.md) | components, sources of truth, trust boundaries, gates, preflight, state machines, vocabulary |
| 2 | [spec/02-layouts.md](spec/02-layouts.md) | repositories and directory layouts, in-sandbox paths, multi-repo decision |
| 3 | [spec/03-formats.md](spec/03-formats.md) | file formats, validation rules, digests |
| 4 | [spec/04-cli.md](spec/04-cli.md) | romeu and julieta commands, flags, exit codes, error ids |
| 5 | [spec/05-security.md](spec/05-security.md) | hardened git, security invariants and their tests, ask-first surfaces, residual risks |
| 6 | [spec/06-kits.md](spec/06-kits.md) | kits v3 (local builds), workload, julieta delivery, product kits, publishing path |
| 7 | [spec/07-mise-egress.md](spec/07-mise-egress.md) | mise integration, egress catalog and derivation |
| 8 | [spec/08-memory-handoff-salvage.md](spec/08-memory-handoff-salvage.md) | memory store, session hooks, handoff, snapshot and salvage |
| 9 | [spec/09-journeys.md](spec/09-journeys.md) | developer journeys, step by step |
| 10 | [spec/10-testing-style.md](spec/10-testing-style.md) | test levels, CI, fake-sbx contract, acceptance evidence, code style, module boundaries |
| 11 | [spec/11-host-probes.md](spec/11-host-probes.md) | host probes, the probe result format and its lifecycle |
| 12 | [spec/12-engineering.md](spec/12-engineering.md) | tech stack, dev commands, capability map, generators, middleware, contributor docs, text standard, delivery practices, delivery metrics |
| 13 | [spec/13-runtime-ledger.md](spec/13-runtime-ledger.md) | the runtime ledger, designed and deferred out of v1: spool, events, ingest, entries, view, versions, starting values, consumers, tests |

## Principles

### Deterministic where it can be, the model where it adds value

Anything that must happen the same way every time is code, a gate or a
hook - never a model's memory or goodwill: sequence identifiers,
timestamps, validations, schema and path checks, hashes, pins,
generated files, ordering, formatting, and every security invariant.
Agents do what they are best at: understanding intent, designing,
writing and reviewing specs and code, and judgment calls. Every place
where an agent is asked to "remember" something mechanical is a defect:
move it into romeu, julieta, a hook, a generator or a CI check, and let
the agent consume the result.

### Tools are code

Everything we write is held to production standard: the CLIs, and
equally `tools/*`, tests, CI workflows, schemas, kits, docs, and text
outside the repository (issues, PR bodies, review comments, commits,
release notes). There is no "just a script" exemption. Every change
leaves the area better than it was found, and each iteration records
what it taught as a lesson, and as a check when one is possible. The bar
is very high and pragmatic: quality is spent where it protects users,
security or future change. A decision that no longer fits is revisited
openly and superseded explicitly, never silently worked around.

### Mechanical middleware on the code that produces code

Before and after every change, the same machinery runs: generators
create the skeleton of a new ADR, invariant, probe, kit, command or
lesson with the next free id; `go generate` derives every table that has
a single source; a tracked pre-push hook and CI enforce the checks; the
PR body states which check or generator the change added or improved
(or why none). The before/after question "what would have caught this,
and does it exist now?" is a required field, not a habit
([12 12.4](spec/12-engineering.md#124-middleware-before-and-after-every-change)).
A generator or a scaffold exists before the change that needs it, not
before that: each lands with the module that owns its first input
([12 12.2](spec/12-engineering.md#122-development-commands-and-capability-map)).

### Simple and explicit

Count concepts; prefer merging two into one over adding a third. The
candidate and generation lifecycles are transition tables in
`internal/state`, one name per state, illegal transitions refused, with
generated tests and diagrams; the promotion commit and the retire move
are ordered step lists with a recovery rule per step, tested by fault
injection ([01 1.6](spec/01-system-model.md#16-state-machines)); the
probe lifecycle is the table of
[11 11.4](spec/11-host-probes.md#114-probe-lifecycle). Every rule with
more than one consumer has exactly one source and the consumers are
generated from it. The result must be trusted by its users and inviting
to contributors: a newcomer reads `ARCHITECTURE.md`, runs one command
to scaffold what they add, and gets a precise error id with a fix hint
when something is wrong.

### Decide at the last responsible moment

Build what v1 needs in order to start, as soon as it is needed. A
mechanism the start does not need is left out, and the decision about
it waits until use has given us more context; that is very often the
better moment to decide. What waits is written down: one row in
[Deferred decisions](#deferred-decisions), with what holds until then
and the observable event that reopens it. A deferral without a row is
an oversight, and a row without an event is a wish. A security
invariant, a merge gate and anything a step of the first-day path (J1
to J3, J5, J6) depends on are needs of the start; a dependency that
exists only because a later section was written does not count. A
part of one is deferred only as a risk the operator accepted by name
in [05 5.4](spec/05-security.md#54-known-residual-risks-accepted-in-v1),
and its row says so.

This is the same discipline as "Simple and explicit", applied in time:
that principle counts the concepts we add, and this one says when a
concept may be added at all. The Deferred decisions table is its
mechanism, and `tools/ci sequences` checks that each row has all three
cells; whether an event is observable is a review judgment. The
principle is the maintainer's. Its nearest written relative is XP's
"You Aren't Gonna Need It", as Martin Fowler describes it
([Yagni](https://martinfowler.com/bliki/Yagni.html), 26 May 2015). The
sources, and the two books we name without having checked a copy, are
in
[ADR 0005, decide at the last responsible moment and record the trigger](adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md).

## Success criteria (measurable)

Evidence for each criterion is listed in `docs/acceptance.json` (schema
`acceptance.v1`, see [10 10.5](spec/10-testing-style.md#105-acceptance-evidence)).
The last plan task commits that file after the v1.0.0 release and
verifies it with `go run ./tools/ci acceptance`. Before the v1.0.0 tag
the maintainer runs the same check, as a step of the release checklist
of [12 12.2](spec/12-engineering.md#122-development-commands-and-capability-map);
the completeness checks join `tools/ci all` once the plan's last task
has landed, and one that fails after the tag is fixed by a patch
release. Probe results of block B are measured on the release
candidate and count for v1.0.0
([11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)).

| ID | Criterion | Evidence |
|---|---|---|
| S1 | v1.0.0 released: `romeu` darwin/amd64 + darwin/arm64, `julieta` linux/amd64 + linux/arm64, each archive with `COPYING` and a third-party notices file ([00 0.4](spec/00-scope.md#04-license)), `checksums.txt`, build provenance attestations | a `ci-run` of `release.yml` for the tag, whose `tools/release build` step wrote the archives and `checksums.txt`, whose attestation step attested them, and whose `tools/release verify` step ran `gh attestation verify --signer-workflow .../release.yml` on each archive (10 10.2); a `success` conclusion is the evidence that the assets exist, and the asset list itself is **[review]** under the acceptance PR's Evidence. A user's side of it: the B1 run on v1.0.0 ([11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)) runs the J1 commands of `e2e/scenarios/commands.yaml` against the downloaded assets and records each asset's sha256, compared with the release's under the same Evidence, **[review]** |
| S2 | CI on GitHub Actions green on the release commit (all jobs, on the runners 10.2 lists at the release commit) | workflow run id with conclusion `success`; the ruleset's required checks carry the administrator bypass (see its [deferred row](#in-how-this-repository-is-run)), so the run, not the ruleset, is the evidence |
| S3 | local v3 kits (`julieta`, `julieta-claude`, `os-base`, `git-ssh-sign`) build and start a sandbox on both sandbox arches: linux/amd64 on an Intel Mac host and linux/arm64 on an Apple silicon Mac host | probe results `docs/probes/B2-<host>.json` from both hosts |
| S4 | the egress catalog covers every `backend:tool` in the reference config repo's projects | a `ci-run` item that names the config repo and its run, where `julieta spec validate --catalog projects/*.yaml` exits 0; and a named unit fixture in which it exits 6 (`check-findings`) on an unknown key |
| S5 | the `julieta-claude` product kit installs `skills/julieta` and `skills/handoff`, the SessionStart and SessionEnd hooks, and makes the agent's own memory dir non-writable | probe results `docs/probes/B2-<host>.json` |
| S6 | every security invariant in [05](spec/05-security.md) has a tagged guard and a tagged test that fails when the guard is disabled | `tools/ci invariants` (the tags pair up); a `ci-run` item of the latest scheduled `fuzz.yml` run on `main` before the tag, which ran `tools/ci mutate` (each guard stub turns a tagged test red) and which `release.yml` requires to be `success` (10 10.2); and `tools/ci acceptance` (every row of 05 5.2 has both tags) |
| S7 | every journey in [09](spec/09-journeys.md) passes, in the meaning the opening of 09 gives a pass in CI: J2, J3b, J6, J7, J10, J11 in CI with the fake sbx, on the Linux runners where a journey reaches julieta ([10 10.1](spec/10-testing-style.md#101-test-levels)); J1-J14 on a real host | CI run; probe results `docs/probes/B3-<host>.json` |
| S8 | `romeu run <p>` on a running sandbox with unchanged locks: romeu's own overhead <= 2 s (median of 5, excluding time inside `sbx` subprocesses); `julieta setup` no-op <= 3 s (median of 5), measured in the container e2e against local origins, so the network is outside that budget. The 2 s and 3 s bounds are starting values; the first B5 measurement confirms or resets them | the timed container e2e in CI for the julieta figure; `docs/probes/B5-<host>.json` from both hosts for the romeu figure, measured by the probe harness ([11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)) |
| S9 | coverage >= 80% statements, per package, across every package `go list` reports under `cmd/...`, `internal/...`, `tools/...`, `e2e/probes/...` and `e2e/fakesbx/...`, a package absent from the profile counting as 0%; >= 90% for `gitsafe`, `gate`, `spec`, `render`, `egress`, `termsafe`, `state`, `canon`. 80% and 90% are starting values that are only raised | `tools/ci coverage` |
| S10 | docs: README, `ARCHITECTURE.md`, `CONTRIBUTING.md`, `SECURITY.md`, `docs/guide/*` per journey, generated `docs/reference/*` (a page per command, `errors.md`, `exit-codes.md`, `project.md` and `ask-first.md`, [12 12.3](spec/12-engineering.md#123-generators)), `docs/adr/*` for the decisions in this specification | `tools/ci docs` and `tools/ci generated`; `tools/ci acceptance` for a guide page per journey (10 10.5); that the ADRs cover the decisions is **[review]** |
| S11 | REUSE 3.3 compliant; every file has an SPDX header or a `REUSE.toml` entry | `reuse lint` inside `tools/ci` |

## Boundaries (for everyone who changes this repo)

- **Always**
  - Route every git call, in both binaries, through `internal/gitsafe`
    in its host or sandbox mode
    ([05 5.1](spec/05-security.md#51-hardened-git-internalgitsafe)), every sbx call
    through `internal/sbxdrv`, and every host output through the CLI
    writer, which escapes with `termsafe` by default.
  - Emit YAML and JSON only through marshallers; never `text/template`
    for structured files.
  - Validate every name and path field from a spec before use.
  - Tag every invariant guard and its test with the invariant id
    (`//romeu:invariant I<n> guard|test`) and change both in the same
    commit.
  - Scaffold with `go run ./tools/new ...` and regenerate with
    `go generate ./...`; the tracked pre-push hook runs
    `go run ./tools/ci fast`; run `go run ./tools/ci all` on your
    platform before opening a PR, and CI runs the other platforms'
    steps; review golden diffs. Every `tools/ci` gate CI runs also runs
    locally, from the same `tools/ci` code
    ([10 10.2](spec/10-testing-style.md#102-ci)).
  - English, Conventional Commits, no em dash character, SPDX headers;
    commit, PR, issue and review text follow the text standard
    ([12 12.7](spec/12-engineering.md#127-text-standard-and-lessons)).
  - Record what a change taught in a file under `docs/lessons/` and
    turn it into a `tools/ci` check when it can be one.
  - Open each PR against the default branch, from a short-lived branch,
    and merge unfinished work dark instead of keeping it on a branch
    ([12 12.9](spec/12-engineering.md#129-delivery-practices)).
- **Ask first**
  - Any change to a path listed in `.github/ask-first.yaml`
    ([05 5.3](spec/05-security.md#53-ask-first-surfaces)), which is
    the one list of those surfaces.
- **Never**
  - Add a code path in `romeu` that configures git hooks, runs mise,
    executes repository content, or opens a terminal pane on the host.
  - Pass `--auto-approve` to sbx.
  - Put a secret value, a host command, a local user name or an absolute
    personal path in the product repo.
  - Delete a clone, a memory directory or a salvage ref from romeu.
  - Run `tools/ci` or the pre-push hook on the host from a commit whose
    diff since the last host run the maintainer has not read; the last
    released tag is the one exception
    ([05 5.4](spec/05-security.md#54-known-residual-risks-accepted-in-v1)).
  - Hand-edit a generated file.
  - Replace an asset of a published release; a fix is a new version
    ([10 10.2](spec/10-testing-style.md#release-and-bootstrap)).
  - Write a name the forbidden-name denylist holds, in any form it
    matches, in a file, a path, a commit message, a commit's author or
    committer identity, a branch or tag name, or the text of a PR
    ([10 10.2](spec/10-testing-style.md#forbidden-names)).

## Open questions

Each has a stated default that the spec already follows; a deferral
has no default, it is absent until its event, and it is a row of
[Deferred decisions](#deferred-decisions) (ADR 0005). A question leaves
this table in the change that settles it: the commit of a probe result
([11 11.4](spec/11-host-probes.md#114-probe-lifecycle)) or the change
that records the maintainer's answer. A settled question becomes an
ADR when the decision is expensive to reverse (ADR 0005); an
overturned probe default always does. The "Settled by" column is the
one place that says which probe settles which question; 11 does not
repeat it. It holds probe ids, or "maintainer, before the first
release-candidate tag", so that a late answer cannot force a new
candidate. Readings the plan took where the spec is silent, with no
effect on the spec, stay in
[plan.md, Questions for the maintainer](plan.md#questions-for-the-maintainer).

| # | Question | Default taken (recommendation) | Settled by |
|---|---|---|---|
| Q2 | Workload kit: Docker's `sbx-kit-claude` by digest, or our own | **Docker's by digest**, with a host-settings allowlist of workload repositories | probes A12, B2 |
| Q4 | Same repo in two projects | **forbidden** in v1 | maintainer, before the first release-candidate tag |
| Q6 | Project rename / repo rename or split | manual path (J13) | maintainer, before the first release-candidate tag |
| Q7 | v3 milestone instability | frontend pinned by digest; probe A11 is go/no-go before kits are written; no v2 fallback | A11 |
| Q8 | Disabling Claude Code's own memory | `julieta-claude` makes the agent memory dir non-writable and the hooks check it; plus the setting if the pinned version has one | C3, B2 |
| Q9 | romeu distribution | GitHub release + checksums + mandatory attestation verify | maintainer, before the first release-candidate tag |
| Q10 | Host git floor | patched point release per series, initial floor 2.45.4 or newer; exact list verified in A1 | A1 |
| Q12 | Salvage of ignored files and transcripts | ignored files included (1 GiB cap, spec excludes are acknowledged loss); transcripts excluded unless `--include-transcripts` | maintainer, before the first release-candidate tag |
| Q13 | Per-project fine-grained tokens | `name@project` host-settings entries; sharing a command is allowed but written per project. The mechanism is per project; the reference instance binds the operator's own token in every project (ADR 0008) | maintainer, before the first release-candidate tag |
| Q14 | Memory across machines | machine-local in v1 | maintainer, before the first release-candidate tag |
| Q15 | Manifest delivery | `sbx env exec --env JULIETA_MANIFEST=...`, with a compatibility check | A4 |
| Q16 | herdr pre-1.0 churn | pinned by version + sha256; protocol number checked, fail closed | A13 |
| Q18 | Secrets change on a live sandbox | **recreate-class** (safe); switch the `apply` tag to live `sbx secret set --command` if A10 shows the next `env run` keeps it | A10 |
| Q19 | How sbx forwards the dedicated signing socket | romeu sets `SSH_AUTH_SOCK` to `signing.agentSocket` for the sbx calls of a project that uses `git-ssh-sign`; the refusal rule of [06 6.4](spec/06-kits.md#64-product-kits) is decided, only the forwarding mechanism is measured | A12, B2 |
| Q20 | Workload and kit capability discovery | `internal/oci` reads the descriptor annotation by digest over HTTPS; replaced by an sbx inspect command if A12 finds one | A12 |
| Q21 | Sandbox arch equals host arch | romeu selects `julieta-linux-<GOARCH>` from its own `GOARCH`; the compatibility check fails closed otherwise | A4 |
| Q22 | Claude Code hook order on `/clear` | SessionEnd fires before SessionStart; the handoff rules of [08 8.3](spec/08-memory-handoff-salvage.md#83-handoff) hold in either order, and the container e2e runs both. The probe says which order the guide describes | C5, B3 |
| Q26 | Pieces of a module that land before its place in the build order of 12 12.2 (the error table, the pins of `tools/kitpin`, two sbx argv builders), and the one pin file | a module's first task may precede its place when it has no dependency there; the pin file joins the tree of 02 2.1, and `tools/ci kits` checks that every kit's pin equals it | maintainer, before the first release-candidate tag |
| Q27 | Network reads in CI beyond the container e2e's mise install (10 10.1): the workload's base image and the herdr download | both join the rule of 10 10.1, pinned and checksum-verified like mise; a transport failure is an infrastructure error, a digest or checksum mismatch a test failure | maintainer, before the first release-candidate tag |
| Q28 | What "the workload's Debian base" of 10 10.1 is | the pinned workload image, pulled by digest and run as a plain container; if it does not start without `sbx`, a Debian image pinned by digest until A12 reports the workload's missing set | A12 |
| Q29 | Whether block A plants a global secret or a broad allow rule on the host (A9, I22) | no live plant: the doctor fixtures of I22 are synthesized files, kept apart from the recordings and marked as such, each parsing the way the A9 recording does | maintainer, before the first release-candidate tag |
| Q30 | How probe results and recordings reach the default branch | a pull request from a branch pushed through the hook; the recorder's filter, scrub and typed parsers are named in the Redaction row of 10 10.3 | maintainer, before the first release-candidate tag |
| Q31 | The signature mode of mise's releases that `tools/kitpin` verifies (07 7.4) | read from the signature file's header first; only a prehashed minisign mode needs BLAKE2b, which would be a `dependencies` approval for `tools/kitpin` alone | maintainer, before the first release-candidate tag |
| Q32 | Install steps and lifecycle hooks in personal kits | not refused, and shown in the gate 2 diff ([06 6.1](spec/06-kits.md#61-decisions)); once A14 reports where they run and what they reach, the operator decides whether `sync` refuses them | A14 |

## Deferred decisions

A mechanism that v1 does not need in order to start is left out and
listed here, with what holds until then and the event that reopens it.
A question has a default the spec follows (Open questions); a deferral
is absent until its event. A row leaves the table when its event
happens and the decision is made. A cell states what holds and the
event; the reasoning goes to an ADR. Where a check or a command output
detects the event, the Reopened by cell names it. Any other event is
reported, by whoever meets it, as an issue labelled `deferred` that
names the row, and the tables are read at each release-candidate tag.

### In the product

| Deferred | Until then | Reopened by |
|---|---|---|
| the runtime ledger ([13](spec/13-runtime-ledger.md); [ADR 0006](adr/0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md) stays the design), with its rules that no entry is edited or deleted and that no event has a free-text field | julieta state ([08 8.2](spec/08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)) and each command's stderr are the record. Cost of waiting: events before the ledger exists are not recorded, and its two mounts cost one recreate per project when it lands (13 13.1) | the first `docs/lessons.md` entry that names an in-sandbox failure julieta state could not reconstruct; or a second machine |
| a user-facing success criterion, and an acceptance run from a fresh account without the reference config repo | `examples/` is the only config not the maintainer's that the tests use; B3 records the operator's commands, prompts and wall clock of J1, J2 and the daily resume, compared with [00 0.2](spec/00-scope.md#02-users) in the acceptance PR's review ([11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)) | the first B3 measurement; or the first issue from an adopter blocked in J1 to J3b |
| a second run-layout renderer and its interface | herdr, pinned by version and sha256, is the one renderer; the run layout format names no multiplexer ([03 3.2](spec/03-formats.md#run-layout-run)) | a herdr release that breaks the protocol number A13 recorded; a CVE in herdr; or six months without a herdr release |
| publishing and signing product kits | local builds only, trusted as the release plus the config commits ([06 6.1](spec/06-kits.md#61-decisions)) | the first issue from a user who cannot build the local kits; or sbx accepting a signature for a local kit |
| listing sbx's native egress approval queue in `status` | sbx's own approval list, named in [07 7.5](spec/07-mise-egress.md#75-egress-derivation-internalegress) | the first issue from a user who resolved a native approval without knowing it existed |
| filters for `julieta handoff list` | the unfiltered list | the first issue asking for a filter |
| a Remote-SSH helper command | no helper; the operator connects by hand | the first issue asking for it |
| a host-settings credential for a private workload registry | descriptors are read anonymously over HTTPS and cached by digest ([03 3.4](spec/03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)) | the first workload in a private registry, reported as an issue |
| a Homebrew tap | GitHub releases only, verified as in J1 step 2 | the first issue asking for a tap; the decision says how the formula verifies provenance |
| signing and notarizing the darwin binary | not signed; installed with `curl` as in J1, which sets no quarantine attribute (inferred; B1 measures it) | the first issue that reports a Gatekeeper refusal |
| an operator signature over `checksums.txt`, made outside GitHub | v1 releases carry `checksums.txt` and a keyless build provenance attestation and no other signature; the attestation proves that `release.yml` built the archive, not that the maintainer approved the release | the first release whose tag the maintainer did not push; a second contributor; or the token narrowing of [ADR 0008](adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md) |
| docs versioning per release: which docs a reader of an older release sees | the release notes and the README's install section link the docs at the release tag, and `tools/release` writes that link into the notes; the default branch's docs describe the next release | the first change to a user-facing page after the v1.0.0 tag |
| snapshot history, salvage of a sandbox that does not start, a host-visible time of the last good snapshot, and host-state migration | the guarantee of [00 0.1](spec/00-scope.md#01-objective) over a sandbox that starts, with one snapshot | the first salvage record whose reason says the sandbox half failed |
| reporting and reclaiming the space of salvage payloads, snapshots, `.attic` and review checkouts | they only grow; romeu deletes none of them (I16) and no command reports their size | the first issue that reports a full disk |
| a create-only host ref per fetched origin head per generation, so a pushed commit the agent later deletes on origin stays on the host | a push to origin is a copy an agent can take back when the project's token can delete branches and tags; salvage covers what the sandbox still holds (I17, [05 5.4](spec/05-security.md#54-known-residual-risks-accepted-in-v1)) | the first pushed commit lost from origin, reported as an issue labelled `deferred`; or the token narrowing of [ADR 0008](adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md) |
| a signing key per project, or a signing path that is not forwarded into the sandbox | the one-key socket of [06 6.4](spec/06-kits.md#64-product-kits) (I34), with the key added by `ssh-add -c` | the first commit signed in the operator's name that the operator did not ask for, reported as an issue labelled `deferred`; or a second contributor |
| `romeu memory list\|show`: reading memory entries, lessons included, on the host through `memstore`'s readers | `romeu handoff` shows handoffs; other memory is read with a pager that shows bytes and runs nothing (05 5.4) | the first time the operator needs to read lessons on the host, reported as an issue labelled `deferred` |
| moving the salvage payloads of a closed generation out of the mounted memory dir | they stay in the mount, dirs 0700 and files 0600, a risk accepted in 05 5.4 | a second adopter; or a host shared by more than one user |
| a `romeu restore` command that brings salvaged or lost-generation work into the next sandbox | recovery is the operator's step: a branch or salvage commit comes back by J10 step 3, and payload files are copied in by hand ([08 8.5](spec/08-memory-handoff-salvage.md#recovery)) | the operator recovers by hand a second time, or a recovery goes wrong, reported as an issue labelled `deferred` |
| a periodic snapshot of uncommitted changes | a snapshot follows each commit and each `stop` and `pull`; uncommitted work since the last snapshot is lost when a sandbox dies unplanned ([08](spec/08-memory-handoff-salvage.md)) | an operator loses uncommitted work to an unplanned sandbox death, reported as an issue labelled `deferred` |
| importing snapshots at every `sync` and `run` | a snapshot lives in the sandbox's memory mount until `romeu pull` or a preservation imports it; for the primary repo `refs/sandboxes/<name>/*` is a second copy as of its last fetch ([08 8.4](spec/08-memory-handoff-salvage.md#84-snapshot-after-every-commit)) | a lost snapshot, reported as an issue labelled `deferred` |
| a runtime feature-flag mechanism in `romeu` or `julieta` | unfinished work merges dark: on the default branch, reached by no command (12 12.9) | the first capability that must be in a release before it is finished and that a user has to be able to turn on |
| recording, for a team, the inputs of a return-on-investment model (00 0.6): dated change markers, activity per project, time per change, cost per change | the product records none of them and computes no return on investment. It needs data from more than one machine, which this specification does not have | the first team that runs the product on more than one machine and asks for a metric |
| turning on the agent's own telemetry export (its token, cost and duration figures, as OpenTelemetry metrics, events and traces) from a product kit, and passing a trace context from the host into a sandbox | no product kit sets it. romeu starts `sbx` with a scrubbed environment (plus `SSH_AUTH_SOCK` for a `git-ssh-sign` project) and passes one `--env` into a sandbox, `JULIETA_MANIFEST` (04 4.1), so no other variable reaches the agent through romeu. A personal kit (06 6.5) or a pane `env` (03 3.2) can already turn it on inside one sandbox; an export it sends over the network needs an endpoint that sandbox's egress allows, and romeu reads nothing of what it emits. Cost of waiting: no token, cost or duration figure of a session is recorded | the first question about the tokens, the cost or the duration of one agent session (one run of the agent's process, not one change), written as an issue; a question about the cost of one change stays with the gate-run row. The decision then says where those figures are stored, which values may group them (a member of a closed set may; an event `id`, an entry key, a session id or an email address may not), whether a pane `env` may carry the agent's telemetry variables, and whether I1's argv gains a variable |
| approving a candidate rendered without a terminal | a `sync` without a TTY whose widening digest differs deletes its candidate, prints the digest and exits 3; the operator runs `sync` again on a terminal ([01 1.6](spec/01-system-model.md#candidate)) | the first scripted `sync`, reported as an issue labelled `deferred` |
| a capability type sbx adds to v3 descriptors after v1 | `sync` fails closed on a type the strict grammar does not model, until a romeu release models it ([01 1.4](spec/01-system-model.md#gate-2-project-widening-per-project)) | the first `sync` that fails on an unmodeled capability type |
| generating `docs/reference/data.md` from the table of where data lives | the table of [01 1.2](spec/01-system-model.md#where-data-lives) is written by hand | the first time that table and [02](spec/02-layouts.md) disagree |
| a generated glossary page from the vocabulary table | [01 1.7](spec/01-system-model.md#17-vocabulary) is the glossary, and `tools/ci vocabulary` reads its "Not" column | the first guide page that needs a glossary |
| compacting closed generations of a project record into the state attic | `generations[]` only grows: v1 keeps every generation in the record ([03 3.8](spec/03-formats.md#38-host-state-schemas-state-v1)), and `doctor` warns past 256 KiB | `doctor`'s warning on a record over 256 KiB; or `status` measured slower than S8 on a real record |
| separate instances per machine: an own root and state for, say, work and personal projects | one root and one state per machine; `ROMEU_SETTINGS` is for tests ([02 2.4](spec/02-layouts.md#24-host-settings-and-state)) | the first user who asks to keep two sets of projects apart, reported as an issue labelled `deferred` |
| seeding the egress catalog from a public, named set of common backends | the v1 catalog covers the reference config repo's stack; any other tool is declared with `egress.tools`, always gated ([03 3.5](spec/03-formats.md#35-egress-catalog-catalogegressyaml-schema-catalogv1)) | the first adopter other than the maintainer |
| shared defaults across project specs (for example `projects/_defaults.yaml` merged under each spec, still gated per project) | each spec carries its own workload and kits, so a bump is one edit, one approval and one recreate per project | the second workload bump that touches more than one project |
| counting escape-hatch use: `egress.tools` and `egress.extra` overrides per project, `tool: other`, uses of `--accept-loss` and `--overwrite-drift` | nothing counts them; each is bounded where it is defined and visible per use, and the overrides are read in the config repo's `projects/*.yaml` ([07 7.5](spec/07-mise-egress.md#75-egress-derivation-internalegress)) | the same `egress.tools` override in the specs of two projects; or the first catalog release that removes `egress.tools` entries from two or more projects |
| a lock per project in place of the one `romeu.lock` per machine | commands on different projects wait for each other, a create that builds kits included ([04 4.1](spec/04-cli.md#41-conventions-both-binaries)) | the first issue from an operator who routinely starts two projects together |
| pinning the apt delta of `os-base` (exact versions and a dated snapshot mirror, moved by `tools/kitpin` as a reviewed diff) | `os-base` exists only if A12 reports a missing set, and its apt delta floats with the Debian mirror at build, a residual risk of [05 5.4](spec/05-security.md#54-known-residual-risks-accepted-in-v1) ([06 6.4](spec/06-kits.md#64-product-kits)) | the first breakage traced to that delta; or `os-base`'s set growing past the A12 minimum |
| mise's monorepo mode | not used in v1, so `monorepo.lockfile` is not set and `julieta lock --check` has no rule for it ([07 7.3](spec/07-mise-egress.md#73-lockfile-rules)) | the first repo that enables it; it then sets `monorepo.lockfile` explicitly and `--check` gains a rule |

### With the runtime ledger

The design of [13](spec/13-runtime-ledger.md) leaves these decisions
for later. They wait for the ledger row above: none of their events
can happen before the ledger is built.

| Deferred | Until then | Reopened by |
|---|---|---|
| rotation of the runtime ledger, a hard size limit that refuses an ingest, and the starting values of [13 13.8](spec/13-runtime-ledger.md#138-starting-values). Under the rule that nothing is edited or deleted, rotation may not delete or rewrite an entry; how is decided then | the ledger only grows; `doctor` warns past the size in 13 13.8, and no ingest is refused; the starting values are the proposed defaults. View derivation reads the whole ledger on each `run` | `doctor`'s size warning fires for the first time; or ingest reports the size cap or the listing bound reached |
| a hash chain or an anchored head over the ledger's entries, to detect a removed entry | a removed entry is undetected, so "never deleted" is a promise; `doctor` detects a content mismatch of ingested event bytes and nothing else (13 13.1) | the ledger is restored from a backup or copied between machines; or it is used as evidence off the machine; or a second writer of the ledger is proposed; each reported as an issue labelled `deferred` |
| cross-machine sync of the ledger | one ledger per machine | a lesson has to be learned again on a second machine, reported as an issue labelled `deferred` |
| finer visibility per project in the cross-project view | each sandbox on a machine sees the other projects' names, their activity timing and the allowlisted fields ([13 13.11](spec/13-runtime-ledger.md#1311-residual-risks-of-the-design)) | projects whose repositories belong to more than one owner (a GitHub owner or a `gitHosts` entry) are synced on one machine, reported as an issue labelled `deferred`; or an issue asks that a project not be seen by the others |
| analysis of the ledger beyond `romeu ledger query` and `julieta event list`, for example a field that joins a lesson entry to the event it explains | those two commands; a lesson cites an event by its key (13 13.9) | the first question about the ledger that `query` cannot answer |
| a free-text note, for a project's own view only | a `note` event carries closed fields (13 13.2); something no event id expresses goes into the handoff, and the julieta skill tells the agent to open an issue labelled `deferred` that names this row | an issue labelled `deferred` asking for it |
| keeping the bytes of a rejected spool file, for debugging | a rejected entry holds the key and the reason id, and julieta deletes the file (13 13.5) | the first rejection that cannot be diagnosed from its reason id |
| a cleaner for the spool files ingest skips: path-level violations, and events newer than the version window | they stay in the spool; each ingest examines a bounded number of names and, like `doctor`, reports them (13 13.3) | ingest reports one in real use |
| more event types and fields: romeu-side events, successes and probes; the hook and the repo of a `hook-failed`; the signal behind an `exit_code` of -1 | three event types with the fields of 13 13.2; julieta state holds the hook and the repo for the session. Cost of waiting: events written before a field exists never carry it, and without success events no failure rate can be computed from the ledger for that period. No start or end of an agent session is recorded; once one is known, an entry is matched to it by `project` and time, and two sessions that run at once in one sandbox cannot be told apart | a consumer needs one; each is an additive format version |
| a `path` field on an event, naming the file a failure concerns, and with it a check of that value against secret patterns | an event names no file: an install failure says which tool failed and not which file, and no field of an event is wider than a closed set, a bounded number, a timestamp or an id (13 13.2) | a consumer of 13 13.9 needs the file to act on a failure, and the need is written as an issue; the field then comes with the rule that says which producer sets it |
| removing a member from a membership table of the event format (a catalog `tools` key, an error id, a probe id, a command), which needs ingest to validate against the previous release's tables during the version window | members are only added; a removal is a breaking change of the event format (13 13.7) | the first release that needs to remove a member |

### In how this repository is run

| Deferred | Until then | Reopened by |
|---|---|---|
| a mutation stub per guard site (`mutate_I<n>_<site>`) | one stub per invariant id covers one guard site ([05 5.2](spec/05-security.md#52-invariants-and-their-tests)) | the first invariant with a second guard site that a `mutate` run left unobserved |
| `ci.yml` running the base branch's `tools/ci` against the merge result | a change to `tools/ci` is checked by the changed `tools/ci`, and its only check is the `checks` approval line ([05 5.3](spec/05-security.md#53-ask-first-surfaces)) | the first pull request that changes `tools/ci` and a package it checks together |
| a second owner for the ask-first surfaces | one owner, the maintainer; an outside pull request on a surface is reviewed best effort (05 5.3) | the first outside pull request on a surface |
| running the pre-push hook and `tools/ci` on the host from a trusted `hooksPath` checkout | the host runs `tools/ci` only from a commit whose diff since the last host run the maintainer has read, or from the last released tag (05 5.4) | the first `tools/ci` change the maintainer did not read before a host run, reported as an issue labelled `deferred` |
| a scheduled workflow that checks external links (ADR 0001 rule 5) | `tools/ci links` is run by hand, and once by the final plan task; between those runs nothing reminds anyone to | a dead link in the final plan task's run, or the first one reported after it |
| a scheduled run of the pin-freshness check ([12 12.1](spec/12-engineering.md#121-tech-stack)) | the check is run by hand, and once by the plan task that builds it; between runs nothing reports a newer version of a pinned tool | the first run that finds a pinned tool behind a release that fixes a security advisory for it, or a security advisory published against a pinned tool, however it is learned |
| a pin-freshness check for the GitHub Actions SHAs, the kit frontend digest ([06 6.1](spec/06-kits.md#61-decisions)), the herdr and mise pins of `kits/pins.yaml` and the product's workload digest ([06 6.2](spec/06-kits.md#62-workload-choice)) | the tool pin rule and `tools/ci pins` ([12 12.1](spec/12-engineering.md#121-tech-stack)) cover tools only, the `reuse` image among them; the Actions, the frontend, herdr, mise and the workload stay pinned by SHA, digest or sha256 under their own rules, and move by hand when a CI run or a release note says so. For workloads one piece exists: `julieta pin check` reports a spec's workload that is not a digest or is behind ([04 4.3](spec/04-cli.md#43-julieta-sandbox)); nothing reports a newer Action, frontend, herdr or mise | a security advisory against a pinned Action, image or kit pin; or the first upstream security fix to a pinned tool found after the fact |
| which Markdown linter and which spell checker `tools/ci docs` runs, and their configuration (ADR 0001 rule 5) | those two halves of rule 5 check nothing: they have no tool and no configuration, so no oracle. The link and anchor check and the other rules `docs` owns run | the PR that lands `tools/ci docs`. It picks both tools and puts their configuration files on the `checks` globs of `.github/ask-first.yaml` |
| generating part of the spell-check word list from the vocabulary table (ADR 0001 rule 5) | one accepted-words list; its format and path are fixed in the plan task that picks the spell checker | the vocabulary table gains a term the chosen checker rejects |
| generating the step table of [10 10.2](spec/10-testing-style.md#102-ci) and its per-step detail from `tools/ci` | the table and the detail are hand-written, and 12 12.4 links to them instead of copying them | the next change to the steps `fast` runs that the table of 10 10.2 does not already list. A test that compares the step names `tools/ci all` prints with the rows of 10 10.2 detects it, and that test landing closes this row |
| a time budget for the hosted legs of each pull request | `fast` targets three minutes on a warm cache and `tools/ci` prints each step's duration ([10 10.2](spec/10-testing-style.md#102-ci)); the hosted legs have no budget | the first month the Actions quota runs out |
| how a local-gate run is recorded, and what satisfies the ruleset's required checks when Actions cannot run | no per-PR record format (the `ci-bootstrap` fallback keeps its `interim` item, 10 10.2). One constraint is fixed: `tools/ci` writes the record, and nobody types it | the first time the maintainer chooses the local gates for a merge |
| stacked pull requests: a pull request based on another one's branch | each pull request targets the default branch, and a change that depends on an unmerged one waits for it or is part of it (12 12.9) | the second time a merged pull request's Lessons section records that it waited on another pull request |
| a check that bounds dark code: a package that no command reaches | review judges it, and `coverage` (S9) counts its tests (12 12.9) | the first dark package found more than one release after it merged, by a listing of the packages no command reaches, read at each release-candidate tag |
| whether the maintainer's merge stays a manual checkpoint or gives way to automated gates | every merge waits for the maintainer, by working agreement: the operator's token, which the development sandbox holds, can merge too (05 5.4), so the wait is a convention and not a mechanism. The maintainer merges a green pull request with its approval lines at the next sitting, with at least one merge each business day ([12 12.9](spec/12-engineering.md#129-delivery-practices)) | the first checkpoint with more than three pull requests waiting, counted by hand on GitHub |
| narrowing the development sandbox's GitHub token: a fine-grained token of the same user limited to this repository and without the Administration permission; and no longer forwarding the GitHub authentication SSH key (deferred by the operator, recorded in ADR 0008) | the operator's own token and key, whose reach is every repository the operator's account can (inferred from the account's rights, not measured; 05 5.4). The GitHub security log is read as a judgment, not as a detector: by GitHub's documentation, read on 2026-10-09, it records ruleset and listed settings changes and no merge, push, tag, branch deletion or release (05 5.4) | the first of: a GitHub action by an agent that the maintainer did not ask for; the first issue or pull request from an account other than the operator's; a second contributor; the plan's checkpoint before its last build phase (checkpoint C7); the second final handoff of this repository's development sessions since the last review, on 2026-10-07, counted by hand from the handoff record those sessions keep until julieta runs in the development sandbox (each review rewrites that date here), when the decision returns to the operator. The count is of agent-written handoffs, and checkpoint C7 is the backstop. The interval becomes every 10 final handoffs once a view shows the agent's GitHub actions per session; that view existing is the event, and the operator then decides the change (ADR 0008) |
| developing this repository inside a romeu-managed sandbox | the development sandbox, which romeu does not manage, forwards what ADR 0008 measured: its forwarded agent holds the signing key and a GitHub authentication key. The rule of [06 6.4](spec/06-kits.md#64-product-kits) that the full host agent is never forwarded governs the sandboxes romeu renders | the v1.0.0 release; or the first run of J1 on the maintainer's host |
| `release.yml` taking its own file from `main` and releasing from `main` only (ADR 0008) | no `release.yml` exists; a `v*` tag can be pushed by the development sandbox's token (05 5.4). One rule is fixed now: the release workflow file must not come from the tagged commit. "From `main` only" blocks a stray tag, not the token, which merges through the API | the pull request that adds `release.yml` |
| a check that reads the rulesets again (ADR 0008); a merge-gate part, accepted in 05 5.4 | nothing reads them after the maintainer block; `tools/ci all` makes no GitHub API call, and its network reads are the list in 10 10.1 | the token is narrowed (the token narrowing row above), or `release.yml` lands |
| what replaces the code-owner review, now that the development sandbox acts with the operator's account (ADR 0008) and the default-branch ruleset requires no review | review holds by working agreement, and no ruleset requires it ([05 5.4](spec/05-security.md#risks-of-how-this-repository-is-developed)); the approval line has no check yet | the first pull request that adds a check reading the approval line |
| a signed approval record kept on the host: each approval of an ask-first change signed with a key the development sandbox never reaches, so a check can tell who gave it (follows from ADR 0008, decision 4); a merge-gate part, accepted in 05 5.4 | an approval is the PR body's approval line of [12 12.4](spec/12-engineering.md#124-middleware-before-and-after-every-change), which names what was approved and no person; it proves the approval was recorded, not who gave it, and with one account and one token it is a record and a guard against mistakes, not a barrier against an agent | the token is narrowed (the token narrowing row above) |
| required status checks without the administrator bypass (item e of the maintainer block, 10 10.2); a merge-gate part, accepted in 05 5.4 | the rulesets give the administrator role a bypass, and the token is that role (ADR 0008) | the jobs of the green run are added as required checks |
| `tools/ci dora`, the delivery metrics of [12 12.10](spec/12-engineering.md#1210-delivery-metrics) ([ADR 0004](adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md) stays the design), with its targets and a check that a lesson citing a metric names two periods | review judges the three rows of 12 12.9 that a number would inform, from `gh pr list --json` read by hand, and no number gates a merge; the `Fixes-release:` trailer is checked by `tools/ci pr` now, since its data cannot be filled in later | the third release that is not a prerelease, or the first lesson that wants a delivery number |
| a check that the testifylint fixtures cover every checker of the pinned golangci-lint | the 19 fixtures of `tools/ci/lintconfig_test.go` list the checkers of testifylint v1.6.4 by hand; the pinned binary offers no machine-readable checker list, so a new checker goes unnoticed until someone reads the testifylint changelog | a golangci-lint version bump in `mise.toml` |
| a record of each gate run (which gate, which round, the verdict, counts by severity, duration), and with it a finding-class field on gate findings and a count of repeated classes | no record; the review's text is all there is, and findings are free text. Two things are unresolved. No deterministic producer exists: a gate is run by an agent, and the verdict would have to come from a machine-readable line of each reviewer's report, not from prose. And the join key from a gate run to a change is unresolved: a PR number does not exist when a gate runs before the PR opens, and a commit id is rewritten by a rebase. Cost of waiting: gate runs before the field exists are never classified | a tool runs a gate and reads its verdict, so a deterministic producer exists; the first question about the cost of one change; or `docs/lessons.md` records the same class of finding twice by hand |
| running the change's tests in a job (a virtual machine) separate from the job that judges the commit (`license`, `hygiene`, `workflows`, `generated`); a merge-gate part, accepted in 05 5.4 | the judging steps run in the same process tree and account as the change's tests. The step order of `all` closes the file channel only, a test that rewrites a file a later judging step reads ([10 10.2](spec/10-testing-style.md#102-ci)); an attack on the `tools/ci` process itself (ptrace, root, the same account) is out of its reach, and what holds is the read-only token permissions of `ci.yml`; the ruleset requires no review (05 5.4). `release.yml` already separates them: its publish job runs no test ([10 10.2](spec/10-testing-style.md#release-and-bootstrap)) | the first pull request from an author outside the maintainer's development sandbox, or a ruleset that starts requiring a review |
| the release rule after v1.0.0: which changed paths need a candidate and block B, which ship on CI alone, and who tags | v1.0.0 follows [11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last); a later release has no rule | the first change that merges after the v1.0.0 tag |
| losing one of the two probe hosts (unavailable, or sbx drops Intel): whether the arch-sensitive probes settle on one host and S3 narrows to that arch | both hosts are needed (11) | a sitting that cannot reach a host |
