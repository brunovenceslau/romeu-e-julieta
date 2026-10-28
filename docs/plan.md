---
type: reference
reader: the implementer picking the next task of romeu e julieta v1
---

# Plan: romeu e julieta v1

This is the implementation plan of v1. Its source of truth is the
specification, [spec.md](spec.md) with the fourteen pages under
`spec/`, and the decision records under `adr/`. Where this plan and
the specification disagree, the specification is right and this page
has a defect.

This page is the narrative: how the work is ordered, what rules it
follows, what each milestone delivers, and what stays open. The tasks
live beside it, one file each, under [`plan/tasks/`](plan/tasks/); see
[Where the task checklist lives](#where-the-task-checklist-lives).

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

## How this plan was made

The plan was re-planned on 2026-10-12, at the end of stage A of the
specification's review, from the validated specification and from the
earlier plan of 103 tasks in ten phases. Six requirements were set for
the re-plan:

1. Daily use first: order the work so that romeu and julieta enter the
   maintainer's daily work as early as possible. The engines are best
   built by using them in real work, and each milestone ends in a
   measured fact of that use.
2. 80/20 first: the smallest set of tasks that delivers most of the real
   value comes first.
3. Maximize parallelism. Several worktrees conflict often today, so
   tasks and file ownership are shaped so that parallel worktrees rarely
   touch the same files: shared append-heavy files, generated files,
   indexes, golden files.
4. Anything already built may be changed or dropped, after weighing
   what was built and why.
5. The planning skill in use saves a plan document and a separate
   checklist file, the convention its build command expects. This
   repository instead keeps one `docs/plan.md` with the tasks beside
   it; the re-plan states where the task checklist lives and why.
6. Milestones M0 to M6, each a statement about `main` plus one sitting
   of the maintainer that produces a measured fact.

Three proposals were written against those requirements, one leading
with each of the first three, and compared on seven criteria: the first
five requirements, the concept count, consistency with the
specification, and risk. The result is a synthesis: the 80/20 proposal
as the base, with the parallel-first proposal's milestone M0, its
`gitsafe` split by verb, its one-file-per-family catalog, its `owns`,
`needs` and `after` fields with a deterministic check, and its
error-id-per-package rule; and the daily-use-first proposal's milestone
shape (each ends in one sitting that produces a measured fact), its
"kits in daily use before `sync`" order, its rule that the bridge stays
outside the product, and its thin `doctor` and preflight.

What was measured for it: the pull requests merged so far and which
files they conflicted on; the dependency graph of the earlier plan (49
of its edges were plan order, not data); the code and records already
built, each with a verdict (see [What was built, kept and changed](#what-was-built-kept-and-changed)); and the sentences of the
specification the new order touches, each read at the commit the
re-plan started from. What is inferred: the critical path and the width
of each milestone, counted by hand from the `needs` edges and not yet
measured by a tool; the duration of sitting M2; that the kit frontend
accepts a local directory source (sitting M2's first act tests it).

Every rule marked "proposed on 2026-10-12" below was taken with the
answers of that day, recorded under
[Decisions of the re-plan](#decisions-of-the-re-plan); no item of that
block is left open, and the items it defers name the sitting that
decides each.

## How to read this plan

**What the specification fixes about the plan.** Four things, and this
plan follows each:

- The plan selects work by the module ids of the capability map
  (12 12.2). Each task names one.
- The build order between modules follows 12 12.2 as amended on
  2026-10-12 (ADR 0012; T122).
- `ci-bootstrap` is the first task (10 10.2, "Release and bootstrap");
  its first three tasks (T001 to T003) are merged, and T004 to T006 are
  deferred with triggers (12 12.2, as amended on 2026-10-12).
- The last task commits `docs/acceptance.json` after the v1.0.0
  release, verifies it with `go run ./tools/ci acceptance`, and runs
  `go run ./tools/ci links` once (index, "Success criteria"; 10 10.2;
  10 10.5).

**Where the tasks are.** One file per task under `plan/tasks/`, named
`Tnnn-<slug>.md`. A task's file is its definition and its checkbox. A
kept task keeps its id, so every citation of an id in the
specification, the decision records and the review records still
resolves; a folded task keeps a stub file that names the id that
absorbed it; a deferred task keeps its file, its earlier text and its
trigger; a new task takes T104 onward.

**The shape of a task.** A task is one logical change and one pull
request; consecutive dependent tasks of one chain, written by one
agent, may ship as one pull request (12 12.9). Its front matter
carries:

| Field | Meaning |
|---|---|
| `id`, `title` | the task; the id never changes |
| `milestone` | M0 to M6, or `none` for a folded, removed or left-v1 id |
| `module` | the module id of 12 12.2; `none` for a task with no file in this repository |
| `fate` | `kept`, `new`, `deferred`, `folded`, `merged`, `left-v1` or `removed`; `absorbed_by` names the id a folded task went into |
| `owns` | the globs the task's pull request may edit, besides the task's own file and the generated outputs it regenerates (rule 1); empty for a deferred task until it reopens |
| `needs` | a data or interface dependency: the task starts when these are merged |
| `after` | a file overlap or a review order: the task also waits for these; the critical path counts both kinds, and `after` is kept apart so an overlap can be removed by moving a file |
| `ask_first` | the surfaces of 05 5.3 the diff is expected to touch; a forecast, so the approvals can be given at each sitting for the grantable surfaces of 12 12.4, while every other surface keeps its approval line on each pull request |
| `sitting` | `none`, or the sitting the task waits for or feeds: a milestone id (M0 to M5) names that milestone's sitting, an earlier block id (O1 to O8) names that block |
| `merged`, `pr` | the checkbox: the date and the pull request number, written by the task's own pull request; absent while the task is open |

The body carries the goal, a dated re-plan paragraph that says what
changed from the earlier text, and the fields of before: Implements
(the requirement ids of 12 12.11, each with its section as a reading
aid, in the two forms the spec cites itself with), Operator,
Acceptance (what a test or a command checks) and Verify (the command
that shows it).

**Done, for each task** is the definition of done of
[10 10.8](spec/10-testing-style.md#108-definition-of-done-for-implementing-agents),
linked and not restated; the "Verify" line of a task names what is
specific to it and does not repeat it. Two consequences of `all` that
the task lines do not repeat either: the race run of 10 10.2 carries
no `e2e` tag, so a task whose Verify line is `-tags e2e` alone also
adds the unit tests that keep its packages inside S9, and `coverage` is
what shows it; and `go test` exits 0 on a `-run` pattern that matches
no test, so a Verify line that names a pattern names test functions
the task adds, with each space of a case name written as `_`, the
reviewer reads the `-v` output for them, and a task whose named test
did not run is not done.

**Order.** The ids are not a queue and say nothing about order: a task
starts when every id in its `needs` and `after` is merged, whatever
its milestone. A milestone is a statement about `main` plus one
sitting, not a gate on starting. Lanes of different milestones run at
the same time; the milestone says when the value is collected and what
it delivers.

## How the work is done

Carried over unchanged from the earlier plan:

- **Language.** Go for the two binaries, the shared packages and the
  tools (12 12.1). POSIX sh in two places only, both bounded by the
  specification: the `.githooks/pre-push` of 12 12.1, and kit install
  steps of at most five lines (12 12.1). No task adds a script in
  another language.
- **Trunk.** Each pull request targets `main` from a short-lived
  branch, carries one logical change and merges as a merge commit
  (12 12.9;
  [ADR 0002, develop on the trunk with short-lived branches](adr/0002-develop-on-the-trunk-with-short-lived-branches.md)).
  v1 has no stack: a task that depends on an unmerged one waits for it,
  or ships with it in one pull request when both are consecutive tasks
  of one chain written by one agent.
- **Dark merges.** Work that is not finished merges on `main` and is
  reached by no command (12 12.9). A task marked **dark** adds code
  that a later task connects.
- **Test first.** The test comes before the code it tests (12 12.9;
  [ADR 0003, adopt six Extreme Programming practices and review as pairing](adr/0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md)).
- **One entry point for the gates.** `go run ./tools/ci all` is what CI
  runs and what a developer runs before a pull request; the pre-push
  hook runs `go run ./tools/ci fast` (10 10.2).
- **Review before the pull request opens.** A change is read by a
  reviewer, another agent or a person, before it reaches the
  maintainer (12 12.9, "The agent and the reviewer"). The maintainer's
  merge is the last step of each task, by working agreement; what the
  default-branch ruleset requires, and what that is worth, is in 05 5.4
  ([ADR 0009, drop the required review and amend the records that assumed it](adr/0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)).
- **Middleware lands with its first input.** A `tools/new` kind and a
  generator row land in the first pull request of the module that owns
  their first input (12 12.2, 12 12.3), and so does each `tools/ci`
  check that 10 10.2 lists and `ci-bootstrap` does not (plan question
  2, answered 2026-10-01, under
  [Questions for the maintainer](#questions-for-the-maintainer)).
- **Decide late, and write it down** (index, "Decide at the last
  responsible moment";
  [ADR 0005, decide at the last responsible moment and record the trigger](adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)).
  What this plan leaves undecided is in
  [Decisions this plan defers](#decisions-this-plan-defers), each with
  its trigger.
- **Documents.** Each page follows
  [ADR 0001, adopt a documentation standard with checkable rules and a voice](adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md).
- **Determinism seams.** A package that writes a time, an id or a
  version takes the clock, the ULID entropy source and the version
  stamp as injected values, and its tests fix all three (10 10.4).

New or changed, each proposed on 2026-10-12:

1. **A task owns a declared file set.** `owns` holds globs; the task's
   pull request edits those paths, its own task file and the generated
   outputs it regenerates, and nothing else. Defaults: a package task
   owns `internal/<pkg>/**` with its `testdata/`; a command task owns
   `internal/cmd/<binary>/<command>/**`, its own `errors.go` ids, the
   one-line import file `internal/cmd/<binary>/all/import_<command>.go`
   that links the package in (Go runs a package's `init` only when the
   package is imported), its scenario directory
   `e2e/scenarios/<journey>/**` and its e2e file:
   `e2e/<command>_test.go` for a romeu command (T047, T067, T068 and
   T069 list theirs), `e2e/julieta_<command>_test.go` for a julieta
   command, whose cases run in the container e2e of T013 (10 10.1); a
   kit task owns `e2e/kit_<name>_test.go` the same way. A merged
   check's package, and `.github/workflows/ci.yml`, are owned by the
   task that names them in `owns`, one at a time, ordered by `after`.
   After T108, a change to `docs/plan.md` is a re-plan pull request of
   its own. A generated output (a path a
   generator registered under rule 4 writes: the pages under
   `docs/reference/`, `schemas/*.json`, `.github/CODEOWNERS`,
   `docs/adr/README.md`, `docs/diagrams/`, the Go files
   `internal/cli/*_gen.go`, `internal/gate/*_gen.go` and
   `internal/gate/*_gen_test.go`) is owned by no task: each
   pull request regenerates it on its merged tree, which is 12 12.3's
   merge rule, and `tools/ci plan` leaves it out of the overlap check.
   Where two tasks share a directory (`render` in three file sets;
   `gitsafe` by verb; `sync` and its signing rule and live changes), the
   later one names its own files and comes `after` the earlier one; a
   task that adds a step to another task's journey owns that step's
   file (`e2e/scenarios/j2/step1_*.go` for T055) and comes `after` it.
2. **Two kinds of edge.** `needs` is a data or interface dependency;
   `after` is a file overlap or a review order. A task starts when both
   are merged; the critical path counts both kinds, since a task starts
   when both are merged, and `after` is kept apart so that an overlap
   can be removed by moving a file. A deferred task keeps true edges;
   `tools/ci plan` checks them like any other. The 49 "plan order"
   edges of the earlier plan are dropped (12 12.2, as amended on
   2026-10-12).
3. **Disjointness is checked.** `tools/ci plan` (T106) fails when two
   open tasks that no `needs` or `after` edge orders, directly or
   through other tasks, own an overlapping
   path (a path in the tree, or a literal path, matched by both), the
   generated outputs of rule 1 left out. Each pull request still
   measures `git diff --name-only` against every in-flight pull request
   before landing.
4. **Registration by file, no shared table.** A `tools/ci` check is
   `tools/ci/internal/<check>/` plus `tools/ci/check_<check>.go`, whose
   `init` registers its name, usage line, membership in `fast` and
   `all` and order key (the `generated` check's package is `drift`); a
   generator lives in the package that owns its data and is registered
   by `tools/ci/generator_<name>.go` (02 2.1); a `tools/new` kind is
   `tools/new/<kind>*.go` with the same `init` (T106 moves `adr` to
   that form and drops the switch); a command is its package under
   `internal/cmd/<binary>/<command>/` with an `init` that registers its
   definition, plus the import file of rule 1 that links it in. Adding
   one adds files and edits none.
5. **No task appends to a list another task appends to.** Every
   check, command, generator output, error id, scenario, catalog family,
   kit, probe and task lives in a file of its own. Error ids: each
   package declares its ids in its own `errors.go`, taken from the list
   of 04 4.4; one test collects the union and fails on a duplicate, an
   id not in 04 4.4, or an id of 04 4.4 that no package declares.
   Catalog entries: one file per tool family under
   `internal/catalog/egress/<family>.yaml`, beside the package that
   embeds them by glob. Scenarios: one directory per journey with its
   own `commands.yaml`.
6. **The specification states contracts; the detail lives with the
   code.** A code task edits no spec page, except where the
   specification assigns a task an edit of its own text (12 12.4 and
   05 5.3 for the task that builds `pr`; the generator rule of 03
   (opening), 01 1.4, 04 4.1 and 04 4.4 for the task that lands a
   generator): such a task names those pages in `owns`, and the
   requirement ids of a table it replaces by a link are retired with
   `Replaced by: <the generated page>`. A spec defect found while
   coding is a separate docs pull request. The inventory of the checks
   is a generated page, `docs/reference/checks.md`, one record per line
   (12 12.3's merge rule), which 10 10.2 links beside its step table
   (T105). The volatile environment rows of 10 10.2 and 12 12.1 stay
   where they are, with a trigger.
7. **Dependencies land with their first importer.** A module of 12
   12.1's list enters `go.mod` and `go.sum` in the task whose code first
   imports it, under that task's `dependencies` forecast, since `go mod
   tidy` drops a requirement that no package imports: the JSON Schema
   validator with T125, the TOML parser with T053, which T036 then comes
   after, and cobra with pflag and `cobra/doc`, which no binary links,
   with T008 (ADR 0015, decided on 2026-10-17), which T125 and T053
   come after.
   Those three tasks own `go.mod` and `go.sum`, ordered by `after`, and
   no other task edits them; `mise.toml` and `mise.lock` change only in a task that
   names them.
8. **Derived views are computed, not committed**, unless a consumer
   outside the repository reads them from the tree (`CODEOWNERS`). This
   page holds no task count, no "what waits for a sitting" list and no
   traceability table; `docs/adr/README.md` stays generated as today,
   and its removal is deferred with a trigger.
9. **The bridge is outside the product.** Running julieta in a
   sandbox that romeu does not render yet (M1 and M2) is four edits per
   project definition (three that supply julieta: the bin mount, the
   memory mount, and the manifest with the entry of the `julieta-claude`
   kit of T079, pinned at a SHA of this repository and its `julietaBin`
   arg set to the path of the julieta binary in the bin mount; and one that removes the store's
   writer kit and turns the mount of the store read-only until T115's
   recreate), one change of the sandbox
   agent payload that removes the inbox skill from every sandbox at
   once, and a host build in M1 (T111) and, in M2, a fifth edit: the
   other kit entries with their args, `signingKey` set to the public
   key, by hand until `romeu sync` writes them at sitting M3 (T115);
   all recorded in the config repository's runbook with each
   step's undo, never in a spec section or a product command. Each of
   those steps is an ask-first change of the config repository, under
   that repository's own rules, since it changes what enters every
   sandbox (the hooks, the kits and their args, the gate that guards
   every push); the task files that record them (T111, T112, T114, T115)
   say so, and their `ask_first` field stays empty because it lists the
   surfaces of this repository only. The manifest is the product's type
   (03 3.6 names the format, not the writer), so a hand-written one
   validates like a rendered one; [julieta by
   hand](notes/julieta-by-hand.md) records this reading.
10. **Dogfood before recording.** The fake `sbx` replays sessions
    written by hand from the pinned sbx's help text and from what daily
    use showed, each marked `synthetic: true`; the maintainer's daily
    runs on the host are the source of truth for behaviour until the
    probe harness records block A (M6), and `tools/ci probes` then
    fails on any synthetic session left. The recorder (T018) lands
    with the harness, not earlier: a recording of the maintainer's host
    committed to a public repository before the redaction package has
    its unit tests is an irreversible-harm risk, and hand-written
    sessions hold no host data by construction.
11. **Sittings are short and per project.** The two 75 to 90 minute
    two-host recordings of the earlier plan belong to M6; sittings M1 to
    M5 are the maintainer's own use of the product, minutes per project, with
    their output pasted as Evidence of the next pull request (12 12.2's
    smoke steps, applied per project).
12. **Runtime isolation.** Each worktree runs `tools/ci` with its own
    caches where a lock is shared and never uses `git stash`; mutation
    checks use `tools/mutants`.
13. **Grants per milestone.** `docs/grants.yaml` (12 12.4) carries
    one grant per milestone for the grantable surfaces of 12 12.4 that
    its tasks forecast (`contracts`, `checks`, `dependencies`,
    `catalog`, `ledger`, `digests` and `termsafe`); every other surface
    keeps its approval line on each pull request. T107 creates the file,
    and each grant lands in a one-line pull request of its own on the
    `ask-first` surface: at each sitting, one grant for every task that
    can become ready before the next sitting, read from `tools/ci plan
    --ready`, on the grantable surfaces it forecasts, so a task on those
    surfaces waits for no sitting of its own. `tools/ci pr` reads the
    file only when that check lands (M6): until then the grant is a
    recorded intent, the approval line on each pull request stays the
    record, and that window is a row of [Accepted
    risks](#accepted-risks).
14. **Extension seams.** An order-sensitive step list (the setup steps
    of 04 4.3, the preflight steps of 01 1.5, the sync steps of 04 4.2,
    the promotion steps of 01 1.6, a journey's steps) is one explicit
    slice in one file of the owning package, which the extending task
    names in `owns` and edits by one line, with an `after` edge; an
    unordered set (doctor rows, hook actions keyed by hook name, status
    report sections, sbxdrv environment contributions, render-time rules
    of `sync`) uses the registration by file of rule 4: a package-level
    `var _ = register(...)` in the row's own file, one registry per
    package written during initialization and read-only after. The
    owning task names its seam file in its Acceptance.

## Milestones and sittings

A milestone is a statement about `main` plus one sitting of the
maintainer. The pass that placed the requirement ids opens M0 (T121,
T122); T104, T105 and T137 start beside it, T106 to T108 follow them,
T138 comes after T105, and T113 runs at sitting M0; M1's foundation
packages T007, T012 and T017 start beside them, since they need nothing
of M0, and T023 once T137 (`internal/procgroup`) has merged. M3 has two
lanes. Lane A, the packages that read no sbx
fact (`spec`, `catalog`, `state`, `egress`, the host-only verbs of
`gitsafe`, `sbxdrv` with its hand-written sessions, the preflight),
starts once T125 has landed (the schema generator and check that `spec`,
lane A's root, needs), in parallel with the rest of M1 and with M2. Lane
B (`oci`, and through it `render`, `gate`, `sync` and `run`) waits for
the fixtures of sitting M2 (T113) and for the `julieta` kit (T077), so
it runs behind M2's sitting. The critical path and the width of each
milestone are read from the task files, not written here (a deferred row
covers a command for them).

| M | Name | Ends when (observable, on `main` and on the host) | Delivers |
|---|---|---|---|
| M0 | Shape the work | the kit-frontend go/no-go read (T113); `tools/ci all` green with checks registered by file; the requirement ids in the specification and `tools/ci reqs` green; the plan as task files, checked by `tools/ci plan`; one source for the ask-first list; the re-plan record Accepted; `internal/procgroup` and the hook settings merged (T137, T138) | the shape that lets M1 to M4 run in parallel |
| M1 | julieta in daily use | sitting M1: every daily sandbox runs `julieta setup` once after it is created, `julieta memory`, `handoff show --hook` at SessionStart, `hooks run` on every commit and `snapshot` after it, from a read-only bin mount and a hand-written manifest; the names gate of the config repository runs through the dispatcher on every push (T112) | memory, handoff, hooks and snapshots in every sandbox the maintainer works in; julieta as the one path for new items; the store's open items entered into `julieta memory`, counted on both sides (T111) |
| M2 | Kits in daily use | sitting M2: recreate wave 1, one project per slot, each sandbox built from the `julieta`, `julieta-claude` and `git-ssh-sign` product kits plus one personal kit from the config repository | every daily sandbox built from the product kits plus one personal kit; mise per repository through `julieta install` and `lock`; commits signed with the one key of `git-ssh-sign` |
| M3 | romeu renders and runs | sitting M3: J1 for real on the config project from a `--dry-run` build of `main` (`init`, `doctor`, `approve --toolchain`, `sync`, `run`), then one project per slot until every project definition is rendered by `romeu sync` | every project definition rendered under gate 2, its egress derived and its secrets bound in host settings, the kit pins in one file, and `romeu run` as the one way into a sandbox |
| M4 | Nothing lost | sitting M4: one `romeu recreate` rehearsal with a planted dirty worktree, stash and ignored file; J6 and J10 pass in CI with the fake sbx; on the host, the rehearsal's `romeu recreate` transcript and salvage listing, read by the maintainer, are the Evidence | recreate, rm and retire with a complete salvage; `romeu pull` and `romeu memory` |
| M5 | Every project on romeu | sitting M5: one month of daily use after the last slot of sitting M3 with no hand-applied step and no open runbook section; every open item of a project is still a `julieta memory` entry or closed, and the store holds no item that the open-items list of the cut-over of sitting M1 does not name (T111, read again by T119). M5 is a wait, not a gate: the release and docs tasks of M6 whose trigger fired at the last slot of sitting M3 run during it, and blocks O3 to O7 wait for the end of M5 only where they need a sitting | the product as the one path, with memory as the one place for open items |
| M6 | Public v1 | the release checklist of 12 12.2 completes: blocks A and B on both hosts, `v1.0.0`, `docs/acceptance.json` | the v1.0.0 release with its evidence |

**The sittings.** A sitting is named by its milestone: sitting Mn ends
Mn and opens Mn+1, the grants it writes are Mn+1's (rule 13), and the
answers of [Decisions of the re-plan](#decisions-of-the-re-plan) come
with the re-plan pull request (T108), before sitting M0, except those
it defers to a named sitting; the form `S<n>` is a success criterion id (10 10.2)
and names no sitting. Each sitting is the maintainer's own use of the
product on one host, and its output is the Evidence of the next pull
request. Each sitting builds from a commit whose diff since the last
host build the maintainer has read, named with its SHA in the sitting's
Evidence.

- **Sitting M0** merges M0 and writes the M0 and M1 grants of rule 13;
  it also reads whether the approval-line check (a deferred row) lands
  with the first grant of a milestone with parallel lanes, which is
  that row's trigger; and it runs the kit-frontend go/no-go (T113),
  once T012 merged (about 40 minutes on one host; one approval).
- **Sitting M1** (ends M1): the bridge (T111) on two projects first,
  then every project; the open-items cut-over, the one-off entry of
  the store's items per project (T111); one week of daily use; the
  entry run again at the end of the sitting and the names gate
  verified from a sandbox again (T111). Output: `julieta status` and
  `handoff show --hook`, read in each sandbox, and the entry's counts
  in the runbook (4 definition edits per project and the entry run
  twice, counted from T111, plus one host build and one payload
  change; one approval per wave and one for the payload change).
- **Sitting M2** (ends M2): recreate wave 1 (T115), after T114; the
  kit-frontend go/no-go (T113) ran at sitting M0, once T012 merged; on
  a no-go, M2 stops with a decision record that names the option taken
  and the tasks it releases, while M3's lane A continues (5 steps and
  one definition edit per project, counted from T115; one approval per
  wave).
- **Sitting M3** (ends M3): T116, one project per slot, the config
  project first and this repository second (8 steps per project,
  counted from T116; one approval per sitting, the runbook pull
  request).
- **Sitting M4** (ends M4): T117, the recreate rehearsal (one project:
  the three planted cases, `romeu recreate` and the read of its
  listing, counted from T117; one approval).
- **Sitting M5** (ends M5): T119, the closing read (per project: the
  read that the store holds no item that the open-items list of
  sitting M1 does not name, counted from T119; one approval).
- **M6 has two sittings**, the earlier blocks O3 and O5 to O7, under
  [Operator blocks](#operator-blocks).

## Where the task checklist lives

One file per task, `plan/tasks/Tnnn-<slug>.md`, with the front matter
of [How to read this plan](#how-to-read-this-plan). `merged` and `pr`
are the checkbox: the task's own pull request writes them, which is the
one write a task pull request makes to the plan. The directory is the
checklist: `grep -L '^merged:' docs/plan/tasks/*.md` is the open list;
`tools/ci plan` (T106) validates the fields, the id sequence, the edges
and the overlap rule, and fails on `merged` without `pr` or the
reverse. `tools/new task` scaffolds a file with the next free id (the
rebase-and-rerun rule of 12 12.3 covers a collision). A task with no
file in this repository is ticked by the pull request whose Evidence
carries its sitting's output; that pull request names the sitting task
in its body and writes `merged:` and `pr:` into the sitting task's
file, the one write rule 1 allows outside its own file.

This page keeps what changes at re-plan time only: how to read, the
rules above, the milestones and sittings, the deferred decisions, the
accepted risks, and the decision block of the re-plan. It holds no
task body, no count and no traceability table.

**Why not a plan document plus a separate checklist file**, the
convention of the planning skill in use: a second checklist file is a
second source of the same list, the reason the answer of 2026-10-01
(plan question 1) gave against a tick file; and a single append-heavy
file is the measured conflict class: the earlier `docs/plan.md` was
edited by 11 of the 37 merge commits reachable from eedda13, the
re-plan's base (`git log --merges --oneline eedda13 | wc -l` gives the
37, and
`git log --first-parent --merges --oneline eedda13 -- docs/plan.md | wc -l`
the 11), and one pull request existed only to tick a task. A directory
of task files is the checklist with one writer per item. A build brief
names the task file in place of a checklist file; the skill expects a
path and gets one.

**Why not derive "done" from merged pull requests, with no tick at
all:** it needs a tool and a GitHub read before it is usable, while the
tick in the task's own file already has one writer; deriving it is
deferred with a trigger.

**Why not keep the inline ticks:** the 2026-10-01 answer holds against
a separate checklist file and does not hold against one file per task,
where the task and its tick stay together; the answer is reopened, not
contradicted, and item 2 of Decisions of the re-plan records it. The earlier
plan's own deferral row, "splitting this page into one file per phase
... the page passes 3 000 lines, or two open pull requests conflict on
it twice", had fired: the page stood at 2 984 lines with recorded
conflicts.

## Operator blocks

`sbx` runs only on the host (11, opening), so a step that needs it is
the maintainer's. So are the two `v*` tags, which the maintainer pushes
by working agreement, since the tag ruleset does not bind the operator's
token, which the sandbox holds until the note on the maintainer block
records its first narrowed entry (05 5.4; 12 12.2), and whether the tag
ruleset binds the narrowed token is measured only by an entry made
before `release.yml` lands; so are the denylist entries, which an agent
must not hold (10 10.2, "Forbidden names"), and anything that reads the
reference config repo.

Blocks O1 and O2 are done. Block O8 stands as written and is off the
critical path. Blocks O3 to O7 belong to M6: they are the two-host
recordings and the release steps, and sittings M1 to M5 of
[Milestones and sittings](#milestones-and-sittings) come before them.
Each block says what it needs, what it unblocks, what wait it puts on
the critical path, and what pass means: one command whose exit status
the maintainer reads, wherever one exists. Block A of 11 is sitting O3
here, and block B is sitting O6.

| Block | Needs merged first | What the maintainer does | Also needs | Unblocks | Wait on the critical path | Pass | Recommendation |
|---|---|---|---|---|---|---|---|
| O1 | T001, written and open: the block finishes it and merges it | the sitting of the maintainer block, items a to d (10 10.2): rulesets and the two repository settings first; one `go run ./tools/ci hygiene add` per forbidden name in a plain host clone; enable the hook, push the branch, open the pull request; then the four tries from inside the sandbox, read on the host (a measurement: 10 10.2, item d, which records the result) | a host clone, the repository's admin rights, the operator's token, which the sandbox holds, created after item a. | T002 directly; every later task has it in its dependencies | done | `go run ./tools/ci fast` exits 0 in that clone after item b and non-zero before it; the four tries are read by the host checks of 10 10.2, item d, which no one command joins | done; its output is recorded in [ADR 0008, let the sandbox act as the maintainer on GitHub](adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md), which decides Q25. From then on the maintainer also reads the GitHub security log at each sitting, for an action by the operator's token that was not asked for; a judgment, not a detector: the log records ruleset and listed settings changes, and no merge, push, tag, branch deletion or release (05 5.4) |
| O2 | T002 | item e (10 10.2): save the API's answer that lists the required checks (the rule exists, as read on 2026-10-09) | the green run's job names | T003 directly; every task after T002 has it in its dependencies | done | the rule exists (read 2026-10-09); saving the API's answer under Evidence is to do, at sitting M0 (measured on 2026-10-12: no merged pull request body holds it); no command reads a ruleset back (05 5.4), so this one is **[review]** | the wait is done; the saved answer lands at sitting M0 |
| O3 | T019, T020 | block A on both hosts, Intel and Apple silicon (11 11.1): `go run ./e2e/probes --block A --out docs/probes/`, which lists and plants nothing (Q29), apart from one global setting: A2 sets `env.rememberHostCommands` (11 11.1) and records the prior value in its result; then commit the results and the recordings on a branch in a plain host clone, with the hook enabled, run `go run ./tools/ci fast`, read the diff of everything staged, push and open the pull request | `sbx` at or above 0.49.2 on both hosts; the pins of the pin file | T026 directly, and every task that reads a block A fact | about 75 minutes per host (11 11.1), plus the wait for a sitting with both machines; in M6, off the path of M1 to M5 | `go run ./tools/ci probes --require-pass A1,A2,A3,A4,A5,A6,A9,A10,A11,A12,A13,A14,A15,A16,A17` exits 0 on the branch with both hosts' results | run both hosts in one sitting: the arch-sensitive probes (A4, A5, A11, A12, A13, A17) count only with both (11, opening). Read the diff of everything staged before the push: it is data from your host, and the recorder's scrubbing (T018) is a filter, not a reviewer. The second host's files reach the branch by a push from that host's own clone, through its hook, and the harness refuses an `--out` inside a clone whose hook is not enabled. A results-only branch runs `main`'s own `tools/ci` through the hook; if a resolution commit joins it (T026), read that diff in the clone before checking the branch out, as item c of block O1 requires (10 10.2) |
| O4 | T055 | the catalog handover: build julieta for linux from `main`, run `julieta spec validate --catalog projects/*.yaml` on the reference config repo inside a Linux container, and hand over its output after reading and redacting it: it may name hosts, tool keys or project names that are not for the public repository | a Linux container with read-only access to the config repo's origins and no host credential mounted; julieta has no darwin build (12 12.1) | the acceptance of T102, into which T091 folded | minutes; from M3 on, `julieta spec validate` runs in each project's own sandbox at sitting M3, so the handover is the release candidate's confirmation | `julieta spec validate --catalog projects/*.yaml` exits 0 or 1, and its output is the handover; an exit of 2 means a spec does not validate, and the block is not done; `go run ./tools/ci hygiene --file <output>` exits 0 on the output before it is handed over | do it with O5 |
| O5 | every task of M0 to M5, and T082, T084 to T088, T090, T094, T095 | the candidate: run `julieta spec validate --catalog` again, in the container of O4, and see it exit 0; the rehearsal on one host (below); then step 1 of the release checklist of 12 12.2, the tag `v1.0.0-rc.1` (11 11.2) | the same container as O4; one host with `sbx` | O6 directly, and through it T092 to T095 and T102 | the rehearsal is about one block B run on one host, 90 minutes plus the sandbox-side checks (11 11.2). Meanwhile T096 to T101 can proceed | `go test -tags host -json ./e2e/host/...` exits 0, then `go run ./tools/ci probes --require-pass B2,B3,B4,B5,C2,C3,C4,C5,C6 --hosts 1 --dir <the rehearsal directory>` exits 0 | the rehearsal is this plan's addition, not the specification's. After the tag, a change outside `docs/` and the root Markdown files needs a new candidate and block B again (11 11.2) |
| O6 | O5 | step 2 of the release checklist of 12 12.2, block B on both hosts (11 11.2): B1 to B6 and the sandbox-side checks C2 to C6; then commit the results on a branch in a plain host clone, with the hook enabled, run `go run ./tools/ci fast`, read the diff of everything staged, push and open the pull request | the candidate's published release; both hosts | T092 directly, and through it T093 to T095 and T102 | about 90 minutes per host for B1 to B6, plus the sandbox-side checks (11 11.2). Meanwhile T096 to T101 can proceed | `go run ./tools/ci probes --require-pass B1,B2,B3,B4,B5,B6,C2,C3,C4,C5,C6` exits 0 on the branch with both hosts' results | run B1 first on each host: each other result copies the candidate's tag and commit from it (11 11.2). The reading rules of block O3 apply: the staged diff before the push, each host pushing from its own clone through its hook, and a resolution commit's diff before checkout |
| O7 | T094, T095, T101, T093 | steps 3 to 5 of the release checklist of 12 12.2: `go run ./tools/ci acceptance --pre-tag` exits 0; push the tag `v1.0.0`; run B1 and B2 once more on v1.0.0, on one host, with `--out` outside the repository: `go run ./e2e/probes --block B --only B1 --out <dir>`, then the same with `--only B2` (11 11.2). Then pin `validate.yml` in the reference config repo to that release, run it and note its run id (S4) | the documentation tasks merged, each of them; the config repo | T102 directly; no other task | one B1 and one B2 run and one workflow run. No task can proceed meanwhile | `go run ./tools/ci probes --require-pass B1,B2 --hosts 1 --dir <dir>` exits 0 on that run | the pre-tag check is a command here, and `tools/ci acceptance` runs it again at the end (10 10.5); a check that fails after the tag is fixed by a patch release (12 12.2). The release exists only after the tag, so the pin comes after it. Paste under the Evidence of T102 the result files of those runs, which the harness wrote through its redaction, and no raw output |
| O8 | T002, T126 | once T126 has merged, which accepts [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md) after the App checks its Consequences lists: the token narrowing and the required-name try, items g and h of the maintainer block (10 10.2): install the GitHub App of decision 2 of that record on selected repositories, this one among them, set the global dynamic secret with `sbx secret set github --command <absolute host helper path> --refresh 55m`, with the helper and the App's private key outside every path mounted into any sandbox and not writable from any sandbox, stop forwarding the GitHub authentication key, then write the dated entry of the note on the maintainer block and run the required-name try | the host, the host helper that mints the App's installation token, two throwaway pull requests (read 3 of the entry and item h) | the Deferred decisions rows whose trigger is "the token is narrowed" | none: no task waits on it, and every task proceeds around it; it is due before M6 | the first entry whose verdict is narrowed, read against the expected answers the note lists, and that entry's fourth part, the answers of the required-name try; and the read, from inside a sandbox, that finds neither the host helper nor the App's private key readable: an `ls` of the helper path and of the key path from inside a sandbox, each refused; no command reads them, so it is **[review]** | run it at the first sitting after T126 has merged. To stop the forwarding, forward a socket that holds only the signing key, as [06 6.4](spec/06-kits.md#64-product-kits) requires of romeu's sandboxes, rather than the host's own agent; read 2 of the entry shows whether it holds |

**The rehearsal in O5.** Build with
`go run ./tools/release build --version v1.0.0-rc.1 --dry-run`, then,
on one host against that build, run
`go test -tags host -json ./e2e/host/...` and
`go run ./e2e/probes --block B --version v1.0.0-rc.1 --out <a directory outside the repository>`
which leaves B1 out, as it needs a published release (T082). It
includes C3, whose result can change a kit (Q8). It is not a block B
result and is committed nowhere. Pass is one command on that directory,
`go run ./tools/ci probes --require-pass B2,B3,B4,B5,C2,C3,C4,C5,C6 --hosts 1 --dir <it>`
(T020), after the host suite exited 0; a result that overturns a
default is handled before the tag, the way T092 says. It needs the
release candidate, a published `v1.0.0-rc.<n>` tag that T088's
`release.yml` builds with the full kit set, so it stays here.

**Results are a pull request.** Blocks O3 and O6 end with a branch, the
hook, `go run ./tools/ci fast`, the staged diff read by the maintainer,
and a pull request, not with a push to `main`. Two reasons. The
recordings and results hold output of the maintainer's real host: the
harness keeps real values only for its own sandboxes, gives every
other row typed placeholders and scrubs the host's own identifiers
(T018), and the maintainer's read of the staged diff is the one review
of what the filter let through. And this repository has names it must
not contain (index, Boundaries): a direct push is public before any
check reads it, while a branch is read by the hook, by `hygiene` and by
`pr` first (10 10.2).

**Outside the blocks.** Each merge waits for the maintainer: a green
pull request with its approval lines is merged at the next sitting, with
at least one merge each business day, and a sitting with more than
three pull requests waiting reopens the deferred decision on that manual
step (12 12.9). A pull request that touches an ask-first surface needs
the maintainer's approval, recorded in an approval line that names what
was approved (12 12.4). The `ask_first` field of each task is there so
those approvals can be given at each sitting, for every task that can
become ready before the next one, on the grantable surfaces of 12 12.4;
every other surface keeps its per-PR line.

## Toolchain pins

Each pin is a reviewed diff. The table says where each lives and which
task lands it.

| Pin | Form | Section | Task |
|---|---|---|---|
| Go | version in `mise.toml`, locked in `mise.lock` | 12 12.1, 02 2.1 | T001 |
| `golangci-lint`, `govulncheck` | `mise.lock`; `govulncheck` and `golangci-lint` run from the paths `mise which` resolves, at the version `mise.lock` locks; the `govulncheck` lock entry has no checksum (it rests on the Go checksum database) | 12 12.1, 10 10.2 | T001, T002 |
| `reuse` | the container image `fsfe/reuse:6.2.0@sha256:<digest>` in `tools/ci`, run on Linux only | 12 12.1, 10 10.2 | T002 |
| `gh`, for `tools/release verify` | an exact version in `mise.toml`, with a sha256 per platform in `mise.lock`; started by the path `mise which` resolves, never through `mise exec` | 12 12.1, 10 10.2 | PR #27 (round 8, DR7), merged |
| `gopls`, a dev tool for editors and agents | an exact version in `mise.toml`, locked in `mise.lock` with no checksum, like `govulncheck` (it rests on the Go checksum database); no step of `tools/ci` starts it | 12 12.1 | PR #44 |
| GitHub Actions | `uses` with a 40-hex commit SHA | 10 10.2 | T002, T088 |
| mise on the CI runners and on a developer machine | one `uses` step with a 40-hex commit SHA, the mise version and its sha256 in `with`; Go and the other tools come from `mise.lock` through it; `tools/ci setup` reads the same version and sha256 to install that mise where another one is found | 10 10.2, 12 12.1 | T002; T141 |
| Go dependencies | `go.mod`, `go.sum`; the list in use is in 12 12.1, and a new one follows ADR 0038 | 12 12.1 | the first importer of each (rule 7): T125 and T053 |
| kit frontend | `docker/sandbox-kit:3.0.0-m.<N>@sha256:<digest>`, in the pin file | 06 6.1 | T012 |
| workload | `<registry>/<repository>@sha256:<64 hex>`, in the pin file | 03 3.1, 06 6.2 | T012 |
| herdr | version and a sha256 per architecture, from an immutable release, in the pin file | 06 6.4 | T012 |
| mise | version and both linux sha256 values, in the pin file; the signed checksum file is verified at pin time against a committed public key the maintainer confirmed | 07 7.4 | T012 |

**One pin file.** The frontend, the workload, herdr and mise are read
by more than one consumer: the probes, the container e2e, `layout` and
the kits. They live in one file, written by `tools/kitpin`, so what
sitting M2 read by hand, and what block A later probes, is what the
kits use. A pin that changes after a fact was read needs that fact read
again: by hand until block A, by the probe after it.

**The rule for `gh` and every pinned tool** (12 12.1). Each tool pin is
an exact version with a sha256 per architecture, never "latest", and
a pin-freshness check reports each pin for which a newer version is
published. The tool rows above already have that form, except the three
whose form 12 12.1 keeps (`govulncheck`, `gopls` and `reuse`); 12 12.1 holds
the one list of tool pins. T088 builds the check, `tools/ci pins`: it
is the last task that adds a tool pin, so every pin exists when it
lands. The check reads the network and runs by hand; its scheduled
run is a
[deferred decision](spec.md#deferred-decisions).

## Changes to merged work outside the task list

Review round 8 left changes to merged work that go in tooling pull
requests of their own, each with its own approval line:

- the pre-push hook adds `GOWORK=off` and `GOFLAGS=-mod=readonly` to
  the `GOENV=off` and `GOTOOLCHAIN=local` it has set since PR #30, so
  that an untracked `go.work` or `vendor/` and a caller's `GOFLAGS`
  cannot act on `go run` (ADR 0007, Threat model; round 8, DR3, its
  form decided on 2026-10-09), on the `checks` surface; merged in PR
  #34;
- the sentence on outside contributors in the generated
  `docs/reference/ask-first.md` (R8-12-18), moot once T107 removes the
  page;
- the `sbxdrv` surface and `skills/**` on the `kits` surface of
  `.github/ask-first.yaml` (R8-05-3): now part of T107.

Three other changes round 8 left became tasks on 2026-10-12: the
release grammar row (T127), the never-tracked list (T128) and
`setup`'s module sources (T129). The hook's settings and comment are
T138, the `romeu:invariant` allowlist line is T105's, and the pending
items of the kept tooling are T136.

## What was built, kept and changed

Requirement 4 asks that what was built be weighed before it is changed.
The verdicts:

| Built | Verdict | Reason |
|---|---|---|
| `tools/ci` (`fast`, `all`, `setup`, `hygiene`, `workflows`, `generated`, `generate`, `coverage`), the hook, `ci.yml`, the pins, the licensing | kept; reshaped by T105 with its behaviour frozen by a one-off comparison of the step lists before and after the move, under the pull request's Evidence; the hardening is frozen, and its pending items after the round caps are T136, deferred | the most hardened code in the repository; its four hand-wired registration points are the top code conflict source (10 merge commits on `fast.go` at eedda13, the re-plan's base, by `git log --first-parent --merges --oneline eedda13 -- tools/ci/fast.go \| wc -l`) |
| `tools/mutants` | kept as the tool, named in 12 12.1 and 02 2.1 (T122); no gate consumes it until `tools/ci mutate` runs it in M6 (item 13 of Decisions of the re-plan) | it removed a measured hazard; a gate needs an invariant with code |
| `tools/ci/procgroup` | kept; moves to `internal/procgroup` in T137 (M0) and serves both binaries | one process layer for the tools and the product |
| `tools/new adr` | kept; registered by file from T106 (rule 4); gains `task` (T106), `command` and `error` (T009), `kit` (T076); `invariant`, `lesson` and `probe` in M6 | each kind with its first input |
| `.github/ask-first.yaml` | the file kept; one source (T107); two narrowings decided on 2026-10-17 and applied by T107 (items 8 and 9 of Decisions of the re-plan) | one surface change touches four files today |
| `docs/reference/ask-first.md` (generated) | dropped (T107) | the YAML with its reasons is the reference |
| `docs/adr/README.md` (generated) | kept; its removal deferred | 12 12.3's merge rule covers the textual conflict |
| the four-runner matrix | kept on `main` and pull requests | a deferred row holds its trigger |
| ADR 0001 | kept as review rules; its checks in M6 | |
| ADR 0002 (no stacks, dark merges), 0003, 0005, 0007 | kept; 0005 used harder, its rule 1 superseded in part by ADR 0014 (T108) for the merge-gate parts 05 5.4 defers by name | they make an 80/20, parallel plan legal |
| ADR 0004, 0006 | kept deferred | |
| ADR 0008 to 0011 | kept; O8 off the path | governance, not product |
| `docs/reviews/` | kept as history; no further spec review round as a phase | stage A is done |
| `docs/plan.md` as one page | changed (T104) | its own trigger fired |
| spec pages 10 and 12 | the step table of 10 10.2 stays; `docs/reference/checks.md` is the generated inventory (T105); the amendments were applied by T122; the environment rows stay with a trigger | the rows changed with the hardening, which is frozen |
| the Deferred decisions tables of `docs/spec.md` | kept as they are; one file per row deferred with a trigger, which now also covers the retired-requirements table of 12 12.11 | |

## The fate of each earlier task

The earlier plan's ids are kept, so a citation in a review record, a
decision record or the specification still names the same work; this
table is the one mapping note for them, and the review records stay as
written. Each id's file under `plan/tasks/` carries the same fate in
its front matter until the task merges; the file's fate then reads
`merged`, with the date and the pull request.

| Id | Fate | Reason |
|---|---|---|
| T001, T002, T003 | merged | done |
| T004 `sequences` | deferred to M6 | `tools/new adr` and `tools/ci plan` remove the main errors; trigger: the first duplicate id or one-way supersede link found in review |
| T005 `pr` | deferred to M6 | the approval-line check alone may land early by its trigger; the sections and trailers are friction before a product exists |
| T006 lessons, docs rules, contributor skeletons | deferred to M6 | checks before the documents exist; trigger: the first outside contributor or the release candidate |
| T007 | kept, narrowed | the invariant tooling is T133; the guard tags are written now |
| T008 | kept, changed | ids per package, registration by file |
| T009 | kept | both binaries, the cross-build as a package |
| T010 | kept, pulled to M1 | its first input exists at T009 |
| T011 `vocabulary` | deferred to M6 | polish; trigger: the release candidate |
| T012 | kept; T052 folded in | one tool, one pull request |
| T013 | kept, M1 | the specification's first julieta level (10 10.1); no new level is added |
| T014, T018, T019, T020 | deferred to M6 | the harness, the recorder, the argv table, `probes`; trigger: a real-sbx behaviour the fake cannot model breaks a daily run, or the release candidate |
| T015 | folded into T029 | one seam, one owner |
| T016 | folded into T066 | the handshake is `run`'s compatibility check and hybrid test; no slice of its own |
| T030 | folded into T026 | its two rows wait for the A4 recording, which arrives with block A |
| T017 | kept | |
| T021 | kept; T022 folded in | one package, one agent |
| T023 | kept, split by verb | the runner for both modes, fetch, refs and bundle create now; unbundle, verify, fsck and the tree walk (T024) in M3 |
| T024 | kept, M3 | needed by salvage and materialization |
| T025 | kept, M2 | dark until T080 |
| T026, T027, T028 | deferred to M6 | block A results and records |
| T029 | kept, changed | hand-written sessions, no recorder |
| T031 | kept, changed | one file per family; T091 folded into T102's acceptance |
| T032, T033 (T034 folded in), T035 (T057 folded in), T036 | kept | |
| T037, T038, T039, T040 | kept | three file sets of one package |
| T041, T042, T043 (T045 folded in) | kept | gate 1 kept whole: ADR 0005 rule 1 |
| T044 | the I-level cases into T043 (I12, I24, I27, I28); the breadth deferred to M6 | the invariant now, the breadth when it has a consumer |
| T046, T049 | kept, narrowed | the rows J1 needs; the breadth deferred with a trigger |
| T047 | kept, M4 | |
| T048 | kept, narrowed | |
| T050 `adopt` | deferred to M6 | memory leaves the sandbox in M1, so a recreate loses nothing salvage cannot capture; trigger: a sandbox whose recreate would; or the first release candidate |
| T051 | kept, changed | a hand-written manifest validates like a rendered one |
| T052 | folded into T012 | |
| T053 | kept, M2 | |
| T054, T059, T060 | kept, M1 | the sandbox-side 80/20 |
| T055 | kept, M3 | |
| T056 `layout` | deferred to M6 | `run` attaches a single pane through the minimal `julieta layout up` of T066; trigger: the first project whose daily work needs two panes at start; or the first release candidate; the kit still ships herdr (06 6.4 as written) |
| T057 | folded into T035 | |
| T058 | left v1 (unchanged) | the product has no import command in v1 |
| T061, T062, T067, T068, T069, T070 | kept, M4 | after memory and snapshot exist |
| T063, T064, T065, T071, T072 | left v1 (unchanged) | ADR 0006's row |
| T073, T074 | kept, M4 | after the last command |
| T075 | deferred to M6 | the host suite and the remaining scenario functions; the CI journeys cover J2, J3b, J6, J7, J10, J11 |
| T076, T077, T081 | kept, M2 | kits in daily use before `sync` |
| T079 | kept, M1 (moved from M2 on 2026-10-23) | the bridge of sitting M1 carries the skills and hooks through it |
| T078 | kept, M1 | sitting M1 needs the skills |
| T080 | kept, M3 | with the signing kit already in use |
| T082 | deferred to M6 | release only |
| T083 | removed (unchanged) | |
| T084 to T088, T090 | deferred to M6 | release and docs; trigger: the first adopter other than the maintainer, or the maintainer's environments all on the new tools |
| T089 | left v1 (unchanged) | |
| T091 | folded into T102's acceptance | the catalog entries come from the locks in T031; the handover sitting folds into the release candidate |
| T092 to T102 | deferred to M6 | release only; one pull request per record |
| T103 | kept, M3 | a source build refuses `run` and `sync` |
| T104 to T108, T111 to T119, T121 to T138 | new (T133, T134 and T136 deferred) | M0's tasks; the sittings of M1 to M5 and the steps of the config repository they need, with a file here and no code; `README.md`; the tasks the stage B pass added on 2026-10-12 |
| T109 | removed | a dependency batch with no importer cannot pass `go mod tidy`; each dependency lands with its first importer, T125 or T053 (rule 7) |
| T110, T120 | removed | their work is outside this repository, and no task of this plan waits on it |

**The earlier checkpoints.** The earlier plan ended each of its phases
in a checkpoint, C0 to C8, each a statement about `main`. The
milestones take their place. A citation of C4 in a kept task's earlier
text reads as the first slot of sitting M3.

## Traceability

Each task file's Implements line cites the requirement ids (12 12.11)
it builds, each with its section as a reading aid, and its text names
the success criteria, journeys and invariants it covers, by id. The
traceability from a requirement, a criterion, a journey or an
invariant to its tasks is read from the task files, not kept as a
table here (rule 8): `grep -l REQ-0123 docs/plan/tasks/*.md` lists the
tasks that implement that requirement, `grep -l 'I24'
docs/plan/tasks/*.md` the tasks that name I24, and the same for `S8`
or `J6`. `tools/ci reqs` checks both directions: every live id has a
live or merged task, and every entry is a live id; `tools/ci plan`
(T106) reads the field's shape.

## Spec sentences the re-plan amends

Every amendment the re-plan listed here was applied to the specification
by T122 (ADR 0012, proposed; decision (c) of 2026-10-12); the
specification and this plan
read the same sentences, and T108 records the decisions. The readings
that stay true live where they are used: a hand-written manifest
validates like a rendered one (T111, T135); `run` attaches a single pane
while `layout` is absent, through the minimal `julieta layout up` of
T066, so I1's argv holds (T066); the `julieta` kit ships herdr (T077);
the catalog's v1 entries come from the reference locks read in a sandbox
(T102); `romeu adopt` stays in 04 4.2, deferred (T050); a sitting of 12
12.4 is M0 to M5 (rule 13); the definition of done is linked (How to
read this plan).

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

**Designed and not built**: the runtime ledger
([13](spec/13-runtime-ledger.md)), a row of Deferred decisions like the
publishing path for kits.

**Deferred decisions** (index, [Deferred decisions](spec.md#deferred-decisions)).
Each row has what holds until then and the event that reopens it;
the table is the one place they are written, and this page does not
copy them.
These rows meet a task of this plan:

| Row | Task | What happens there |
|---|---|---|
| which Markdown linter and spell checker `tools/ci docs` runs | T085 | the row's event is that pull request, which picks both |
| generating part of the spell-check word list from the vocabulary table | T085 | the list's format and path are fixed there; the generation stays deferred |
| a scheduled workflow that checks external links | T102 | a dead link in that run reopens it |
| a scheduled run of the pin-freshness check | T088 | the task builds the check and runs it once by hand |
| how a local-gate run is recorded | each task | reopened the first time the maintainer chooses the local gates for a merge (10 10.2) |
| what replaces the code-owner review | T005 | the task adds the first check that reads the approval line, which is the row's event |
| `release.yml` taking its own file from `main` | T088 | the pull request decides it and closes the row |
| a check that reads the rulesets again | T088 | its trigger |
| required status checks without the administrator bypass | T126 | the acceptance of ADR 0011 |
| the views of the plan as a command | T106 | `--critical-path` and `--ready` |

## Decisions this plan defers

These are choices of the plan, not of the specification. Each waits
for the event that has the context to make it. The rows of the
re-plan come first; the rows of the earlier plan that still stand
follow them. The earlier row "splitting this page into one file per
phase" fired and is gone, and so is the row of the timeout classes of
`tools/ci` and `tools/mutants`, whose trigger, T105 starting, fired on
2026-10-13; its decision is in T105's file.

| Deferred | Until then | Reopened by |
|---|---|---|
| the recorder and the probe harness (T014, T018 to T020, T026 to T028, T082, T092, T093) | hand-written sessions marked `synthetic: true`; the maintainer's daily runs | a real-sbx behaviour the fake cannot model breaks a daily run; or the first release candidate |
| the approval-line and grant part of `tools/ci pr`, run in the base-defined pr job of 12 12.4 (the first slice of T005) | the maintainer's eyes at merge; the grant file as a recorded intent | the first grant written for a milestone with parallel lanes (sitting M0 writes M1's, rule 13, and reads this row then); or the first merged pull request missing an approval line it needed |
| `tools/ci pr` sections, the Middleware and Lessons fields, the `Fixes-release:` trailer (T005) | the sections are read in review | the first merged pull request missing a section the review wanted; or before the first release-candidate tag |
| `sequences`, `lessons`, `docs`, `vocabulary` (T004, T006, T011, T085, T086) | `tools/new adr`, `tools/ci plan` and review | a duplicate id or one-way supersede link found in review; the first outside contributor |
| the `invariant` kind, `tools/ci invariants` and `mutate` (T133) | tags written by hand; `tools/mutants` by hand on a security invariant | the first invariant whose guard has code and whose test could be green by omission, found in review; or the release candidate |
| `layout` and the attach to it (T056) | `run` attaches a single pane; the kit ships herdr | the first project whose daily work needs two panes at start; or the first release candidate |
| `romeu adopt` (T050) | the preflight refuses a sandbox romeu has no generation for; the maintainer removes it by hand | the first host-state loss, or the first sandbox created outside romeu that must be kept; or the first release candidate |
| `doctor` and preflight breadth (T134) | the I-level rows and the rows J1 needs | the first incident a missing row would have caught; or the release candidate |
| the host suite and the remaining scenario functions (T075) | the CI journeys with the fake | the release candidate |
| release tooling, attestations, notices, `pins`, `acceptance`, `links`, `fuzz.yml`, guides, root docs, `SECURITY.md` (T084 to T088, T090, T094, T095, T102) | `go build` and the `--dry-run` build on the host | the first adopter other than the maintainer; or the maintainer's environments all on the new tools |
| the decision records of review rounds 2 to 6 and of the settled questions (T096 to T101) | the review files hold the decisions | the release candidate; or one record when its decision is first touched by code |
| generated dispatcher tables for the `tools/ci` steps and the commands, in place of registration by file (ADR 0014) | each step and command registers itself from its own file's `init` | the first check or command present in the tree that its registry does not list at run time |
| deriving `merged` from merged pull requests | the tick in the task file | the first task file whose `merged` disagrees with the merged pull requests |
| one file per deferred-decision row | the tables of `docs/spec.md` and the retired-requirements table of 12 12.11 | two open pull requests conflict on one of them twice |
| removing the generated ADR index; lessons by date-slug | the index as today, with 12 12.3's merge rule | two concurrent ADR pull requests conflict on it twice |
| moving the volatile rows of 10 10.2 and 12 12.1 into package docs; splitting `10-testing-style.md` by section | the pages thinned by T108 | two open pull requests conflict on either page after T108 |
| a check that a code pull request to a check touches no spec page | the rule in this page | the first such pull request found in review |
| a check that fails a diff changing the mise pin unless `miseFiles` of the mise package was re-measured in the same pull request | the sentence of 05 5.3 and review | the first mise pin bump merged without the re-measure named in its Evidence |
| the four-runner matrix on every pull request | kept | the median pull request wall time passes 20 minutes over one month, or the Actions quota runs out |
| a Go port of the names gate of the config repository | the script as it is | closes when T112 lands the sandbox gate of [ADR 0036, keep the forbidden names as a convenience list in the config repository](adr/0036-keep-the-forbidden-names-as-a-convenience-list-in-the-config-repository.md) |
| keeping the sandbox agent payload's own `handoff` skill in step with `skills/handoff` | not kept in step: in a bridged sandbox the `julieta-claude` kit, listed after the payload's kit, installs this repository's skills over the payload's (decided on 2026-10-23) | T114 merged: the personal kit replaces the payload, and the payload's `handoff` skill goes with it |
| the live secret path (`apply:"live"`, T047 timing) | recreate-class | the first secret rotation that costs an unwanted recreate |
| a public home for the public part of the personal kit | the whole kit lives in the config repository (T114) | sitting M5 |
| the frontend milestone, the workload digest and the herdr version | none chosen | T012, which writes the pin file |
| the `apply` tag of `secrets` | recreate-class (Q18) | the A10 result, in T026 |
| which hook order the J5 guide describes | SessionEnd before SessionStart (Q22) | the C5 result, in T094 |
| whether the rehearsal of block O5 becomes a rule of the specification, and whether it moves earlier | a recommendation of this plan, before the candidate tag | the first candidate that block B fails for a reason the rehearsal would have shown |
| one pull request per decision record, or one per task | one per task (T028, T096 to T101) | the first review that asks for a record on its own |

## Accepted risks

The plan's single accepted-risk list. Every row is accepted (item 22
of [Decisions of the re-plan](#decisions-of-the-re-plan)): the rows
that link a row of 05 5.4 were accepted on 2026-10-12 with the rows
they link, and the plan's own rows on 2026-10-13. Each row has the
window it covers, why it is accepted and what reopens it.

| Risk | Window | Why accepted | Reopened by |
|---|---|---|---|
| uncommitted work lost to a recreate before salvage exists | the wave of sitting M2 and the slots of sitting M3, until M4 | the window has no salvage, narrowed by `julieta snapshot` from M1 and by `/handoff --final` plus push per slot | the first uncommitted change lost to a recreate: T068 then moves ahead of any remaining M3 task |
| the bridge's hand-written manifest and mounts are the "remember" pattern | M1 to sitting M3 | one committed runbook per project in the config repository with the undo of each step; `julieta status` prints the manifest's repos, so a drift is read at sitting M1 | sitting M3 writes every manifest through `sync` |
| the signing rule (I34) is enforced by no code while the `git-ssh-sign` kit is in daily use before T080: accepted by name in 05 5.4, the row "I34 is enforced by no code" | M2 to T080 (M3) | as 05 5.4 says; the outcome of an unanswered confirmation is read at sitting M2 (T115) and named here with the away window it covers | T080 lands; or a sandbox whose `ssh-add -L` lists more than one key |
| a grant is read by no check until `tools/ci pr` lands: accepted by name in 05 5.4, the row "no check reads the approval line or a checkpoint grant" | the first grant (sitting M0) to T005 (M6) | as 05 5.4 says | T005 lands; or a merged pull request whose grant matched no approval line |
| the inbox skill's one-step prefix capture is gone (item 19 of [Decisions of the re-plan](#decisions-of-the-re-plan)) | from sitting M1 | the julieta skill's rule that anything worth keeping goes to `julieta memory` covers it | a capture missed in daily use |
| hand-written sbx sessions diverge from the real binary | until the harness (M6) | the maintainer runs the real binary daily from sitting M3; a divergence is a one-line session fix | the harness's trigger above |
| the facts of sitting M2 drive `oci`, `kits` and the signing kit by hand | M2, M3 | each fact is a fixture marked `source: sitting M2`, re-measured by block A in M6 with the version rule | a changed fact is a re-probe, as the pin file's rule says today |
| the security invariants' tooling (tag pairing, mutate, the `pr` rule that a pull request removes no last guard or test tag) is thinner than the earlier plan's until M6: accepted by name in 05 5.4, the row "the invariant tag pairing and `mutate` land in M6" | all | as 05 5.4 says; the I-level cases sit in the task that lands the code, never in a breadth task | a reviewer finds an untested guard before M6: a tag-pairing check is one small task |
| the forbidden-name check over PR titles and bodies runs from no check of this repository | M0 to T005's first slice | the hook checks commits and ref names; the agents' own pre-PR gate checks the text before opening | the first title or body found holding a denylisted name |

## Decisions of the re-plan

The block the re-plan opened on 2026-10-12, with its answers, each
taken as recommended unless the item says otherwise. Items 1 to 7, 12
to 15 and 17 to 19 were decided on 2026-10-12; items 10, 16 and 20 are
deferred decisions, each with the sitting that decides it and nothing
decided now; items 21 to 26 record the other decisions of that day.
Items 19 and 22 were amended on 2026-10-13, each as the stage-B review
recommended, and item 19's removal of the store's writer was decided
later that day, after the review of that amendment. Items 8, 9 and 11
were decided on 2026-10-17, for T107. No item is left open.
[ADR 0014, re-plan v1 for early daily use: task files, registration by file, build order, dogfood before recording](adr/0014-re-plan-v1-for-early-daily-use-task-files-registration-by-file-build-order-dogfood-before-recording.md)
records these answers again with the rules they decide (T108).

1. **Registration by file** in place of generated dispatcher tables
   for `tools/ci` steps and commands. Decided on 2026-10-12: yes, as
   decision (c) of that day had applied it in the specification (T122);
   T105 splits the packages. The fallback not taken: the specification
   as it was written, sorted generated tables with regenerate-on-merge.
2. **The plan's shape**: `docs/plan.md` plus `docs/plan/tasks/`, the
   checklist in the task files, `tools/ci plan` in `fast`. Decided on
   2026-10-12: yes, as applied in the specification by T122 under
   decision (c) of that day. It reopened plan question 1, answered
   2026-10-01, and answers it again with this shape.
3. **Build order amended**: julieta's daily commands and the kits
   before `sync`; block A as the last word, in M6; the smoke steps as
   per-project slots. Decided on 2026-10-12: yes, as applied in the
   specification by T122 under decision (c) of that day. The fallback
   not taken: block A as one two-host sitting before M3's lane B (about
   2.5 hours on the critical path).
4. **Hand-written sbx sessions marked `synthetic: true`** until the
   harness records them; no recorder before the harness. Decided on
   2026-10-12: yes, as applied in the specification by T122 under
   decision (c) of that day. The alternative not taken: a recorder seam
   in `sbxdrv` with the redaction package landed early, declined for
   the irreversible-harm reason of rule 10.
5. **The check inventory as a generated page** linked from 10 10.2,
   beside its step table, emitted by T105. Decided on 2026-10-12: yes,
   as applied in the specification by T122 under decision (c) of that
   day.
6. **The bridge is outside the product** (rule 9): recorded in the
   config repo's runbook, never in a spec section. Decided on
   2026-10-12: yes, as applied in the specification by T122 under
   decision (c) of that day.
7. **The order of M3 and M4**: render and run before salvage, with the
   accepted risk above. Decided on 2026-10-12: as proposed, applied in
   the specification by T122 under decision (c) of that day. The
   alternative not taken: M4 before the slots of sitting M3, which
   delays M3's value by M4's tasks.
8. **Narrow `checks`** to the runner and the security-relevant packages
   (`tools/ci/*.go`, `tools/ci/denylist.yaml`, `.githooks/**`,
   `.golangci.yml`,
   `tools/ci/internal/{hygiene,names,pr,generated,workflows,plan,askfirst,kits,catalog,callsites}/**`,
   `docs/acceptance.json`): the denylist and the kits, catalog and
   call-site packages stay because 05 5.3 names them as what the
   `checks` line guards; the other check packages are plain. That list
   is the proposal as written on 2026-10-12. Decided on
   2026-10-17: yes, applied by T107, with the list read after T105
   moved the checks below `tools/ci/internal/`: the runner and the
   moved security code stay on `checks`, with `tools/mutants/**`, and
   the steps whose result decides whether `all` passes (`coverage`,
   `license`, `vulnerabilities`, `race`, `unit`, `vet`, `lint`, and
   their shared fixtures `citest`) stay on it too, since a one-line
   edit there weakens a gate. Off it: `format`, `prose`, `reqs`,
   `adrdir` and `slug`. `.github/ask-first.yaml` holds the globs.
9. **Narrow `contracts`** to `internal/cli/exit.go` and
   `internal/spec/versions.go` once error ids are per package (T008).
   Decided on 2026-10-17: yes, in the form of T107's acceptance:
   `contracts` is `internal/*/errors.go`, `internal/cmd/*/*/errors.go`,
   `internal/spec/versions.go` and `internal/manifest/versions.go`.
10. **The grants** in `docs/grants.yaml`, one per milestone, written at
    the sitting that ends the milestone before (rule 13). A deferred
    decision: it is decided at sitting M0 (decided on 2026-10-13), and
    nothing is decided now. The agreed default, confirmed on
    2026-10-17: write the M0 and M1 grants at sitting M0.
11. **The surfaces**: add `sbxdrv`, put `skills/**` under `kits`
    (R8-05-3), replace `catalog/egress.yaml` by `internal/catalog/**`,
    leave `ledger` while its row is deferred. Decided on 2026-10-17:
    yes, applied by T107. `sbxdrv` (`internal/sbxdrv/**`,
    `e2e/fakesbx/**`) holds the I4 and I25 guards, so it also joins the
    surfaces of 12 12.4 that keep an approval line on each pull
    request.
12. **The merge cadence for a parallel plan.** Decided on 2026-10-12:
    (b), a merge sitting whenever three green pull requests wait, read
    from `tools/ci plan --ready`, until the token is narrowed. The
    options not taken: (a) two merge sittings a day while lanes run;
    (c) the reopened deferred decision now.
13. **One mutation mechanism.** Decided on 2026-10-12: `tools/ci
    mutate` runs `tools/mutants` over one committed entry per guard
    (T133). The alternative not taken: an ADR that records
    `tools/mutants` as interim.
14. **The PreToolUse guard.** Decided on 2026-10-12: keep, with
    `julieta hooks guard` defined in 04 4.3 and built by T079. The
    alternative not taken: a deferral row with the trigger of the first
    refusal a sitting shows missing.
15. **`tools/ci pins`.** Decided on 2026-10-12: keep, as answered on
    2026-10-08. The alternative not taken: a deferral.
16. **The lint set.** Decided on 2026-10-18, ahead of sitting M0:
    option A with the `tidy` rule, the default agreed on 2026-10-12,
    which T139 applies in the lint package T105 made. The set:
    `errorlint`, `usetesting`, `thelper`, `forbidigo` with the
    Boundaries patterns (`fmt.Print*`,
    `os.Stdout`, `os.Stderr`, `log.` and `text/template` outside an
    allow list that T139 proposes, for the operator to confirm in its
    pull request; 04 4.1 names no package set for them) and revive
    `package-comments`, each linter with its fixture, and a `tidy` rule
    of `generated` (`go mod tidy` changes neither `go.mod` nor
    `go.sum`). `exec.Command(` is not a forbidigo pattern: the call-site
    scan of `internal/procgroup` holds `os/exec` to that package (04
    4.1, Subprocesses), more strictly; `"sh", "-c"` is held by a Go
    test in the lint package, since forbidigo reads no string argument.
    Option B, not taken: A plus `noctx`, `contextcheck`,
    `modernize` and `gosec`, whose G204 flags every
    `exec.Command` with variable arguments in `tools/ci` (restructuring
    frozen code).
17. **The id-less error chain.** Decided on 2026-10-12: the `internal`
    row (`RJ-348`, exit 1, hint: open an issue with the details), as
    04 4.4 and 10 10.1 state it; T008 builds it. The alternative not
    taken: a panic.
18. **The names-gate mechanism of the config repository.** Decided on
    2026-10-12: (a), each guarded repository carries its hashed
    denylist and runs the hygiene check from its tracked pre-push hook,
    which needs the check runnable outside this repository (a `checks`
    change with its approval line); T112 builds it, and the deferred
    row "a Go port of the names gate of the config repository" closes
    when T112 lands. The alternative not taken: (b) the config
    repository as a secondary clone of every project (02 2.6), its
    plain list cloned into every sandbox.
    Amended on 2026-10-19: replaced by ADR 0036, one plain list in the
    config repository and one gate in the sandbox for every push to a
    public remote; T112 is re-planned on it.
19. **The open-items cut-over.** Decided on 2026-10-12: at T119 (M5).
    Amended on 2026-10-13: at T111 (M1), as the stage-B review
    recommended, because a second store left open until M5 would hold
    items the count never sees. The one-off, count-verified entry of
    the store's items runs at sitting M1, and again at its end, after
    the store's writer has left every sandbox (decided on 2026-10-13):
    a fourth edit of T111 removes the store's writer kit and the mount
    of the store from every definition, and one change of the sandbox
    agent payload removes the inbox skill. From sitting M1 julieta is
    the one path for new items. The inbox skill's message-prefix
    trigger is not carried over to julieta: the julieta skill's rule
    that anything worth keeping goes to `julieta memory` covers it, and
    the loss of the one-step prefix capture is accepted (decided on
    2026-10-13). The host build of the store's tool stays a writer of
    the store, outside every sandbox, so T119 reads at sitting M5 that
    the store holds no item that the open-items list of sitting M1 does
    not name. The alternative not taken: the cut-over at T119 (M5).
    Amended on 2026-10-25: the fourth edit turns the mount of the store
    read-only instead of removing it, and the entry command reads the
    store's item files, so the end-of-sitting entry runs in the daily
    sandbox after the writer has left; the mount leaves at the recreate
    of T115.
20. **The render types slice.** A deferred decision: it is decided at
    sitting M2, the sitting that opens M3, from the measured lane, and
    nothing is decided now. The options: a slice, with the next free
    task id, `render: the candidate and render.json types` that lets
    T037, T038 and T039 land in parallel (two links off lane B,
    measured in the stage B pass) against one interface with one
    implementation; or the three in sequence as planned.
21. **ADR 0012.** Decided on 2026-10-12: the record is accepted after
    the pull request that carries it merges; until its status says
    Accepted it is cited as proposed.
22. **The risks accepted by name.** Decided on 2026-10-12: the five
    rows of 05 5.4 that the re-plan added or amended (the rows on
    `tools/ci pr`, on `tools/ci sequences` and on `tools/ci invariants`
    with `mutate`, the row "I34 is enforced by no code", and the
    approval-line row amended for the grants) are accepted, with the
    rows of [Accepted risks](#accepted-risks) that link them. Amended on
    2026-10-13: every row of [Accepted risks](#accepted-risks) is
    accepted, the plan's own rows as well as the rows that link 05 5.4.
    The alternative not taken: only the rows that link 05 5.4, with the
    plan's own rows left proposed.
23. **The records that name their approver.** Decided on 2026-10-12:
    the records written before 2026-10-08 that name their approver are
    not edited.
24. **The sandbox host list of 10 10.8.** Decided on 2026-10-12: 10
    10.8 keeps its pending clause now; the list is measured with `sbx
    policy log` during one `go run ./tools/ci all` in a sandbox at
    sitting M0, and the pull request that carries that sitting's
    Evidence writes it into 10 10.8.
25. **The deferral triggers of `romeu adopt` and `layout`.** Decided on
    2026-10-12: the triggers of T050 and T056 gain "or the first release
    candidate", as their task files and
    [Decisions this plan defers](#decisions-this-plan-defers) say.
26. **The specification sweep of 2026-10-01.** Decided on 2026-10-12:
    closed as superseded by review rounds 1 to 18 and the re-plan's
    pass.

## Questions for the maintainer

Places where the specification is silent or says two things, found
while the earlier plan was written. The plan states the reading it
took, so work can start; none of them is a decision of ours to keep.

A question about the specification is a row of the index's
[Open questions](spec.md#open-questions); Q26 to Q31 came from this
list in review round 8. This list keeps the plan's own readings, and
its numbers stay, so that the tasks can cite them.

**To answer before the work that needs it, in this order.** Each names
the first task that waits for the answer.

1. **The plan's place** (index, 02 2.1). Needed before: now, with this
   page. The specification says "first plan task" and "last plan task",
   and names no path, format or task shape for the plan; the tree of 02
   2.1 had no entry for it.
   Recommendation: Accept `docs/plan.md` as one page and add it to the
   tree of 02 2.1.
   Answered (maintainer, 2026-10-01): accepted - `docs/plan.md` as one
   page, added to the tree of 02 2.1.
   Reopened on 2026-10-12 by the re-plan: item 2 of
   [Decisions of the re-plan](#decisions-of-the-re-plan), decided on
   2026-10-12, puts one file per task beside this page, and
   [Where the task checklist lives](#where-the-task-checklist-lives)
   says why the 2026-10-01 reasoning still holds against a separate
   checklist file.

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
   Answered (maintainer, 2026-10-01): each check lands with its first
   input.

3. **The pieces that land early.** Moved to the index as Q26, with
   this plan's recommendation as its default.

4. **The image and herdr downloads in CI.** Moved to the index as Q27.

5. **What the workload's Debian base is.** Moved to the index as Q28,
   which A12 settles.

6. **What A3 probes in block A.** Settled by the specification: A3
   records the mount facts, and C6 of block B checks what romeu and
   julieta do with the plants (11 11.2).

7. **Whether block A ever plants a global secret or a broad allow rule
   on your host.** Moved to the index as Q29.

8. **How probe results reach `main`.** Moved to the index as Q30.

9. **A result that overturns a default.** Settled by the
   specification: `tools/ci probes` does not fail on an overturned
   result, and `tools/ci acceptance` fails while one has no
   `resolvedBy` (11 11.4).

10. **The signature check of `kitpin mise`.** Moved to the index as
   Q31.

11. **The two diagrams with no table source.** Settled by the
   specification: 12 12.8 draws the candidate and generation machines
   only, the promotion commit is the numbered procedure of 01 1.6, and
   the probe has no lifecycle diagram, so T083 is removed.

12. **The text of `SECURITY.md` on what happens next** (12 12.8, ADR
   0001 rule 20). Needed before: T084. 12 12.8 fixes the channel and the
   response expectation and says that what happens next "is written with
   the page", which makes that text the maintainer's and not a task's;
   no block and no question carried it.
   Recommendation: Write it in the review of that pull request, a few
   sentences after the acknowledgement: what you read, that a fix has no
   promised time, and where the outcome is published. Routed to stage
   C: the answer is written in T084's pull request review.

13. **The rule for `gh`** (12 12.1, 02 2.1, 07 7.3, 04 4.3). Needed
   before: T088. A rule this plan considered has three parts: an exact
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
   Answered (2026-10-08): the rule is in 12 12.1, the specification
   change it needed; T088 builds the freshness check, and its scheduled
   run is a deferred decision.

**The other gaps.** No task waits on these; the reading is in the last
column.

| # | Section | What we found | Reading taken here |
|---|---|---|---|
| 14 | 12 12.2, 02 2.1 | the map names no owner for `e2e/fakesbx`, `e2e/scenarios`, `internal/shquote` and `examples/` | `probes`, `romeu-cli`, `render` and `spec` |
| 15 | 12 12.2, 12 12.3 | `tools/schemagen` is under `spec`, and `probes`, which comes first and depends on nothing, needs `probe-result.v1` | T125 lands the generator with a fixture type and the check, T135 the first schema under `schemas/`, before T021 and T051; T035 and T059 generate through them, and T021 (M3) adds its two types; the harness (T014, M6) reads it from there |
| 16 | 11 (opening), 10 10.2 | "the first plan task can pass `tools/ci all` before any host run" is said of the probe harness, and the first plan task is `ci-bootstrap`; wording only | the harness is the first task of its module |
| 17 | 12 12.2, 11 11.2 | the `probes` row comes first in the build order, and the block B and sandbox-side definitions need the kits, the host suite and the release tooling's candidate | T082, after the kits and before the candidate tag |
| 18 | 12 12.3, S10 | S10 asks for a generated reference page per file format, and the generator table has a row for `project.md` and `runtime-event.md` only | one generator, landed in T051 (gap 15), fed by each schema task |
| 19 | 12 12.10, 13, 02 2.1, 03 (opening) | 03 says each format has a schema in `schemas/`; `dora.v1`, `ledger-entry/v1` and the view files belong to designs that v1 defers (12 12.10, 13) | no schema is planned for them until they are built |
| 20 | 12 12.7, 12 12.2 | `docs/lessons/` and the `lesson` kind have no owning module, and each pull request has a Lessons section from T005 on | the directory, its generated index, the kind and the check land in T006 |
| 21 | S4, 03 3.5, 02 2.2, 12 12.1 | 02 2.2 names the reference instance. What a task lacks is access to it and a Linux place to run julieta, which has no darwin build | from M3 on, each project's own sandbox at sitting M3; blocks O4 and O5 for the candidate |
| 22 | 06 6.1, 06 6.4, 02 2.1 | `tools/kitpin` has subcommands for the frontend, mise and herdr, and none for the workload digest that A12 and the container e2e read | settled by the specification at 1e63d6a: 06 6.2 and 02 2.1 name `tools/kitpin workload <digest>`, which T012 builds |
| 23 | 11 11.1 (A5), 03 3.3, 12 12.2 | A5 runs `sbx env plan` on the `sbxenv.yaml` goldens and records their hashes, and `render`, which owns the goldens, comes after block A in the build order; 03 3.3 already calls the goldens "probe-confirmed" | the goldens are written with `render` (T037) and hashed by A5 when it runs (T019, M6); until then they are confirmed by the maintainer's daily use (rule 10) |
| 24 | 10 10.3, 11 11.1 (A4), 04 4.2 | the fake `sbx` refuses a shape that is in no recording, and no table says which shapes and which starting sessions block A must record; A4 lists `-it` forms of `sbx env exec`, and `run` steps 5 and 6 use the non-interactive form | settled by the specification at 1e63d6a: A4's definition records the two non-interactive `sbx env exec` shapes of 04 4.2 beside the `-it` forms, and the table of 11 11.1 (Recorded sessions) lists the shapes; the sessions are written by hand with each task whose end-to-end tests call the fake (T029) |
| 25 | 12 12.3, 10 10.2 | the earlier plan page had derived parts (counts, block lists, traceability rows) and no generator row or check | closed with the re-plan (item 2 of Decisions of the re-plan, decided on 2026-10-12): the derived parts are gone from this page, and `tools/ci plan` (T106) validates the task files |
| 26 | 09 (opening), 09 (J12) | the opening of 09 says J12 runs J1 steps 3 to 6, and the J12 section says steps 1 to 5 | settled by the specification at 1e63d6a: the opening and the J12 section agree on J1 steps 3 to 5 and J12 step 2; T075 follows them |
| 27 | 12 12.8, 12 12.3, ADR 0001 rule 17 | the state-machine diagrams of `ARCHITECTURE.md` are generated, and the page is hand-written; ADR 0001 rejected, among its alternatives, a generator that injects text between markers in a page people edit | T033 writes each diagram as a generated file of its own that `ARCHITECTURE.md` links to; nothing is injected into the page |
| 28 | 05 5.3, 10 10.2 (the `mutate` step), 05 5.2 | the I4, I25 and I8 guards live in `sbxdrv`, `fakesbx` and `render`, which are on no ask-first surface, so `mutate` runs on a pull request that changes only them when the scheduled `fuzz.yml` next runs it | no plan change; `fuzz.yml` runs `mutate` over everything (10 10.2), and 05 5.3 could list those packages; `sbxdrv` joins the surfaces in T107 |
| 29 | 10 10.2 (the `pr` step; the personal-path rule) | the personal absolute path rule runs in `hygiene` over tracked files, and `pr` does not read a pull request's body for one; text pasted under Evidence meets the forbidden-name check and the prose rules only | the operator blocks paste only files the harness wrote through its redaction, and the maintainer reads the rest first; extending `pr` to that rule is a change to 10 10.2 |
| 30 | 01 1.4, 05 5.2 (I7), 12 12.3 | the render digest strips every field without a `gate:"widening"` tag, so a new field whose tag is forgotten is left out of the gate, and the flip table passes | settled by the specification at 1e63d6a: 01 1.4 names the committed list of the untagged fields, which T021 keeps, and the generator fails on a field on neither |
| 31 | 11 11.4, 10 10.4 | a golden with no recorded hash is not flagged, so after an A5 result exists a golden added later passes `probes` without a probe | A5's definition hashes every file under `internal/render/testdata/` (T019), so none lacks a hash at T026; a golden added later is a re-probe, as the pin file's comment (T012) says of a pin. No check is added |
| 32 | 10 10.5, 11 11.2, 12 12.2 | `tools/ci acceptance --pre-tag` is run by hand before a tag that cannot be redone | settled by the specification: it is step 3 of the release checklist of 12 12.2, outside `release.yml`, and a check that fails after the tag is fixed by a patch release |
