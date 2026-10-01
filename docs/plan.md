---
type: reference
reader: the implementer picking the next task of romeu e julieta v1
---

# Plan: romeu e julieta v1

This is the implementation plan of v1. Its source of truth is the
specification, [spec.md](spec.md) with the fourteen pages under
`spec/`, and the six decision records under `adr/`. Where this plan and
the specification disagree, the specification is right and this page
has a defect.

A section of the specification is cited the way the specification cites
itself: the page number, then the section, as in "10 10.2". The pages:

| # | Page | # | Page |
|---|---|---|---|
| index | [spec.md](spec.md) | 7 | [07-mise-egress.md](spec/07-mise-egress.md) |
| 0 | [00-scope.md](spec/00-scope.md) | 8 | [08-memory-handoff-salvage.md](spec/08-memory-handoff-salvage.md) |
| 1 | [01-system-model.md](spec/01-system-model.md) | 9 | [09-journeys.md](spec/09-journeys.md) |
| 2 | [02-layouts.md](spec/02-layouts.md) | 10 | [10-testing-style.md](spec/10-testing-style.md) |
| 3 | [03-formats.md](spec/03-formats.md) | 11 | [11-host-probes.md](spec/11-host-probes.md) |
| 4 | [04-cli.md](spec/04-cli.md) | 12 | [12-engineering.md](spec/12-engineering.md) |
| 5 | [05-security.md](spec/05-security.md) | 13 | [13-runtime-ledger.md](spec/13-runtime-ledger.md) |
| 6 | [06-kits.md](spec/06-kits.md) | | |

## How to read this plan

**What the specification fixes about the plan.** Four things, and this
page follows each:

- The plan selects work by the module ids of the capability map
  (12 12.2). Each task below names one.
- The build order between modules is the one of 12 12.2, and "Depends
  on" there is plan order.
- `ci-bootstrap` is the first task (10 10.2, "Release and bootstrap").
- The last task commits `docs/acceptance.json` after the v1.0.0
  release, verifies it with `go run ./tools/ci acceptance`, and runs
  `go run ./tools/ci links` once (index, "Success criteria"; 10 10.2;
  10 10.5).

**What it leaves open, and what we chose.** The specification names no
path, no file format and no task shape for the plan. We chose the
simplest form: this one page, `docs/plan.md`, with the task list and
its checkboxes inside it. One file keeps a task and its tick in one
place. A second file for the ticks would be a second source of the
same list.

