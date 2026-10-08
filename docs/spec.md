# Spec: romeu e julieta v1

Reader: the planner and implementers of romeu e julieta v1, and
reviewers of the design. Type: reference index; start here, then read
[01](spec/01-system-model.md).

Status: **draft v1 specification, review round 7 applied**. It is the
source of truth for the implementation plan and describes the current
state only. How it got here lives in [reviews/](reviews/round-3.md),
what was folded in after round 3 in
[reviews/round-4.md](reviews/round-4.md) and
[reviews/round-5.md](reviews/round-5.md), the sweep of that result
in [reviews/round-6.md](reviews/round-6.md), and its re-audit in
[reviews/round-7.md](reviews/round-7.md);
decisions live in ADRs under [adr/](adr/), every settled decision
becomes one, and a spec change that reverses one adds an ADR that
supersedes it ([12 12.5](spec/12-engineering.md#125-decisions-and-history)).
Every document follows the documentation standard of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md).

romeu e julieta organizes development environments built on Docker
Sandboxes (`sbx`). **romeu** runs on the host: a deterministic,
auditable binary that turns reviewed project specs into one directory
tree and drives `sbx`. **julieta** runs inside each sandbox: tools (via
mise), git hooks, memory, handoff, salvage, runtime events and the
in-sandbox terminal layout. No agent and no LLM ever runs on the host.

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
| 6 | [spec/06-kits.md](spec/06-kits.md) | kits v3 (local builds), workload, julieta delivery, product mixins, publishing path |
| 7 | [spec/07-mise-egress.md](spec/07-mise-egress.md) | mise integration, egress catalog and derivation |
| 8 | [spec/08-memory-handoff-salvage.md](spec/08-memory-handoff-salvage.md) | memory store, session hooks, handoff, snapshot and salvage |
| 9 | [spec/09-journeys.md](spec/09-journeys.md) | developer journeys, step by step |
| 10 | [spec/10-testing-style.md](spec/10-testing-style.md) | test levels, CI, fake-sbx contract, acceptance evidence, code style, module boundaries |
| 11 | [spec/11-host-probes.md](spec/11-host-probes.md) | host probes, the probe result format and its lifecycle |
| 12 | [spec/12-engineering.md](spec/12-engineering.md) | tech stack, dev commands, capability map, generators, middleware, contributor docs, text standard, delivery practices, delivery metrics |
| 13 | [spec/13-runtime-ledger.md](spec/13-runtime-ledger.md) | the runtime ledger: spool, events, ingest, entries, view, versions, starting values, consumers, tests |

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

Count concepts; prefer merging two into one over adding a third. Every
lifecycle is an explicit state machine with a transition table, one
name per state, illegal transitions refused, and table-driven tests
([01 1.6](spec/01-system-model.md#16-state-machines)). Every rule with
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
invariant, a merge gate and anything a later step depends on are needs
of the start and are never deferred.

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
verifies it with `go run ./tools/ci acceptance`. Probe results of
block B are measured on the release candidate and count for v1.0.0
([11 11.2](spec/11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)).

| ID | Criterion | Evidence |
|---|---|---|
| S1 | v1.0.0 released: `romeu` darwin/amd64 + darwin/arm64, `julieta` linux/amd64 + linux/arm64, `checksums.txt`, build provenance attestations | a `ci-run` of `release.yml` on the tagged commit, whose `tools/release build` step wrote the archives and `checksums.txt`, whose attestation step signed them, and whose `tools/release verify` step ran `gh attestation verify --signer-workflow .../release.yml` on each archive (10 10.2); a `success` conclusion is the evidence that the assets exist, and the asset list itself is **[review]** under the acceptance PR's Evidence |
| S2 | CI on GitHub Actions green on the release commit (all jobs, all runners in 10.2) | workflow run id with conclusion `success` |
| S3 | local v3 kits (`julieta`, `julieta-claude`, `os-base`, `git-ssh-sign`) build and start a sandbox on both sandbox arches: linux/amd64 on an Intel Mac host and linux/arm64 on an Apple silicon Mac host | probe results `docs/probes/B2-<host>.json` from both hosts |
| S4 | the egress catalog covers every `backend:tool` in the reference config repo's projects | a `ci-run` item that names the config repo and its run: `julieta spec validate --catalog projects/*.yaml` exits 0 there, and it exits 1 on an unknown |
| S5 | the `julieta-claude` mixin installs `skills/julieta` and `skills/handoff`, the SessionStart and SessionEnd hooks, and makes the agent's own memory dir non-writable | probe results `docs/probes/B2-<host>.json` |
| S6 | every security invariant in [05](spec/05-security.md) has a tagged guard and a tagged test that fails when the guard is disabled | `tools/ci invariants` (the tags pair up), `tools/ci mutate` (each guard stub turns a tagged test red) and `tools/ci acceptance` (every row of 05 5.2 has both tags) |
| S7 | every journey in [09](spec/09-journeys.md) passes: J2, J3b, J7, J10, J11 in CI with the fake sbx, on the Linux runners where a journey reaches julieta ([10 10.1](spec/10-testing-style.md#101-test-levels)); J1-J13 on a real host | CI run; probe results `docs/probes/B3-<host>.json` |
| S8 | `romeu run <p>` on a running sandbox with unchanged locks: romeu's own overhead <= 2 s (median of 5, excluding time inside `sbx` subprocesses); `julieta setup` no-op <= 3 s (median of 5), measured in the container e2e against local origins, so the network is outside that budget | CI: `romeuMs` of `run --timings --json` with the fake sbx, and the timed container e2e; host confirmation of the romeu figure in `docs/probes/B5-<host>.json` |
| S9 | coverage >= 80% statements, per package, across `internal/...`, `tools/...`, `e2e/probes/...` and `e2e/fakesbx/...`; >= 90% for `gitsafe`, `gate`, `spec`, `render`, `egress`, `termsafe`, `state`, `canon` | `tools/ci coverage` |
| S10 | docs: README, `ARCHITECTURE.md`, `CONTRIBUTING.md`, `SECURITY.md`, `docs/guide/*` per journey, generated `docs/reference/*` per command, file format, error id and exit code, `docs/adr/*` for the decisions in this spec | `tools/ci docs` and `tools/ci generated`; `tools/ci acceptance` for a guide page per journey (10 10.5); that the ADRs cover the decisions is **[review]** |
| S11 | REUSE 3.3 compliant; every file has an SPDX header or a `REUSE.toml` entry | `reuse lint` inside `tools/ci` |

## Boundaries (for everyone who changes this repo)

- **Always**
  - Route every git call through `internal/gitsafe`, every sbx call
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
    `go run ./tools/ci fast`; run `go run ./tools/ci all` before opening
    a PR; review golden diffs. Every merge gate CI runs also runs
    locally, from the same `tools/ci` code
    ([10 10.2](spec/10-testing-style.md#102-ci)).
  - English, Conventional Commits, no em dash character, SPDX headers;
    commit, PR, issue and review text follow the text standard
    ([12 12.7](spec/12-engineering.md#127-text-standard-and-lessons)).
  - Record what a change taught in `docs/lessons.md` and turn it into a
    `tools/ci` check when it can be one.
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
  - Edit or delete a runtime ledger entry, or give an event a
    free-text field
    ([13](spec/13-runtime-ledger.md)).
  - Hand-edit a generated file.
  - Write a name the forbidden-name denylist holds, in any form it
    matches, in a file, a path, a commit message, a commit's author or
    committer identity, a branch or tag name, or the text of a PR
    ([10 10.2](spec/10-testing-style.md#forbidden-names)).

## Open questions

Each has a stated default that the spec already follows. A question
leaves this table when it is settled: by a committed probe result
([11 11.4](spec/11-host-probes.md#114-probe-lifecycle)) or by a
maintainer decision, and in both cases it becomes an ADR. The
"Settled by" column is the one place that says which probe settles
which question; 11 does not repeat it.

| # | Question | Default taken (recommendation) | Settled by |
|---|---|---|---|
| Q2 | Workload kit: Docker's `sbx-kit-claude` by digest, or our own | **Docker's by digest**, with a host-settings allowlist of workload repositories | probes A12, B2 |
| Q4 | Same repo in two projects | **forbidden** in v1 | maintainer |
| Q6 | Project rename / repo rename or split | manual path (J13) | maintainer |
| Q7 | v3 milestone instability | frontend pinned by digest; probe A11 is go/no-go before kits are written; no v2 fallback | A11 |
| Q8 | Disabling Claude Code's own memory | `julieta-claude` makes the agent memory dir non-writable and the hooks check it; plus the setting if the pinned version has one | C3, B2 |
| Q9 | romeu distribution | GitHub release + checksums + mandatory attestation verify; Homebrew tap later | maintainer |
| Q10 | Host git floor | patched point release per series, initial floor 2.45.4 or newer; exact list verified in A1 | A1 |
| Q12 | Salvage of ignored files and transcripts | ignored files included (1 GiB cap, spec excludes are acknowledged loss); transcripts excluded unless `--include-transcripts` | maintainer |
| Q13 | Per-project fine-grained tokens | `name@project` host-settings entries; sharing a command is allowed but written per project | maintainer |
| Q14 | Memory across machines | machine-local in v1 | maintainer |
| Q15 | Manifest delivery | `sbx env exec --env JULIETA_MANIFEST=...`, with a compatibility check | A4 |
| Q16 | herdr pre-1.0 churn | pinned by version + sha256; protocol number checked, fail closed | A13 |
| Q18 | Secrets change on a live sandbox | **recreate-class** (safe); switch the `apply` tag to live `sbx secret set --command` if A10 shows the next `env run` keeps it | A10 |
| Q19 | How sbx forwards the dedicated signing socket | romeu sets `SSH_AUTH_SOCK` to `signing.agentSocket` for the sbx calls of a project that uses `git-ssh-sign`; the refusal rule of [06 6.4](spec/06-kits.md#64-product-kits) is decided, only the forwarding mechanism is measured | A12, B2 |
| Q20 | Workload and kit capability discovery | `internal/oci` reads the descriptor annotation by digest over HTTPS; replaced by an sbx inspect command if A12 finds one | A12 |
| Q21 | Sandbox arch equals host arch | romeu selects `julieta-linux-<GOARCH>` from its own `GOARCH`; the compatibility check fails closed otherwise | A4 |
| Q22 | Claude Code hook order on `/clear` | SessionEnd fires before SessionStart; the handoff rules of [08 8.3](spec/08-memory-handoff-salvage.md#83-handoff) hold in either order, and the container e2e runs both. The probe says which order the guide describes | C5, B3 |
| Q23 | Registry access for descriptors | anonymous HTTPS reads only, cached by digest; an explicit host-settings credential is added only if a private workload registry is needed | maintainer |
| Q25 | With one GitHub account, do the rulesets refuse the sandbox's token while the maintainer still merges and tags | **decided on 2026-10-07, recorded in [ADR 0008, let the sandbox act as the maintainer on GitHub](adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md)**: they do not, except for a direct push to the default branch, which is a control only against an agent that does not edit the ruleset or the repository settings. The sandbox acts with the operator's account, and a second account is declined. The rule is stated once, in [05 5.4](spec/05-security.md#54-known-residual-risks-accepted-in-v1) | decided; the record leaves this table in T101 |

## Deferred decisions

A mechanism that v1 does not need in order to start is left out and
listed here, with what holds until then and the event that reopens it.
A row leaves the table when its event happens and the decision is made.

| Deferred | Until then | Reopened by |
|---|---|---|
| a scheduled workflow that checks external links (ADR 0001 rule 5) | `tools/ci links` is run by hand, and once by the final plan task; between those runs nothing reminds anyone to | a dead link in the final plan task's run, or the first one reported after it |
| which Markdown linter and which spell checker `tools/ci docs` runs, and their configuration (ADR 0001 rule 5) | those two halves of rule 5 check nothing: they have no tool and no configuration, so no oracle. The link and anchor check and the other rules `docs` owns run | the PR that lands `tools/ci docs`. It picks both tools and puts their configuration files on the `checks` globs of `.github/ask-first.yaml` |
| generating part of the spell-check word list from the vocabulary table (ADR 0001 rule 5) | one accepted-words list; its format and path are fixed in the plan task that picks the spell checker | the vocabulary table gains a term the chosen checker rejects |
| how `tools/schemagen` and the later generators of [12 12.3](spec/12-engineering.md#123-generators) join `generateFiles`, the one entry `tools/ci generated` judges | only the generators of `tools/ci` exist (CODEOWNERS, the ask-first page, the ADR index), and `generated` judges each of them | the PR that lands `tools/schemagen` |
| docs versioning per release: which docs a reader of an older release sees | the docs on the default branch are the only docs | the first change to a user-facing page after the v1.0.0 tag |
| generating the step table of [10 10.2](spec/10-testing-style.md#102-ci) from `tools/ci` | the table is hand-written and 12 12.4 links to it instead of copying it | the next change to the steps `fast` runs that the table of 10 10.2 does not already list |
| how a local-gate run is recorded, and what satisfies the ruleset's required checks when Actions cannot run | no per-PR record format (the `ci-bootstrap` fallback keeps its `interim` item, 10 10.2). One constraint is fixed: `tools/ci` writes the record, and nobody types it | the first time the maintainer chooses the local gates for a merge |
| stacked pull requests: a pull request based on another one's branch | each pull request targets the default branch, and a change that depends on an unmerged one waits for it or is part of it (12 12.9) | the second time a merged pull request's Lessons section records that it waited on another pull request |
| a runtime feature-flag mechanism in `romeu` or `julieta` | unfinished work merges dark: on the default branch, reached by no command (12 12.9) | the first capability that must be in a release before it is finished and that a user has to be able to turn on |
| a check that bounds dark code: a package that no command reaches | review judges it, and `coverage` (S9) counts its tests (12 12.9) | the first dark package found more than one release after it merged |
| whether the maintainer's merge stays a manual checkpoint or gives way to automated gates | every merge waits for the maintainer, by working agreement: the operator's token, which the sandbox holds, can merge too (05 5.4), so the wait is a convention and not a mechanism | the first `tools/ci dora` output, a release's `dora.json` or a run, whose `maintainerWaitSeconds` is not `null` (12 12.10). The trigger cannot tell by itself whose merge it measured: a merge by the sandbox's token and one by the maintainer carry the same account, so the decision first says how the two are told apart |
| narrowing the sandbox's GitHub token: a fine-grained token of the same user limited to this repository and without the Administration permission; and no longer forwarding the GitHub authentication SSH key (deferred by the operator, recorded in ADR 0008) | the operator's own token and key, whose reach is every repository the operator's account can (inferred from the account's rights, not measured; 05 5.4) | the first of: a GitHub action by an agent that the maintainer did not ask for; a second contributor; the first release-candidate tag; every 2 autonomous work sessions on the product (a session runs from a resume to its handoff checkpoint), when the decision returns to the operator. A trigger that depends on the GitHub security log is unverified, because its coverage of merges, tag pushes and ruleset updates on a personal account is unverified |
| the review interval of the token narrowing (every 2 autonomous work sessions, then every 10 once the panel or the ledger makes the review cheap) (ADR 0008) | every 2 autonomous work sessions | the operator saying so |
| `release.yml` taking its own file from `main` and releasing from `main` only (ADR 0008) | no `release.yml` exists; a `v*` tag can be pushed by the sandbox's token (05 5.4). Two rules are fixed now: the release workflow file must not come from the tagged commit, and the credential that signs release artifacts is held outside GitHub and out of the sandbox's reach (05 5.4, "Release signing"). "From `main` only" blocks a stray tag, not the token, which merges through the API | the pull request that adds `release.yml` |
| a check that reads the rulesets again (ADR 0008) | nothing reads them after the maintainer block; `tools/ci all` is offline | the token is narrowed (the token narrowing row above), or `release.yml` lands |
| how an approval is recorded, and what replaces the code-owner review, now that the sandbox acts with the operator's account (ADR 0008) and the default-branch ruleset requires no review | the rules that cite the approval line and the code-owner review stand as written, and no ruleset enforces the review half of them; the approval line has no check yet, because T005 builds it | the start of T005 |
| required status checks without the administrator bypass (item e of the maintainer block, 10 10.2) | the rulesets give the administrator role a bypass, and the token is that role (ADR 0008) | item e, when the jobs of the green run are added as required checks |
| targets or performance levels for the delivery metrics | `tools/ci dora` reports and compares with nothing; no number gates a merge (12 12.10) | three releases exist, so each of the five metrics has a value, and the maintainer asks for a target |
| a check that a lesson which cites delivery metrics names two periods of the tool's output | the rule is a review judgment (12 12.10) | the first `docs/lessons.md` entry that cites a metric |
| a check that the testifylint fixtures cover every checker of the pinned golangci-lint | the 19 fixtures of `tools/ci/lintconfig_test.go` list the checkers of testifylint v1.6.4 by hand; the pinned binary offers no machine-readable checker list, so a new checker goes unnoticed until someone reads the testifylint changelog | a golangci-lint version bump in `mise.toml` |
| a record of each gate run (which gate, which round, the verdict, counts by severity, duration) | no record; the review's text is all there is. The runtime ledger ([13](spec/13-runtime-ledger.md)) has no gate-run event type. It would have closed fields only, and two things are unresolved. No deterministic producer exists: a gate is run by an agent, an event that depends on an agent remembering to emit it is what 13 13.2 rules out, and the verdict would have to come from a machine-readable line of each reviewer's report, not from prose. And the join key from a gate run to a change is unresolved: a PR number does not exist when a gate runs before the PR opens, and a commit id is rewritten by a rebase | a tool runs a gate and reads its verdict, so a deterministic producer exists; or the first question about the cost of one change |
| a finding-class field on gate findings, and a count of repeated classes | findings are free text. Cost of waiting: gate runs before the field exists are never classified, so that baseline starts when the field does | `docs/lessons.md` records the same class of finding twice by hand, once the gate-run record above exists |
| recording, for a team, the inputs of a return-on-investment model (00 0.6): dated change markers, activity per project, time per change, cost per change | the product records none of them and computes no return on investment. It needs data from more than one machine, which this spec does not have | the first team that runs the product on more than one machine and asks for a metric |
| rotation of the runtime ledger, and a hard size limit that refuses an ingest. Under the rule that nothing is edited or deleted, rotation may not delete or rewrite an entry; how is decided then | the ledger only grows; `doctor` warns past the size in [13 13.8](spec/13-runtime-ledger.md#138-starting-values), and no ingest is refused. View derivation reads the whole ledger on each `run`, and CI measures S8 on an empty one | `doctor`'s size warning fires for the first time; or `romeu run --timings` reports a `romeuMs` over the S8 budget on a machine whose ledger has entries |
| a hash chain or an anchored head over the ledger's entries, to detect a removed entry | a removed entry is undetected, so "never deleted" is a promise; `doctor` detects a content mismatch of ingested event bytes and nothing else (13 13.1) | the ledger is restored from a backup or copied between machines; or it is used as evidence off the machine; or a second writer of the ledger is proposed. The maintainer observes these |
| cross-machine sync of the ledger | one ledger per machine | a lesson has to be learned again on a second machine. The maintainer observes it |
| finer visibility per project in the cross-project view | each sandbox on a machine sees the other projects' names, their activity timing and the allowlisted fields (05 5.4) | the first private project on a machine that also runs a public one |
| analysis of the ledger beyond `romeu ledger query` and `julieta event list` | those two commands (13 13.9) | the first question about the ledger that `query` cannot answer |
| a free-text note, for a project's own view only | a `note` event carries closed fields (13 13.2) | an agent needs to record something no id expresses, and the need is written as an issue |
| keeping the bytes of a rejected spool file, for debugging | a rejected entry holds the key and the reason id, and julieta deletes the file (13 13.5) | the first rejection that cannot be diagnosed from its reason id |
| a cleaner for the spool files ingest skips: path-level violations, and events newer than the version window | they stay in the spool; each ingest examines a bounded number of names and, like `doctor`, reports them (13 13.3) | ingest reports one in real use |
| more event types and fields: romeu-side events, successes and probes; the hook and the repo of a `hook-failed`; the signal behind an `exit_code` of -1 | three event types with the fields of 13 13.2; julieta state holds the hook and the repo for the session. Cost of waiting: events written before a field exists never carry it, and without success events no failure rate can be computed from the ledger for that period. v1 records no start or end of an agent session; once one is known, an entry is matched to it by `project` and time, and two sessions that run at once in one sandbox cannot be told apart | a consumer needs one; each is an additive format version |
| turning on the agent's own telemetry export (its token, cost and duration figures, as OpenTelemetry metrics, events and traces) from a product kit, and passing a trace context from the host into a sandbox | no product kit sets it. romeu starts `sbx` with a fixed environment and sets one variable in a sandbox, `JULIETA_MANIFEST` (04 4.1, I1 in [05 5.2](spec/05-security.md#52-invariants-and-their-tests)), so no other variable reaches the agent through romeu. A personal kit (06 6.5) or a pane `env` (03 3.2) can already turn it on inside one sandbox; an export it sends over the network needs an endpoint that sandbox's egress allows, and romeu reads nothing of what it emits. Cost of waiting: no token, cost or duration figure of a session is recorded | the first question about the tokens, the cost or the duration of one agent session (one run of the agent's process, not one change), written as an issue; a question about the cost of one change stays with the gate-run row above. The decision then says where those figures are stored, which values may group them (a member of a closed set may; an event `id`, an entry key, a session id or an email address may not), whether a pane `env` may carry the agent's telemetry variables, and whether I1's argv gains a variable |
| tuning the ledger's starting values (13 13.8) | the proposed defaults | ingest reports the size cap or the listing bound reached; or `doctor`'s size warning fires |
| a `path` field on an event, naming the file a failure concerns, and with it a check of that value against secret patterns | an event names no file: an install failure says which tool failed and not which file, and no field of an event is wider than a closed set, a bounded number, a timestamp or an id (13 13.2) | a consumer of 13 13.9 needs the file to act on a failure, and the need is written as an issue; the field then comes with the rule that says which producer sets it |
| removing a member from a membership table of the event format (a catalog `tools` key, an error id, a probe id, a command), which needs ingest to validate against the previous release's tables during the version window | members are only added in v1; a removal is a breaking change of the event format (13 13.7) | the first release that needs to remove a member |
