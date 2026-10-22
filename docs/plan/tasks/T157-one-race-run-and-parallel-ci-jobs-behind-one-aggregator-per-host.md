---
id: T157
title: "One race run and parallel CI jobs behind one aggregator per host"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - tools/ci/all.go
  - tools/ci/all_test.go
  - tools/ci/fast.go
  - tools/ci/fast_test.go
  - tools/ci/checks_test.go
  - tools/ci/main.go
  - tools/ci/jobs*.go
  - tools/ci/check_unit.go
  - tools/ci/check_race.go
  - tools/ci/check_coverage.go
  - tools/ci/check_container.go
  - tools/ci/internal/unit/**
  - tools/ci/internal/race/**
  - tools/ci/internal/check/check*.go
  - tools/ci/internal/workflows/**
  - tools/ci/internal/citest/citest.go
  - .github/workflows/ci.yml
  - .github/workflows/x64.yml
  - docs/spec/10-testing-style.md
  - docs/spec/12-engineering.md
  - docs/reference/checks.md
  - docs/plan/tasks/T001-hygiene-the-denylist-fast-and-the-pre-push-hook.md
  - docs/plan/tasks/T002-tools-ci-workflows-tools-ci-all-coverage.md
  - docs/plan/tasks/T003-the-ask-first-list-tools-new-adr-and-tools-ci.md
  - docs/plan/tasks/T013-the-container-e2e-on-the-workload-s-base.md
  - docs/plan/tasks/T132-step-durations-and-the-fast-budget.md
  - docs/plan/tasks/T139-the-lint-split-and-the-lint-set.md
needs: []
after:
  - T155
  - T156
ask_first:
  - checks
  - dependencies
  - approvals
  - spec
sitting: none
---

# T157 - One race run and parallel CI jobs behind one aggregator per host

Goal: the last part of "One race run, test tags and parallel CI jobs"
(01m5drzqsp6zg34xzmywckwb7b), approved for planning on 2026-10-20,
with the two decisions of 2026-10-21 ("Work authorized while the
operator is away on 2026-10-21", 01m5eg0qefcvwdx05q4vnqcwfe): one
aggregator job per host keeps the required check names of `main`, and
the macOS job runs only the race with coverage.

The macOS job of `ci.yml` took 13 to 25 minutes in the runs of
2026-10-20, against 9 on Linux, and it runs every test twice: `unit`
(4m22 there) and then the race (9m17), which runs every test `unit`
runs.

1. **One test run.** The `unit` step leaves `fast` and `all`
   (`tools/ci/internal/unit/unit.go`, REQ-0824, retired with a row of
   the retired table, 12 12.11); the race step (REQ-0833) is the one
   run of the tests, with `-race`, over `./...`. The rows that order a
   step before `unit` (REQ-0822, REQ-0831, REQ-0847) and the order of
   `all` (REQ-0860) name the race step instead.
2. **Jobs.** Each registered check names the job it runs in: `static`
   (the default, so a check that names none is static), `race` (the
   race step and the coverage check after it) or `e2e` (the container
   e2e of REQ-0837). `tools/ci static`, `tools/ci race` and `tools/ci
   e2e` each run the checks of one job, with the platform and
   working-tree reports `all` prints today; `tools/ci all` runs the
   union in the order of today, so a developer still runs one command.
   The runner-class table of REQ-0812 still selects inside `tools/ci`
   what a platform runs.
3. **Workflow.** On Linux (`ubuntu-26.04-arm`) three jobs, `static`,
   `race` and `e2e`; on macOS (`macos-26`) one, `race`, since lint and
   fix cover the darwin targets from Linux and license runs on Linux
   only. macOS so drops `vet` too: no file of the module builds on
   darwin alone today (measured on 2026-10-21: the constraints in use
   are `e2e`, `unix`, `linux || darwin` and `linux`). Reopened by the
   first file whose constraint selects darwin and not linux, which puts
   a darwin `vet` back. Each job builds `tools/ci` and runs its
   subcommand, as the job of today runs `all`. `x64.yml` keeps the same
   shape on its two labels. Every job that installs mise keeps a matrix
   of `os` and `mise_sha256`, one sha256 per label: `matrixShas`
   (`tools/ci/internal/mise/pin.go:121`) reads the matrix of every job
   and keeps one sha256 per label, and `stepVersions`
   (`misefiles.go:460`) reads the mise version of every job's step
   once, so `tools/ci/internal/mise`, owned by T053, needs no change.
4. **Aggregators.** Two jobs, named exactly `all on ubuntu-26.04-arm`
   and `all on macos-26`, the required checks of `main` today. GitHub
   reports a skipped job as a success, also for a required check
   (GitHub's "About status checks", read on 2026-10-21), and a job is
   skipped when a job it needs fails: an aggregator with `needs` alone
   would let a red job merge. So each aggregator `needs` every other
   job of its file that is not an aggregator, runs on
   `ubuntu-26.04-arm`, holds this `if`, verbatim:

   ```yaml
   if: ${{ always() && (contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled') || contains(needs.*.result, 'skipped')) }}
   ```

   and one step, `run: exit 1`. A needed job's result is one of
   `success`, `failure`, `cancelled` and `skipped`, so the aggregator
   runs, and fails, unless every needed job's result is `success`, a
   job cancelled by its own `timeout-minutes` or skipped included; when
   every result is `success` it is skipped, which GitHub reports as a
   success. `always()` also runs it when the whole run is cancelled (a
   newer push to the pull request), so a superseded commit shows a red
   aggregator, which blocks nothing. Both aggregators need every job, so
   a red Linux job turns both red: the per-host name is the one the
   ruleset requires, not a promise about which host failed. Each
   aggregator is a job of its own, not a matrix, since GitHub names a
   skipped matrix job with its expression unexpanded, and a required
   check would then never match.
5. **Grammar.** Three rows of 10 10.2 change, each with a fixture each
   way in `tools/ci/internal/workflows`:
   - REQ-0803 (job): the keys gain `if`, admitted only in an aggregator
     job: a job whose `name` is a literal (no `${{`), whose `runs-on` is
     a literal label, with no `strategy`, whose `needs` lists every
     other job of the same file that is not an aggregator, whose `if` is
     the text above byte for byte, and whose one step is `run: exit 1`.
   - REQ-0806 (`run` step): a fifth form, `exit 1`, admitted only as
     the one step of an aggregator job.
   - REQ-0809: "a workflow has no `if`" becomes "no `if` but the
     aggregator's", and the rule that the text between the braces is one
     context path gains its one exception, that `if`, compared as a
     whole string, so no other operator or function call is admitted.
   - Fixtures that fail: an `if` on a job that is not an aggregator; an
     aggregator whose `needs` omits a job; an aggregator with a
     `strategy` matrix; an aggregator whose `name` holds `${{`; an
     aggregator `if` that differs by one character; `exit 1` in another
     job. Fixtures that pass: `ci.yml` and `x64.yml` as this task writes
     them.
6. **Other text.** 05 5.3 lists the steps that decide whether `all`
   passes as "coverage, license, vulnerabilities, race, unit, vet and
   lint" (`docs/spec/05-security.md:420`, in REQ-0543): `unit` leaves
   that list. The row of the index's Deferred decisions on "running the
   change's tests in a job (a virtual machine) separate from the job
   that judges the commit" (`docs/spec.md:364`) is amended to say that
   in CI the `static` job judges the commit and the `race` and `e2e`
   jobs run the tests, each on its own runner, while a local `all`
   still runs both in one process tree, so the row stays open for the
   local run. `docs/spec/05-security.md` and `docs/spec.md` are owned by
   the open tasks T039 and T126: this task amends those two passages
   without owning the files, as T151 did with 05 5.1, and does not wait
   for them; `owns` does not list them.

The measurement of T155 says what the race job costs: about 47 s of
the 226 s of a local race run of 2026-10-21 is the build of the test
binaries, and the two packages that held most of the rest are fixed
there. Sharding and GOCACHE stay deferred
(01m5drzqtpe5w9q37kv3g04xnp); its trigger is the macOS race job,
still the slowest after this task.

- Implements: REQ-0800 (10 10.2), REQ-0803 (10 10.2), REQ-0806 (10
  10.2), REQ-0809 (10 10.2), REQ-0812 (10 10.2), REQ-0833 (10 10.2),
  REQ-0824 (10 10.2), REQ-0856 (10 10.2), REQ-0860 (10 10.2), REQ-0822
  (10 10.2), REQ-0831 (10 10.2), REQ-0847 (10 10.2), REQ-0543 (05 5.3).
- Operator: no action at merge: the aggregators keep the names of the
  required checks, so the ruleset of `main` does not change. The
  approval lines of `checks`, `dependencies`, `approvals` and `spec`
  are the operator's, as for any change to `tools/ci`, the workflows
  and 05.
- Acceptance:
  - `fast` and `all` run no `unit` step; `go test` runs once in `all`,
    with `-race`; REQ-0824 is retired; REQ-0856 lists the command steps
    of `fast` without it; REQ-0822, REQ-0831, REQ-0847 and REQ-0860
    order their steps against the race step; a test holds that no step
    but race and e2e runs `go test`.
  - Each registered check names its job, a test holds that `static`,
    `race` and `e2e` are the only jobs and that `all` runs their union
    in the order of today, and `docs/reference/checks.md` lists the job
    of each check (regenerated).
  - `tools/ci static`, `race` and `e2e` each exit as `all` does (3 for
    an infrastructure error), print the platform and working-tree
    reports, and run only their job's checks; `race` runs the coverage
    check after the race step; `e2e` on macOS prints the line of a step
    not run on that class.
  - `ci.yml` has the Linux jobs `static`, `race` and `e2e`, the macOS
    job `race`, and the two aggregators named `all on ubuntu-26.04-arm`
    and `all on macos-26`, each needing every other job and holding the
    `if` above; `x64.yml` has the same shape on its labels; `go run
    ./tools/ci workflows` passes on both and fails each fixture of item
    5.
  - On the pull request itself, three proofs, each run linked in the
    pull request body with the aggregators' results:
    1. every job passes: both aggregators are skipped and the pull
       request is mergeable;
    2. a pushed commit makes one Linux job fail: both aggregators run
       and are red;
    3. a pushed commit gives one job a `timeout-minutes` it cannot meet,
       so that job alone is cancelled: both aggregators run and are
       red.
    The two commits are then reverted, and the last run is the green
    one of proof 1.
  - The job times of the green run, per job, are in the pull request
    body, beside the 13 to 25 minutes of the macOS job of 2026-10-20.
  - The 05 5.3 list and the Deferred decisions row say what item 6
    says; REQ-0800, REQ-0803, REQ-0806, REQ-0809, REQ-0812 and REQ-0833
    say what the code does; the task files that cite a changed id (T001,
    T002, T003, T013, T132, T139) are updated (12 12.11); `go run
    ./tools/ci reqs` passes.
- Verify: `go run ./tools/ci all`, `go run ./tools/ci static`, `go run
  ./tools/ci race`, `go run ./tools/ci workflows`, the `pr` check of the
  pull request green on the new job names (REQ-1154: a head job's name
  that matches the pr job's name, or holds `${{`, outside the pr job's
  file is refused), and the three CI proofs above.

Question for the maintainer: with `unit` gone, the pre-push hook's
`fast` runs no test at all. The note named the other reading, the race
in the hook, about 4m30 then and 226 s on 2026-10-21. Recommended: the
race joins `fast` only if, after T155, a warm `go test -race -count=1
./...` stays under `fastBudget` (`tools/ci/fast.go:24`, 3 minutes) on
the reference machine; until then `fast` runs no test. `fast` has no
`After` step, so the race in `fast` writes no cover profile and checks
no coverage; coverage stays in the `race` job and in `all`.

## Decisions taken while building

Taken on 2026-10-21, each with what reopens it:

- The race joins `fast`, by the maintainer's rule of 2026-10-21. A warm
  `go test -race -count=1 ./...` measured 118 s, 116 s and 116 s on this
  machine (10 cores, linux/arm64, after T155 and with the build cache
  warm), against the 180 s of `fastBudget`; `fast` without `unit` took
  about 10 s on top (format, vet, fix, generated, lint, hygiene, plan),
  so the whole `fast` is about 130 s. The race step run by `fast` itself
  took 47 s on a second run in the same session; the larger figure is
  the one the rule is applied to. The race step in `fast` has no
  `-coverprofile`, since `fast` has no step after it to read one
  (`race.Steps` omits the flag when the plan has no profile). Reopened
  when a warm run no longer fits the budget: the check then drops
  `Fast: true` and `fast` runs no test.
- The row `unit + golden + race` is now `race` (it has no unit or golden
  part), with `In fast` yes. REQ-0824 is retired, row in the retired
  table of 12 12.11.
- An aggregator is a job that holds an `if`. That one mark makes every
  rule of the shape apply to it, so an `if` on any other job fails for
  what the job is not (a literal name, no strategy, one step `exit 1`),
  and `exit 1` is admitted only as the one step of such a job. A job
  that has the step but no `if` fails as a run line outside the grammar.
  Reopened by a second kind of job that needs an `if`.
- The aggregators of `x64.yml` run on `ubuntu-26.04`, the Linux label of
  that file, and are named `all on ubuntu-26.04` and `all on
  macos-26-intel`; those of `ci.yml` run on `ubuntu-26.04-arm`. The task
  gives the arm label for the aggregators and says x64 follows its labels;
  reopened if the maintainer wants the x64 aggregators on the arm runner.
- Job ids are `static`, `race`, `e2e`, `all_linux` and `all_macos`; the
  names of the three jobs are `<id> on ${{ matrix.os }}`. Timeouts: 30
  minutes for `static` and `e2e` and 60 for `race` (the race step has a
  40-minute budget, which a test holds below the race job's timeout).
  The 30-minute values are not measured: reopened by the job times of
  the green run, in the pull request body.
- Not changed, since the task does not own them: the reason text and the
  `tools/ci/internal/unit/**` glob of `.github/ask-first.yaml` (the glob
  matches nothing now), `docs/plan.md` and the handoff notes that still
  say `unit` or that `fast` runs no race. Reopened by the tasks that
  own those files.