**This page is generated.** A generator writes it from task data: the
ids, every "Depends on" line, the phase ranges, the counts in the
operator blocks and the traceability rows are derived there, not typed.
The generator lives with the project's planning records until the
specification gives it a place (see the gap on the generator's place
under [Questions for the maintainer](#questions-for-the-maintainer)).
A hand edit to this page is lost at the next generation. The one thing
a generation reads back from the page it replaces is each ticked
`Merged` box, matched by the task's title, so a tick survives; a change
to a task goes through the task data, and a regenerated page is reviewed
as a diff.

**The shape of a task.** A task is one logical change and one pull
request. It carries:

| Field | Meaning |
|---|---|
| `Merged` | the checkbox; ticked in the pull request that completes the task |
| Module | the module id of 12 12.2 |
| Implements | the sections of the specification, by number |
| Depends on | the tasks and operator blocks that come first |
| Operator | `no`, or the block (O1 to O7) or the question the task waits for or feeds |
| Ask-first | the surfaces of 05 5.3 we expect the diff to touch. `tools/ci pr` computes the real list from the diff (12 12.4); this field is a forecast, so the maintainer can give the approval words early |
| Acceptance | what a test or a command checks |
| Verify | the command that shows it |

**Done, for each task** (10 10.8): `go run ./tools/ci all` passes, the
invariant tags are in place, the generated files are current, and for
host-facing behavior the matching scenario function exists. The
"Verify" line of a task names what is specific to it and does not
repeat this.

**Order.** Task ids follow the build order, and a task depends only on
ids below its own. The ids are not a queue: a task may start as soon
as what its "Depends on" names is merged, whatever its phase. "Operator
blocks" lists what can proceed while a block is waited for.

## How the work is done

- **Language.** Go for the two binaries, the shared packages and the
  tools (12 12.1). POSIX sh in two places only, both bounded by the
  specification: the one-line `.githooks/pre-push`, and kit install
  steps of at most five lines (12 12.1). No task here adds a script in
  another language.
- **Trunk.** Each pull request targets `main` from a short-lived
  branch, carries one logical change and merges as a merge commit
  (12 12.9;
  [ADR 0002, develop on the trunk with short-lived branches](adr/0002-develop-on-the-trunk-with-short-lived-branches.md)).
  v1 has no stack: a task that depends on an unmerged one waits for it.
- **Dark merges.** Work that is not finished merges on `main` and is
  reached by no command (12 12.9). A task marked **dark** adds code
  that a later task connects.
- **Test first.** The test comes before the code it tests (12 12.9;
  [ADR 0003, adopt six Extreme Programming practices and review as pairing](adr/0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md)).
  `go run ./tools/new invariant` scaffolds the failing test first.
- **One entry point for the gates.** `go run ./tools/ci all` is what CI
  runs and what a developer runs before a pull request; the pre-push
  hook runs `go run ./tools/ci fast` (10 10.2).
- **Review before the pull request opens.** A change is read by a
  reviewer, another agent or a person, before it reaches the
  maintainer (12 12.9, "The agent and the reviewer"). The maintainer's
  merge is the last step of each task (12 12.4, the default-branch
  ruleset).
- **Middleware lands with its first input.** A `tools/new` kind and a
  generator row land in the first pull request of the module that owns
  their first input (12 12.2, 12 12.3). We apply the same rule to the
  `tools/ci` checks that 10 10.2 lists and `ci-bootstrap` does not:
  each lands with the first code it checks, and `ci-release` keeps the
  ones with no earlier input. The specification says `ci-release`
  holds "the remaining" checks (12 12.2), which this reading fits; it
  is question 2 under
  [Questions for the maintainer](#questions-for-the-maintainer).
- **Scenarios grow with the commands.** The task that adds a
  host-facing command adds its journey's scenario function and its
  lines of `e2e/scenarios/commands.yaml` (10 10.8).
- **Decide late, and write it down** (index, "Decide at the last
  responsible moment";
  [ADR 0005, decide at the last responsible moment and record the trigger](adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)).
  What this plan leaves undecided is in
  [Decisions this plan defers](#decisions-this-plan-defers), each with
  its trigger.
- **Documents.** Each page follows
  [ADR 0001, adopt a documentation standard with checkable rules and a voice](adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md).

## The first slice: a version handshake

The walking skeleton is the smallest thing that crosses each seam the
architecture depends on. It is this:

> `romeu`, built for the host, asks a `julieta` binary inside a Linux
> container, through the fake `sbx`, for `version --json`, and accepts
> or refuses it by the compatibility check of 06 6.3.

It is whole before block O3, and block A then replaces the one part of
it that was written by hand.

| Step | Tasks | Ends in |
|---|---|---|
| 1 | T007 to T009 | `romeu version` and `julieta version --json` built for their four targets by `tools/ci all` on the four runners, with error ids, escaped output and a generated reference page |
| 2 | T012, T013, T015, T016 | the compatibility check passing, and failing three ways, at the hybrid level (10 10.1; I30), on a synthetic session |
| 3 | after block O3: T029, T030 | the same test on the session block A recorded, and the synthetic one deleted |

Why this slice and not a larger one:

- It needs no gate and no state, and of `render` only two fixture
  files, so it can start the day `ci-bootstrap` ends.
- It proves the four things the other tasks assume: one `tools/ci`
  run on four runners and locally (10 10.2); two binaries
  cross-built from one module with an import rule between them
  (10 10.7); the fake `sbx` replaying a session and forwarding
  `env exec` into a julieta container (10 10.3); and the protocol,
  sha256 and read-only checks of julieta delivery (06 6.3).
- A defect in any of those four is cheapest to find before the
  packages of the later phases depend on it.
- Writing the session by hand shows which argv shapes romeu needs,
  which is what block A must record (T019).

What step 2 does not prove: that real `sbx` behaves as the synthetic
session says. 10 10.3 makes block A the source of truth, so step 3 is
part of the slice and not an afterthought.

The slices that follow widen it in the order of 12 12.2: `sync`
(phase 4), julieta's chores (phase 5), `run` and the destructive
commands (phase 6), the kits (phase 7).

**Four pieces land before their module's place in the map**, each
because a module above it needs that piece and the piece needs nothing
from the modules between (question 3):

| Piece | Task | Module in 12 12.2 | Needed early by |
|---|---|---|---|
| the error table and the dispatcher | T008 | `romeu-cli` | `spec`, `state` and `gitsafe`, which return error ids (03, opening; 01 1.6; 10 10.6) |
| `julieta version`, and `romeu version` | T009 | `julieta-core`, `romeu-cli` | the cross-build, the import rule and the handshake |
| `tools/kitpin` and the pin file | T012 | `kits` | probes A11, A12 and A13, and the container e2e |
| two argv builders | T016 | `sbxdrv` | the handshake |

The CLI writer is not one of them: 12 12.2 puts it under `termsafe`,
which is where T007 lands it. The rest of each module keeps its
place in the order.

## Operator blocks

`sbx` runs only on the host (11, opening), so a step that needs it is
the maintainer's. So are the two `v*` tags, which the tag ruleset
refuses from anyone else (12 12.2), the denylist entries, which an agent
must not hold (10 10.2, "Forbidden names"), and anything that reads the
reference config repo. The maintainer works in sittings, so these steps
are seven blocks, and each says what it needs, what it unblocks and what
wait it puts on the critical path.

| Block | Needs merged first | What the maintainer does | Also needs | Unblocks | Wait on the critical path | Recommendation |
|---|---|---|---|---|---|---|
| O1 | T001, written and open: the block finishes it and merges it | the sitting of the maintainer block, items a to d (10 10.2): rulesets and the two repository settings first; one `go run ./tools/ci hygiene add` per forbidden name in a plain host clone; enable the hook, push the branch, open the pull request; then the four refusal tries from inside the sandbox, proved on the host | a host clone, the repository's admin rights, the sandbox's token created after item a. Also one network step: the sandbox that builds T012 and T013 must reach the registry that holds the pinned images and the release API (`api.github.com`), which a default-deny sandbox does not. Run one `sbx policy allow network` command scoped to that sandbox (`--sandbox`), with every host in it; the agent that writes T012 names the hosts | T002 directly; 101 of 102 tasks have it in their dependencies | the specification gives no duration; nothing is pushed until it ends, so the whole plan waits. No task can proceed meanwhile | do item a before the sandbox that writes the first task receives its token (10 10.2). Save each API answer in a file as you go: the pull request's Evidence needs them. Its output settles Q25 |
| O2 | T002 | item e (10 10.2): add the jobs of the green run to the default-branch ruleset as required status checks, and save the API's answer | the green run's job names | T003 directly; 100 of 102 tasks have it in their dependencies | one call; do it in the sitting that merges that task, and it adds no wait. No task can proceed meanwhile | one call, in the same sitting as the merge |
| O3 | T019, T020 | block A on both hosts, Intel and Apple silicon (11 11.1): `go run ./e2e/probes --block A --out docs/probes/`; then commit the results and the recordings on a branch, with the hook enabled, run `go run ./tools/ci fast`, push and open the pull request | `sbx` at or above 0.46.0 on both hosts; question 6 answered; the pins of the pin file | T026 directly; 53 of 102 tasks have it in their dependencies | about 75 minutes per host (11 11.1), plus the wait for a sitting with both machines. Meanwhile 29 later tasks can proceed | run both hosts in one sitting: the arch-sensitive probes (A4, A5, A11, A12, A13) count only with both (11, opening). Read the diff of the recordings before the push: it is data from your host |
| O4 | T055 | the catalog handover: build julieta for linux from `main`, run `julieta spec validate --catalog projects/*.yaml` on the reference config repo inside a Linux container, and hand over its output | a Linux container with network access to the config repo's origins; julieta has no darwin build (12 12.1) | T091 directly; 6 of 102 tasks have it in their dependencies | minutes; it can happen any time after its one task, so it is off the critical path if done by checkpoint C5. Meanwhile 41 later tasks can proceed | do it at C5, not at the end: it is the only input the catalog task waits for |
| O5 | every earlier task, T001 to T091 (91 in all) | the candidate: run `julieta spec validate --catalog` again and see it exit 0; the rehearsal on one host (below); then push the tag `v1.0.0-rc.1` (11 11.2) | the same container as O4; one host with `sbx` | O6 directly; 5 of 102 tasks have it in their dependencies | the rehearsal is about one block B run on one host, 90 minutes plus the sandbox-side checks (11 11.2). Meanwhile 6 later tasks can proceed | the rehearsal is this plan's addition, not the specification's. After the tag, a change outside `docs/` and the root Markdown files needs a new candidate and block B again (11 11.2) |
| O6 | O5 | block B on both hosts (11 11.2): B1 to B5 and the sandbox-side checks C1 to C5; then commit the results on a branch, with the hook enabled, run `go run ./tools/ci fast`, push and open the pull request | the candidate's published release; both hosts | T092 directly; 5 of 102 tasks have it in their dependencies | about 90 minutes per host for B1 to B5, plus the sandbox-side checks (11 11.2). No task can proceed meanwhile | run B1 first on each host: each other result copies the candidate's tag and commit from it (11 11.2) |
| O7 | T094, T095, T101 | pin `validate.yml` in the reference config repo to the release; push the tag `v1.0.0`; run B1 once more on v1.0.0, on one host, with `--out` outside the repository (11 11.2); run that `validate.yml` and note its run id (S4) | the documentation tasks merged, each of them; the config repo | T102 directly; 1 of 102 tasks have it in their dependencies | one B1 run and one workflow run. No task can proceed meanwhile | before tagging, check that the diff from the candidate's commit touches only `docs/` and root Markdown files: `tools/ci acceptance` fails otherwise (10 10.5) |

**What does not wait for block O3.** 49 of the 102 tasks have no block A
fact in their dependencies. 29 of them come after the tasks block O3
itself needs, and may proceed while the maintainer finds a sitting with
both hosts: T021 to T025, T031 to T036, T041, T051 to T063, T070, T078,
T084, T091. They include the first build layer, `catalog`, `oci`,
`state`, `memstore`, `egress` and `romeu init`, and julieta's chores up
to `ledger`'s event, which run at the container level and need no
recording.

**The rehearsal in O5.** Build with
`go run ./tools/release build --version v1.0.0-rc.1 --dry-run`, then,
on one host against that build, run
`go test -tags host -json ./e2e/host/...` and
`go run ./e2e/probes --block B --out <a directory outside the repository>`
without B1, which needs a published release. It is the first run of
the product against real `sbx`, and it includes C3, whose result can
change a kit (Q8). It is not a block B result and is committed nowhere.
We considered running it at checkpoint C7 instead: it needs the build
of T088, so it stays here.

**Results are a pull request.** Blocks O3 and O6 end with a branch, the
hook, `go run ./tools/ci fast` and a pull request, not with a push to
`main`. Two reasons. The recordings and results hold output of the
maintainer's real host, and the harness keeps only its own throwaway
sandboxes in them (T018). And this repository has names it
must not contain (index, Boundaries): a direct push by the bypass actor
is read by neither `hygiene`, the pushed range, nor `pr` (10 10.2).

**Outside the blocks.** Each merge waits for the maintainer (12 12.4),
and a pull request that touches an ask-first surface needs the
maintainer's words in an approval line (12 12.4). The "Ask-first"
field of each task is there so those words can be given per phase, at
the checkpoint, instead of one pull request at a time.

## Toolchain pins

Each pin is a reviewed diff. The table says where each lives and which
task lands it.

| Pin | Form | Section | Task |
|---|---|---|---|
| Go | version in `mise.toml`, locked in `mise.lock` | 12 12.1, 02 2.1 | T001 |
| `golangci-lint`, `govulncheck`, `reuse` | `mise.lock`, started through `mise exec` | 12 12.1, 10 10.2 | T001, T002 |
| `gh`, for `tools/release verify` | `mise.lock`, started through `mise exec` | 12 12.1, 10 10.2 | T088 |
| GitHub Actions | `uses` with a 40-hex commit SHA | 10 10.2 | T002, T088 |
| Go dependencies | `go.mod`, `go.sum`; the complete v1 list is in 12 12.1 | 12 12.1 | first use |
| kit frontend | `docker/sandbox-kit:3.0.0-m.<N>@sha256:<digest>`, in the pin file | 06 6.1 | T012 |
| workload | `<registry>/<repository>@sha256:<64 hex>`, in the pin file | 03 3.1, 06 6.2 | T012 |
| herdr | version and a sha256 per architecture, from an immutable release, in the pin file | 06 6.4 | T012 |
| mise | version and both linux sha256 values, in the pin file; the signed checksum file is verified at pin time | 07 7.4 | T052 |

**One pin file.** The frontend, the workload, herdr and mise are read
by more than one consumer: the probes, the container e2e, `layout` and
the kits. They live in one file, written by `tools/kitpin`, so what
block A probed is what the kits use. A pin that changes after block O3
needs the probe that read it run again.

**The rule for `gh`.** The rule asked of this plan is: `gh` is pinned
to an exact release with a sha256 per architecture, never to a floating
latest, and a freshness check compares the pin with the latest release.
The first two parts follow from 12 12.1 and 02 2.1: `gh` is "pinned in
`mise.lock`", and that lock covers four platforms. The freshness check
is not in the specification, and by the rule under
[What v1 leaves out](#what-v1-leaves-out) it enters through a change to
the specification first. No task here carries it; it is question 11.

## Phases and checkpoints

| Phase | Tasks | Modules | Checkpoint: what must be true before the next phase |
|---|---|---|---|
| 0 Bootstrap | T001 to T006 | `ci-bootstrap` | C0: the hook and CI run the same `tools/ci`; the rulesets refuse the sandbox's token (O1, O2); a pull request without its sections fails `pr`. The maintainer answers questions 3 to 8 here; questions 1 and 2 come before the first task |
| 1 The skeleton and the probe harness | T007 to T020 | `termsafe`, `romeu-cli`, `julieta-core`, `ci-release`, `kits`, `probes`, `canon` | C1: both binaries print `version` on the four runners; the handshake passes on the synthetic session; the harness passes `tools/ci all` before any host run (11, opening). Then block O3 |
| 2 First build layer | T021 to T025 | `spec`, `gitsafe`, `signing` | C2: block A results from both hosts are merged (T026), and `tools/ci probes` passes on them |
| 3 Block A applied; second layer | T026 to T035 | `probes`, `gitsafe`, `sbxdrv`, `romeu-cli`, `catalog`, `oci`, `state`, `memstore` | C3: the handshake passes on the recorded session on both Linux runners |
| 4 The `sync` slice | T036 to T050 | `egress`, `render`, `gate`, `romeu-cli` | C4: the J2 and J11 scenario functions run in CI with the fake `sbx`; the I27 hostile trees are refused on the macOS runners |
| 5 julieta's chores | T051 to T065 | `julieta-core`, `kits`, `layout`, `memstore`, `handoff`, `salvage`, `ledger` | C5: the container e2e passes on amd64 and arm64, with the `julieta setup` no-op inside its S8 bound. Block O4 is done by here. The maintainer takes here the decisions that T101 records, so that task has no one left to wait for |
| 6 `run` and the destructive commands | T066 to T075 | `romeu-cli` | C6: the five CI journeys of S7 pass; the meta-tests and the sweeps pass |
| 7 Kits, skills, the block B definitions and the diagrams | T076 to T083 | `kits`, `skills`, `romeu-cli`, `probes`, `state` | C7: `tools/ci kits` passes; romeu embeds the four product kits; a definition exists for each probe id of 11; the four diagrams of `ARCHITECTURE.md` are generated (T083), or question 10 says otherwise |
| 8 Release tooling | T084 to T091 | `ci-release`, `catalog` | C8: `go run ./tools/release build --version v1.0.0 --dry-run` succeeds; then block O5 |
| 9 Acceptance | T092 to T102 | `probes`, `docs`, `ci-release` | `go run ./tools/ci acceptance` exits 0 on the committed `docs/acceptance.json` |

A checkpoint is a statement about `main`, not a gate on starting: a task
of a later phase whose dependencies are merged may start before it. At
each checkpoint the maintainer reads the next phase's "Ask-first" fields
and the open rows of [Questions for the
maintainer](#questions-for-the-maintainer).

## Tasks

### Phase 0 - Bootstrap

#### T001 - Hygiene, the denylist, `fast` and the pre-push hook

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (step 1; "Forbidden
  names"), 00 0.4, 12 12.1, 12 12.4, S11.
- Depends on: nothing. Operator: block O1 finishes and pushes it.
  Ask-first: none exists yet; the maintainer's review is the guard (10
  10.2).
- Acceptance:
  - `go.mod`, `mise.toml` and `mise.lock` (Go and `golangci-lint`
    pinned, four platforms, 02 2.1), `COPYING`, `REUSE.toml` and
    `LICENSES/` exist (00 0.4).
  - `go run ./tools/ci hygiene` fails a fixture for each of its rules in
    10 10.2: U+2014, a listed prose word (with the self-test of
    `tools/ci/prose.yaml`), a `TODO` without an issue, a personal
    absolute path, a tracked `go.work` or `vendor/`, a second file in
    `.githooks/`, and a missing or empty denylist.
  - The matcher's self-test plants a made-up name written together, with
    each separator and with a slash, against its own denylist, and
    expects a failure for each form except the slash.
  - `hygiene add` refuses to start without a terminal and takes no name
    as an argument; a `/dev/ptmx` test drives it (10 10.1).
  - The pushed-range walk passes its fixture-repository table: a new
    ref, a ref the remote lacks, a URL that is no remote, a merge
    commit, a deleted ref (10 10.2).
  - `.githooks/pre-push` is the one line of 12 12.1, mode 100755.
- Verify: `go run ./tools/ci fast` exits 0 after block O1 adds the
  denylist entries, and exits non-zero before.

#### T002 - `tools/ci workflows`, `tools/ci all`, `coverage` and a green `ci.yml`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (step 2; the workflow
  grammar; Runners; the `coverage` step), S2, S9, S11.
- Depends on: T001, O1. Operator: block O2 follows its green run.
  Ask-first: none exists yet.
- Acceptance:
  - `tools/ci workflows` fails a fixture workflow for each construct
    outside the grammar of 10 10.2 (an `if`, a `shell`, an expression
    with an operator, a `uses` without a 40-hex SHA, a `run` line that
    is not `go run ./tools/ci` or `go run ./tools/release`, a file with
    another name ending), and fails a `ci.yml` without `edited`.
  - `go run ./tools/ci all` runs the steps of 10 10.2 that have code to
    check at this point: format and vet, lint (with `.golangci.yml` and
    the fixture test of ADR 0001 rule 14), unit, hygiene, workflows,
    `govulncheck`, the race run with its cover profile, `coverage`,
    `reuse lint`.
  - `tools/ci coverage` enforces the two thresholds of S9 and reports
    the packages below them. It lands here because `tools/...` is its
    first input.
  - `ci.yml` passes on `ubuntu-latest`, `ubuntu-24.04-arm`,
    `macos-latest` and an Intel macOS runner whose label is verified to
    exist and to report `x86_64` (10 10.2, Runners).
- Verify: the run id of the green workflow, under the pull request's
  Evidence.

#### T003 - The ask-first list, `tools/new adr` and `tools/ci generated`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 05 5.3, 12 12.3, 10 10.2 (step 3;
  the `generated` step).
- Depends on: T002, O2. Operator: no. Ask-first: none exists before this
  task.
- Acceptance:
  - `.github/ask-first.yaml` holds the surfaces of 05 5.3 and lists
    itself.
  - `go generate ./...` writes the three consumers of that file that
    have no later input: `.github/CODEOWNERS`,
    `docs/reference/ask-first.md` and, from the ADR titles and statuses,
    `docs/adr/README.md` (12 12.3). The `mutate` trigger, the fourth
    consumer, lands with `mutate` in T007.
  - `tools/ci generated` fails a changed, a new and a removed generated
    file, comparing content before and after the run (10 10.2).
  - `go run ./tools/new adr <title>` writes the next number, the date
    from the injected clock and the filename from `slug(title)`, status
    Proposed (12 12.3; ADR 0001 rule 6).
  - `.github/pull_request_template.md` has the five sections of 12 12.7.
- Verify: `go run ./tools/ci generated`; `go test ./tools/new/...`.

#### T004 - `tools/ci sequences`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (the `sequences` step and
  the id table), ADR 0001 rules 6 and 7.
- Depends on: T003. Operator: no. Ask-first: `checks`.
- Acceptance:
  - A fixture fails for each rule of the step: a gap in the ADR numbers,
    a filename that differs from `slug(Title)`, a status outside the
    set, a one-way supersede link, a cited record that is not Accepted,
    a Deferred decisions row with an empty cell, a duplicate id, a
    referenced id or id range that is not defined.
  - It passes on this repository's `docs/spec.md`, `docs/spec/` and
    `docs/adr/`.
- Verify: `go run ./tools/ci sequences`.

#### T005 - `tools/ci pr`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 12 12.4, 10 10.2 (the `pr` step
  and its inputs; it lands whole), 12 12.9 (the base branch), 12 12.10
  (the form of a fix marker), 10 10.4 (goldens under Evidence), ADR 0001
  rules 4, 8 and 11.
- Depends on: T003. Operator: no. Ask-first: `checks`.
- Acceptance:
  - Each line below is a fixture: a recorded event payload and a fixture
    repository (10 10.1).
  - A body without one of Why, What changed, Evidence, Lessons, or
    without a well-formed Middleware line, fails.
  - A touched ask-first surface without its approval line fails, with
    the list read at the base and at the head and joined; a line that
    names an unknown id fails; a body with CR LF line ends passes.
  - A changed golden whose repository path is not written verbatim under
    Evidence fails (10 10.4).
  - A title or a one-parent commit subject outside the Conventional
    Commit form fails; a merge commit's subject is not checked.
  - A prose-rule violation (ADR 0001 rule 4) in the title, in the body
    or in a commit message fails, one fixture per place.
  - A base other than the default branch fails.
  - A malformed `Fixes-release:` trailer fails.
  - The forbidden-name check runs over the title, the body, the head ref
    name and the four readings of each commit.
  - A payload with neither accepted shape, or without one of the seven
    fields, fails.
- Verify: `go run ./tools/ci pr <payload file>` on a saved payload.

#### T006 - Contributor page skeletons, the lessons file and `tools/ci lessons`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 12 12.8 (outlines), 12 12.7, 12
  12.3 (the `lesson` kind), 10 10.2 (the `lessons` step).
- Depends on: T005. Operator: no. Ask-first: `checks`.
- Acceptance:
  - `ARCHITECTURE.md` and `CONTRIBUTING.md` exist with front matter (ADR
    0001 rule 1) and the headings of their outlines.
  - `docs/lessons.md` exists with its first entry, what the bootstrap
    taught, written by `go run ./tools/new lesson`. The kind and the
    check land here because this entry is their first input (12 12.2).
  - `tools/ci lessons` fails an entry that names no existing subcommand
    or test and gives no "no check possible" reason.
- Verify: `go run ./tools/ci lessons`; `go test ./tools/new/...`.

### Phase 1 - The skeleton and the probe harness

#### T007 - `termsafe`, the CLI writer and the invariant tooling

- [ ] Merged
- Module: `termsafe`. Implements: 05 5.2 (I29, the tagging rules), 12
  12.2 (the `termsafe` row), 12 12.3 (the `invariant` kind; the `mutate`
  trigger), 10 10.2 (`invariants`, `mutate`), S6.
- Depends on: T006. Operator: no. Ask-first: `termsafe`, `checks`.
- Acceptance:
  - A table test has one row per escaped class of I29 (control, bidi,
    zero-width, tag characters, U+2028 and U+2029); a fuzz target exists
    with committed seeds (10 10.1).
  - The CLI writer (`internal/cli/writer.go`) escapes by default (04
    4.1, Output).
  - `go run ./tools/new invariant` scaffolds a tagged guard stub and a
    failing tagged test.
  - `tools/ci invariants` fails a guard tag without a test tag, the
    reverse, and a tagged id that is no row of 05 5.2.
  - `tools/ci mutate` builds the `mutate_I29` stub and a tagged test
    fails; it reads `.github/ask-first.yaml` to decide which pull
    requests it runs on (05 5.3).
- Verify: `go test ./internal/termsafe/...`; `go run ./tools/ci
  invariants`.

#### T008 - The CLI contract: the error table, exit codes and the dispatcher

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.1, 10 10.6, 12 12.3 (the
  error-table row).
- Depends on: T007. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - Landed early; see "The first slice".
  - An error has an id `RJ-<exit><nn>`, a slug, a message and a fix hint
    in one data table; the text and `--json` forms match 04 4.1; tests
    assert ids, not message text.
  - `go generate` writes the three consumers of the table (12 12.3): the
    exit-code sections of `--help`, `docs/reference/errors.md` and
    `docs/reference/exit-codes.md`, the pages with the generated-file
    header and front matter.
- Verify: `go test ./internal/cli/...`; `go run ./tools/ci generated`.

#### T009 - Two binaries that print their version

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.1 (Parsing, Version), 04 4.2,
  04 4.3, 12 12.3 (the `command` kind, command definitions), 10 10.2
  (cross-build).
- Depends on: T008. Operator: no. Ask-first: `checks`.
- Acceptance:
  - `julieta-core` and the `version` command of `romeu-cli` land early;
    see "The first slice".
  - `romeu version` and `julieta version [--json]` print the fields of
    04 4.1; the julieta `--json` form reports whether its own directory
    is writable.
  - The cross-build step builds `romeu` for darwin amd64 and arm64 and
    `julieta` for linux amd64 and arm64.
  - `go run ./tools/new command` writes a stub, its help test and its
    reference entry; `docs/reference/` gains one generated page per
    command (12 12.3).
- Verify: `go run ./tools/ci all` on the four runners.

#### T010 - `tools/ci imports`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.7, 10 10.2 (the `imports`
  step), 05 5.2 (I1, I2).
- Depends on: T009. Operator: no. Ask-first: `checks`.
- Acceptance:
  - The check enforces the table of 10 10.7 over the packages that
    exist, and a fixture package fails for each column.
  - It carries the I1 and I2 guard tags, each with a one-violation
    fixture as its tagged test (05 5.2): `go list -deps ./cmd/romeu`
    holds none of `internal/layout`, `internal/hooks`, `internal/mise`.
- Verify: `go run ./tools/ci imports`.

#### T011 - `tools/ci vocabulary`

- [ ] Merged
- Module: `ci-release`. Implements: 01 1.7, 10 10.2 (the `vocabulary`
  step), 12 12.3 (the vocabulary row).
- Depends on: T009. Operator: no. Ask-first: `checks`.
- Acceptance:
  - The denylist is generated from the table of 01 1.7.
  - A "Not" phrase in a doc, in help text or in an error message fails;
    `docs/reviews/` and the "Not" column itself are exempt.
- Verify: `go run ./tools/ci vocabulary`.

#### T012 - `tools/kitpin` and the one pin file

- [ ] Merged
- Module: `kits`. Implements: 06 6.1 (Frontend), 06 6.4 (herdr), 03 3.1
  (the workload reference), 02 2.1 (`tools/kitpin`).
- Depends on: T006. Operator: no. Ask-first: `kits`.
- Acceptance:
  - Landed early; see "The first slice" and "Toolchain pins".
  - One committed data file under `kits/` holds the pins that more than
    one consumer reads: the kit frontend with its digest, the workload
    by digest, and herdr with a sha256 per architecture. The harness,
    the container e2e, `layout` and the kits read that file, so a probe
    and a kit cannot disagree on a pin.
  - `kitpin frontend` rewrites the frontend pin; `kitpin herdr
    <version>` takes each hash from the release API's digest and refuses
    a release that is not marked immutable (06 6.4). Each has a test
    against a recorded fixture.
  - The workload entry is written in this pull request by hand, since
    the specification gives `tools/kitpin` no subcommand for it;
    `julieta pin workload` moves it in a spec later (06 6.2).
  - The file says, in a comment the check of T020 cannot make: a pin
    that changes after block O3 needs the probe that read it run again.
- Verify: `go test ./tools/kitpin/...`.

#### T013 - The container e2e, on the workload's base

- [ ] Merged
- Module: `julieta-core`. Implements: 10 10.1 (E2E container), 12 12.1.
- Depends on: T009, T012. Operator: no. Ask-first: `checks`.
- Acceptance:
  - The container e2e runs `julieta version --json` in the workload's
    Debian base, pulled by the digest of the pin file, on
    `ubuntu-latest` and `ubuntu-24.04-arm`, native.
  - A failed image pull is reported as an infrastructure error, not as a
    test failure, the way 10 10.1 treats a failed mise download.
  - `tools/ci all` selects the step from the OS and architecture it runs
    on (10 10.2).
- Verify: `go test -tags e2e ./e2e/...` on a Linux runner.

#### T014 - Probe harness core and the result format

- [ ] Merged
- Module: `probes`. Implements: 11 (opening), 11 11.3, 12 12.3 (the
  `probe` kind), 10 10.1 (Schema), 03 (opening).
- Depends on: T006. Operator: no. Ask-first: `checks`, `dependencies`
  (the JSON Schema validator of 12 12.1).
- Acceptance:
  - The pure core computes `verdict` and `decision` from declared
    `onPass`, `onFail` or a decision table; a typed decision is refused
    (11 11.3).
  - `tools/schemagen` writes `schemas/probe-result.v1.json` from the Go
    type; `tools/ci schema` fails when the generated schema differs from
    the committed one and validates the examples.
  - `go run ./tools/new probe` writes a definition with `expect`, a
    decision table and `affects` to fill.
- Verify: `go test ./e2e/probes/...`; `go run ./tools/ci schema`.

#### T015 - The fake `sbx`

- [ ] Merged
- Module: `probes`. Implements: 10 10.3, 05 5.2 (I4, I25).
- Depends on: T014. Operator: no. Ask-first: none expected.
- Acceptance:
  - The map names no owner for `e2e/fakesbx`; see [Questions for the
    maintainer](#questions-for-the-maintainer).
  - On recordings of its own test data: an argv shape absent from the
    recording fails the test with the argv (Replay only); a call out of
    the recorded order fails (Stateful replay); for `env exec`, the argv
    before `--` is matched and the command after it is forwarded.
  - It exits 99 on `--auto-approve` (I4) and on a value form of `sbx
    secret set` (I25), whatever the recording holds.
  - The placeholder normalizer has a unit test (10 10.1).
- Verify: `go test ./e2e/fakesbx/...`.

#### T016 - The version handshake, on a synthetic session

- [ ] Merged
- Module: `romeu-cli`. Implements: 06 6.3, 05 5.2 (I30), 10 10.1 (E2E
  hybrid).
- Depends on: T013, T015. Operator: no. Ask-first: none expected.
- Acceptance:
  - At the hybrid level on both Linux runners, the compatibility check
    passes for a julieta of an accepted protocol, with the expected
    sha256, in a read-only directory; a protocol mismatch, a
    `SHA256SUMS` mismatch and a writable bin mount each fail it with
    exit 2.
  - The session the fake `sbx` replays is hand-written, lives in the
    test's own data and is named `synthetic`; it is not under
    `e2e/testdata/sbx/<version>/`, which holds block A recordings only
    (10 10.3). T030 replaces it.
  - `internal/sbxdrv` gains the two argv builders the check needs, `sbx
    version` and `sbx env exec`. They are the first rows of the argv
    table that T019 completes.
  - The test supplies `render.json` and `SHA256SUMS` as fixtures;
    `render` writes them from T039 on.
  - The function is **dark** until T066 calls it.
- Verify: `go test -tags e2e ./e2e/...`.

#### T017 - `canon`

- [ ] Merged
- Module: `canon`. Implements: 03 3.12.
- Depends on: T008. Operator: no. Ask-first: `digests`.
- Acceptance:
  - `canon.Digest(kind, v)` hashes `"romeu/<kind>/v1"`, a NUL byte and
    the canonical JSON; a test shows that two kinds over one value
    differ.
  - Each kind of the table in 03 3.12 is a typed constant, and a value
    of one kind does not compile against a digest of another.
- Verify: `go test ./internal/canon/...`.

#### T018 - Probe exec layer and the recorder

- [ ] Merged
- Module: `probes`. Implements: 11 (opening), 10 10.3 (Source of truth,
  Redaction), index (Boundaries, Never).
- Depends on: T014. Operator: feeds block O3. Ask-first: none expected.
- Acceptance:
  - The exec layer is tested against a helper binary re-executed from
    the test (10 10.1).
  - The recorder's test plants a fake token and a home path and asserts
    that neither reaches the file; each U+2014 becomes `-` (10 10.3).
  - The recorder keeps, of an `sbx ls --json`, a policy listing or a
    settings read, only the entries of the harness's own throwaway
    sandboxes, env dirs, global secret and global allow rule (the last
    two are planted by A9 under the harness's prefix), and replaces the
    rest with a count. A test plants a foreign sandbox name and a
    foreign workspace path and asserts that neither reaches the file; a
    second test plants a throwaway global secret and allow rule of the
    harness beside a foreign pair and asserts that the harness's own
    survive the filter and the foreign pair does not. The reason: those
    reads come from the maintainer's real host, and a name on it may be
    one this repository must not hold (index, Boundaries).
  - It keeps stdout only for the allowlist of read commands whose output
    the parsers need, and no secret output (10 10.3).
- Verify: `go test ./e2e/probes/...`.

#### T019 - The block A definitions, the argv table and the goldens

- [ ] Merged
- Module: `probes`. Implements: 11 11.1, 10 10.3 (Replay only, Stateful
  replay, Known argv), 04 4.2, 07 7.5 (Application), 03 3.3, 10 10.4.
- Depends on: T018, T012, T017, T016. Operator: question 6 is answered
  first; feeds block O3. Ask-first: none expected.
- Acceptance:
  - A definition exists for each probe id of 11 11.1: A1, A2, A3, A4,
    A5, A6, A9, A10, A11, A12, A13 and A14, each with its expectation
    written before any run. A3's expectation follows the maintainer's
    answer to question 6.
  - A committed table lists the `sbx` argv shapes romeu needs: the ones
    04 4.2 names for `sync`, `run`, `adopt`, `stop`, `salvage`, `rm`,
    `retire`, `status`, `doctor` and `pull`, and the three policy
    commands of 07 7.5. It also lists the sessions the fake `sbx` starts
    from, derived from every end-to-end test of this plan that calls the
    fake, not from the journeys alone: the five CI journeys J2, J3b, J7,
    J10 and J11 (10 10.1) and the tests of T042, T043, T044, T045, T047,
    T048, T050, T066, T067, T068, T069, T071, T073, T074, T075 and T081,
    each with the state its sandbox starts in: removed, stopped or
    created outside romeu (10 10.3). That covers `adopt`, `stop` and
    `pull`, `salvage` alone and `recreate`, whose call orders no journey
    session holds, and stateful replay fails a call order no session
    holds.
  - A test fails when a row of that table is exercised by no block A
    definition. A shape block A did not record cannot be replayed, and
    recording it later means a second sitting on both hosts.
  - The `sbxenv.yaml` goldens that A5 runs `sbx env plan` on are written
    here, under `internal/render/testdata/`, in the shape of 03 3.3,
    with kit directory names computed by `canon`'s `kit-tree` digest
    over fixture kits. T037 must produce the same bytes.
  - A11, A12 and A13 read the frontend, the workload and herdr from the
    pin file of T012.
  - A9 plants a throwaway global secret, set with `--command` (I25), and
    a throwaway global allow rule, both under the harness's own prefix,
    reads them as `doctor` would, and removes them. Its result therefore
    holds a global secret and a broad allow rule for the doctor fixtures
    of T046 and T049 to fail on (I22 of 05 5.2 takes its doctor fixtures
    "from block A recordings").
- Verify: `go run ./e2e/probes --block A --out <dir>` against the helper
  binary in a test.

#### T020 - `tools/ci probes`

- [ ] Merged
- Module: `probes`. Implements: 11 11.4, 10 10.2 (the `probes` step), 10
  10.4.
- Depends on: T014. Operator: no. Ask-first: `checks`.
- Acceptance:
  - A fixture per row of the lifecycle table of 11 11.4: a result whose
    `decision` differs from the computed one fails; an overturned result
    fails until `resolvedBy` names a commit that descends from the
    result's `commit`, touches each `affects` path and adds an ADR.
  - A golden whose sha256 moved since its A5 result is flagged, and one
    with no recorded hash is not.
- Verify: `go run ./tools/ci probes`.

### Phase 2 - First build layer

These tasks need no `sbx` fact and run while block O3 is under way (12
12.2, the build order).

#### T021 - `spec`: the project spec and host settings

- [ ] Merged
- Module: `spec`. Implements: 03 3.1, 03 3.2, 03 3.4, 01 1.4 (the struct
  tags), 12 12.3, S10, 05 5.2 (I8, I11, I12).
- Depends on: T014, T017. Operator: no. Ask-first: `gates`
  (`internal/spec/project.go`), `contracts`
  (`internal/spec/versions.go`).
- Acceptance:
  - Strict decode: an unknown key fails; `command`, `argv` or `env`
    under `secrets` fails (I8).
  - The validators are generated from `rules.go`; a table and a fuzz
    target with committed seeds cover the name and path rules of 03 3.1
    (I12), and a YAML round-trip fuzz covers each string field (I11).
  - `schemas/project.v1.json` and `schemas/host-settings.v1.json` are
    generated.
  - The generated reference holds the field table of
    `docs/reference/project.md`, from the `gate` and `apply` tags, and
    the rule table, from `rules.go` (12 12.3).
  - A reference page per file format is generated with its schema (S10):
    this task writes the generator and the pages of its two formats, and
    each later task that adds a schema adds its page through it.
- Verify: `go test ./internal/spec/...`; `go run ./tools/ci schema`.

#### T022 - `spec`: the run layout and the examples

- [ ] Merged
- Module: `spec`. Implements: 03 3.2 ("Run layout"), 02 2.1
  (`examples/`), 03 (opening).
- Depends on: T021. Operator: no. Ask-first: `gates` (if `project.go`
  changes).
- Acceptance:
  - The validator refuses a pane id used twice, two focused panes in a
    tab, a ratio outside 0.1 to 0.9, an `env` key on the refused list
    and a `cwd` with a `..` element.
  - `examples/` holds an example config repo whose files validate
    against the generated schemas in `tools/ci schema`.
- Verify: `go test ./internal/spec/...`.

#### T023 - `gitsafe`: the hardened runner, fetch and refs

- [ ] Merged
- Module: `gitsafe`. Implements: 05 5.1, 05 5.2 (I3, I5, I6), 02 2.3
  (the ref namespaces).
- Depends on: T008. Operator: no. Ask-first: `gitsafe`.
- Acceptance:
  - A test asserts the environment and the flags of 05 5.1 on each
    invocation, and that nothing ambient passes through (I3).
  - A `git://` URL whose host is not `127.0.0.1`, a `file://` URL and a
    plain path are refused, with a `caFile` set too.
  - The refspec validator table refuses a destination outside the
    allowlisted prefixes (I5); `CreateRef` fails on an existing ref
    (I6).
  - `floor.go` holds the initial floor 2.45.4; T027 fills the per-series
    list from A1.
- Verify: `go test ./internal/gitsafe/...`.

#### T024 - `gitsafe`: bundles, fsck and the tree walk

- [ ] Merged
- Module: `gitsafe`. Implements: 05 5.1, 05 5.2 (I27, unit level).
- Depends on: T023. Operator: no. Ask-first: `gitsafe`.
- Acceptance:
  - The table of hostile trees of I27 is refused by `WalkTree` without
    relying on fsck: a symlink, a gitlink, the names `.`, `..` and
    `.git` with their case and HFS-ignorable variants, and a case
    collision.
  - `bundle verify` and `unbundle` run under the hardened flags.
- Verify: `go test ./internal/gitsafe/...`.

#### T025 - `signing`

- [ ] Merged
- Module: `signing`. Implements: 06 6.4 (the signing rule's check), 12
  12.1.
- Depends on: T008. Operator: no. Ask-first: `signing`.
- Acceptance:
  - Against a test agent socket, the identity listing returns the keys;
    a socket that is unset, unreachable, or holds a number of keys other
    than one, or one that differs from the given key, is reported as
    such.
  - No dependency and no subprocess is used.
  - The code is **dark** until T081.
- Verify: `go test ./internal/signing/...`.

### Phase 3 - Block A applied; second layer

#### T026 - The block A results and recordings

- [ ] Merged
- Module: `probes`. Implements: 11 11.1, 11 11.3, 11 11.4, 10 10.3, 10
  10.2 ("Forbidden names").
- Depends on: O3. Operator: block O3 produces it; the maintainer opens
  it. Ask-first: none for results alone; each surface a resolution
  touches.
- Acceptance:
  - The pull request holds the `probe-result.v1` files under
    `docs/probes/`, from both hosts, and the recordings under
    `e2e/testdata/sbx/<version>/`, and nothing else.
  - It reaches `main` like any change, not by a direct push: a branch in
    the maintainer's plain clone with the hook enabled, `go run
    ./tools/ci fast` before the push, then a pull request, so `hygiene`,
    the pushed range and `pr` read host data before it is public. A push
    by the bypass actor to `main` would be read by none of them.
  - `tools/ci probes` passes. A result with `decision:
    default-overturned` keeps that step red until it is resolved (11
    11.4), so the commit that applies it, its ADR and the `resolvedBy`
    commit join this pull request; that is the one case where it holds
    more than results.
  - The sha256 of each `sbxenv.yaml` golden is in the A5 result.
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes`.

#### T027 - The git floor from A1

- [ ] Merged
- Module: `gitsafe`. Implements: 05 5.1 (the host git floor), index
  (Open questions, Q10), 12 12.5.
- Depends on: T026, T023. Operator: no. Ask-first: `gitsafe`,
  `decisions`.
- Acceptance:
  - `internal/gitsafe/floor.go` holds the per-series minimums A1
    verified, with sources.
  - Q10 leaves the Open questions table and becomes an ADR.
- Verify: `go test ./internal/gitsafe/...`; `go run ./tools/ci
  sequences`.

#### T028 - The records of the questions block A settles

- [ ] Merged
- Module: `probes`. Implements: index (Open questions), 11 11.4, 12
  12.5.
- Depends on: T026. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - Each of the six questions that block A settles alone, by the
    "Settled by" column of the index, becomes an ADR and leaves the
    table: Q7 (A11), Q15 (A4), Q16 (A13), Q18 (A10), Q20 (A12) and Q21
    (A4).
  - A question whose probe is arch-sensitive is settled only with a
    result from each host (11, opening).
  - The review may ask for one pull request per record; the task is the
    six.
- Verify: `go run ./tools/ci sequences`.

#### T029 - `sbxdrv`

- [ ] Merged
- Module: `sbxdrv`. Implements: 10 10.3 (Parsers, Known argv), 04 4.1
  (Subprocesses), 05 5.2 (I4, I25), 12 12.1 (the version floor).
- Depends on: T015, T026, T021. Operator: no. Ask-first: none expected.
- Acceptance:
  - Each argv builder produces a shape present in a block A recording;
    the table of T019 has a builder per row.
  - A scan finds no builder that emits `--auto-approve` (I4); the
    builder table rejects a value form of `sbx secret set` (I25).
  - The parsers run over each recorded `sbx` version.
  - `sbx` runs by absolute path with the scrubbed environment of 04 4.1
    and a timeout (10 10.6).
- Verify: `go test ./internal/sbxdrv/...`.

#### T030 - The version handshake, on the recorded session

- [ ] Merged
- Module: `romeu-cli`. Implements: 06 6.3, 05 5.2 (I30), 10 10.3 (Source
  of truth).
- Depends on: T029, T016. Operator: no. Ask-first: none expected.
- Acceptance:
  - The hybrid test of T016 replays the A4 recording of `sbx env exec`,
    and the synthetic session is deleted in the same pull request.
  - A test over `e2e/` fails when the fake `sbx` is given a session from
    anywhere but `e2e/testdata/sbx/<version>/`, so a hand-written
    session cannot return after this task.
- Verify: `go test -tags e2e ./e2e/...`.

#### T031 - `catalog`

- [ ] Merged
- Module: `catalog`. Implements: 03 3.5, 07 7.6, 10 10.2 (the `catalog`
  step).
- Depends on: T021. Operator: no. Ask-first: `catalog`, `checks`.
- Acceptance:
  - `catalog/egress.yaml` is embedded and decoded strictly;
    `schemas/catalog.v1.json` is generated.
  - `tools/ci catalog` fails a domain used anywhere without an explicit
    `upload` entry, and a malformed key.
  - The entries are the examples of 03 3.5; T091 builds the v1 table.
- Verify: `go run ./tools/ci catalog`.

#### T032 - `oci`

- [ ] Merged
- Module: `oci`. Implements: 01 1.4 (the normalized capability set), 06
  6.1 (Capabilities), 05 5.2 (I28).
- Depends on: T021. Operator: no. Ask-first: none expected.
- Acceptance:
  - The `oci` table of I28 is refused row by row: a digest mismatch at
    each hop, an HTTP URL, an HTTPS-to-HTTP redirect, an oversize body,
    a credential helper in a fake Docker config.
  - The strict grammar refuses an anchor, an alias, a merge key, an
    include and an unknown key or capability type; a fuzz target covers
    the descriptor grammar (10 10.1).
  - The conformance fixtures of A11 and A12 are parsed in T038, which
    waits for them.
- Verify: `go test ./internal/oci/...`.

#### T033 - `state`: records, atomic writes and the lock

- [ ] Merged
- Module: `state`. Implements: 02 2.4, 03 3.8, 04 4.1 (Locking).
- Depends on: T017, T021. Operator: no. Ask-first: `gates`.
- Acceptance:
  - A record is written by temp file, fsync and rename, mode 0600; the
    state directory is 0700.
  - A second process that takes `romeu.lock` exits 1 with `RJ-101`.
  - `toolchain.json`, the project record and the descriptor cache have
    generated `state-*.v1` schemas.
- Verify: `go test ./internal/state/...`.

#### T034 - `state`: the candidate and generation machines

- [ ] Merged
- Module: `state`. Implements: 01 1.6, 12 12.3 (the transition-table
  row), 05 5.2 (I7 and I17, unit level).
- Depends on: T033. Operator: no. Ask-first: `gates`.
- Acceptance:
  - Each machine is a transition table; the generated tests exercise
    each row and assert each missing pair illegal, with exit 2 and an
    error id.
  - The candidate and generation diagrams in `ARCHITECTURE.md` are
    generated from the tables. The promotion-commit and probe diagrams
    are the task of T083, by question 10.
- Verify: `go test ./internal/state/...`; `go run ./tools/ci generated`.

#### T035 - `memstore`: the shared readers

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1, 05 5.2 (I12, I24), 03 3.9.
- Depends on: T017, T021. Operator: no. Ask-first: none expected.
- Acceptance:
  - The allowlist table refuses, through `os.Root` and Lstat, a symlink,
    a FIFO, a dotfile, a `.git` directory, a `..` name and a top-level
    name outside the five of 08 8.1; the caps are enforced.
  - `tools/ci imports` fails a romeu-linked package that imports
    `memstore/write` (I24).
  - `schemas/memory-entry.v1.json` is generated.
- Verify: `go test ./internal/memstore/...`.

### Phase 4 - The `sync` slice

#### T036 - `egress`

- [ ] Merged
- Module: `egress`. Implements: 07 7.5 (derivation), 07 7.3, 05 5.2 (I9,
  I10).
- Depends on: T023, T031. Operator: no. Ask-first: `dependencies` (the
  TOML parser of 12 12.1).
- Acceptance:
  - The eight steps of 07 7.5 have a table test: a `mise.toml` without a
    lock is an error; an unknown `backend:tool` without a spec entry is
    exit 2 naming the tool; a lock host the catalog did not produce is
    gated.
  - A domain with `upload: true` or with no `domains` entry is gated,
    for catalog, base and kit-declared domains (I10).
  - The lock is read with `git cat-file` at `egressCommit`, and no mise
    process runs (07 7.1; I9).
- Verify: `go test ./internal/egress/...`.

#### T037 - `render`: `sbxenv.yaml` and `render.json`

- [ ] Merged
- Module: `render`. Implements: 03 3.3, 03 3.7, 13 13.1 (the fixed
  mounts), 10 10.4, 10 10.2 (`golden`), 05 5.2 (I8, I11).
- Depends on: T026, T032, T036, T019. Operator: no. Ask-first: `checks`.
- Acceptance:
  - The renderer produces, byte for byte, the goldens T019 wrote and A5
    probed: the shape of 03 3.3, the three fixed mounts, no `x-*` key,
    no `lifecycle` hook. A difference is not fixed by `-update`: it
    moves a hash A5 recorded, `tools/ci probes` flags it, and A5 runs
    again on both hosts.
  - The secret command is built by `internal/shquote` (table-tested) in
    the `/usr/bin/env -i` form; no literal value is rendered (I8).
  - `tools/ci imports` rejects `text/template` in `internal/render`
    (I11); `tools/ci golden` fails on `-update` in CI and lists the
    changed goldens.
  - `schemas/render.v1.json` is generated.
- Verify: `go test ./internal/render/...`; `go run ./tools/ci golden`.

#### T038 - `render`: kit materialization and the workspace files

- [ ] Merged
- Module: `render`. Implements: 06 6.1 (Materialization, Capabilities,
  Personal kits), 01 1.4, 02 2.3, 05 5.2 (I13, I27, I28).
- Depends on: T037, T024. Operator: no. Ask-first: none expected.
- Acceptance:
  - A personal kit is read from the config commit's tree through `git
    fsck --strict` and `WalkTree`, and written through `os.Root` into
    the candidate only (I27).
  - A personal kit that declares a network, mount, volume, credential or
    ssh-agent capability is a sync error; another capability type is
    kept for the gate diff.
  - Each conformance fixture A11 and A12 recorded, one per capability
    type, parses to the recorded capability set, in `internal/render`
    for local kits and in `internal/oci` for the workload (I28).
  - Each workspace file decodes to the key set {`folders`}; a memory
    directory is in neither, and a review checkout is only in
    `review.code-workspace` (I13).
  - Product kits come from an embed seam that tests fill with fixture
    kits; T077 fills it with the real ones.
- Verify: `go test ./internal/render/...`.

#### T039 - `render`: the promotion commit, recovery and drift

- [ ] Merged
- Module: `render`. Implements: 01 1.6 ("Promotion commit"), 01 1.2 (the
  drift rule), 06 6.3 (step 1), 05 5.2 (I15, I16).
- Depends on: T034, T038. Operator: no. Ask-first: `checks` (the
  admitted-call-site table).
- Acceptance:
  - The four steps run in order; a test hook stops after each, and
    recovery finishes a promotion whose candidate verifies against its
    `filesDigest`, or refuses with exit 4 when it does not.
  - A hand edit of `sbxenv.yaml`, of a kit file or of
    `.romeu/bin/SHA256SUMS` is reported as drift (I15).
  - The I16 scan fails each listed call kind outside a function the
    admitted-call-site table names (05 5.2).
- Verify: `go test ./internal/render/...`.

#### T040 - `gate`

- [ ] Merged
- Module: `gate`. Implements: 01 1.4 (gate 2), 12 12.3 (the struct-tag
  row), 05 5.2 (I7, I29).
- Depends on: T039. Operator: no. Ask-first: `gates`.
- Acceptance:
  - The widening set and the recreate digest field lists are generated
    from the struct tags; the generated flip table shows that each
    `gate:"widening"` field flips the digest and each untagged field
    does not (I7).
  - The diff is not truncated: a 10 000-line kit file change appears in
    full, the summary is repeated above the prompt, and a binary file is
    shown as size and sha256, flagged (I29).
  - The prompt needs a terminal; a `/dev/ptmx` test drives `y` and `N`
    (10 10.1).
- Verify: `go test ./internal/gate/...`.

#### T041 - `romeu init`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`init`), 09 (J1, step 3), 03
  3.4, 05 5.2 (I20).
- Depends on: T023, T033. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - In the e2e with origins served over TLS (10 10.1), `init` writes
    host settings with absolute tool paths and the real root path, and
    clones the config repo to `<root>/<cfg>-env/<dir>`.
  - It refuses to overwrite existing settings.
  - It refuses a root inside a git repository, equal to `$HOME`, or
    holding host settings or state (I20).
  - `e2e/scenarios` starts here: the package, `commands.yaml` with the
    `romeu init` line, the J1 scenario function up to its step 3, which
    reads that line, and the package's own test, which fails a table
    entry no function reads (ADR 0001 rule 13). Each later task that
    adds a host-facing command adds its journey's function and command
    lines (10 10.8). The package needs no `sbx` fact, so the tasks that
    add a julieta-side journey function wait for this one and not for
    block O3.
- Verify: `go test -tags e2e ./e2e/...`.

#### T042 - Gate 1: the toolchain acknowledgement

- [ ] Merged
- Module: `romeu-cli`. Implements: 01 1.4 (gate 1), 04 4.2 (`approve
  --toolchain`), 03 3.8, 09 (J9), 05 5.2 (I7).
- Depends on: T029, T033, T031. Operator: no. Ask-first: `gates`,
  `contracts`.
- Acceptance:
  - `approve --toolchain` needs a terminal, prints the julieta version
    change and the catalog diff per affected project before the prompt,
    and writes `toolchain.json`.
  - A unit table shows that gate 1 flips on a different `sbx` version, a
    different romeu version and a different catalog digest (I7).
  - The check function is the one the preflight and `sync` call;
    `status` and `doctor` report a mismatch instead of exiting 3.
- Verify: `go test -tags e2e ./e2e/...`.

#### T043 - `romeu sync`: render, gate and promote

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("How `romeu sync` renders,
  gates and promotes", steps 1 to 8 and 10), 01 1.6 ("Candidate"), 09
  (J2), 10 10.3 (Shared scenarios), ADR 0001 rule 13, 05 5.2 (I5, I7,
  I8, I9, I12).
- Depends on: T035, T040, T041, T042. Operator: no. Ask-first:
  `contracts`, `gates`.
- Acceptance:
  - In the e2e with the fake `sbx`, a first `sync` on a terminal
    promotes after `y`, and leaves the live files byte-identical after
    `N` (I7).
  - A stale toolchain makes `sync` exit 3 at step 1, before it promotes.
  - `--from <sha>` refuses a commit not reachable from
    `refs/romeu/origin/*`.
  - A spec secret without its `name@project` binding is exit 2 naming
    the key, and `github@other` does not satisfy project `shop` (I8).
  - An uncommitted lock change, a commit on another branch and a local
    `refs/remotes/origin` rewrite leave egress unchanged; changing `ref`
    changes egress and meets gate 2 (I9).
  - After `sync`, `refs/heads`, `refs/tags`, `refs/remotes` and `HEAD`
    of each clone are unchanged (I5).
  - An absent project is reported as `orphaned`, and nothing is removed.
  - The J2 scenario function joins `e2e/scenarios` (started in T041) and
    reads its `commands.yaml` lines.
- Verify: `go test -tags e2e ./e2e/...`.

#### T044 - `romeu sync` against hostile input

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`sync` steps 5 and 6), 05
  5.1, 05 5.2 (I12, I24, I27, I28).
- Depends on: T043. Operator: no. Ask-first: none expected.
- Acceptance:
  - A memory directory with a symlink, a FIFO or a `..` name makes
    `sync` exit 2 and is not followed (I12, I24).
  - A personal kit with each hostile entry of I27 fails `sync` before a
    file is written outside `.romeu/candidates/`, on the macOS runners
    too.
  - A hostile tree delivered by a bundle is refused as `--from <sha>`,
    and refused by the walk when it is made reachable (I27).
  - Adding a capability to a kit flips gate 2 (I28).
- Verify: `go test -tags e2e ./e2e/...` on the four runners.

#### T045 - `romeu approve <name>` and the awaiting candidate

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`approve`), 01 1.6
  ("Candidate"), 09 (J11, step 5), 05 5.2 (I7).
- Depends on: T043. Operator: no. Ask-first: `contracts`, `gates`.
- Acceptance:
  - Without a terminal `sync` records an awaiting candidate and exits 3;
    `approve <name>` on a terminal promotes it and prints that live
    changes apply on the next `run` or `sync`.
  - `approve` without a terminal, or with nothing awaiting, exits 2;
    after the candidate files changed it exits 4 (I7).
  - A new `sync` supersedes an awaiting candidate, and a candidate dir
    the record does not name is deleted by the next `sync`.
- Verify: `go test -tags e2e ./e2e/...`.

#### T046 - The preflight

- [ ] Merged
- Module: `romeu-cli`. Implements: 01 1.5, 04 4.2 (the rows of the
  doctor table marked **pre**), 05 5.2 (I15, I22, I23, I26).
- Depends on: T045. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - The preflight is one function that runs its six steps in the order
    of 01 1.5 and stops at the first failure with the exit of its row. A
    table test covers each step.
  - Step 5 holds the checks marked **pre** that this phase can build:
    the `sbx` globals, from block A recordings (I22), and the symlink,
    mise, direnv and VS Code trust rows, with `HOME` in a temp dir
    (I23). The signing row lands with T081.
  - Step 6 refuses a sandbox romeu has no open generation for, with
    `RJ-203` naming `romeu adopt`, and one whose workspace path differs
    (I26).
  - It touches no network: it passes with the registry and the origins
    unreachable (10 10.1).
  - No **P** command exists yet; T047 is its first caller.
- Verify: `go test ./internal/... -run Preflight`.

#### T047 - The live changes of `sync`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`sync` step 9), 07 7.5
  (Application), 09 (J11), 05 5.2 (I7, I26).
- Depends on: T046. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - With a running sandbox, `sync` runs the rest of the preflight, then
    reconciles egress with the recorded argv and verifies it with `sbx
    policy check`.
  - A gate 1 or a gate 2 change makes a `sync` whose sandbox is running
    exit 3 before a mutating call (I7); a foreign sandbox of the
    project's name makes it exit 2 with `RJ-203` (I26).
  - The `secrets` field follows its `apply` tag: recreate-class, or `sbx
    secret set --command` if the A10 result overturned the default
    (Q18).
  - It reports "recreate needed" when the recreate digest differs from
    the running generation.
  - The J11 scenario function runs in CI.
- Verify: `go test -tags e2e ./e2e/...`.

#### T048 - `romeu status`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`status`), 09 (J7 step 2, J10
  step 1, J13 step 3).
- Depends on: T047. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - On fixtures of host state, `status --json` reports drift, an
    interrupted promotion, an awaiting candidate, a stale toolchain, an
    orphaned project and an orphaned repo dir, without taking the lock.
  - It exits 0 or 1 and never 3.
  - The fields that come from julieta land with T066.
- Verify: `go test -tags e2e ./e2e/...`.

#### T049 - `romeu doctor`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("Checks run by `romeu
  doctor`"), 05 5.2 (I14, I20, I22, I23).
- Depends on: T046. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - Each row of the doctor table that is not marked **pre** and that
    this phase can build has a fixture with `HOME` in a temp dir: the
    `sbx` and git floors, the toolchain record, the host global git
    config, `env.rememberHostCommands`, `kit.allowedSources`, the root
    rules (I20), the file modes, the secret bindings and the tree rows.
  - The rows marked **pre** are the functions of T046, called from here
    (I22, I23).
  - A trusted root or disabled workspace trust fails (I14).
  - The ledger rows land with T072.
- Verify: `go test ./internal/... -run Doctor`.

#### T050 - `romeu adopt`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`adopt`), 01 1.6
  ("Generation"), 05 5.2 (I26).
- Depends on: T047. Operator: no. Ask-first: `contracts`, `gates`.
- Acceptance:
  - `adopt` without a terminal, with a workspace path that differs, with
    an open generation or with no such sandbox exits 2; with a stale
    toolchain it exits 3.
  - After `adopt` the record has `adopted: true` and an unknown recreate
    digest, and the preflight of T046 and `sync` step 9 accept the
    sandbox they refused before.
  - That `salvage` and `rm` succeed after `adopt` is tested in T068.
- Verify: `go test -tags e2e ./e2e/...`.

### Phase 5 - julieta's chores

No task of this phase needs block O3 until `ledger`'s ingest; see
"Operator blocks" for what may start early.

#### T051 - The manifest, `julieta setup` and `julieta status`

- [ ] Merged
- Module: `julieta-core`. Implements: 03 3.6, 04 4.3 (`setup`,
  `status`), 02 2.5, 06 6.3 (step 5).
- Depends on: T013, T023, T022. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - In the container e2e, a manifest of protocol N or N-1 is accepted
    and another is refused; `schemas/manifest.v1.json` is generated.
  - `setup` links `$HOME/.local/bin/julieta` to the running binary,
    clones a missing secondary repo, and fast-forwards a default branch
    that is checked out and clean, reporting the others.
  - A command whose Reads cell names the manifest exits 2 without one
    (04 4.3).
  - `status --json` reports the per-repo fields of 04 4.3.
- Verify: `go test -tags e2e ./e2e/...`.

#### T052 - `kitpin mise`

- [ ] Merged
- Module: `kits`. Implements: 07 7.4.
- Depends on: T012. Operator: an agent looks up the signature format
  first; question 9 is the maintainer's only if that adds a dependency.
  Ask-first: `kits`, `dependencies` (if the lookup adds one).
- Acceptance:
  - `kitpin mise <version>` writes the version and both linux sha256
    values from the release's `SHASUMS256.txt` into the pin file, after
    it verifies that file's minisign signature (07 7.4).
  - A test with a recorded fixture covers a good signature, a bad one
    and a checksum file that lacks an architecture.
  - How the signature is verified follows the lookup of question 9.
- Verify: `go test ./tools/kitpin/...`.

#### T053 - The mise driver: `install`, `lock` and `tools/ci mise`

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`install`, `lock`; the
  mise environment), 07 7.2, 07 7.3, 10 10.2 (the `mise` step), 09 (J8),
  S8.
- Depends on: T051, T052, T041. Operator: no. Ask-first: `checks`.
- Acceptance:
  - `install` runs with the environment of 04 4.3 and skips when the
    `lock-set` digest equals the last success; `--force` overrides; a
    `mise.toml` without a lock fails with the hint.
  - `lock` passes the four platforms and rewrites `mise.lock`; `lock
    --check` writes nothing, fails a tool without a lock entry and a
    lockable entry without both linux platforms, and warns on a missing
    macOS entry.
  - `tools/ci mise` runs that logic on this repository's lock and the
    examples'.
  - The `julieta setup` no-op is at most 3 s, median of 5, in the
    container e2e against local origins (S8).
  - The J8 scenario function, added to the package T041 started, replays
    the agent's lock change as a commit on the fixture origin.
- Verify: `go test -tags e2e ./e2e/...`; `go run ./tools/ci mise`.

#### T054 - The hook dispatcher

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`hooks run`), 08 8.2
  (recorded failures), 12 12.4 (the dispatcher runs the tracked hook),
  05 5.2 (I21).
- Depends on: T053. Operator: no. Ask-first: none expected.
- Acceptance:
  - In the container, the dispatcher runs julieta's own action, then
    `.githooks/<hook>`, then `$GIT_DIR/hooks/<hook>`, passing arguments
    and stdin as 04 4.3 says; the first non-zero status stops (I21).
  - A failure is recorded in julieta state and shown until a later run
    of the same hook succeeds. The `hook-failed` event it also emits is
    tested in T063.
  - `pre-commit` runs `lock --check` when a mise file is staged.
- Verify: `go test -tags e2e ./e2e/...`.

#### T055 - `julieta spec validate` and `julieta pin`

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`spec validate`, `pin
  workload`, `pin check`), 07 7.6, 06 6.2, 09 (J2 step 1, J9).
- Depends on: T032, T051, T031, T036. Operator: feeds block O4.
  Ask-first: none expected.
- Acceptance:
  - `spec validate` exits 2 on a decode or rule error, per file and
    across files; `--catalog` lists each unknown `backend:tool` and lock
    host and exits 1 when it lists one.
  - `pin workload` rewrites the digest after it verifies that the
    manifest list covers linux/amd64 and linux/arm64, through the
    guarded client of `oci`.
  - `pin check` reports a pin that is not a digest or is behind.
- Verify: `go test ./internal/... -run 'Validate|Pin'`.

#### T056 - `layout`

- [ ] Merged
- Module: `layout`. Implements: 03 3.2 (the execution rule), 04 4.3
  (`layout up`, `pane run`), 10 10.1 (Golden).
- Depends on: T022, T051, T012. Operator: no. Ask-first: none expected.
- Acceptance:
  - `layout up --dry-run --json` matches the golden `layout.apply`
    requests for the example run layout.
  - `pane run` runs each step as `mise exec -C <pane dir> -- <argv>`,
    stops at a failing step with its status and drops to a login shell.
  - `layout up` applies against herdr at the version and hash of the pin
    file, in the container; it checks the socket protocol number and
    fails closed on another (Q16), and reports "layout stale" when the
    `run` digest differs.
- Verify: `go test -tags e2e ./e2e/...`.

#### T057 - `memstore/write` and the memory commands

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1, 03 3.9, 04 4.3 (`memory
  add|list|show|edit|rm|search`, `memory check`).
- Depends on: T035, T051. Operator: no. Ask-first: none expected.
- Acceptance:
  - julieta stamps `id`, `created` and `updated`; a supplied id or
    timestamp is an error.
  - Two julieta processes writing one store lose no entry (10 10.1,
    Concurrency).
  - `memory check` exits 1 on an unmounted, unwritable or non-conforming
    directory.
- Verify: `go test -tags e2e ./e2e/...`.

#### T058 - Memory import and verify

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1 ("Import and verification"), 03
  3.9, 04 4.3, 00 0.5.
- Depends on: T057. Operator: no. Ask-first: none expected.
- Acceptance:
  - A second import of one file imports nothing and reports `imported +
    skipped-duplicate == read`; an invalid line makes the exit 1.
  - `verify --against` fails on a digest set difference and on a
    `status` difference of a matched entry.
- Verify: `go test -tags e2e ./e2e/...`.

#### T059 - `handoff`

- [ ] Merged
- Module: `handoff`. Implements: 08 8.2, 08 8.3, 03 3.10, 04 4.3
  (`handoff write|show|list`), 09 (J5).
- Depends on: T054, T057, T041. Operator: no. Ask-first: none expected.
- Acceptance:
  - `write` refuses a narrative without its headings (six for `final`)
    and stamps the front matter; `--facts` has no body;
    `schemas/handoff.v1.json` is generated.
  - The handoff reader returns the greatest ULID among `clear` and
    `final` as the narrative; a later `facts` file does not hide it.
  - The container e2e runs SessionEnd and SessionStart in both orders
    (Q22), and the J5 scenario function, from the package T041 started,
    uses it.
  - `show --hook` prints the open entries, the `lesson` entries and
    julieta's warnings; the front matter has a golden (10 10.1).
  - `list` prints one line per handoff file with its kind, and `--json`
    one document.
- Verify: `go test -tags e2e ./e2e/...`.

#### T060 - `julieta snapshot`

- [ ] Merged
- Module: `salvage`. Implements: 08 8.4, 04 4.3 (`snapshot`).
- Depends on: T054. Operator: no. Ask-first: none expected.
- Acceptance:
  - The dispatcher calls it after `post-commit`, `post-rewrite` and
    `post-merge`.
  - The bundle holds the local branches, tags, notes, `salvage/*` refs
    and each worktree HEAD minus the objects reachable from
    `repos[].base`.
  - A snapshot that races a commit leaves a valid bundle (10 10.1); a
    repo-local `core.hooksPath` is reported as "snapshots disabled".
- Verify: `go test -tags e2e ./e2e/...`.

#### T061 - `julieta salvage`, the sandbox half

- [ ] Merged
- Module: `salvage`. Implements: 08 8.5 ("Sandbox half"), 03 3.11, 04
  4.3 (`salvage`), 05 5.2 (I18), 10 10.7.
- Depends on: T060. Operator: no. Ask-first: none expected.
- Acceptance:
  - In the container, the salvage completeness cases of 10 10.1: dirt in
    a linked worktree, a stash, a detached HEAD, a nested repository, an
    ignored file over the cap listed as `over-cap`, a transcript listed
    as `transcript-excluded`.
  - Salvage commits carry no signature with `commit.gpgsign=true` set
    (I18).
  - `salvage` does not import `agent/*` and takes the profile as an
    argument; `schemas/salvage.v1.json` is generated.
- Verify: `go test -tags e2e ./e2e/...`.

#### T062 - Salvage verification and ref import, the host half

- [ ] Merged
- Module: `salvage`. Implements: 08 8.5 ("Host half"), 05 5.2 (I6, I17).
- Depends on: T024, T034, T061, T035. Operator: no. Ask-first: none
  expected.
- Acceptance:
  - The manifest is read through `os.Root`; completeness is recomputed
    and the manifest's own `complete` ignored (I17).
  - Refs are created only under `refs/romeu/salvage/...` with
    `CreateRef` (I6).
  - The package is **dark** until T068.
- Verify: `go test ./internal/salvage/...`.

#### T063 - `ledger`: the event, julieta's emit and drain

- [ ] Merged
- Module: `ledger`. Implements: 13 13.2, 13 13.3 (Draining), 13 13.7, 04
  4.3 (`event add`; the `command-failed` rule), 04 4.4, 12 12.3 (the
  event row), 05 5.2 (I33).
- Depends on: T054, T031. Operator: no. Ask-first: `ledger`,
  `contracts`.
- Acceptance:
  - `go generate` writes the consumers of the event row (12 12.3): the
    validator, the membership tables of `type`, `tool` and `ref`, the
    cross-project allowlist, `schemas/runtime-event.v1.json` and
    `docs/reference/runtime-event.md`. The per-field table of 13 13.10
    passes (I33).
  - The hook dispatcher of T054 writes one `hook-failed` event for a
    hook that exits non-zero, and still records the failure in julieta
    state; the event names neither the hook nor the repo (13 13.2, 13
    13.3; 04 4.3).
  - A julieta command that exits non-zero writes one `command-failed`
    event when the manifest names a spool; `hooks run`, `pane run` and
    `event` write none; a failure to emit changes no output and no exit
    status.
  - The drain rows of 13 13.10 pass.
  - `event add` takes no free text and needs at least one flag.
- Verify: `go test ./internal/ledger/...`.

#### T064 - `ledger`: the spool reader, ingest and entries

- [ ] Merged
- Module: `ledger`. Implements: 13 13.3, 13 13.4, 13 13.5, 13 13.8, 05
  5.2 (I31, I32).
- Depends on: T063, T035, T039. Operator: no. Ask-first: `ledger`,
  `checks`.
- Acceptance:
  - The hostile spool table of 13 13.10 passes row by row, with the
    `afterOpen` hook and an injected version table (I32).
  - The entries table of 13 13.10 passes, the racing ingests without the
    lock included (I31).
  - The I16 scan fails a fixture of `internal/ledger` with each call 13
    13.5 does not admit; `tools/ci imports` fails a caller of the spool
    reader the table does not name.
  - The starting values are those of 13 13.8.
  - The ingest function is **dark** until T071.
- Verify: `go test ./internal/ledger/...`.

#### T065 - `ledger`: the view and `julieta event list`

- [ ] Merged
- Module: `ledger`. Implements: 13 13.6, 04 4.3 (`event list`), 05 5.2
  (I33), 10 10.1 (Golden).
- Depends on: T064. Operator: no. Ask-first: `ledger`.
- Acceptance:
  - The two views of the two-project fixture match their goldens,
    ordered by `ingested`, project and key, with no `id` and no key of
    the other project in `others.jsonl` (I33).
  - A derivation that changes nothing writes nothing.
  - `julieta setup` exits 2 with `ledger-view-writable` when it can
    write the view.
  - `julieta event list` prints this project's entries from the view,
    rejected ones included, and `--others` the allowlisted fields of the
    other projects'.
- Verify: `go test ./internal/ledger/...`.

### Phase 6 - `run` and the destructive commands

#### T066 - `romeu run`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("How `romeu run` reaches the
  run layout"), 01 1.5, 01 1.6 ("Generation"), 07 7.5, 09 (J3, J10), S8,
  05 5.2 (I1, I7, I30, I33).
- Depends on: T050, T056, T065, T030, T055. Operator: no. Ask-first:
  `contracts`, `gates`.
- Acceptance:
  - Hybrid, on the Linux runners, the J3b scenario function passes: a
    generation is recorded before `sbx env run`, egress is reconciled,
    the compatibility check and `setup` run, and the final argv equals
    the one of I1 byte for byte.
  - The 32 KiB manifest round-trip of I30 runs against the recorded
    `--env` shape of T030 and the manifest reader of T051, which this
    task is the first to hold together.
  - The J3a variant: with a sandbox that exists, `run` starts or
    reattaches with `sbx env run -d` without re-provisioning, and
    `julieta setup` is a no-op when the locks are unchanged (09, J3;
    S8).
  - A failed create removes the record when the sandbox is absent and
    keeps it when present.
  - A writable view mount stops `run` before `layout up` (I33); a failed
    compatibility check stops it with exit 2 (I30).
  - A gate 1 or a gate 2 change makes `run` exit 3 before a mutating
    call (I7).
  - `run --timings --json` prints `run-timings.v1`
    (`schemas/run-timings.v1.json` is generated); `romeuMs` is at most 2
    s, median of 5, with the fake `sbx` (S8).
  - The lock is released before the final `exec` (04 4.1).
  - `romeu status` gains julieta's warnings and recorded hook failures
    for a running sandbox, and says so when it skips that exec (04 4.2).
- Verify: `go test -tags e2e ./e2e/...`.

#### T067 - `romeu stop` and `romeu pull`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`stop`; "How `romeu pull`
  updates a review checkout"), 09 (J4), 05 5.2 (I2, I5, I19).
- Depends on: T060, T066. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - `stop` runs `julieta snapshot --all`, then the recorded `sbx stop`
    argv.
  - A unit table of stale state covers I19: a daemon URL read before a
    port change is not used. In the e2e the fake `sbx` changes the port
    between calls and reports stopped; the fetch uses the new URL or is
    not attempted.
  - A head mismatch with `julieta status` exits 1 and updates nothing.
  - After `pull`, `refs/heads`, `refs/tags`, `refs/remotes` and `HEAD`
    are unchanged (I5); the review repo's local config equals the fixed
    set (I2).
  - `pull` succeeds while a `run` of the project is attached (10 10.1).
    The J4 scenario function lands here.
- Verify: `go test -tags e2e ./e2e/...`.

#### T068 - `romeu salvage`, `rm` and `recreate`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`salvage`, `rm`, `recreate`),
  08 8.5, 01 1.6, 09 (J6, J10, J13), 05 5.2 (I5, I6, I17, I26).
- Depends on: T062, T066. Operator: no. Ask-first: `contracts`, `gates`.
- Acceptance:
  - `rm` exits 5 for each case of I17: a missing repo in the manifest, a
    forged `complete: true`, a hidden branch, a detached HEAD commit,
    dirt in a linked worktree, an unbundled tag, a corrupted payload
    file, a salvage id mismatch, daemon heads not in the bundle, and a
    repo removed from the spec after the generation was created.
  - `--accept-loss` removes the sandbox after an incomplete salvage;
    `rm` warns when the generation has no `final` handoff.
  - `recreate` is `rm`, then `run`: it salvages, removes, records a new
    generation, and exits 5 where `rm` would.
  - After `salvage`, `refs/heads`, `refs/tags`, `refs/remotes` and
    `HEAD` are unchanged (I5).
  - After a same-name recreate, a fetch with `--prune` and a forced
    refspec leaves the salvage refs, and a second create on one of them
    fails (I6).
  - After `adopt`, `salvage` and `rm` succeed on the adopted sandbox
    (I26).
  - `salvage --from-host` and the absent-sandbox row of `run` record
    `result: lost`. The J6 and J13 scenario functions land here, and J10
    runs in CI.
- Verify: `go test -tags e2e ./e2e/...`.

#### T069 - `romeu retire`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`retire`), 09 (J7), 02 2.3
  (the invariants of the tree).
- Depends on: T068, T048. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - The J7 scenario function passes in CI: `retire` lists the unpushed
    and salvage-only work per repo, exits 5 on a declined confirmation
    or without a terminal and without `--force`, moves `<name>-env/` to
    `.attic/<name>/<ts>/`, drops the project from the workspace files
    and moves its state to the state attic.
  - Nothing is deleted.
- Verify: `go test -tags e2e ./e2e/...`.

#### T070 - `romeu handoff`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`handoff`), 08 8.3 (Host), 05
  5.2 (I24, I29).
- Depends on: T059. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - It reads through the handoff reader and the `os.Root` readers of
    `memstore` (I24), and prints the latest narrative and the newest
    facts.
  - A body and a branch name with ESC sequences print escaped (I29).
- Verify: `go test -tags e2e ./e2e/...`.

#### T071 - The ledger in the commands, and `romeu ledger`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`ledger ingest`, `ledger
  query`), 13 13.4, 13 13.9, 08 8.5 (the ingest before the sandbox
  half), 05 5.2 (I32).
- Depends on: T069. Operator: no. Ask-first: `contracts`, `ledger`.
- Acceptance:
  - Ingest runs inside `sync`, `run`, `salvage` and `retire` at the step
    each names, and `retire` ingests a second time before the move (13
    13.4).
  - With an ingest that fails, `salvage` and `rm` end with the exit
    status they would have had, and the salvage record holds
    `ledger-incomplete` (13 13.10; I32).
  - `ledger ingest` exits 1 with `ledger-incomplete`; `ledger query`
    orders by `ingested`, project and key, and prints through the CLI
    writer.
- Verify: `go test -tags e2e ./e2e/...`.

#### T072 - The ledger rows of `romeu doctor`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (the three ledger rows of the
  doctor table), 13 13.5, 13 13.6, 13 13.8.
- Depends on: T071, T049. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - On fixtures, `doctor` re-hashes each ingested entry against its name
    and derives each view again and compares it byte for byte (13
    13.10).
  - It fails on a spool root that is missing, a symlink or not a
    directory, and on wrong modes; it warns on skipped spool files, a
    leftover temporary name and the ledger's size past 64 MiB.
- Verify: `go test ./internal/... -run Doctor`.

#### T073 - The meta-tests

- [ ] Merged
- Module: `romeu-cli`. Implements: 10 10.1 ("Additional required
  tests"), 04 4.1 (Idempotency, Locking), 01 1.6, 05 5.2 (I7).
- Depends on: T072, T067, T070. Operator: no. Ask-first: none expected.
- Acceptance:
  - The idempotency meta-test covers each converging command and each
    appending command of 04 4.1.
  - Two romeu processes: the second exits 1 with `RJ-101`.
  - Promotion fault injection: after a kill at each step, each **P**
    command finishes the promotion or exits 4, and a re-sync converges
    (I7).
  - Each **P** command runs with the registry and the origins
    unreachable after `sync`, and succeeds.
- Verify: `go test -tags e2e ./e2e/...`.

#### T074 - The invariant sweeps over the commands

- [ ] Merged
- Module: `romeu-cli`. Implements: 05 5.2 (I2, I3, I4, I7, I14, I15,
  I16, I25).
- Depends on: T073. Operator: no. Ask-first: none expected.
- Acceptance:
  - After each romeu command, each clone's `.git/` equals what it was,
    by the comparison of I2.
  - With the hostile configuration of I3 planted in the repos, in a
    global config and in the environment, `init`, `sync`, `run`, `pull`,
    `salvage`, `rm`, `retire`, `status` and `doctor` leave no marker
    file.
  - A filesystem diff over a temp `HOME` after each command shows no VS
    Code trust or settings write (I14) and no deletion outside what
    romeu owns (I16).
  - A hand edit of a live derived file makes `sync` and each **P**
    command exit 4 (I15).
  - Each gate 2 change and each gate 1 change makes `run`, `stop`,
    `salvage`, `rm`, `retire` and `pull` exit 3 before a mutating call
    (I7).
  - No e2e run reaches the fake `sbx`'s exit 99 for `--auto-approve`
    (I4) or for a secret value form (I25).
- Verify: `go test -tags e2e ./e2e/...`.

#### T075 - The remaining scenario functions and the host suite

- [ ] Merged
- Module: `romeu-cli`. Implements: 10 10.3 (Shared scenarios), 10 10.1
  (Host), 09 (J1, J9, J12), S7.
- Depends on: T074. Operator: no. Ask-first: none expected.
- Acceptance:
  - `e2e/scenarios` gains the functions the command tasks did not add:
    the rest of J1 after its step 3, J9 and J12. J12 runs J1 steps 3 to
    6 under a fresh root, settings and state (09).
  - A function exists for each `## J<n>` heading of 09; a test compares
    the two lists.
  - The five CI journeys of S7 pass, at the hybrid level where they
    reach julieta.
  - `e2e/host` compiles under the `host` tag and calls the same
    functions (10 10.3); it runs in block O6.
- Verify: `go test -tags e2e ./e2e/...`; `go vet -tags host
  ./e2e/host/...`.

### Phase 7 - Kits, skills, the block B definitions and the diagrams

#### T076 - The `kit` kind and `tools/ci kits`

- [ ] Merged
- Module: `kits`. Implements: 12 12.3 (the `kit` kind), 10 10.2 (the
  `kits` step).
- Depends on: T052, T026. Operator: no. Ask-first: `kits`, `checks`.
- Acceptance:
  - `go run ./tools/new kit <name>` writes `kits/<name>/` with a
    descriptor on the pinned frontend, under the file name A11 recorded.
  - `tools/ci kits` fails a second frontend pin, an install step over
    five lines, a download without a checksum line after it, and a
    download line with a pipe or a command substitution.
  - It also fails a kit whose frontend, mise or herdr pin differs from
    the pin file.
- Verify: `go run ./tools/ci kits`.

#### T077 - The `julieta` and `os-base` kits, embedded

- [ ] Merged
- Module: `kits`. Implements: 06 6.4, 02 2.1 (Embedding), 07 7.2, S3.
- Depends on: T076, T038. Operator: no. Ask-first: `kits`.
- Acceptance:
  - The two kits use the descriptor file name and the capability names
    A11 recorded.
  - `julieta` installs mise and herdr at the pins of the pin file and
    declares the two install-phase domains of 06 6.4; the `os-base`
    package list is the missing set A12 measured.
  - romeu embeds `kits/*`, and `sync` materializes them into
    content-addressed directories.
  - That they build and start is measured by B2 (S3).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/...`.

#### T078 - The skills

- [ ] Merged
- Module: `skills`. Implements: 02 2.1 (`skills/`), 08 8.1, 08 8.3.
- Depends on: T059. Operator: no. Ask-first: none expected.
- Acceptance:
  - `skills/julieta/SKILL.md` tells the agent to use `julieta memory`
    and to tag a lesson; `skills/handoff/SKILL.md` pipes the required
    headings to `julieta handoff write [--final]`.
  - A test checks that each `julieta` command a skill names exists in
    the command definitions.
- Verify: `go test ./... -run Skills`.

#### T079 - The `julieta-claude` kit

- [ ] Merged
- Module: `kits`. Implements: 06 6.4, 08 8.1 ("How julieta replaces the
  agent's built-in memory"), 08 8.2, S5.
- Depends on: T077, T078. Operator: no. Ask-first: `kits`.
- Acceptance:
  - The kit installs the two skills of T078 and the SessionStart and
    SessionEnd hooks, and makes the agent's own memory directory
    non-writable (Q8).
  - It is the one agent-aware kit: a test fails an agent-specific path
    in `kits/julieta/`.
  - That the hooks and the skills are present in a real sandbox is
    measured by B2 (S5).
- Verify: `go run ./tools/ci kits`.

#### T080 - The `git-ssh-sign` kit

- [ ] Merged
- Module: `kits`. Implements: 06 6.4.
- Depends on: T077. Operator: no. Ask-first: `kits`, `signing`.
- Acceptance:
  - The kit declares the ssh-agent capability and writes
    `gpg.format=ssh`, `commit.gpgsign=true`, `user.signingkey` and the
    allowed-signers file from the `signingKey` arg.
  - Its capability set appears in the gate 2 diff of a project that adds
    it.
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/...`.

#### T081 - The signing rule

- [ ] Merged
- Module: `romeu-cli`. Implements: 06 6.4 ("Signing rule"), 01 1.5 (step
  5), 04 4.2 (the signing row of `doctor`), 04 4.1 (`SSH_AUTH_SOCK` for
  the `sbx` calls).
- Depends on: T080, T025, T049. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - `sync` refuses to render `git-ssh-sign` with exit 2 and `RJ-204`
    unless the socket is set, reachable and holds one key equal to
    `signingKey`.
  - `doctor` and the preflight of that project's **P** commands run the
    same check.
  - For a project that uses the kit, the `sbx` calls carry
    `SSH_AUTH_SOCK=<signing.agentSocket>` and no other project's do.
- Verify: `go test -tags e2e ./e2e/...`.

#### T082 - The block B and sandbox-side definitions

- [ ] Merged
- Module: `probes`. Implements: 11 11.2, 11 11.3, 10 10.3 (Redaction),
  S3, S5, S7, S8.
- Depends on: T075, T079, T081. Operator: feeds blocks O5 and O6.
  Ask-first: none expected.
- Acceptance:
  - A definition exists for each id of 11 11.2: B1, B2, B3, B4 and B5,
    each `kind: check` with the predicate its Step cell names, and C1,
    C2, C3, C4 and C5, each with its declared `kind`, `expect` and
    decision table.
  - B1 installs the candidate from its GitHub release and verifies
    checksums and attestation; each other block B result copies two
    observations from it, the candidate's tag and the commit that tag
    names.
  - B3 runs `go test -tags host -json ./e2e/host/...`, writes the output
    through the recorder's redaction to
    `docs/probes/B3-<host>-test.json`, records its sha256 and has one
    observation per journey.
  - B5's predicate is the S8 bound; the sandbox-side checks run through
    `sbx env exec` in the sandbox B2 started.
  - Each is tested against the helper binary. This task lands before the
    candidate tag because the harness is outside `docs/`: a definition
    added after the tag would need a new candidate (11 11.2).
- Verify: `go run ./e2e/probes --block B --out <dir>` against the helper
  binary in a test.

#### T083 - The promotion-commit and probe diagrams of `ARCHITECTURE.md`

- [ ] Merged
- Module: `state`. Implements: 12 12.8 (`ARCHITECTURE.md`), 12 12.3 (the
  transition-table row), 01 1.6 ("Promotion commit"), 11 11.4.
- Depends on: T034, T039, T020. Operator: question 10 is answered first.
  Ask-first: `checks`.
- Acceptance:
  - **The plan's own reading, which the specification does not state:**
    12 12.8 says all four state machines of `ARCHITECTURE.md` are
    generated diagrams, and 12 12.3 names one source for them, the
    transition tables of `internal/state`, which hold two. The four
    steps of the promotion commit (01 1.6) and the lifecycle table of 11
    11.4 each become a transition table beside those two, and the one
    generator of T034 draws all four.
  - `go generate` writes the two diagrams into `ARCHITECTURE.md`, and
    `tools/ci generated` fails a hand edit of either.
  - The generator's table test covers each row of the two new tables, as
    for the first two.
  - If the answer to question 10 is that the two are hand-written, this
    task is empty, 12 12.3 and 12 12.8 say so, and T095 writes them;
    that path needs no change outside `docs/`.
  - This task lands before the candidate tag because the generator is
    outside `docs/` (11 11.2).
- Verify: `go test ./internal/state/...`; `go run ./tools/ci generated`.

### Phase 8 - Release tooling

#### T084 - `SECURITY.md` and the `README.md` skeleton

- [ ] Merged
- Module: `ci-release`. Implements: 12 12.8, 12 12.2 (the `ci-release`
  row), ADR 0001 rules 12 and 20.
- Depends on: T006. Operator: no. Ask-first: `checks`
  (`tools/ci/headings.yaml`).
- Acceptance:
  - `SECURITY.md` has the three headings of rule 20, the channel and the
    response expectation of 12 12.8.
  - `README.md` has the heading order of rule 12, with the strings fixed
    here.
  - Both heading lists are in `tools/ci/headings.yaml`, which does not
    change after the candidate (11 11.2).
- Verify: `go test ./tools/ci/... -run Headings`.

#### T085 - `tools/ci docs`: front matter, links and the two tools

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 (the `docs` step), ADR 0001
  rules 1 and 5, index ("Deferred decisions": the linter and spell
  checker rows), S10.
- Depends on: T084, T075. Operator: no. Ask-first: `checks`,
  `dependencies` (if a tool enters `mise.lock`).
- Acceptance:
  - A fixture Markdown file fails for a missing or invalid front matter
    key and for a relative link or anchor that does not resolve; text in
    a code span is not a link.
  - The pull request picks the Markdown linter and the spell checker,
    puts their configuration files on the `checks` globs, and fixes the
    format and path of the accepted-words list. The Deferred decisions
    table says this pull request is the one that picks them, so the
    choice is not a task of its own; the two rows leave the table in the
    same change.
  - `docs/spec.md` and the pages under `docs/spec/` move to front matter
    in this change (ADR 0001, Consequences).
- Verify: `go run ./tools/ci docs`.

#### T086 - `tools/ci docs`: the remaining rules

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 (the `docs` step), ADR 0001
  rules 9, 12, 13, 17 and 20, S10.
- Depends on: T085. Operator: no. Ask-first: `checks`.
- Acceptance:
  - A fixture fails for each case of rule 9: a first mention that is not
    the link form, one in a heading, one in front matter, one inside
    another link, a number with no record, a plural form.
  - A fixture fails for a heading that is missing, extra or out of order
    against `tools/ci/headings.yaml` (rules 12 and 20).
  - A fixture fails for a fenced block with no language, for a `romeu`
    or `julieta` command in an `sh` block that equals no entry of
    `e2e/scenarios/commands.yaml`, and for a guide page whose `journey:`
    is no heading of 09 (rule 13).
  - A fixture fails for an image and for a file under `docs/` with
    another extension (rule 17).
- Verify: `go run ./tools/ci docs`.

#### T087 - `tools/ci links` and `fuzz`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 (the subcommands outside
  `all`), 10 10.1 (Fuzz), ADR 0001 rule 5.
- Depends on: T085. Operator: no. Ask-first: `checks`, `dependencies`
  (`fuzz.yml`).
- Acceptance:
  - `links` fetches each external URL the docs cite and is called by no
    workflow.
  - `fuzz` runs each fuzz target for a fixed time, and the scheduled
    `fuzz.yml` calls it and then `tools/ci mutate`; the file passes
    `tools/ci workflows`.
- Verify: `go run ./tools/ci links`; `go run ./tools/ci fuzz`.

#### T088 - `tools/release` and `release.yml`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 ("Release and bootstrap"),
  12 12.6, 12 12.1, 02 2.1 (Embedding), S1.
- Depends on: T079, T080, T087. Operator: no. Ask-first: `release`,
  `dependencies`, `checks`.
- Acceptance:
  - `tools/release build --version v1.0.0 --dry-run` builds the two
    julieta binaries, embeds them with the kits and the catalog, builds
    romeu for darwin and writes the archives and `checksums.txt`.
  - `tools/release notes` groups Conventional Commit subjects since the
    last tag that is not a prerelease, with each pull request's Why.
  - `tools/release verify` runs `gh attestation verify` with the signer
    workflow, starting the `gh` of `mise.lock` through `mise exec`;
    `publish` marks a tag with a suffix as a prerelease and never
    creates a draft.
  - `release.yml` passes `tools/ci workflows`.
- Verify: `go run ./tools/release build --version v1.0.0 --dry-run`.

#### T089 - `tools/ci dora`

- [ ] Merged
- Module: `ci-release`. Implements: 12 12.10, [ADR 0004, measure
  delivery with the five DORA metrics computed by a
  tool](adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md).
- Depends on: T088. Operator: no. Ask-first: `checks`, `release`.
- Acceptance:
  - Against a recorded API fixture and a fixture repository, offline,
    the same input gives the same bytes.
  - A period with no deployment has `null` times and `n/a` in the text
    form.
  - A rendered output that has a lead time or a deployment count without
    that period's failed and rework counts fails the unit test.
  - A marker that names a tag with no deployment is listed under
    `unmatched`; `--attach` is the last step of `release.yml`.
- Verify: `go test ./tools/ci/... -run Dora`.

#### T090 - `tools/ci acceptance`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.5, 10 10.2 (the subcommands
  outside `all`), S6.
- Depends on: T088. Operator: no. Ask-first: `checks`.
- Acceptance:
  - Against a recorded API fixture, each evidence kind is verified as 10
    10.5 says.
  - On the product's own file the four end-of-plan checks run: the S6
    tags per row of 05 5.2; a result per probe of the "Settled by"
    column, from both hosts when arch-sensitive; a guide page per
    journey; block B results that count for v1.0.0.
  - With `--file` a `command` item is not run;
    `schemas/acceptance.v1.json` is generated.
- Verify: `go test ./tools/ci/... -run Acceptance`.

#### T091 - The v1 catalog table

- [ ] Merged
- Module: `catalog`. Implements: 03 3.5 (the closing paragraph), 07 7.6,
  S4.
- Depends on: T055, O4. Operator: block O4 feeds it; block O5 confirms
  it. Ask-first: `catalog`.
- Acceptance:
  - For the keys and hosts the handed-over output lists, the catalog
    gains entries, each domain with an explicit `upload` flag.
  - `tools/ci catalog` passes.
  - That `julieta spec validate --catalog` then exits 0 against the
    reference config repo is the maintainer's first step of block O5.
- Verify: `go run ./tools/ci catalog`.

### Phase 9 - Acceptance

#### T092 - The block B results

- [ ] Merged
- Module: `probes`. Implements: 11 11.2, 11 11.3, 11 11.4, 10 10.2
  ("Forbidden names").
- Depends on: O6. Operator: block O6 produces it; the maintainer opens
  it. Ask-first: none for results alone.
- Acceptance:
  - The pull request holds the results and artifacts under
    `docs/probes/`, from both hosts, and nothing under `e2e/testdata/`
    (11 11.2).
  - It reaches `main` through a branch, the hook, `go run ./tools/ci
    fast` and a pull request, for the reason T026 gives.
  - `tools/ci probes` passes. A result that needs a change outside
    `docs/` and the root Markdown files (a kit, a catalog entry) is
    fixed there, and blocks O5 and O6 run again on a new candidate (11
    11.2).
- Verify: `go run ./tools/ci fast`; `go run ./tools/ci probes`.

#### T093 - The records of the questions block B settles

- [ ] Merged
- Module: `probes`. Implements: index (Open questions), 11 11.4, 12
  12.5.
- Depends on: T092. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - Each of the four questions whose last probe is in block B becomes an
    ADR and leaves the table: Q2 (A12, B2), Q8 (C3, B2), Q19 (A12, B2)
    and Q22 (C5, B3).
  - An overturned decision whose `affects` paths are under `docs/` is
    resolved here; one with a path outside `docs/` needs a new
    candidate.
- Verify: `go run ./tools/ci sequences`; `go run ./tools/ci probes`.

#### T094 - The guide pages

- [ ] Merged
- Module: `docs`. Implements: 12 12.8 (`docs/guide/*`), 09, S10, ADR
  0001 rules 1, 2 and 13.
- Depends on: T092. Operator: no. Ask-first: none (`docs/guide/` is on
  no surface).
- Acceptance:
  - Each `## J<n>` heading of 09 is named by the `journey:` of a page.
  - Each `romeu` or `julieta` command in an `sh` block equals an entry
    of `e2e/scenarios/commands.yaml`, which this task does not change.
  - The J5 page describes the hook order C5 recorded (Q22).
  - Each page gives the expected output and the recovery from each
    expected error id.
- Verify: `go run ./tools/ci docs`.

#### T095 - `README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`

- [ ] Merged
- Module: `docs`. Implements: 12 12.8, S10, ADR 0001 rules 12, 18 and
  19.
- Depends on: T092. Operator: no. Ask-first: none expected.
- Acceptance:
  - The three pages have the content of their outlines in 12 12.8, under
    headings that `tools/ci/headings.yaml` already fixes.
  - The review report of rule 18 scores each at 16 of 20 or more with no
    zero. **[review]**
- Verify: `go run ./tools/ci docs`.

#### T096 - The decision records of review round 2

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T028. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Settled questions" of `docs/reviews/round-2.md` that
    the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T097 - The decision records of review round 3

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T096. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Maintainer decisions" of `docs/reviews/round-3.md`
    that the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T098 - The decision records of review round 4

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T097. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Maintainer decisions" of `docs/reviews/round-4.md`
    that the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T099 - The decision records of review round 5

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T098. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Maintainer decisions" of `docs/reviews/round-5.md`
    that the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T100 - The decision records of review round 6

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T099. Operator: no. Ask-first: `decisions`.
- Acceptance:
  - A record, written with `go run ./tools/new adr`, exists for each
    decision under "Maintainer decisions" of `docs/reviews/round-6.md`
    that the six accepted records do not already hold (12 12.5).
  - `tools/ci sequences` and `tools/ci generated` pass. That the records
    cover the round's decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T101 - The records of the questions the maintainer settles

- [ ] Merged
- Module: `docs`. Implements: index (Open questions), 12 12.5, S10.
- Depends on: T100. Operator: the maintainer's decision on each.
  Ask-first: `decisions`.
- Acceptance:
  - Each question whose "Settled by" cell is the maintainer has an ADR
    and leaves the table: Q4, Q6, Q9, Q12, Q13, Q14, Q23 and Q25. Q25 is
    settled on the output of block O1.
  - After this task the Open questions table is empty, or each remaining
    row says what it waits for.
- Verify: `go run ./tools/ci sequences`.

#### T102 - Acceptance

- [ ] Merged
- Module: `ci-release`. Implements: index ("Success criteria"), 10 10.5,
  10 10.2 (`links`, `acceptance`), 11 11.2.
- Depends on: T094, T095, T101, O7. Operator: block O7 comes first.
  Ask-first: `checks` (`docs/acceptance.json`).
- Acceptance:
  - `go run ./tools/ci links` ran once, and a dead link it finds is
    fixed or reopens the deferred row about a scheduled check.
  - `docs/acceptance.json` has one entry per success criterion, and the
    release and CI-run items name the release and the run of the v1.0.0
    commit.
  - The output of the B1 run on v1.0.0 is under the pull request's
    Evidence. **[review]**
- Verify: `go run ./tools/ci acceptance` exits 0.

## Traceability

Where each success criterion, journey and invariant is built. The rows
are derived from the tasks: a task is listed against an id when its own
text names that id.

| Success criterion | Tasks | Evidence comes from |
|---|---|---|
| S1 | T088 | O7 (the release run) |
| S2 | T002 | O7 (the run on the release commit) |
| S3 | T077, T082 | O6 (B2) |
| S4 | T091 | O7 (the config repo's run) |
| S5 | T079, T082 | O6 (B2) |
| S6 | T007, T090 | the last task |
| S7 | T075, T082 | CI, and O6 (B3) |
| S8 | T053, T066, T082 | CI, and O6 (B5) |
| S9 | T002 | CI |
| S10 | T021, T085, T086, T094, T095, T096, T097, T098, T099, T100, T101 | the last task |
| S11 | T001, T002 | CI |

| Journey | Tasks |
|---|---|
| J1 | T041, T075 |
| J2 | T019, T043, T055 |
| J3 | T066 |
| J4 | T067 |
| J5 | T059, T094 |
| J6 | T068 |
| J7 | T019, T048, T069 |
| J8 | T053 |
| J9 | T042, T055, T075 |
| J10 | T019, T048, T066, T068 |
| J11 | T019, T045, T047 |
| J12 | T075 |
| J13 | T048, T068 |

| Invariant | Tasks |
|---|---|
| I1 | T010, T066 |
| I2 | T010, T067, T074 |
| I3 | T023, T074 |
| I4 | T015, T029, T074 |
| I5 | T023, T043, T067, T068 |
| I6 | T023, T062, T068 |
| I7 | T034, T040, T042, T043, T045, T047, T066, T073, T074 |
| I8 | T021, T037, T043 |
| I9 | T036, T043 |
| I10 | T036 |
| I11 | T021, T037 |
| I12 | T021, T035, T043, T044 |
| I13 | T038 |
| I14 | T049, T074 |
| I15 | T039, T046, T074 |
| I16 | T039, T064, T074 |
| I17 | T034, T062, T068 |
| I18 | T061 |
| I19 | T067 |
| I20 | T041, T049 |
| I21 | T054 |
| I22 | T019, T046, T049 |
| I23 | T046, T049 |
| I24 | T035, T044, T070 |
| I25 | T015, T019, T029, T074 |
| I26 | T046, T047, T050, T068 |
| I27 | T024, T038, T044 |
| I28 | T032, T038, T044 |
| I29 | T007, T040, T070 |
| I30 | T016, T030, T066 |
| I31 | T064 |
| I32 | T064, T071 |
| I33 | T063, T065, T066 |

## What v1 leaves out

The source is the specification's own lists. This plan adds nothing
to them and builds none of them. A candidate that is in none of these
lists and not in the specification enters v1 through a change to the
specification first, and then gets a task here.

**Non-goals** (00 0.3): an external memory server, and mounting the
agent's own memory directory from the host; signed or published
kits; tmux and cmux renderers; herdr agent-team shims; Linux or
Windows hosts; real support for agents other than Claude Code; a
process supervisor in the run layout; project rename, and repo rename
or split, as commands.

**Marked v1.1** (00 0.5): listing `sbx`'s native egress approval
queue in `status`; `romeu handoff --list` filters; a Remote-SSH
helper command; an explicit registry credential (Q23).

**Designed and not built** (06 6.6): the publishing path for kits.

**Deferred decisions** (index, [Deferred decisions](spec.md#deferred-decisions)).
The table has 28 rows at the commit this plan was written from. Each
has what holds until then and the event that reopens it; the table is
the one place they are written, and this page does not copy them.
Five rows meet a task of this plan:

| Row | Task | What happens there |
|---|---|---|
| which Markdown linter and spell checker `tools/ci docs` runs | T085 | the row's event is that pull request, which picks both |
| generating part of the spell-check word list from the vocabulary table | T085 | the list's format and path are fixed there; the generation stays deferred |
| a scheduled workflow that checks external links | T102 | a dead link in that run reopens it |
| tuning the ledger's starting values | T064 | the plan may tune them (13 13.8) and keeps the proposed defaults |
| how a local-gate run is recorded | each task | reopened the first time the maintainer chooses the local gates for a merge (10 10.2) |

## Decisions this plan defers

These are choices of the plan, not of the specification. Each waits
for the task that has the context to make it.

| Deferred | Until then | Reopened by |
|---|---|---|
| splitting this page into one file per phase | one page | the page passes 3 000 lines, or two open pull requests conflict on it twice |
| the label of the Intel macOS runner | none chosen | T002, which verifies that the label exists and reports `x86_64` (10 10.2) |
| the frontend milestone, the workload digest and the herdr version | none chosen | T012, which writes the pin file |
| whether agents push from a second account | the sandbox's fine-grained token (Q25) | the output of block O1, item d |
| the `apply` tag of `secrets` | recreate-class (Q18) | the A10 result, in T026 |
| which hook order the J5 guide describes | SessionEnd before SessionStart (Q22) | the C5 result, in T094 |
| whether the rehearsal of block O5 becomes a rule of the specification, and whether it moves earlier | a recommendation of this plan, before the candidate tag | the first candidate that block B fails for a reason the rehearsal would have shown |
| one pull request per decision record, or one per task | one per task (T028, T096 to T101) | the first review that asks for a record on its own |

## Questions for the maintainer

Places where the specification is silent or says two things, found while
planning and in the review of this plan. The plan states the reading it
took, so work can start; none of them is a decision of ours to keep.

**To answer before the work that needs it, in this order.** Each names
the first task that waits for the answer.

1. **The plan's place** (index, 02 2.1). Needed before: now, with this
   page. The specification says "first plan task" and "last plan task",
   and names no path, format or task shape for the plan; the tree of 02
   2.1 has no entry for it.
   Recommendation: Accept `docs/plan.md` as one page and add it to the
   tree of 02 2.1.

2. **Where the checks land** (12 12.2, 10 10.2). Needed before: T002.
   The checks of 10 10.2 that `ci-bootstrap` does not list (`coverage`,
   `invariants`, `mutate`, `imports`, `vocabulary`, `schema`, `golden`,
   `catalog`, `mise`, `kits`, `lessons`) have no stated landing point
   except "the remaining checks" in `ci-release`, after the code they
   check.
   Recommendation: Each lands with its first input, as 12 12.2 says for
   kinds and generator rows. It is the reading that fits 10 10.8, which
   asks each task to pass `all` with its tags in place. The module
   column of T010 and T011 stays `ci-release`, where 12 12.2 puts the
   check, whenever the task lands.

3. **The pieces that land early** (12 12.2, 04 4.1, 10 10.6, 06 6.1, 11
   11.1, 02 2.1, 10 10.2). Needed before: T008. The map puts the error
   table under `romeu-cli`, `tools/kitpin` under `kits` and the argv
   builders under `sbxdrv`, each after modules that need a part of them:
   `spec`, `state` and `gitsafe` return error ids; A11, A12 and A13 need
   pins before `kits` exists; the handshake needs two builders. Two
   further additions are the plan's own: the one pin file of T012, which
   is not in the tree of 02 2.1 and holds a product-level workload pin
   that no `tools/kitpin` subcommand writes; and the rule in `tools/ci
   kits` (T076) that every kit's pin equals the pin file.
   Recommendation: Accept the four early pieces named in "The first
   slice", the pin file and its equality rule, and say in 12 12.2 that a
   module's first task may precede its place when it has no dependency
   there; add the pin file to the tree of 02 2.1.

4. **The image and herdr downloads in CI** (10 10.1). Needed before:
   T013. The rule that lists where tests may use the network names the
   container e2e's mise install. It omits pulling the workload's base
   image for the container and hybrid e2e, and downloading herdr for
   `layout up`.
   Recommendation: Add both to the rule, pinned and checksum-verified
   like mise, with a failed download reported as an infrastructure
   error. The sandbox that builds T012 and T013 needs the same hosts
   reachable, which is a step of block O1.

5. **What the workload's Debian base is** (10 10.1, 06 6.4, 06 6.1).
   Needed before: T013; an agent's lookup at T012 settles it.
   **Inferred, not checked:** 10 10.1 runs the container e2e "in the
   workload's Debian base" and 06 6.4 calls the workload a Debian base.
   The plan reads that as the pinned workload image, pulled by digest
   and run as a plain container. If that image cannot be pulled, or does
   not start without `sbx`, T013 has no image until A12 measures the
   workload.
   Recommendation: An agent pulls the pinned digest and runs a shell
   command in it when it writes T012. If that works, nothing changes. If
   not, the container e2e uses a Debian image pinned by digest in the
   pin file until A12 reports the workload's missing set, and 10 10.1
   says so.

6. **What A3 probes in block A** (11 (opening), 11 11.1). Needed before:
   T019; answer at checkpoint C0. Block A "needs no product code", and
   A3 expects that "romeu/julieta checks see every plant" and that
   "ingest skips each spool plant". Those checks do not exist when block
   A runs, and the task that writes A3's expectation must know which
   half it writes.
   Recommendation: Keep the mount facts in A3: each plant as the host
   sees it, the read-only mounts, the mount paths. Move the product
   half, what romeu, julieta and ingest do with the plants, to block B,
   as a case of B3 or an id of its own.

7. **How probe results reach `main`** (11 (opening), 11 11.3, 10 10.2,
   12 12.4, 10 10.3). Needed before: block O3. The specification says
   results are "committed" and does not say by whom or how. The
   maintainer is the bypass actor, and a direct push to `main` is read
   by no check; the recordings hold output of the maintainer's real
   host. The plan also adds a filter to the recorder (T018) that keeps
   only the harness's own sandboxes, env dirs, global secret and allow
   rule from a listing, which 10 10.3 does not say.
   Recommendation: A pull request from a branch pushed through the hook,
   as this plan has it, and one line in 11 that says so; and the
   recorder's filter named in 10 10.3's Redaction row.

8. **A result that overturns a default** (11 11.4, 10 10.2, 12 12.4).
   Needed before: T020. `tools/ci probes` fails on an overturned result
   until it is resolved, and it is a step of `all`, which a merge
   requires. A pull request with results alone cannot merge when one is
   overturned.
   Recommendation: The resolution joins the results' pull request, as
   this plan has it; or `probes` treats an overturned result as a
   failure only on `main`, which changes T020. The first needs no code.

9. **The signature check of `kitpin mise`** (07 7.4, 12 12.1). Needed
   before: T052. **Inferred, not checked against mise's releases:** 07
   7.4 has `tools/kitpin` verify a minisign signature. If mise signs in
   minisign's prehashed mode, the check needs BLAKE2b, which the Go
   standard library does not have and the complete v1 dependency list of
   12 12.1 does not name.
   Recommendation: An agent looks first: the header of the signature
   file in a mise release says which mode it is, and the finding goes
   into the pull request of T052. Only if it needs BLAKE2b is there a
   decision for the maintainer, and it is an ask-first approval
   (`dependencies`): add `golang.org/x/crypto` to the list of 12 12.1
   for `tools/kitpin` only; it is not linked into romeu.

10. **The two diagrams with no table source** (12 12.8, 12 12.3, 11
   11.4, 01 1.6). Needed before: T083, which lands before the candidate
   tag. 12 12.8 says `ARCHITECTURE.md` shows the candidate,
   promotion-commit, generation and probe machines "as generated
   diagrams", and 12 12.3 names one source for them, the transition
   tables of `internal/state`, which hold two. The promotion commit is a
   step list (01 1.6) and the probe lifecycle is the table of 11 11.4,
   which `tools/ci probes` implements. A generator change is outside
   `docs/`, so after the candidate tag it would need a new candidate (11
   11.2).
   Recommendation: **The plan's own reading:** the specification says
   generated, so T083 gives each of the two a transition table beside
   the two in `internal/state`, and one generator draws all four. The
   other reading, hand-written diagrams, changes 12 12.3 and 12 12.8,
   empties that task and puts the two diagrams in T095.

11. **The rule for `gh`** (12 12.1, 02 2.1, 07 7.3, 04 4.3). Needed
   before: T088. The rule asked of this plan has three parts: an exact
   release, a sha256 per architecture, and a check that compares the pin
   with the latest release. The first two follow from `gh` being in a
   four-platform `mise.lock` (12 12.1, 02 2.1), though no sentence says
   so for `gh`. The third is not in the specification: `julieta lock
   --check` compares `mise.toml` with `mise.lock` (07 7.3), and `julieta
   pin check` reads spec workloads and the julieta release in a config
   repo's workflows (04 4.3).
   Recommendation: The rule needs a change to the specification first,
   by the rule under "What v1 leaves out". Until then the release task
   builds what 12 12.1 says and carries no freshness criterion.

**The other gaps.** No task waits on these; the reading is in the last
column. The second column says how the review of this plan judged a gap
we reported, or that the review found it.

| # | Section | Review | What we found | Reading taken here |
|---|---|---|---|---|
| 12 | 12 12.2, 02 2.1 | REAL | the map names no owner for `e2e/fakesbx`, `e2e/scenarios`, `internal/shquote` and `examples/` | `probes`, `romeu-cli`, `render` and `spec` |
| 13 | 12 12.2, 12 12.3 | REAL | `tools/schemagen` is under `spec`, and `probes`, which comes first and depends on nothing, needs `probe-result.v1` | the generator lands in T014 with its first type |
| 14 | 11 (opening), 10 10.2 | REAL, wording only | "the first plan task can pass `tools/ci all` before any host run" is said of the probe harness, and the first plan task is `ci-bootstrap` | the harness is the first task of its module |
| 15 | 12 12.2, 11 11.2 | found in review | the `probes` row comes first in the build order, and the block B and sandbox-side definitions need the kits, the host suite and the release tooling's candidate | T082, after the kits and before the candidate tag |
| 16 | 12 12.3, S10 | found in review | S10 asks for a generated reference page per file format, and the generator table has a row for `project.md` and `runtime-event.md` only | one generator in T021, fed by each schema task |
| 17 | 12 12.10, 13 13.5, 13 13.6, 02 2.1, 03 (opening) | PARTIAL: wider than first reported | 03 says each format has a schema in `schemas/`; `dora.v1`, `ledger-entry/v1` and the view files are formats with none in the list of 02 2.1 | no schema is planned for the three |
| 18 | 12 12.7, 12 12.2 | REAL | `docs/lessons.md` and the `lesson` kind have no owning module, and each pull request has a Lessons section from T005 on | the file, the kind and the check land in T006 |
| 19 | S4, 03 3.5, 02 2.2, 12 12.1 | PARTIAL: the instance is named | 02 2.2 names the reference instance. What a task lacks is access to it and a Linux place to run julieta, which has no darwin build | blocks O4 and O5 |
| 20 | 06 6.1, 06 6.4, 02 2.1 | found in review | `tools/kitpin` has subcommands for the frontend, mise and herdr, and none for the workload digest that A12 and the container e2e read | the workload entry of the pin file is written by hand in T012 |
| 21 | 11 11.1 (A5), 03 3.3, 12 12.2 | a clarification: 03 3.3 already calls the goldens "probe-confirmed" | A5 runs `sbx env plan` on the `sbxenv.yaml` goldens and records their hashes, and `render`, which owns the goldens, comes after block A in the build order | the goldens are written with the probe definitions and `render` must reproduce them (T019); 11 11.1 could say so |
| 22 | 10 10.3, 11 11.1, 04 4.2 | the plan's own engineering; only the last step is the maintainer's | the fake `sbx` refuses a shape that is in no recording, and no table says which shapes and which starting sessions block A must record | the table of T019, derived from every end-to-end test that calls the fake; whether it becomes a table of 10 10.3 is the maintainer's |
| 23 | index, 02 2.1, 12 12.3 | found in review | this page is generated from task data that is not in this tree, and the tree of 02 2.1 has no place for a generator; a hand edit of the page is lost at the next generation | the generator lives with the project's planning records until the specification gives it a place; ticks are read back from the page it replaces |

The second review judged questions 1, 2, 3, 4, 6, 7, 8 and 11 real
decisions of the maintainer. It moved two of the earlier questions to
the other gaps: the goldens, since 03 3.3 already calls them
probe-confirmed, and the argv table, which is the plan's own
engineering. Question 9 starts with a lookup an agent can do, and only a
dependency it adds is the maintainer's. Questions 5 and 10 came out of
that review, and 5 is inferred. The registry and release API for the
building sandbox are a step of block O1, not a question.
