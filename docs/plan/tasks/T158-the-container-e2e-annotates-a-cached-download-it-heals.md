---
id: T158
title: "The container e2e annotates a cached download it heals"
milestone: M1
module: ci-bootstrap
fate: new
owns:
  - e2e/internal/container/container.go
  - e2e/internal/container/container_test.go
  - tools/ci/internal/container/container.go
  - tools/ci/internal/container/container_test.go
  - tools/ci/internal/check/annotation.go
  - tools/ci/internal/check/printer.go
  - tools/ci/internal/check/printer_test.go
  - tools/ci/internal/check/step.go
  - tools/ci/all_test.go
  - docs/spec/10-testing-style.md
needs:
  - T013
  - T144
after: []
ask_first:
  - checks
sitting: none
---

# T158 - The container e2e annotates a cached download it heals

Goal: the one question left open by the pending note "The spec catches
up with the hardening of T127, T143 and T144"
(0000000000pz8zh0x006xkqa4w), decided on 2026-10-20: since T144 a cached
download of the container e2e that fails its pin is removed and fetched
once more (`Config.Download`, `e2e/internal/container/container.go`),
and the removal writes no trace (REQ-0798). It now writes a job
annotation, on a channel the infrastructure classifier of
`tools/ci/internal/container` (`read`) does not read, so no text a test
prints can turn a failure into an infrastructure error.

How a notice reaches GitHub from inside the test process, as measured
on 2026-10-22:

- The test process cannot print an annotation itself. The runner reads
  a workflow command from the standard output of a step, and the e2e
  step runs `go test -json`: what a test prints is wrapped in an
  `output` event of the JSON stream, which `tools/ci` reads (`read`) and
  re-prints, as text, only when the step failed (`Printer.Run`). On a
  pass the output is never printed, so a notice printed by the test
  would be lost exactly when the cache healed and the run went green.
- The test process does not know it is on a runner. The environment of
  the step is built from nothing (`check.StepEnv`, `Step.Environ`;
  `procgroup.CombinedOutput` hands the command exactly that), and
  `goenv.PassThrough` holds no `GITHUB_*` or `RUNNER_*` variable, so
  `GITHUB_ACTIONS`, `GITHUB_STEP_SUMMARY` and `GITHUB_OUTPUT` never reach
  a test (held by `TestTheStepsCommandDoesNotSeeTheRunner`). A test
  that wrote to `GITHUB_STEP_SUMMARY` would also be a summary, not an
  annotation.
- What a test prints reaches the log raw when the step fails (the
  output of a step that runs code stays raw, `printer_test.go`), so a
  `::warning::` line a test prints would become an annotation of a
  failed run, and the same line would sit among the lines the classifier
  counts as the test's own (`ownLines`). It is not a channel; it is
  the hole this task keeps shut. It is also an existing one for a
  forged line in a failing test's own output, which this task does
  not change (reopened by a decision to print the output of the e2e
  step through `git.SafeLines`).

The channel is a second private file, as the infrastructure file is.
The step makes `ROMEU_E2E_NOTICE_FILE` (`NoticeFileEnv`) in the same
directory of mode 0700, of mode 0600, empty; the harness appends one
line for each notice (`notice`, the line breaks of the message turned
into spaces); after the run, `tools/ci` reads the file in the `Read` of
the step, in `Step.Notes` (new, `check`), and `Printer.Run` prints each
note before the verdict, whether the step passed or failed: as
`::warning title=e2e::<message>` when `GITHUB_ACTIONS` is `true` in the
process of `tools/ci`, which is the one process that knows, and as a
`warn  e2e: <message>` line elsewhere. The verdict does not change: a
note is not an output event and not a name in the infrastructure file.
A harness run by hand (`go test -tags e2e ./e2e/...`, no step around
it) has no file and writes the line to its standard error, where
nothing classifies it. A missing file fails the removal, as a missing
infrastructure file fails `Fail`: a heal nobody can see is not made.

The notice names the file and the pin: "the herdr download: the
cached file <path> has the sha256 <got>, not the pinned <want>; it was
removed and fetched once more". The spec row that said the removal
writes no line, REQ-0798 in 10 10.1, says this instead (the task owns
`docs/spec/10-testing-style.md`).

The escaping of a workflow command (`%`, CR and LF in the message, `:`
and `,` in a property too; actions/toolkit `command.ts`) is written by
hand, 10 lines in `check/annotation.go`. The search for a maintained
library found `github.com/sethvargo/go-githubactions` (Apache-2.0,
v1.4.0, 2026-07-17), a client of the whole runner (outputs, summary,
OIDC token, environment files); it would pay off only with a second use
of a workflow command, and a new requirement is ask-first, so it is not
imported (reopened by that second consumer). The message goes through
`git.SafeLines` first, which breaks the legacy `##[` marker that the
runner finds anywhere in a line, and a line that starts `::`.

- Implements: REQ-0798 (10 10.1), the exception for the download cache.
- Operator: no.
- Acceptance:
  - A cached download whose file fails its pin, with the step's file
    around, leaves one line in it that names the cached file, the sha256
    it has and the pinned one; the file is fetched once and held to the
    pin as before; a clean download and a cached read leave no line
    (`TestDownloadChecksTheSHA256`).
  - A removal that cannot be recorded (the file is missing) is a test
    failure before any fetch, and a fetch that fails after a removal is
    still the infrastructure error of the download
    (`TestCorruptEntryRetryFails`); a run without the step writes the
    line to the standard error (`TestNoticeIsRecordedInTheStepsFile`).
  - The notes of the step are the lines of the notice file alone: a run
    whose tests print a forged `::warning::`, a forged notice line and a
    forged infrastructure line has no note, and a run that fails keeps
    its notes and its verdict, a test failure
    (`TestNoticesComeFromTheFileAlone`); the command does not see the
    runner's variables (`TestTheStepsCommandDoesNotSeeTheRunner`).
  - The verdicts of the classifier are unchanged: no test, a skipped
    test, a forged infrastructure line, and the others of `TestRead`.
  - The notes print before the verdict as an annotation on a runner and
    as a warning line elsewhere, on a pass and on a failure, with
    `%`, line breaks, `:` and `,` escaped, and the verdict is the same
    either way (`TestAnnotationEscapes`,
    `TestNotesAreAnnotationsOnARunnerAndWarningsElsewhere`).
  - Both files are private, of mode 0600 in a directory of 0700, and
    removed after the run (`TestAroundMakesAPrivateFile`); both
    packages name the same variable (`TestTheHarnessNamesTheSameConstants`).
  - REQ-0798 says the removal writes a notice and where it shows.
- Verify: `go test -race ./e2e/internal/container/...
  ./tools/ci/internal/container/... ./tools/ci/internal/check/...`, and
  `go run ./tools/ci all`. That GitHub shows the line as an annotation
  is read from the workflow-command documentation and the toolkit's
  escaping, not run on a runner: the first run of ci.yml that heals a
  cache entry shows it.
