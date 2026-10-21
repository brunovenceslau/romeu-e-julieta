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
  - docs/plan/tasks/T002-tools-ci-workflows-tools-ci-all-coverage.md
  - docs/plan/tasks/T013-the-container-e2e-on-the-workload-s-base.md
needs: []
after:
  - T155
  - T156
ask_first:
  - checks
  - dependencies
  - approvals
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
   run of the tests, with `-race`, over `./...`.
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
   only. Each job builds `tools/ci` and runs its subcommand, as the job
   of today runs `all`. `x64.yml` keeps the same shape on its two
   labels. Every job that installs mise keeps a matrix of `os` and
   `mise_sha256`, one sha256 per label, so `mise.ReadPin` and the
   `version` read of `tools/ci/internal/mise`, owned by T053, read the
   files as they do now; if they do not, the builder stops and asks.
4. **Aggregators.** Two jobs, one per host, named exactly `all on
   ubuntu-26.04-arm` and `all on macos-26`, the required checks of
   `main` today, each `needs` the jobs of its host. A job whose needed
   job fails is skipped, and GitHub reports a skipped job as a success,
   also for a required check (GitHub's "About status checks", read on
   2026-10-21): an aggregator with `needs` alone would let a red job
   merge. So each aggregator runs only when a needed job failed or was
   cancelled, and then fails, and is skipped, a success, when every
   needed job passed. That takes one `if` per aggregator, which the
   grammar refuses today (REQ-0803, REQ-0809): the grammar admits it
   for that form only (`needs`, one fixed `if` expression, a step that
   fails), with a fixture each way. Each aggregator is a job of its own,
   not a matrix, since GitHub names a skipped matrix job with its
   expression unexpanded, and a required check would then never match.
   The step that fails is the smallest one the grammar can admit (a
   `tools/ci` subcommand after checkout and mise, or one fixed `run`
   word); the builder picks and records it with what reopens it.

The measurement of T155 says what the race job costs: about 47 s of
the 226 s of a local race run of 2026-10-21 is the build of the test
binaries, and the two packages that held most of the rest are fixed
there. Sharding and GOCACHE stay deferred
(01m5drzqtpe5w9q37kv3g04xnp); its trigger is the macOS race job,
still the slowest after this task.

- Implements: REQ-0800 (10 10.2), REQ-0803 (10 10.2), REQ-0806 (10
  10.2), REQ-0809 (10 10.2), REQ-0812 (10 10.2), REQ-0833 (10 10.2),
  REQ-0856 (10 10.2); retires REQ-0824 (10 10.2).
- Operator: no action at merge: the aggregators keep the names of the
  required checks, so the ruleset of `main` does not change. The
  approval lines of `checks`, `dependencies` and `approvals` are the
  operator's, as for any change to `tools/ci` and the workflows.
- Acceptance:
  - `fast` and `all` run no `unit` step; `go test` runs once in `all`,
    with `-race`; REQ-0824 is retired and REQ-0856 lists the command
    steps of `fast` without it; a test holds that no step but race and
    e2e runs `go test`.
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
    and `all on macos-26`, each needing exactly its host's jobs;
    `x64.yml` has the same shape on its labels; `go run ./tools/ci
    workflows` passes on both and fails each fixture of an `if`,
    `needs` or step outside the aggregator form.
  - On the pull request itself: a run where every job passes shows both
    aggregators skipped and the pull request mergeable; a pushed commit
    that makes one Linux job fail shows `all on ubuntu-26.04-arm` red
    and `all on macos-26` skipped; that commit is then reverted. The
    pull request body links both runs.
  - The job times of the green run, per job, are in the pull request
    body, beside the 13 to 25 minutes of the macOS job of 2026-10-20.
  - REQ-0800, REQ-0803, REQ-0806, REQ-0809, REQ-0812 and REQ-0833 say
    what the code does; the task files that cite a changed id (T002,
    T013) are updated (12 12.11); `go run ./tools/ci reqs` passes.
- Verify: `go run ./tools/ci all`, `go run ./tools/ci static`, `go run
  ./tools/ci race`, `go run ./tools/ci workflows`, and the two CI runs
  of the pull request above.

Questions for the maintainer:

1. With `unit` gone, the pre-push hook's `fast` runs no test at all.
   The note named the other reading, the race in the hook, about 4m30
   then and 226 s on 2026-10-21, against the 3 minutes `fast` targets.
   Recommended: `fast` runs no test until T155's numbers are in; then
   the race joins `fast` if the whole race run fits the budget on a
   warm cache.
2. The aggregator's failing step: a `tools/ci` subcommand, which needs
   checkout and mise on a run that has already failed, or one fixed
   `run` word the grammar admits for the aggregator only. Recommended:
   the `run` word, since it reads nothing and starts no tool.
