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

**The shape of a task.** A task is one logical change and one pull
request. It carries:

| Field | Meaning |
|---|---|
| `Merged` | the checkbox; ticked in the pull request that completes the task |
| Module | the module id of 12 12.2 |
| Implements | the sections of the specification, by number |
| Depends on | the tasks and operator blocks that come first |
| Operator | `no`, or the block (O1 to O6) the task waits for or feeds |
| Ask-first | the surfaces of 05 5.3 we expect the diff to touch. `tools/ci pr` computes the real list from the diff (12 12.4); this field is a forecast, so the maintainer can give the approval words early |
| Acceptance | what a test or a command checks |
| Verify | the command that shows it |

**Done, for each task** (10 10.8): `go run ./tools/ci all` passes, the
invariant tags are in place, the generated files are current, and for
host-facing behavior the matching scenario function exists. The
"Verify" line of a task names what is specific to it and does not
repeat this.

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
  is listed under [Questions for the maintainer](#questions-for-the-maintainer).
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

It lands in two steps, because the second needs a fact only a real
host can give.

| Step | Tasks | Ends in |
|---|---|---|
| 1 | T007 to T009 | `romeu version` and `julieta version --json` built for their four targets by `tools/ci all` on the four runners, with error ids, escaped output and a generated reference page |
| 2 | T013 and, after block O3, T021 and T022 | the compatibility check passing, and failing three ways, at the hybrid level (10 10.1; I30) |

Why this slice and not a larger one:

- It needs no gate, no render and no state, so it can start the day
  `ci-bootstrap` ends.
- It proves the four things the other tasks assume: one `tools/ci`
  run on four runners and locally (10 10.2); two binaries
  cross-built from one module with an import rule between them
  (10 10.7); the fake `sbx` replaying a block A recording and
  forwarding `env exec` into a julieta container (10 10.3); and the
  protocol, sha256 and read-only checks of julieta delivery (06 6.3).
- A defect in any of those four is cheapest to find before the
  packages of the later phases depend on it.

The slices that follow widen it in the order of 12 12.2: `sync`
(phase 4), julieta's chores (phase 5), `run` and the destructive
commands (phase 6), the kits (phase 7).

One deviation from the capability map, stated here because the slice
causes it: the map lists the error table under `romeu-cli`, which
depends on the modules above it (12 12.2). T008 and T009 land the
error table, the CLI writer and the `version` command before those
modules, because `spec`, `state` and `gitsafe` return error ids
(03, opening; 01 1.6; 10 10.6). The rest of `romeu-cli` keeps its
place in the order.

## Operator blocks

`sbx` runs only on the host (11, opening), so a step that needs it is
the maintainer's. So are the two `v*` tags, which the tag ruleset
refuses from anyone else (12 12.2), and the denylist entries, which an
agent must not hold (10 10.2, "Forbidden names"). These steps are six
blocks.

| Block | When | What the maintainer does | Unblocks | Recommendation |
|---|---|---|---|---|
| O1 | T001 is written and not pushed | the sitting of the maintainer block, items a to d (10 10.2): rulesets and the two repository settings first; one `go run ./tools/ci hygiene add` per forbidden name in a plain host clone; enable the hook, push the branch, open the pull request; then the four refusal tries from inside the sandbox, proved on the host | every later push; Q25 is settled by its output | do item a before the sandbox that writes T001 receives its token (10 10.2). Save each API answer in a file as you go: T001's Evidence needs them |
| O2 | T002's workflow is green | item e (10 10.2): add the jobs of the green run to the default-branch ruleset as required status checks, and save the API's answer | a merge that requires CI | one call; do it in the same sitting that merges T002 |
| O3 | T011 and T012 are merged | block A on both hosts, Intel and Apple silicon (11 11.1): `go run ./e2e/probes --block A --out docs/probes/`, about 75 minutes per host; commit the results and the recordings under `e2e/testdata/sbx/<version>/` | T020 to T022, and through them `sbxdrv`, the `sbxenv.yaml` goldens and the kits (A11 is go/no-go, 06 6.1) | run both hosts in one sitting: the arch-sensitive probes (A4, A5, A11, A12, A13) count only with both (11, opening). Phase 2 needs nothing from this block and proceeds meanwhile |
| O4 | T067 is merged | hand over the output of `julieta spec validate --catalog projects/*.yaml`, run from a `main` build against the reference config repo (feeds T068); after T068 merges, a rehearsal on one host (below); then push the tag `v1.0.0-rc.1` (11 11.2) | block O5 | the rehearsal is this plan's addition, not the specification's. It costs one run and can save a candidate: after the tag, any change outside `docs/` and the root Markdown files needs a new candidate and block B again (11 11.2) |
| O5 | the candidate's release is published | block B on both hosts (11 11.2): B1 to B5 and the sandbox-side checks C1 to C5, about 90 minutes per host plus those checks; commit the results under `docs/probes/` | T069 and the `docs` tasks; S3, S5, S7 and the host half of S8 | run B1 first on each host: every other result copies the candidate's tag and commit from it (11 11.2) |
| O6 | T072 is merged | push the tag `v1.0.0`; run B1 once more on v1.0.0, on one host, with `--out` outside the repository (11 11.2); run the reference config repo's `validate.yml` on the pinned julieta release and note its run id (S4) | T073, the last task | before tagging, check that the diff from the candidate's commit touches only `docs/` and root Markdown files: `tools/ci acceptance` fails otherwise (10 10.5) |

**The rehearsal in O4.** Build with
`go run ./tools/release build --version v1.0.0-rc.1 --dry-run`, then
run `go test -tags host -json ./e2e/host/...` on one host against that
build. It is not a block B result and is committed nowhere.

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
| `gh`, for `tools/release verify` | `mise.lock`, started through `mise exec` | 12 12.1, 10 10.2 | T065 |
| GitHub Actions | `uses` with a 40-hex commit SHA | 10 10.2 | T002, T065 |
| Go dependencies | `go.mod`, `go.sum`; the complete v1 list is in 12 12.1 | 12 12.1 | first use |
| kit frontend | `docker/sandbox-kit:3.0.0-m.<N>@sha256:<digest>`, one constant | 06 6.1 | T011 (probe input), T059 |
| workload | `<registry>/<repository>@sha256:<64 hex>` | 03 3.1, 06 6.2 | T011 (probe input), T042 |
| mise | version and both linux sha256 values; minisign checked at pin time | 07 7.4 | T059 |
| herdr | version and a sha256 per architecture, from an immutable release | 06 6.4 | T011 (probe input), T059 |

**The rule for `gh`.** `gh` is pinned to an exact release with a
sha256 per architecture, never to a floating latest, and a freshness
check compares the pin with the latest release. The specification
states the first half in part: `gh` is "pinned in `mise.lock`" and
started through `mise exec` (12 12.1). It does not state the sha256
per architecture for `gh` by name, and it has no check that compares
a pinned development tool with its latest release: `julieta lock
--check` compares `mise.toml` with `mise.lock` (07 7.3), and
`julieta pin check` reads spec workloads and the julieta release in a
config repo's workflows (04 4.3). T065 therefore carries the rule as
an acceptance criterion that waits for a change to the specification;
see [Questions for the maintainer](#questions-for-the-maintainer).

## Phases and checkpoints

| Phase | Tasks | Modules | Checkpoint: what must be true before the next phase |
|---|---|---|---|
| 0 Bootstrap | T001 to T006 | `ci-bootstrap` | C0: the hook and CI run the same `tools/ci`; the rulesets refuse the sandbox's token (O1, O2); a pull request without its sections fails `pr` |
| 1 Skeleton, step 1, and the probe harness | T007 to T013 | `termsafe`, `romeu-cli`, `julieta-core`, `probes` | C1: both binaries print `version` on the four runners; the harness passes `tools/ci all` before any host run (11, opening) |
| 2 First build layer (in parallel with O3) | T014 to T019 | `canon`, `spec`, `gitsafe`, `signing` | C2: block A results from both hosts are committed, and `tools/ci probes` passes on them |
| 3 Block A applied; second layer | T020 to T027 | `probes`, `sbxdrv`, `romeu-cli`, `catalog`, `oci`, `state`, `memstore` | C3: the version handshake passes at the hybrid level on both Linux runners |
| 4 The `sync` slice | T028 to T038 | `egress`, `render`, `gate`, `romeu-cli` | C4: J2 through approval, J7 through the `orphaned` report and J11 run in CI with the fake `sbx`; the I27 hostile trees are refused on the macOS runners |
| 5 julieta's chores | T039 to T052 | `julieta-core`, `layout`, `memstore`, `handoff`, `salvage`, `ledger` | C5: the container e2e passes on amd64 and arm64, with the `julieta setup` no-op inside its S8 bound |
| 6 `run` and the destructive commands | T053 to T058 | `romeu-cli` | C6: J2, J3b, J7, J10 and J11 pass in CI (S7, CI half); the meta-tests of 10 10.1 pass |
| 7 Kits and skills | T059 to T062 | `kits`, `skills` | C7: `tools/ci kits` passes; romeu embeds the four product kits |
| 8 Release tooling | T063 to T068 | `ci-release`, `catalog` | C8: `go run ./tools/release build --version v1.0.0 --dry-run` succeeds; then block O4 |
| 9 Acceptance | T069 to T073 | `probes`, `docs`, `ci-release` | `go run ./tools/ci acceptance` exits 0 on the committed `docs/acceptance.json` |

At each checkpoint the maintainer reads the next phase's "Ask-first"
fields and the open rows of
[Questions for the maintainer](#questions-for-the-maintainer).

## Tasks

### Phase 0 - Bootstrap

#### T001 - Hygiene, the denylist, `fast` and the pre-push hook

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (step 1; "Forbidden
  names"), 00 0.4, 12 12.1, 12 12.4, S11.
- Depends on: nothing. Operator: block O1 finishes and pushes it.
  Ask-first: none exists yet; the maintainer's review is the guard
  (10 10.2).
- Acceptance:
  - `go.mod`, `mise.toml` and `mise.lock` (Go and `golangci-lint`
    pinned, four platforms, 02 2.1), `COPYING`, `REUSE.toml` and
    `LICENSES/` exist (00 0.4).
  - `go run ./tools/ci hygiene` fails a fixture for each of its rules
    in 10 10.2: U+2014, a listed prose word (with the self-test of
    `tools/ci/prose.yaml`), a `TODO` without an issue, a personal
    absolute path, a tracked `go.work` or `vendor/`, a second file in
    `.githooks/`, and a missing or empty denylist.
  - The matcher's self-test plants a made-up name written together,
    with each separator and with a slash, against its own denylist,
    and expects a failure for each form except the slash.
  - `hygiene add` refuses to start without a terminal and takes no
    name as an argument; a `/dev/ptmx` test drives it (10 10.1).
  - The pushed-range walk passes its fixture-repository table: a new
    ref, a ref the remote lacks, a URL that is no remote, a merge
    commit, a deleted ref (10 10.2).
  - `.githooks/pre-push` is the one line of 12 12.1, mode 100755.
- Verify: `go run ./tools/ci fast` exits 0 after block O1 adds the
  denylist entries, and exits non-zero before.

#### T002 - `tools/ci workflows`, `tools/ci all` and a green `ci.yml`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (step 2; the workflow
  grammar; Runners), S2, S11.
- Depends on: T001, O1. Operator: block O2 follows its green run.
  Ask-first: none exists yet.
- Acceptance:
  - `tools/ci workflows` fails a fixture workflow for each construct
    outside the grammar of 10 10.2 (an `if`, a `shell`, an expression
    with an operator, a `uses` without a 40-hex SHA, a `run` line that
    is not `go run ./tools/ci` or `go run ./tools/release`, a file
    with another name ending), and fails a `ci.yml` without `edited`.
  - `go run ./tools/ci all` runs the steps of 10 10.2 that have code
    to check at this point: format and vet, lint (with `.golangci.yml`
    and the fixture test of ADR 0001 rule 14), unit, hygiene,
    workflows, `govulncheck`, the race run, `reuse lint`.
  - `ci.yml` passes on `ubuntu-latest`, `ubuntu-24.04-arm`,
    `macos-latest` and an Intel macOS runner whose label is verified
    to exist and to report `x86_64` (10 10.2, Runners).
- Verify: the run id of the green workflow, under the pull request's
  Evidence.

#### T003 - The ask-first list, `tools/new adr` and `tools/ci generated`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 05 5.3, 12 12.3, 10 10.2
  (step 3; the `generated` step).
- Depends on: T002, O2. Operator: no. Ask-first: none exists before
  this task.
- Acceptance:
  - `.github/ask-first.yaml` holds the surfaces of 05 5.3 and lists
    itself.
  - `go generate ./...` writes `.github/CODEOWNERS` and
    `docs/adr/README.md`; `tools/ci generated` fails a changed, a new
    and a removed generated file, comparing content before and after
    the run (10 10.2).
  - `go run ./tools/new adr <title>` writes the next number, the date
    from the injected clock and the filename from `slug(title)`,
    status Proposed (12 12.3; ADR 0001 rule 6).
  - `.github/pull_request_template.md` has the five sections of
    12 12.7.
- Verify: `go run ./tools/ci generated`; `go test ./tools/new/...`.

#### T004 - `tools/ci sequences`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 10 10.2 (the `sequences` step
  and the id table), ADR 0001 rules 6 and 7.
- Depends on: T003. Operator: no. Ask-first: `checks`.
- Acceptance:
  - A fixture fails for each rule of the step: a gap in the ADR
    numbers, a filename that differs from `slug(Title)`, a status
    outside the set, a one-way supersede link, a cited record that is
    not Accepted, a Deferred decisions row with an empty cell, a
    duplicate id, a referenced id or id range that is not defined.
  - It passes on this repository's `docs/spec.md`, `docs/spec/` and
    `docs/adr/`.
- Verify: `go run ./tools/ci sequences`.

#### T005 - `tools/ci pr`

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 12 12.4, 10 10.2 (the `pr` step
  and its inputs), 12 12.9 (the base branch), 12 12.10 (the form of a
  fix marker), 10 10.4 (goldens under Evidence).
- Depends on: T003. Operator: no. Ask-first: `checks`.
- Acceptance, each against a recorded event payload and a fixture
  repository (10 10.1):
  - a body without one of Why, What changed, Evidence, Lessons, or
    without a well-formed Middleware line, fails;
  - a touched ask-first surface without its approval line fails, with
    the list read at the base and at the head and joined; a line that
    names an unknown id fails; a body with CR LF line ends passes;
  - a title or a one-parent commit subject outside the Conventional
    Commit form fails; a merge commit's subject is not checked;
  - a base other than the default branch fails;
  - a malformed `Fixes-release:` trailer fails;
  - the forbidden-name check runs over the title, the body, the head
    ref name and the four readings of each commit;
  - a payload with neither accepted shape, or without one of the seven
    fields, fails.
- Verify: `go run ./tools/ci pr <payload file>` on a saved payload.

#### T006 - Contributor page skeletons and the lessons file

- [ ] Merged
- Module: `ci-bootstrap`. Implements: 12 12.8 (outlines), 12 12.7,
  12 12.3 (the `lesson` kind).
- Depends on: T005. Operator: no. Ask-first: `checks` (if a
  `tools/ci` file changes).
- Acceptance:
  - `ARCHITECTURE.md` and `CONTRIBUTING.md` exist with front matter
    (ADR 0001 rule 1) and the headings of their outlines.
  - `docs/lessons.md` exists with its first entry, what the bootstrap
    taught, written by `go run ./tools/new lesson`, which lands here
    because this entry is its first input (12 12.2).
- Verify: `go test ./tools/new/...`.

### Phase 1 - Skeleton, step 1, and the probe harness

#### T007 - `termsafe`, with the invariant tooling

- [ ] Merged
- Module: `termsafe`. Implements: 05 5.2 (I29, the tagging rules),
  12 12.3 (the `invariant` kind), 10 10.2 (`invariants`, `mutate`), S6.
- Depends on: T006. Operator: no. Ask-first: `termsafe`, `checks`.
- Acceptance:
  - A table test has one row per escaped class of I29 (control, bidi,
    zero-width, tag characters, U+2028 and U+2029); a fuzz target
    exists with committed seeds (10 10.1).
  - `go run ./tools/new invariant` scaffolds a tagged guard stub and a
    failing tagged test.
  - `tools/ci invariants` fails a guard tag without a test tag, the
    reverse, and a tagged id that is no row of 05 5.2.
  - `tools/ci mutate` builds the `mutate_I29` stub and a tagged test
    fails.
- Verify: `go test ./internal/termsafe/...`;
  `go run ./tools/ci invariants`.

#### T008 - The CLI contract: error table, exit codes, escaping writer

- [ ] Merged
- Module: `romeu-cli` (landed early, see "The first slice").
  Implements: 04 4.1, 10 10.6, 12 12.3 (the error-table row).
- Depends on: T007. Operator: no. Ask-first: `contracts`, `termsafe`
  (`internal/cli/writer.go`).
- Acceptance:
  - An error has an id `RJ-<exit><nn>`, a slug, a message and a fix
    hint in one data table; the text and `--json` forms match 04 4.1;
    tests assert ids, not message text.
  - The writer escapes by default (I29).
  - `go generate` writes `docs/reference/errors.md` and
    `docs/reference/exit-codes.md` with the generated-file header and
    front matter (12 12.3).
- Verify: `go test ./internal/cli/...`; `go run ./tools/ci generated`.

#### T009 - Two binaries that print their version

- [ ] Merged
- Module: `julieta-core` (with `romeu version` from `romeu-cli`).
  Implements: 04 4.1 (Parsing, Version), 04 4.2, 04 4.3, 12 12.3 (the
  `command` kind, command definitions), 10 10.2 (cross-build,
  `imports`, `vocabulary`), 10 10.7, 01 1.7, 05 5.2 (I1, I2).
- Depends on: T008. Operator: no. Ask-first: `checks`.
- Acceptance:
  - `romeu version` and `julieta version [--json]` print the fields of
    04 4.1; the julieta `--json` form reports whether its own
    directory is writable.
  - The cross-build step builds `romeu` for darwin amd64 and arm64 and
    `julieta` for linux amd64 and arm64.
  - `go run ./tools/new command` writes a stub, its help test and its
    reference entry; `docs/reference/` gains one generated page per
    command.
  - `tools/ci imports` enforces the table of 10 10.7 and carries the
    I1 and I2 guard tags, each with a one-violation fixture (05 5.2).
  - `tools/ci vocabulary`, generated from the table of 01 1.7, fails a
    "Not" phrase in docs, help text or an error message.
  - The container e2e runs `julieta version --json` in the workload's
    Debian base on both Linux runners (10 10.1).
- Verify: `go run ./tools/ci all` on the four runners.

#### T010 - Probe harness core and the result format

- [ ] Merged
- Module: `probes`. Implements: 11 (opening), 11 11.3, 12 12.3 (the
  `probe` kind), 10 10.1 (Schema), 03 (opening).
- Depends on: T006. Operator: no. Ask-first: `checks`, `dependencies`
  (the JSON Schema validator of 12 12.1).
- Acceptance:
  - The pure core computes `verdict` and `decision` from declared
    `onPass`, `onFail` or a decision table; a typed decision is
    refused (11 11.3).
  - `tools/schemagen` writes `schemas/probe-result.v1.json` from the
    Go type; `tools/ci schema` fails when the generated schema differs
    from the committed one and validates the examples.
  - `go run ./tools/new probe` writes a definition with `expect`, a
    decision table and `affects` to fill.
- Verify: `go test ./e2e/probes/...`; `go run ./tools/ci schema`.

#### T011 - Probe exec layer, the recorder and the block A definitions

- [ ] Merged
- Module: `probes`. Implements: 11 11.1, 10 10.3 (Source of truth,
  Redaction).
- Depends on: T010. Operator: feeds block O3. Ask-first: none
  expected.
- Acceptance:
  - The exec layer is tested against a helper binary re-executed from
    the test (10 10.1).
  - The recorder's test plants a fake token and a home path and
    asserts that neither reaches the file; each U+2014 becomes `-`
    (10 10.3).
  - A definition exists for each probe id of 11 11.1 (A1 to A6, A9 to
    A14), with its expectation written before any run.
  - The frontend, workload and herdr pins that A11, A12 and A13 need
    are constants of the harness (see "Toolchain pins").
- Verify: `go run ./e2e/probes --block A --out <dir>` against the
  helper binary in a test.

#### T012 - `tools/ci probes`

- [ ] Merged
- Module: `probes`. Implements: 11 11.4, 10 10.2 (the `probes` step),
  10 10.4.
- Depends on: T010. Operator: no. Ask-first: `checks`.
- Acceptance: a fixture per row of the lifecycle table of 11 11.4: a
  result whose `decision` differs from the computed one fails; an
  overturned result fails until `resolvedBy` names a commit that
  descends from the result's `commit`, touches each `affects` path and
  adds an ADR; a golden whose sha256 moved since its A5 result is
  flagged, and one with no recorded hash is not.
- Verify: `go run ./tools/ci probes`.

#### T013 - The fake `sbx`

- [ ] Merged
- Module: `probes` (the map names no owner for `e2e/fakesbx`; see
  [Questions for the maintainer](#questions-for-the-maintainer)).
  Implements: 10 10.3.
- Depends on: T011. Operator: no. Ask-first: none expected.
- Acceptance, on recordings of its own test data:
  - an argv shape absent from the recording fails the test with the
    argv (Replay only);
  - a call out of the recorded order fails (Stateful replay);
  - for `env exec`, the argv before `--` is matched and the command
    after it is forwarded;
  - the placeholder normalizer has a unit test (10 10.1).
- Verify: `go test ./e2e/fakesbx/...`.

### Phase 2 - First build layer

These tasks need no `sbx` fact and run while block O3 is under way
(12 12.2, the build order).

#### T014 - `canon`

- [ ] Merged
- Module: `canon`. Implements: 03 3.12, 10 10.2 (`coverage`), S9.
- Depends on: T008. Operator: no. Ask-first: `digests`, `checks`.
- Acceptance:
  - `canon.Digest(kind, v)` hashes `"romeu/<kind>/v1"`, a NUL byte and
    the canonical JSON; a test shows that two kinds over one value
    differ.
  - `tools/ci coverage` enforces the two thresholds of S9 and reports
    the packages below them.
- Verify: `go test ./internal/canon/...`; `go run ./tools/ci coverage`.

#### T015 - `spec`: the project spec and host settings

- [ ] Merged
- Module: `spec`. Implements: 03 3.1, 03 3.2, 03 3.4, 01 1.4 (the
  struct tags), 12 12.3, 05 5.2 (I8, I11, I12).
- Depends on: T010, T014. Operator: no. Ask-first: `gates`
  (`internal/spec/project.go`), `contracts`
  (`internal/spec/versions.go`).
- Acceptance:
  - Strict decode: an unknown key fails; `command`, `argv` or `env`
    under `secrets` fails (I8).
  - The validators are generated from `rules.go`; a table and a fuzz
    target with committed seeds cover the name and path rules of
    03 3.1 (I12), and a YAML round-trip fuzz covers each string field
    (I11).
  - `schemas/project.v1.json` and `schemas/host-settings.v1.json` are
    generated, and the field table of `docs/reference/project.md`
    comes from the `gate` and `apply` tags.
- Verify: `go test ./internal/spec/...`; `go run ./tools/ci schema`.

#### T016 - `spec`: the run layout and the examples

- [ ] Merged
- Module: `spec`. Implements: 03 3.2 ("Run layout"), 02 2.1
  (`examples/`), 03 (opening).
- Depends on: T015. Operator: no. Ask-first: `gates` (if
  `project.go` changes).
- Acceptance:
  - The validator refuses a pane id used twice, two focused panes in a
    tab, a ratio outside 0.1 to 0.9, an `env` key on the refused list
    and a `cwd` with a `..` element.
  - `examples/` holds an example config repo whose files validate
    against the generated schemas in `tools/ci schema`.
- Verify: `go test ./internal/spec/...`.

#### T017 - `gitsafe`: the hardened runner, fetch and refs

- [ ] Merged
- Module: `gitsafe`. Implements: 05 5.1, 05 5.2 (I3, I5, I6), 02 2.3
  (the ref namespaces).
- Depends on: T008. Operator: no. Ask-first: `gitsafe`.
- Acceptance:
  - A test asserts the environment and the flags of 05 5.1 on each
    invocation, and that nothing ambient passes through.
  - A `git://` URL whose host is not `127.0.0.1`, a `file://` URL and
    a plain path are refused, with a `caFile` set too.
  - The refspec validator table refuses a destination outside the
    allowlisted prefixes (I5); `CreateRef` fails on an existing ref
    (I6).
  - `floor.go` holds the initial floor 2.45.4; T020 fills the
    per-series list from A1.
- Verify: `go test ./internal/gitsafe/...`.

#### T018 - `gitsafe`: bundles, fsck and the tree walk

- [ ] Merged
- Module: `gitsafe`. Implements: 05 5.1, 05 5.2 (I27, unit level).
- Depends on: T017. Operator: no. Ask-first: `gitsafe`.
- Acceptance: the table of hostile trees of I27 is refused by
  `WalkTree` without relying on fsck: a symlink, a gitlink, the names
  `.`, `..` and `.git` with their case and HFS-ignorable variants, and
  a case collision; `bundle verify` and `unbundle` run under the
  hardened flags.
- Verify: `go test ./internal/gitsafe/...`.

#### T019 - `signing`

- [ ] Merged
- Module: `signing`. Implements: 06 6.4 (the signing rule's check),
  12 12.1.
- Depends on: T008. Operator: no. Ask-first: `signing`.
- Acceptance: against a test agent socket, the identity listing
  returns the keys; a socket that is unset, unreachable, or holds a
  number of keys other than one, or one that differs from the given
  key, is reported as such. No dependency and no subprocess is used.
  The code is **dark** until T061.
- Verify: `go test ./internal/signing/...`.

### Phase 3 - Block A applied; second layer

#### T020 - Apply the block A results

- [ ] Merged
- Module: `probes`. Implements: 11 11.4, 12 12.5, index ("Open
  questions"), 05 5.1 (the git floor).
- Depends on: O3. Operator: no. Ask-first: `decisions`, `gitsafe`,
  and each surface an `affects` path sits on.
- Acceptance:
  - For each result with `decision: default-overturned`, a commit
    touches each `affects` path and adds an ADR, and `resolvedBy` is
    set; `tools/ci probes` passes.
  - Each open question that block A settles alone (the "Settled by"
    column of the index) has its ADR.
  - `internal/gitsafe/floor.go` holds the per-series minimums A1
    verified, with sources.
  - `tools/ci hygiene` passes on the recordings.
- Verify: `go run ./tools/ci probes`.

#### T021 - `sbxdrv`

- [ ] Merged
- Module: `sbxdrv`. Implements: 10 10.3 (Parsers, Known argv),
  04 4.1 (Subprocesses), 05 5.2 (I4, I25), 12 12.1 (the version
  floor).
- Depends on: T013, T020. Operator: no. Ask-first: none expected.
- Acceptance:
  - Each argv builder produces a shape present in a recording; a scan
    finds no builder that emits `--auto-approve` (I4); the builder
    table rejects a value form of `sbx secret set` (I25).
  - The parsers run over each recorded `sbx` version.
  - `sbx` runs by absolute path with the scrubbed environment of
    04 4.1 and a timeout (10 10.6).
- Verify: `go test ./internal/sbxdrv/...`.

#### T022 - The version handshake

- [ ] Merged
- Module: `romeu-cli`. Implements: 06 6.3, 05 5.2 (I30), 10 10.1
  (E2E hybrid).
- Depends on: T009, T021. Operator: no. Ask-first: none expected.
- Acceptance, at the hybrid level on both Linux runners:
  - the compatibility check passes for a julieta of an accepted
    protocol, with the recorded sha256, in a read-only directory;
  - a protocol mismatch, a `SHA256SUMS` mismatch and a writable bin
    mount each fail it with exit 2.
  - The function is **dark** until T053 calls it from `run`.
- Verify: `go test -tags e2e ./e2e/...`.

#### T023 - `catalog`

- [ ] Merged
- Module: `catalog`. Implements: 03 3.5, 07 7.6, 10 10.2 (the
  `catalog` step).
- Depends on: T015. Operator: no. Ask-first: `catalog`, `checks`.
- Acceptance:
  - `catalog/egress.yaml` is embedded and decoded strictly;
    `schemas/catalog.v1.json` is generated.
  - `tools/ci catalog` fails a domain used anywhere without an
    explicit `upload` entry, and a malformed key.
  - The entries are the examples of 03 3.5; T068 builds the v1 table.
- Verify: `go run ./tools/ci catalog`.

#### T024 - `oci`

- [ ] Merged
- Module: `oci`. Implements: 01 1.4 (the normalized capability set),
  06 6.1 (Capabilities), 05 5.2 (I28).
- Depends on: T015, T020. Operator: no. Ask-first: none expected.
- Acceptance:
  - The `oci` table of I28 is refused row by row: a digest mismatch at
    each hop, an HTTP URL, an HTTPS-to-HTTP redirect, an oversize
    body, a credential helper in a fake Docker config.
  - The strict grammar refuses an anchor, an alias, a merge key, an
    include and an unknown key or capability type; a fuzz target
    covers the descriptor grammar (10 10.1).
  - Each conformance fixture recorded by A11 and A12 parses to the
    recorded capability set.
- Verify: `go test ./internal/oci/...`.

#### T025 - `state`: records, atomic writes and the lock

- [ ] Merged
- Module: `state`. Implements: 02 2.4, 03 3.8, 04 4.1 (Locking).
- Depends on: T014, T015. Operator: no. Ask-first: `gates`.
- Acceptance:
  - A record is written by temp file, fsync and rename, mode 0600; the
    state directory is 0700.
  - A second process that takes `romeu.lock` exits 1 with `RJ-101`.
  - `toolchain.json`, the project record and the descriptor cache
    have generated `state-*.v1` schemas.
- Verify: `go test ./internal/state/...`.

#### T026 - `state`: the candidate and generation machines

- [ ] Merged
- Module: `state`. Implements: 01 1.6, 12 12.3 (the transition-table
  row), 05 5.2 (I7 and I17, unit level).
- Depends on: T025. Operator: no. Ask-first: `gates`.
- Acceptance:
  - Each machine is a transition table; the generated tests exercise
    each row and assert each missing pair illegal, with exit 2 and an
    error id.
  - The diagrams in `ARCHITECTURE.md` are generated from the tables.
- Verify: `go test ./internal/state/...`;
  `go run ./tools/ci generated`.

#### T027 - `memstore`: the shared readers

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1, 05 5.2 (I12, I24), 03 3.9.
- Depends on: T014, T015. Operator: no. Ask-first: none expected.
- Acceptance:
  - The allowlist table refuses, through `os.Root` and Lstat, a
    symlink, a FIFO, a dotfile, a `.git` directory, a `..` name and a
    top-level name outside the five of 08 8.1; the caps are enforced.
  - `tools/ci imports` fails a romeu-linked package that imports
    `memstore/write` (I24).
  - `schemas/memory-entry.v1.json` is generated.
- Verify: `go test ./internal/memstore/...`.

### Phase 4 - The `sync` slice

#### T028 - `egress`

- [ ] Merged
- Module: `egress`. Implements: 07 7.5 (derivation), 07 7.3, 05 5.2
  (I9, I10).
- Depends on: T018, T023. Operator: no. Ask-first: `dependencies`
  (the TOML parser of 12 12.1).
- Acceptance:
  - The eight steps of 07 7.5 have a table test: a `mise.toml`
    without a lock is an error; an unknown `backend:tool` without a
    spec entry is exit 2 naming the tool; a lock host the catalog did
    not produce is gated.
  - A domain with `upload: true` or with no `domains` entry is gated,
    for catalog, base and kit-declared domains (I10).
  - The lock is read with `git cat-file` at `egressCommit`, and no
    mise process runs (07 7.1).
- Verify: `go test ./internal/egress/...`.

#### T029 - `render`: `sbxenv.yaml` and `render.json`

- [ ] Merged
- Module: `render`. Implements: 03 3.3, 03 3.7, 13 13.1 (the fixed
  mounts), 10 10.4, 10 10.2 (`golden`), 05 5.2 (I8, I11).
- Depends on: T020, T024, T028. Operator: no. Ask-first: `checks`.
- Acceptance:
  - The goldens match the shape of 03 3.3, with the three fixed
    mounts, no `x-*` key and no `lifecycle` hook; each carries the
    sha256 A5 recorded.
  - The secret command is built by `internal/shquote` (table-tested)
    in the `/usr/bin/env -i` form; no literal value is rendered (I8).
  - `tools/ci imports` rejects `text/template` in `internal/render`
    (I11); `tools/ci golden` fails on `-update` in CI and lists the
    changed goldens.
  - `schemas/render.v1.json` is generated.
- Verify: `go test ./internal/render/...`; `go run ./tools/ci golden`.

#### T030 - `render`: kit materialization and the workspace files

- [ ] Merged
- Module: `render`. Implements: 06 6.1 (Materialization,
  Capabilities, Personal kits), 01 1.4, 02 2.3, 05 5.2 (I13, I27,
  I28).
- Depends on: T029. Operator: no. Ask-first: none expected.
- Acceptance:
  - A personal kit is read from the config commit's tree through
    `git fsck --strict` and `WalkTree`, and written through `os.Root`
    into the candidate only.
  - A personal kit that declares a network, mount, volume, credential
    or ssh-agent capability is a sync error; another capability type
    is kept for the gate diff.
  - Each workspace file decodes to the key set {`folders`}; a memory
    directory is in neither, and a review checkout is only in
    `review.code-workspace` (I13).
  - Product kits come from an embed seam that tests fill with fixture
    kits; T060 fills it with the real ones.
- Verify: `go test ./internal/render/...`.

#### T031 - `render`: the promotion commit, recovery and drift

- [ ] Merged
- Module: `render`. Implements: 01 1.6 ("Promotion commit"), 01 1.2
  (the drift rule), 06 6.3 (step 1), 05 5.2 (I15, I16).
- Depends on: T026, T030. Operator: no. Ask-first: `checks` (the
  admitted-call-site table).
- Acceptance:
  - The four steps run in order; a test hook stops after each, and
    recovery finishes a promotion whose candidate verifies against
    its `filesDigest`, or refuses with exit 4 when it does not.
  - A hand edit of `sbxenv.yaml`, of a kit file or of
    `.romeu/bin/SHA256SUMS` is reported as drift (I15).
  - The I16 scan fails each listed call kind outside a function the
    admitted-call-site table names (05 5.2).
- Verify: `go test ./internal/render/...`.

#### T032 - `gate`

- [ ] Merged
- Module: `gate`. Implements: 01 1.4 (gate 2), 12 12.3 (the struct-tag
  row), 05 5.2 (I7, I29).
- Depends on: T031. Operator: no. Ask-first: `gates`.
- Acceptance:
  - The widening set and the recreate digest field lists are
    generated from the struct tags; the generated flip table shows
    that each `gate:"widening"` field flips the digest and each
    untagged field does not (I7).
  - The diff is not truncated: a 10 000-line kit file change appears
    in full, the summary is repeated above the prompt, and a binary
    file is shown as size and sha256, flagged (I29).
  - The prompt needs a terminal; a `/dev/ptmx` test drives `y` and
    `N` (10 10.1).
- Verify: `go test ./internal/gate/...`.

#### T033 - `romeu init`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`init`), 09 (J1, step 3),
  03 3.4, 05 5.2 (I20).
- Depends on: T017, T025. Operator: no. Ask-first: `contracts`.
- Acceptance, in the e2e with origins served over TLS (10 10.1):
  - `init` writes host settings with absolute tool paths and the real
    root path, and clones the config repo to
    `<root>/<cfg>-env/<dir>`;
  - it refuses to overwrite existing settings;
  - it refuses a root inside a git repository, equal to `$HOME`, or
    holding host settings or state (I20).
- Verify: `go test -tags e2e ./e2e/...`.

#### T034 - `romeu sync` and `romeu approve <name>`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("How `romeu sync` renders,
  gates and promotes", steps 1 to 8 and 10), 01 1.6 ("Candidate"),
  09 (J2), 05 5.2 (I7, I8, I9, I12, I24, I27, I28).
- Depends on: T027, T032, T033. Operator: no. Ask-first: `contracts`,
  `gates`.
- Acceptance, in the e2e with the fake `sbx`:
  - a first `sync` on a terminal promotes after `y`, and leaves the
    live files byte-identical after `N`;
  - without a terminal it records an awaiting candidate and exits 3;
    `approve <name>` promotes it; `approve` without a terminal exits
    2, and after the candidate files changed it exits 4;
  - `--from <sha>` refuses a commit not reachable from
    `refs/romeu/origin/*`;
  - the I9 cases leave egress unchanged; a memory directory that
    violates the allowlist is exit 2 (I24); each hostile kit entry
    fails before a file is written outside `.romeu/candidates/`, on
    the macOS runners too (I27); adding a capability to a kit flips
    gate 2 (I28);
  - an absent project is reported as `orphaned`, and nothing is
    removed.
- Verify: `go test -tags e2e ./e2e/...`.

#### T035 - Gate 1 and the live changes of `sync`

- [ ] Merged
- Module: `romeu-cli`. Implements: 01 1.4 (gate 1), 04 4.2
  (`approve --toolchain`; `sync` step 9), 07 7.5 (Application),
  09 (J9, J11).
- Depends on: T021, T034. Operator: no. Ask-first: `gates`,
  `contracts`.
- Acceptance:
  - `approve --toolchain` prints the julieta version change and the
    catalog diff per affected project before the prompt, and writes
    `toolchain.json`.
  - A different `sbx version` or catalog digest makes `sync` exit 3
    before it promotes (I7).
  - With a running sandbox, `sync` reconciles egress with the
    recorded argv and reports "recreate needed" when the recreate
    digest differs.
  - The `secrets` field's `apply` tag follows the A10 result (Q18).
  - A spec secret without its `name@project` binding is exit 2
    naming the key (J11, step 4).
- Verify: `go test -tags e2e ./e2e/...`.

#### T036 - `romeu status`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`status`), 09 (J7 step 2,
  J10 step 1, J13 step 3).
- Depends on: T035. Operator: no. Ask-first: `contracts`.
- Acceptance: on fixtures of host state, `status --json` reports
  drift, an interrupted promotion, an awaiting candidate, a stale
  toolchain, an orphaned project and an orphaned repo dir, without
  taking the lock; it exits 0 or 1 and never 3. The fields that come
  from julieta land with T039.
- Verify: `go test -tags e2e ./e2e/...`.

#### T037 - `romeu doctor` and the preflight

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("Checks run by
  `romeu doctor`"), 01 1.5, 05 5.2 (I14, I15, I20, I22, I23).
- Depends on: T035. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - Each row of the doctor table that this phase can build has a
    fixture with `HOME` in a temp dir: the `sbx` and git floors, the
    toolchain record, the host global git config, the `sbx` globals
    from block A recordings (I22), the root rules (I20), the symlink,
    mise, direnv and VS Code trust rows (I14, I23), the file modes,
    the secret bindings, and the tree rows. The signing row lands
    with T061 and the ledger rows with T057.
  - The preflight runs its six steps in the order of 01 1.5 and stops
    at the first failure with the exit of its row; the rows marked
    **pre** share one implementation with `doctor`.
  - The preflight touches no network: it passes with the registry and
    the origins unreachable after `sync` (10 10.1).
- Verify: `go test ./internal/... -run Doctor`;
  `go test -tags e2e ./e2e/...`.

#### T038 - `romeu adopt`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`adopt`), 01 1.6
  ("Generation"), 05 5.2 (I26).
- Depends on: T037. Operator: no. Ask-first: `contracts`, `gates`.
- Acceptance: a foreign sandbox of the project's name makes a **P**
  command exit 2 with `RJ-203` naming `romeu adopt`, before any
  mutation; `adopt` without a terminal, with a workspace path that
  differs, or with an open generation exits 2; after `adopt` the
  record has `adopted: true` and an unknown recreate digest.
- Verify: `go test -tags e2e ./e2e/...`.

### Phase 5 - julieta's chores

#### T039 - The manifest, `julieta setup` and `julieta status`

- [ ] Merged
- Module: `julieta-core`. Implements: 03 3.6, 04 4.3 (`setup`,
  `status`), 02 2.5, 06 6.3 (step 5).
- Depends on: T022. Operator: no. Ask-first: `contracts`.
- Acceptance, in the container e2e:
  - a manifest of protocol N or N-1 is accepted and another is
    refused; a 32 KiB manifest round-trips;
    `schemas/manifest.v1.json` is generated;
  - `setup` links `$HOME/.local/bin/julieta` to the running binary,
    clones a missing secondary repo, and fast-forwards a default
    branch that is checked out and clean, reporting the others;
  - a command whose Reads cell names the manifest exits 2 without
    one (04 4.3);
  - `status --json` reports the per-repo fields of 04 4.3.
- Verify: `go test -tags e2e ./e2e/...`.

#### T040 - The mise driver: `install`, `lock` and `tools/ci mise`

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`install`, `lock`; the
  mise environment), 07 7.2, 07 7.3, 10 10.2 (the `mise` step),
  09 (J8), S8.
- Depends on: T039. Operator: no. Ask-first: `checks`.
- Acceptance:
  - `install` runs with the environment of 04 4.3 and skips when the
    `lock-set` digest equals the last success; a `mise.toml` without
    a lock fails with the hint.
  - `lock --check` fails a tool without a lock entry and a lockable
    entry without both linux platforms, and warns on a missing macOS
    entry.
  - `tools/ci mise` runs that logic on this repository's lock and the
    examples'.
  - The `julieta setup` no-op is at most 3 s, median of 5, in the
    container e2e against local origins (S8).
- Verify: `go test -tags e2e ./e2e/...`; `go run ./tools/ci mise`.

#### T041 - The hook dispatcher

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`hooks run`), 08 8.2
  (recorded failures), 12 12.4 (the dispatcher runs the tracked
  hook), 05 5.2 (I21).
- Depends on: T040. Operator: no. Ask-first: none expected.
- Acceptance, in the container: the dispatcher runs julieta's own
  action, then `.githooks/<hook>`, then `$GIT_DIR/hooks/<hook>`,
  passing arguments and stdin as 04 4.3 says; the first non-zero
  status stops; a failure is recorded in julieta state and shown
  until a later run of the same hook succeeds; `pre-commit` runs
  `lock --check` when a mise file is staged.
- Verify: `go test -tags e2e ./e2e/...`.

#### T042 - `julieta spec validate` and `julieta pin`

- [ ] Merged
- Module: `julieta-core`. Implements: 04 4.3 (`spec validate`,
  `pin workload`, `pin check`), 07 7.6, 06 6.2, 09 (J2 step 1, J9).
- Depends on: T024, T039. Operator: no. Ask-first: none expected.
- Acceptance:
  - `spec validate` exits 2 on a decode or rule error, per file and
    across files; `--catalog` lists each unknown `backend:tool` and
    lock host and exits 1 when it lists one.
  - `pin workload` rewrites the digest after it verifies that the
    manifest list covers linux/amd64 and linux/arm64, through the
    guarded client of `oci`.
  - `pin check` reports a pin that is not a digest or is behind.
- Verify: `go test ./internal/... -run 'Validate|Pin'`.

#### T043 - `layout`

- [ ] Merged
- Module: `layout`. Implements: 03 3.2 (the execution rule), 04 4.3
  (`layout up`, `pane run`), 10 10.1 (Golden).
- Depends on: T016, T039. Operator: no. Ask-first: none expected.
- Acceptance:
  - `layout up --dry-run --json` matches the golden `layout.apply`
    requests for the example run layout.
  - `pane run` runs each step as `mise exec -C <pane dir> -- <argv>`,
    stops at a failing step with its status and drops to a login
    shell.
  - `layout up` applies against the pinned herdr in the container,
    checks the socket protocol number and fails closed on another
    (Q16), and reports "layout stale" when the `run` digest differs.
- Verify: `go test -tags e2e ./e2e/...`.

#### T044 - `memstore/write` and the memory commands

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1, 03 3.9, 04 4.3 (`memory
  add|list|show|edit|rm|search`, `memory check`).
- Depends on: T027, T039. Operator: no. Ask-first: none expected.
- Acceptance:
  - julieta stamps `id`, `created` and `updated`; a supplied id or
    timestamp is an error.
  - Two julieta processes writing one store lose no entry (10 10.1,
    Concurrency).
  - `memory check` exits 1 on an unmounted, unwritable or
    non-conforming directory.
- Verify: `go test -tags e2e ./e2e/...`.

#### T045 - Memory import and verify

- [ ] Merged
- Module: `memstore`. Implements: 08 8.1 ("Import and verification"),
  03 3.9, 04 4.3, 00 0.5.
- Depends on: T044. Operator: no. Ask-first: none expected.
- Acceptance: a second import of one file imports nothing and reports
  `imported + skipped-duplicate == read`; an invalid line makes the
  exit 1; `verify --against` fails on a digest set difference and on
  a `status` difference of a matched entry.
- Verify: `go test -tags e2e ./e2e/...`.

#### T046 - `handoff`

- [ ] Merged
- Module: `handoff`. Implements: 08 8.2, 08 8.3, 03 3.10, 04 4.3
  (`handoff write|show|list`), 09 (J5).
- Depends on: T041, T044. Operator: no. Ask-first: none expected.
- Acceptance:
  - `write` refuses a narrative without its headings (six for
    `final`) and stamps the front matter; `--facts` has no body.
  - The handoff reader returns the greatest ULID among `clear` and
    `final` as the narrative; a later `facts` file does not hide it.
  - The container e2e runs SessionEnd and SessionStart in both
    orders (Q22).
  - `show --hook` prints the open entries, the `lesson` entries and
    julieta's warnings; the front matter has a golden (10 10.1).
- Verify: `go test -tags e2e ./e2e/...`.

#### T047 - `julieta snapshot`

- [ ] Merged
- Module: `salvage`. Implements: 08 8.4, 04 4.3 (`snapshot`).
- Depends on: T041. Operator: no. Ask-first: none expected.
- Acceptance: the dispatcher calls it after `post-commit`,
  `post-rewrite` and `post-merge`; the bundle holds the local
  branches, tags, notes, `salvage/*` refs and each worktree HEAD
  minus the objects reachable from `repos[].base`; a snapshot that
  races a commit leaves a valid bundle (10 10.1); a repo-local
  `core.hooksPath` is reported as "snapshots disabled".
- Verify: `go test -tags e2e ./e2e/...`.

#### T048 - `julieta salvage`, the sandbox half

- [ ] Merged
- Module: `salvage`. Implements: 08 8.5 ("Sandbox half"), 03 3.11,
  04 4.3 (`salvage`), 05 5.2 (I18), 10 10.7.
- Depends on: T047. Operator: no. Ask-first: none expected.
- Acceptance, in the container: the salvage completeness cases of
  10 10.1: dirt in a linked worktree, a stash, a detached HEAD, a
  nested repository, an ignored file over the cap listed as
  `over-cap`, a transcript listed as `transcript-excluded`; salvage
  commits carry no signature with `commit.gpgsign=true` set (I18);
  `salvage` does not import `agent/*` and takes the profile as an
  argument; `schemas/salvage.v1.json` is generated.
- Verify: `go test -tags e2e ./e2e/...`.

#### T049 - Salvage verification and ref import, the host half

- [ ] Merged
- Module: `salvage`. Implements: 08 8.5 ("Host half"), 05 5.2 (I6,
  I17).
- Depends on: T018, T026, T048. Operator: no. Ask-first: none
  expected.
- Acceptance: the manifest is read through `os.Root`; completeness
  is recomputed and the manifest's own `complete` ignored; refs are
  created only under `refs/romeu/salvage/...` with `CreateRef`. The
  package is **dark** until T055.
- Verify: `go test ./internal/salvage/...`.

#### T050 - `ledger`: the event, julieta's emit and drain

- [ ] Merged
- Module: `ledger`. Implements: 13 13.2, 13 13.3 (Draining), 13 13.7,
  04 4.3 (`event add`, `event list`; the `command-failed` rule),
  04 4.4, 12 12.3 (the event row), 05 5.2 (I33).
- Depends on: T041, T023. Operator: no. Ask-first: `ledger`,
  `contracts`.
- Acceptance:
  - The validator and the membership tables of `type`, `tool` and
    `ref` are generated; the per-field table of 13 13.10 passes.
  - A julieta command that exits non-zero writes one
    `command-failed` event when the manifest names a spool; `hooks
    run`, `pane run` and `event` write none; a failure to emit
    changes no output and no exit status.
  - The drain rows of 13 13.10 pass.
  - `event add` takes no free text and needs at least one flag.
  - `schemas/runtime-event.v1.json` is generated.
- Verify: `go test ./internal/ledger/...`.

#### T051 - `ledger`: the spool reader, ingest and entries

- [ ] Merged
- Module: `ledger`. Implements: 13 13.3, 13 13.4, 13 13.5, 13 13.8,
  05 5.2 (I31, I32).
- Depends on: T050, T027, T031. Operator: no. Ask-first: `ledger`,
  `checks`.
- Acceptance:
  - The hostile spool table of 13 13.10 passes row by row, with the
    `afterOpen` hook and an injected version table.
  - The entries table of 13 13.10 passes, the racing ingests without
    the lock included.
  - The I16 scan fails a fixture of `internal/ledger` with each call
    13 13.5 does not admit; `tools/ci imports` fails a caller of the
    spool reader the table does not name (I32).
  - The starting values are those of 13 13.8.
  - The ingest function is **dark** until T057.
- Verify: `go test ./internal/ledger/...`.

#### T052 - `ledger`: the view

- [ ] Merged
- Module: `ledger`. Implements: 13 13.6, 05 5.2 (I33), 10 10.1
  (Golden).
- Depends on: T051. Operator: no. Ask-first: `ledger`.
- Acceptance: the two views of the two-project fixture match their
  goldens, ordered by `ingested`, project and key, with no `id` and
  no key of the other project in `others.jsonl`; a derivation that
  changes nothing writes nothing; `julieta setup` exits 2 with
  `ledger-view-writable` when it can write the view.
- Verify: `go test ./internal/ledger/...`.

### Phase 6 - `run` and the destructive commands

#### T053 - `romeu run`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 ("How `romeu run` reaches
  the run layout"), 01 1.5, 01 1.6 ("Generation"), 07 7.5, 09 (J3,
  J10), S8, 05 5.2 (I1, I7, I30, I33).
- Depends on: T038, T043, T052. Operator: no. Ask-first: `contracts`,
  `gates`.
- Acceptance, hybrid on the Linux runners:
  - J3b passes: a generation is recorded before `sbx env run`, egress
    is reconciled, the compatibility check and `setup` run, and the
    final argv equals the one of I1 byte for byte.
  - A failed create removes the record when the sandbox is absent and
    keeps it when present.
  - A writable view mount stops `run` before `layout up` (I33).
  - `run --timings --json` prints `run-timings.v1` (schema generated);
    `romeuMs` is at most 2 s, median of 5, with the fake `sbx` (S8).
  - The lock is released before the final `exec` (04 4.1).
- Verify: `go test -tags e2e ./e2e/...`.

#### T054 - `romeu stop` and `romeu pull`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`stop`; "How `romeu pull`
  updates a review checkout"), 09 (J4), 05 5.2 (I2, I5, I19).
- Depends on: T047, T053. Operator: no. Ask-first: `contracts`.
- Acceptance:
  - `stop` runs `julieta snapshot --all`, then the recorded
    `sbx stop` argv.
  - `pull` re-reads the daemon URL before the fetch (I19: the fake
    `sbx` changes the port between calls); a head mismatch with
    `julieta status` exits 1 and updates nothing.
  - After `pull`, `refs/heads`, `refs/tags`, `refs/remotes` and
    `HEAD` are unchanged (I5); the review repo's local config equals
    the fixed set (I2).
  - `pull` succeeds while a `run` of the project is attached
    (10 10.1).
- Verify: `go test -tags e2e ./e2e/...`.

#### T055 - `romeu salvage`, `rm` and `recreate`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`salvage`, `rm`,
  `recreate`), 08 8.5, 01 1.6, 09 (J6, J10, J13), 05 5.2 (I6, I17).
- Depends on: T049, T053. Operator: no. Ask-first: `contracts`,
  `gates`.
- Acceptance:
  - `rm` exits 5 for each case of I17: a missing repo in the
    manifest, a forged `complete: true`, a hidden branch, a detached
    HEAD commit, dirt in a linked worktree, an unbundled tag, a
    corrupted payload file, a salvage id mismatch, daemon heads not
    in the bundle, and a repo removed from the spec after the
    generation was created.
  - `--accept-loss` removes the sandbox after an incomplete salvage;
    `rm` warns when the generation has no `final` handoff.
  - After a same-name recreate, a fetch with `--prune` and a forced
    refspec leaves the salvage refs, and a second create on one of
    them fails (I6).
  - `salvage --from-host` and the absent-sandbox row of `run` record
    `result: lost` (J10).
- Verify: `go test -tags e2e ./e2e/...`.

#### T056 - `romeu retire`

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`retire`), 09 (J7),
  13 13.4 (the second ingest), 02 2.3 (the invariants of the tree).
- Depends on: T055. Operator: no. Ask-first: `contracts`.
- Acceptance: J7 passes in CI: `retire` lists the unpushed and
  salvage-only work per repo, exits 5 on a declined confirmation or
  without a terminal and without `--force`, moves `<name>-env/` to
  `.attic/<name>/<ts>/`, drops the project from the workspace files
  and moves its state to the state attic; nothing is deleted.
- Verify: `go test -tags e2e ./e2e/...`.

#### T057 - `romeu handoff`, `romeu ledger` and the ledger's wiring

- [ ] Merged
- Module: `romeu-cli`. Implements: 04 4.2 (`handoff`, `ledger
  ingest`, `ledger query`; the three ledger rows of the doctor
  table), 13 13.4, 13 13.9, 08 8.5 (the ingest before the sandbox
  half), 05 5.2 (I24, I29, I32).
- Depends on: T046, T055, T056. Operator: no. Ask-first: `contracts`,
  `ledger`.
- Acceptance:
  - `romeu handoff` reads through the handoff reader and prints a
    body and a branch name with ESC sequences escaped (I29).
  - Ingest runs inside `sync`, `run`, `salvage` and `retire` at the
    step each names; with an ingest that fails, `salvage` and `rm`
    end with the exit status they would have had, and the salvage
    record holds `ledger-incomplete` (13 13.10).
  - `ledger ingest` exits 1 with `ledger-incomplete`; `ledger query`
    orders by `ingested`, project and key.
  - The doctor rows re-hash each ingested entry and derive each view
    again, on fixtures.
- Verify: `go test -tags e2e ./e2e/...`.

#### T058 - The meta-tests, the scenarios and the host suite

- [ ] Merged
- Module: `romeu-cli` (the map names no owner for `e2e/scenarios`).
  Implements: 10 10.1 ("Additional required tests"), 10 10.3 (Shared
  scenarios), 09, 05 5.2 (I2, I3, I14, I15, I16), ADR 0001 rule 13,
  S7.
- Depends on: T057. Operator: no. Ask-first: none expected.
- Acceptance:
  - The idempotency meta-test covers each converging command and each
    appending command of 04 4.1.
  - Promotion fault injection: after a kill at each step, each **P**
    command finishes the promotion or exits 4, and a re-sync
    converges (I7).
  - The sweeps pass: the `.git/` comparison of I2, the hostile
    configuration of I3 over the nine commands it lists, the
    filesystem diff of I14 and I16, drift for each **P** command
    (I15).
  - `e2e/scenarios` has a function per journey, J1 to J13, reading
    `commands.yaml`; a table entry no function reads fails the
    package's test. The J8 function replays the agent's lock change
    as a commit on the fixture origin, and the J12 function runs J1
    steps 3 to 6 under a fresh root, settings and state (09).
  - J2, J3b, J7, J10 and J11 pass in CI (S7); `e2e/host` compiles
    under the `host` tag and calls the same functions.
- Verify: `go test -tags e2e ./e2e/...`;
  `go vet -tags host ./e2e/host/...`.

### Phase 7 - Kits and skills

#### T059 - `tools/kitpin`, the `kit` kind and `tools/ci kits`

- [ ] Merged
- Module: `kits`. Implements: 06 6.1 (Frontend), 06 6.4 (herdr),
  07 7.4, 12 12.3 (the `kit` kind), 10 10.2 (the `kits` step).
- Depends on: T020. Operator: no. Ask-first: `kits`, `checks`.
- Acceptance:
  - `kitpin frontend` rewrites the one frontend constant; `kitpin
    mise <version>` writes the version and both linux hashes after it
    verifies the signed checksum file; `kitpin herdr <version>` takes
    each hash from the release API's digest and refuses a release
    that is not marked immutable. Each has a test against a recorded
    fixture.
  - `tools/ci kits` fails a second frontend pin, an install step
    over five lines, a download without a checksum line after it,
    and a download line with a pipe or a command substitution.
- Verify: `go run ./tools/ci kits`; `go test ./tools/kitpin/...`.

#### T060 - The `julieta` and `os-base` kits, embedded

- [ ] Merged
- Module: `kits`. Implements: 06 6.4, 02 2.1 (Embedding), 07 7.2,
  S3.
- Depends on: T059, T030. Operator: no. Ask-first: `kits`.
- Acceptance: the two kits use the descriptor file name and the
  capability names A11 recorded; `julieta` pins mise and herdr and
  declares the two install-phase domains of 06 6.4; the `os-base`
  package list is the missing set A12 measured; romeu embeds
  `kits/*` and `sync` materializes them into content-addressed
  directories; `tools/ci kits` passes. That they build and start is
  measured by B2 (S3).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/...`.

#### T061 - The `julieta-claude` and `git-ssh-sign` kits, and the signing rule

- [ ] Merged
- Module: `kits`. Implements: 06 6.4 (the two kits; "Signing rule"),
  08 8.1 ("How julieta replaces the agent's built-in memory"),
  08 8.2, 01 1.5 (step 5), 04 4.2 (the signing row of `doctor`),
  04 4.1 (`SSH_AUTH_SOCK` for the `sbx` calls), S5.
- Depends on: T019, T060. Operator: no. Ask-first: `kits`, `signing`,
  `contracts`.
- Acceptance:
  - `julieta-claude` installs the two skills and the SessionStart and
    SessionEnd hooks, and makes the agent's own memory directory
    non-writable (Q8).
  - `sync` refuses to render `git-ssh-sign` with exit 2 and
    `RJ-204` unless the socket is set, reachable and holds one key
    equal to `signingKey`; `doctor` and the preflight of that
    project's **P** commands run the same check.
  - That the hooks, the skills and the one key are present in a real
    sandbox is measured by B2 (S5).
- Verify: `go test -tags e2e ./e2e/...`.

#### T062 - The skills

- [ ] Merged
- Module: `skills`. Implements: 02 2.1 (`skills/`), 08 8.1, 08 8.3.
- Depends on: T046. Operator: no. Ask-first: none expected.
- Acceptance: `skills/julieta/SKILL.md` tells the agent to use
  `julieta memory` and to tag a lesson; `skills/handoff/SKILL.md`
  pipes the required headings to `julieta handoff write [--final]`;
  the `julieta` commands each names exist in the command
  definitions, which a test checks.
- Verify: `go test ./... -run Skills`.

### Phase 8 - Release tooling

#### T063 - `tools/ci docs`, `SECURITY.md` and the `README.md` skeleton

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 (the `docs` step),
  ADR 0001 rules 1, 5, 9, 12, 13, 17 and 20, 12 12.8, S10, index
  ("Deferred decisions": the linter and spell checker row).
- Depends on: T058. Operator: no. Ask-first: `checks`,
  `dependencies` (if a tool enters `mise.lock`).
- Acceptance:
  - A fixture Markdown file fails for each rule the step owns, with
    one violation per case of rule 9 (10 10.1).
  - The pull request picks the Markdown linter and the spell checker,
    with their configuration files on the `checks` globs, and fixes
    the format and path of the accepted-words list; the two rows of
    the Deferred decisions table leave it in the same change.
  - `docs/spec.md` and the pages under `docs/spec/` move to front
    matter (ADR 0001, Consequences).
  - `SECURITY.md` has the three headings of rule 20, and `README.md`
    the heading order of rule 12; both lists are in
    `tools/ci/headings.yaml`.
- Verify: `go run ./tools/ci docs`.

#### T064 - `tools/ci lessons`, `links` and `fuzz`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 (the `lessons` step; the
  subcommands outside `all`), 10 10.1 (Fuzz), 12 12.7.
- Depends on: T063. Operator: no. Ask-first: `checks`,
  `dependencies` (`fuzz.yml`).
- Acceptance: `lessons` fails an entry that names no existing
  subcommand or test and gives no "no check possible" reason;
  `links` fetches each external URL the docs cite and is called by
  no workflow; `fuzz` runs each target for a fixed time, and the
  scheduled `fuzz.yml` calls it and then `tools/ci mutate`.
- Verify: `go run ./tools/ci lessons`; `go run ./tools/ci links`.

#### T065 - `tools/release` and `release.yml`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.2 ("Release and
  bootstrap"), 12 12.6, 02 2.1 (Embedding), S1.
- Depends on: T060, T064. Operator: no. Ask-first: `release`,
  `dependencies`, `checks`.
- Acceptance:
  - `tools/release build --version v1.0.0 --dry-run` builds the two
    julieta binaries, embeds them with the kits and the catalog,
    builds romeu for darwin and writes the archives and
    `checksums.txt`.
  - `tools/release notes` groups Conventional Commit subjects since
    the last tag that is not a prerelease, with each pull request's
    Why.
  - `tools/release verify` runs `gh attestation verify` with the
    signer workflow, through `mise exec`; `publish` marks a tag with
    a suffix as a prerelease and never creates a draft.
  - `release.yml` passes `tools/ci workflows`.
  - `gh` is in `mise.lock` at an exact release (12 12.1). Waiting on
    a change to the specification: a sha256 per architecture stated
    for it, and a check that compares the pin with the latest
    release (see "Toolchain pins").
- Verify: `go run ./tools/release build --version v1.0.0 --dry-run`.

#### T066 - `tools/ci dora`

- [ ] Merged
- Module: `ci-release`. Implements: 12 12.10,
  [ADR 0004, measure delivery with the five DORA metrics computed by a tool](adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md).
- Depends on: T065. Operator: no. Ask-first: `checks`, `release`.
- Acceptance, against a recorded API fixture and a fixture
  repository, offline: the same input gives the same bytes; a period
  with no deployment has `null` times and `n/a` in the text form; a
  rendered output that has a lead time or a deployment count without
  that period's failed and rework counts fails the unit test; a
  marker that names a tag with no deployment is listed under
  `unmatched`; `--attach` is the last step of `release.yml`.
- Verify: `go test ./tools/ci/... -run Dora`.

#### T067 - `tools/ci acceptance`

- [ ] Merged
- Module: `ci-release`. Implements: 10 10.5, 10 10.2 (the
  subcommands outside `all`).
- Depends on: T065. Operator: no. Ask-first: `checks`.
- Acceptance, against a recorded API fixture: each evidence kind is
  verified as 10 10.5 says; on the product's own file the four
  end-of-plan checks run (S6 tags per row of 05 5.2; a result per
  probe of the "Settled by" column, from both hosts when
  arch-sensitive; a guide page per journey; block B results that
  count for v1.0.0); with `--file` a `command` item is not run;
  `schemas/acceptance.v1.json` is generated.
- Verify: `go test ./tools/ci/... -run Acceptance`.

#### T068 - The v1 catalog table

- [ ] Merged
- Module: `catalog`. Implements: 03 3.5 (the closing paragraph),
  07 7.6, S4.
- Depends on: T042, O4 (the `validate --catalog` output). Operator:
  block O4. Ask-first: `catalog`.
- Acceptance: for the keys and hosts the output lists, the catalog
  gains entries, each domain with an explicit `upload` flag;
  `tools/ci catalog` passes; a second run of `julieta spec validate
  --catalog` against the reference config repo exits 0.
- Verify: `go run ./tools/ci catalog`.

### Phase 9 - Acceptance

#### T069 - Apply the block B results

- [ ] Merged
- Module: `probes`. Implements: 11 11.2, 11 11.4, index ("Open
  questions").
- Depends on: O5. Operator: no. Ask-first: `decisions`.
- Acceptance: `tools/ci probes` passes on the committed block B
  results. A result that needs a change outside `docs/` and the root
  Markdown files (a kit, a catalog entry) is fixed there, and blocks
  O4 and O5 run again on a new candidate (11 11.2). An overturned
  decision whose `affects` paths are under `docs/` is resolved here
  with its ADR.
- Verify: `go run ./tools/ci probes`.

#### T070 - The guide pages

- [ ] Merged
- Module: `docs`. Implements: 12 12.8 (`docs/guide/*`), 09, S10,
  ADR 0001 rules 1, 2 and 13.
- Depends on: T069. Operator: no. Ask-first: none (`docs/guide/` is
  on no surface).
- Acceptance: each `## J<n>` heading of 09 is named by the
  `journey:` of a page; each `romeu` or `julieta` command in an `sh`
  block equals an entry of `e2e/scenarios/commands.yaml`, which this
  task does not change; the J5 page describes the hook order C5
  recorded (Q22); each page gives the expected output and the
  recovery from each expected error id.
- Verify: `go run ./tools/ci docs`.

#### T071 - `README.md`, `ARCHITECTURE.md` and `CONTRIBUTING.md`

- [ ] Merged
- Module: `docs`. Implements: 12 12.8, S10, ADR 0001 rules 12, 18
  and 19.
- Depends on: T069. Operator: no. Ask-first: none expected.
- Acceptance: the three pages have the content of their outlines in
  12 12.8, under headings that `tools/ci/headings.yaml` already
  fixes; the review report of rule 18 scores each at 16 of 20 or
  more with no zero. **[review]**
- Verify: `go run ./tools/ci docs`.

#### T072 - The decision records of the specification

- [ ] Merged
- Module: `docs`. Implements: 12 12.5, S10, ADR 0001 rules 6 to 9.
- Depends on: T069. Operator: no. Ask-first: `decisions`.
- Acceptance: a record, written with `go run ./tools/new adr`,
  exists for each decision listed in the review rounds that 12 12.5
  names and for each open question settled by then;
  `tools/ci sequences` and `tools/ci generated` pass. That the
  records cover the decisions is **[review]** (S10).
- Verify: `go run ./tools/ci sequences`.

#### T073 - Acceptance

- [ ] Merged
- Module: `ci-release`. Implements: index ("Success criteria"),
  10 10.5, 10 10.2 (`links`, `acceptance`), 11 11.2.
- Depends on: T070, T071, T072, O6. Operator: block O6 comes first.
  Ask-first: `checks` (`docs/acceptance.json`).
- Acceptance:
  - `go run ./tools/ci links` ran once, and a dead link it finds is
    fixed or reopens the deferred row about a scheduled check.
  - `docs/acceptance.json` has one entry per success criterion, S1
    to S11, and the S1 and S2 items name the release and the run of
    the v1.0.0 commit.
  - The output of the B1 run on v1.0.0 is under the pull request's
    Evidence. **[review]**
- Verify: `go run ./tools/ci acceptance` exits 0.

## Traceability

Where each success criterion, journey and invariant is built. A task
listed here names the id in its own "Implements" or "Acceptance".

| Success criterion | Tasks | Evidence comes from |
|---|---|---|
| S1 | T065 | O6 (the release run) |
| S2 | T002 | O6 (the run on the release commit) |
| S3 | T060 | O5 (B2) |
| S4 | T068 | O6 (the config repo's run) |
| S5 | T061 | O5 (B2) |
| S6 | T007, T067 | T073 |
| S7 | T058 | CI, and O5 (B3) |
| S8 | T040, T053 | CI, and O5 (B5) |
| S9 | T014 | CI |
| S10 | T063, T070, T071, T072 | T073 |
| S11 | T001, T002 | CI |

| Journey | Tasks | Journey | Tasks |
|---|---|---|---|
| J1 | T033, T058 | J8 | T040, T058 |
| J2 | T034, T042 | J9 | T035, T042 |
| J3 | T053 | J10 | T053, T055 |
| J4 | T054 | J11 | T035 |
| J5 | T046 | J12 | T058 |
| J6 | T055 | J13 | T036, T055 |
| J7 | T036, T056 | | |

| Invariant | Tasks | Invariant | Tasks | Invariant | Tasks |
|---|---|---|---|---|---|
| I1 | T009, T053 | I12 | T015, T027, T034 | I23 | T037 |
| I2 | T009, T054, T058 | I13 | T030 | I24 | T027, T034, T057 |
| I3 | T017, T058 | I14 | T037, T058 | I25 | T021 |
| I4 | T021 | I15 | T031, T037, T058 | I26 | T038 |
| I5 | T017, T054 | I16 | T031, T058 | I27 | T018, T030, T034 |
| I6 | T017, T049, T055 | I17 | T026, T049, T055 | I28 | T024, T030, T034 |
| I7 | T026, T032, T034, T058 | I18 | T048 | I29 | T007, T008, T032, T057 |
| I8 | T015, T029, T034 | I19 | T054 | I30 | T022, T053 |
| I9 | T028, T034 | I20 | T033, T037 | I31 | T051 |
| I10 | T028 | I21 | T041 | I32 | T051, T057 |
| I11 | T015, T029 | I22 | T037 | I33 | T050, T052, T053 |

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
| which Markdown linter and spell checker `tools/ci docs` runs | T063 | the row's event is that pull request, which picks both |
| generating part of the spell-check word list from the vocabulary table | T063 | the list's format and path are fixed there; the generation stays deferred |
| a scheduled workflow that checks external links | T073 | a dead link in that run reopens it |
| tuning the ledger's starting values | T051 | the plan may tune them (13 13.8) and keeps the proposed defaults |
| how a local-gate run is recorded | each task | reopened the first time the maintainer chooses the local gates for a merge (10 10.2) |

## Decisions this plan defers

These are choices of the plan, not of the specification. Each waits
for the task that has the context to make it.

| Deferred | Until then | Reopened by |
|---|---|---|
| splitting this page into one file per phase | one page | the page passes 2 500 lines, or two open pull requests conflict on it twice |
| the label of the Intel macOS runner | none chosen | T002, which verifies that the label exists and reports `x86_64` (10 10.2) |
| the frontend milestone, the workload digest and the herdr version | none chosen | T011, which needs them as inputs of A11, A12 and A13 |
| whether agents push from a second account | the sandbox's fine-grained token (Q25) | the output of block O1, item d |
| the `apply` tag of `secrets` | recreate-class (Q18) | the A10 result, in T020 |
| which hook order the J5 guide describes | SessionEnd before SessionStart (Q22) | the C5 result, in T070 |
| whether the rehearsal of block O4 becomes a rule of the specification | a recommendation of this plan | the first candidate that block B fails for a reason the rehearsal would have shown |

## Questions for the maintainer

Places where the specification is silent or says two things, found
while planning. The plan states the reading it took, so work can
start; none of them is a decision of ours to keep.

| # | Section | What we found | Reading taken here |
|---|---|---|---|
| 1 | index, 02 2.1 | the plan's path, format and task shape are not stated, and the tree of 02 2.1 has no entry for it | one page, `docs/plan.md` |
| 2 | 12 12.2, 10 10.2 | the checks of 10 10.2 that `ci-bootstrap` does not list (`coverage`, `invariants`, `mutate`, `imports`, `vocabulary`, `schema`, `golden`, `catalog`, `mise`, `kits`) have no stated landing point except "the remaining checks" in `ci-release`, after the code they check | each lands with its first input |
| 3 | 12 12.2, 04 4.1, 10 10.6 | the error table is under `romeu-cli`, which depends on the modules above it, and those modules return error ids | `internal/cli` and `version` land early (T008, T009) |
| 4 | 12 12.2, 02 2.1 | the map names no owner for `e2e/fakesbx`, `e2e/scenarios`, `internal/shquote` and `examples/` | `probes`, `romeu-cli`, `render` and `spec` |
| 5 | 12 12.2, 12 12.3 | `tools/schemagen` is under `spec`, and `probes`, which comes first and depends on nothing, needs `probe-result.v1` | the generator lands in T010 with its first type |
| 6 | 11 (opening), 11 11.1 | block A "needs no product code", and A3 expects that "romeu/julieta checks see every plant" and that "ingest skips each spool plant" | not resolved here: block O3 runs A3 as the harness defines it, and the question is whether its product half moves to block B |
| 7 | 11 (opening), 10 10.2 | "the first plan task can pass `tools/ci all` before any host run" is said of the probe harness, and the first plan task is `ci-bootstrap` | the harness is the first task of its module |
| 8 | 06 6.1, 11 11.1 | A11, A12 and A13 need a pinned frontend, workload and herdr before `kits`, which owns `tools/kitpin`, exists | the three pins are constants of the harness in T011 |
| 9 | 10 10.1 | the rule that lists where tests may use the network omits pulling the workload's base image for the container and hybrid e2e | the image is pulled by digest; the rule needs the row |
| 10 | 12 12.8, 12 12.3, 11 11.4 | `ARCHITECTURE.md` shows four generated diagrams (candidate, promotion commit, generation, probe), and the generator's one source is the transition tables of `internal/state`, which hold two of them | T026 generates the two; the other two wait for an answer |
| 11 | 12 12.10, 02 2.1, 03 (opening) | `dora.v1` is a format with no schema in the list of 02 2.1, and 03 says each format has one | no schema is planned for it |
| 12 | 12 12.1, 07 7.3, 04 4.3 | `gh` is "pinned in `mise.lock`"; the sha256 per architecture is not stated for it, and no check compares a pinned development tool with its latest release | T065 carries both as a criterion that waits for the change |
| 13 | 12 12.7, 12 12.2 | `docs/lessons.md` and the `lesson` kind have no owning module, and each pull request has a Lessons section from T005 on | the file and the kind land in T006 |
| 14 | S4, 03 3.5, 02 2.2 | the v1 catalog is "built from the reference config repo's locks", and the product names no such repository in a way a task can read | block O4 hands over the output; T068 applies it |
