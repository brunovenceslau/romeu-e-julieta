# 7. Adopt testify assert and require in tests

Date: 2026-10-05

## Status

Accepted

Superseded in part by [9. Drop the required review and amend the records that assumed it](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)

## Context

Romeo read the news of Juliet's death and stopped at the first line.
He never learned that the letter carrying the rest of the story had
not arrived. A test that calls `t.Fatal` after a failed setup does the
same, on purpose, and that is the right behavior for a precondition.
The trouble starts when the same test uses `t.Fatal` for a check that
nothing else depends on: the first failure hides the others, and the
reader fixes them one run at a time.

Our tests were written with the standard library alone. The code that
exists today has 193 lines that call `t.Fatal`, `t.Fatalf`, `t.Error`
or `t.Errorf` (`git grep -cE 't\.(Fatal|Fatalf|Error|Errorf)\('` over
`tools/**/*_test.go` at commit `ce9eafd`), thirteen `reflect.DeepEqual`
comparisons, and in several places a hand-written `if got != want`
whose message says "want" and "got" in an order that differs from file
to file. Each of them is a small decision that a reader has to check
again. The proposal for this change was a library that makes the two
behaviors, stop here and keep going, explicit in the name of the call.

The change adds the dependency, pinned to the latest version at the
time, and brings the linter configuration of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
(rule 14) into the same housekeeping change, with the rule that no lint
rule is loosened and no ignore comment is used: what the linter finds
is fixed in the code. This record carries that
rule into the decision below.

What we measured before deciding:

- The latest testify is v1.12.1, released 2026-08-19. Importing its
  `assert` and `require` packages adds one module to `go.mod`,
  `github.com/stretchr/testify`, and two lines to `go.sum`;
  `go.yaml.in/yaml/v3`, which testify also needs, is already ours.
- golangci-lint 2.14.0, the version in `mise.toml`, bundles testifylint
  v1.6.4 (`go version -m` on the installed binary).
- `NotContains` prints the whole text it searched when it fails. The
  source at v1.12.1 (`assert/assertions.go`, `NotContains`, line 983)
  formats it with `truncatingFormat` (line 621), which cuts the value
  at `bufio.MaxScanTokenSize/2 - 100` bytes, 32668, and appends
  `<... truncated>`.

| Source | Their decision | Their why | What it cost them |
|---|---|---|---|
| [pkg.go.dev, testify `assert`](https://pkg.go.dev/github.com/stretchr/testify/assert) and [`require`](https://pkg.go.dev/github.com/stretchr/testify/require) | two packages with the same assertions: "All functions in this package return a bool value indicating whether the assertion has passed" in `assert`, and in `require` the assertions "call [testing.T.FailNow]" instead | one name says whether a failure stops the test, so the author chooses per assertion and the reader sees the choice | `require` "must be called from the goroutine running the test function, not from other goroutines created during the test", so a test with a goroutine needs `assert` there |
| testify v1.12.1, `assert/assertions.go` (`NotContains` at line 983, `truncatingFormat` at line 621) | a failure message prints the values compared, cut at 32668 bytes | the comment on `truncatingFormat` says it keeps "formatted error messages lines from exceeding the bufio.MaxScanTokenSize max line length that the go testing framework imposes" | a failure on a long value shows a prefix of it, marked `<... truncated>`, and a failure of `NotContains` always repeats the searched text |
| [github.com/Antonboom/testifylint](https://github.com/Antonboom/testifylint) README at v1.6.4 | a linter for testify's ambiguous parts, with checkers such as `require-error`, `expected-actual`, `go-require`, `contains`, `len` and `empty`, and `--enable-all` | its problem statement: testify "has a terrible ambiguous API in places, and the purpose of this linter is to protect you from annoying mistakes" | `require-error` skips some cases to avoid false positives (the last assertion of a block, an assertion in a goroutine or in a cleanup function), so it is a guard and not a proof |

Does their constraint hold for us? The first row's does: our tests
have preconditions and independent checks, and the `pty` tests run
goroutines, so we need both functions and the goroutine rule. The
second row's cost is real and small: our fixtures use made-up names
and short text, and every table test already logs the command output
when it fails (`if t.Failed() { t.Logf(...) }`), so a repeated
haystack adds nothing a reader lacks and does not hide anything the
log would show. The third row's constraint is ours as well: a
convention that only reviewers enforce drifts, and rule 14 of the
documentation standard already sends such conventions to the linter.

Rule 8 asks for a check that no finding is silenced. golangci-lint
and the linters it runs each read their own directives, so we read
their sources at the versions golangci-lint 2.14.0 pins (its `go.mod`:
revive v1.17.0, staticcheck v0.8.1) before choosing how to check it:

| Source | Their decision | Their why | What it cost them |
|---|---|---|---|
| [Go Doc Comments, directives](https://go.dev/doc/comment#directives) and Go 1.27 `go/ast` (`isDirective` in `ast.go`, `ParseDirective` in `directive.go`) | a directive is `//tool:name` with no space after `//` (`[a-z0-9]+:[a-z0-9]`), plus `//line `, `//extern ` and `//export `; `CommentGroup.Text` drops them from a doc comment | tools get one shape to read and doc readers never see it | the shape is not a registry: any tool may define one, and a `//line` directive moves positions, which golangci-lint follows (`FilenameUnadjuster`) and then drops a finding that lands outside a Go file (`InvalidIssue`, `pkg/result/processors`) |
| golangci-lint v2.14.0, `NewNolintFilter` and `extractInlineRangeFromComment` (`pkg/result/processors/nolint_filter.go`), the processor list of `pkg/lint/runner.go` and `NoLintLintSettings` (`pkg/config/linters_settings.go`) | `nolint` is read after `strings.TrimLeft(text, "/ ")` and matched by `^nolint( \|:\|$)`; the filter is always in the chain; `nolintlint` offers `require-explanation`, `require-specific`, `allow-no-explanation` and `allow-unused` | a team that allows suppressions wants them specific and explained | there is no setting that stops honouring `nolint`, and `nolintlint` polices its form, it does not forbid it |
| revive v1.17.0, `directiveRegexp` and `File.lint` (`lint/file.go`) | `revive:disable` and its `-line` and `-next-line` forms, after any white space following `//`; the `directives` setting only adds `specify-disable-reason` and `specify-disable-rule` | in-code control per rule for a project that wants it | no setting turns the directives off |
| staticcheck v0.8.1, `ParseDirectives` (`analysis/lint/lint.go`) and `unused` (`unused/unused.go`) | `//lint:ignore` and `//lint:file-ignore`, read from comments that start with `//lint:` | in-code suppression with a reason, for the `staticcheck` command | `unused`, which golangci-lint runs by default, honours `//lint:ignore U1000` inside the analyzer itself |

Does their constraint hold for us? Theirs is a codebase that sometimes
wants a finding off; ours forbids that (rule 8), so none of their
settings is the check we need, and enabling `nolintlint` would add a
linter that never fires once the test forbids every `nolint`. We take
the definition of a directive from Go itself and turn the check into an
allowlist over it, read as leniently as the most lenient of these
readers, so a new tool's directive is caught without a new entry. The
cost is a stricter reading than any one tool's: a comment line that
starts with a lowercase `word:word` is a finding until it is phrased
another way.

The lint step runs `golangci-lint`, which runs the go command, and both
read the environment: `GOFLAGS` can add build tags, `GOOS`, `GOARCH`
and `CGO_ENABLED` choose the files, `GOLANGCI_DIFF_PROCESSOR_PATCH`
(`NewDiff` in `pkg/result/processors/diff.go`) keeps only the findings
of a patch, and a `go env -w` file can set any go variable unless
`GOENV=off` (`EnvFile` in Go 1.27 `cmd/go/internal/cfg/cfg.go`; an
empty variable falls back to that file, `Getenv` in the same file). The
`os/exec` documentation of `Cmd.Env` says a nil `Env` is the parent's
environment and that the last of duplicate keys wins. The same
`GOFLAGS` reaches `go vet` and `go test` too, where a `-run` that
matches no test would pass the unit step with nothing run. We build the
environment of every step of `tools/ci fast` from nothing: the
variables that say where things are, then fixed values for the rest.

### Alternatives considered

- **Keep the standard library alone.** It has no dependency to pin,
  and `t.Fatal` and `t.Error` are already the stop and the continue.
  What it lacks is the readable pair on one line, and a way for a
  linter to see the intent: `if got != want` can be written in either
  order and nothing checks it. Rejected.
- **testify with `assert.New(t)` objects.** It saves passing `t`. It
  also hides which `t` an assertion uses inside a subtest or a
  goroutine, which is the mistake `go-require` exists to catch.
  Rejected; package functions only.
- **testify's `suite` and `mock` packages.** Our tests are table tests
  over real fixture repositories, with no object to mock and no state
  to set up for a group. Neither package has a job here. Rejected, and
  recorded so that the question has an answer when it comes back.
- **Our own small helpers.** A handful of `mustNoError` and `equal`
  functions would cover most of it. Every helper is a concept the next
  contributor learns, and none of them gets a linter. Rejected.

## Decision

We use testify's `assert` and `require` packages in tests, under these
rules:

1. **Package functions only.** A test calls `assert.Equal(t, want,
   got)` and `require.NoError(t, err)`. It does not call `assert.New`,
   and no test imports `suite`, `mock` or `http`. A unit test reads
   the Go files of tests, `_test.go` files and the packages whose name
   ends in `test`, with `go/parser`, and fails on those imports and on
   a reference to `assert.New`, `require.New` or `reflect.DeepEqual`
   (rule 5).
2. **`require` for preconditions and for every error assertion.** A
   setup step, a value a later line depends on, a length checked
   before an index, and every `Error`, `NoError`, `ErrorIs`, `ErrorAs`
   and `ErrorContains` use `require`, because the test cannot say
   anything useful after them.
3. **`assert` for independent checks.** An exit code, an output line
   and a final comparison use `assert`, so one run lists every
   problem. In a goroutine other than the test's the test uses
   `assert`, since `FailNow` must run on the test's own goroutine.
4. **Expected first.** `assert.Equal(t, want, got)`, with the value
   the test wrote before the value the code returned.
5. **The precise assertion.** `Contains` and `NotContains` for text,
   `Len` and `Empty` for size, `True` and `False` for booleans,
   `Equal` for comparisons. `reflect.DeepEqual` and a hand-written
   `if got != want` do not appear in tests.
6. **A message only when it adds information**: a label for a fixture,
   or the command output that explains a failure. A test that logs its
   output on failure keeps doing so.
7. **testifylint runs with `enable-all: true`** in `.golangci.yml`,
   next to the doc-comment, error-string and commented-out-code
   checkers of rule 14. A unit test runs the pinned linter over a
   fixture package with one violation of each rule 14 checker and of
   each testifylint checker that applies outside suites, and reads
   `.golangci.yml` to check that `enable-all: true` is set, which
   covers the `suite-*` checkers.
8. **No finding is silenced.** The configuration disables no linter,
   lowers no setting and excludes no path, and no Go file has a
   generated-code header. A finding is fixed in the code. Directives
   are held to an allowlist, which today is `go:build`, and
   `go:generate` at one site only: the one directive
   `go:generate go run . generate` in `tools/ci/generate.go`, which
   `go generate ./...` runs (12 12.3). That directive is judged on raw
   lines, as `go generate` reads them (`isGoGenerate` in
   `cmd/go/internal/generate`: a line that starts with `//go:generate`
   and a space or a tab, in a comment, a raw string or a block
   comment alike), in every `.go` file below the root but `.git`, with
   no regard for build constraints, `testdata` or nested modules: the
   repository holds exactly one such line, the directive above, and
   `tools/ci generated` refuses to run `go generate` while that does
   not hold. A comment holds no other
   directive of the `tool:name` form (the shape of
   `isDirective` in `go/ast`, which `nolint:`, `revive:disable` and
   `lint:ignore` have), read in every line of every comment after
   the leading `/`, `*` and white space (every rune of
   `unicode.IsSpace`, a superset of revive's `\s` and of golangci-lint's
   `"/ "`) are trimmed; no `nolint` word at the start of such a line,
   as golangci-lint v2.14.0 reads it; and no `line`, `extern` or
   `export` directive right after `//` or `/*`, as the Go toolchain
   reads them. Unit tests read `.golangci.yml` and the Go files,
   tracked, not yet added and ignored, to check this, and that each of
   those files is one the linter reaches: it builds, with cgo off, for
   one of the four targets the lint step runs for (linux and darwin,
   each on amd64 and arm64, one list in `tools/ci/fast.go` that the
   test reads too), with the tool tags the pinned go command reports
   for that target (`amd64.v1`, `arm64.v8.0` and the default
   `goexperiment` tags); it is outside `testdata`, `vendor`, a `_` or
   `.` directory and a nested `go.mod`; and `go.mod` holds no directive
   but `module`, `go` and `require`, so no `ignore` takes a directory
   out of `./...`. `tools/ci fast` lints once for each target, in an
   environment built from an allowlist (`lintEnv`), and runs the
   golangci-lint that `mise which` resolves directly, not through `mise
   exec`, which would add the `[env]` table of a mise configuration;
   it refuses a resolved tool that is not below the directory where
   mise installs it. A test also holds every mise configuration file of
   the repository to the `[settings]` and `[tools]` tables, with
   `lockfile` the only setting and `go` and `golangci-lint` the only
   tools, each at the exact version `mise.lock` locks, and allows no
   `.tool-versions` file. The linters bound what any configuration reaches: revive's
   `exported` rule, the doc-comment checker of rule 14, reads only a
   file that another package can import, so it skips package `main`
   and `_test.go` files (`File.IsImportable` in `lint/file.go`, revive
   v1.17.0), and checks nothing in `tools/ci` today. No setting of the
   rule extends it to them; its `checkPrivateReceivers` and
   `checkPublicInterface` options widen what it reads in an importable
   package, and neither is enabled by this decision.
9. **The version is exact.** `go.mod` pins `github.com/stretchr/testify`
   at one version, and a newer one arrives as a reviewed diff.

Why this fits us: the rules are the ones testifylint can check, so the
review of a test spends its time on what the test proves and not on
how it spells a comparison. The cost of the library is one pinned
module that only tests import.

### Threat model

The checks of rules 7 and 8 and `tools/ci fast` guard against two
things:

1. **What the review cannot see.** Content in the working tree that
   the push does not send, and the environment of the caller once
   `tools/ci` runs. The review sees commits, so `fast` run by the
   pre-push hook judges the commit the push sends (`judgeCommit`): it
   refuses to run when a pushed tip, other than a deleted ref and
   followed through an annotated tag, is not HEAD, when any tracked
   file differs from HEAD (staged, an intent-to-add entry of `git add
   -N`, or modified), and when the tree holds an untracked or ignored
   Go file, `go.work` or `go.work.sum` file, root `vendor` directory,
   mise configuration, `.tool-versions` file or nested repository,
   which `git ls-files --others` lists as a directory while `go list`
   reads the packages inside it. Its git commands run without the
   variables that point git at another repository, index, object store,
   work tree or configuration (`git rev-parse --local-env-vars` and every
   `GIT_CONFIG` variable), so the index it reads is the repository's
   own. The steps take minutes, so after the last one `fast` judges
   the tree again and stops the push when HEAD has moved or a refusal
   now holds; hygiene and the pushed range read the commit judged, by
   its id. The forbidden-names check of the pushed range matches with
   the union of the denylist at that commit and the one at the
   remote's default branch as the clone knows it
   (`refs/remotes/<remote>/HEAD`), so an entry that a pushed commit
   removes stays in force until its removal reaches the default branch
   through review; a push to no configured remote, a missing default
   branch ref and a denylist there that cannot be read stop the push.
   `fast` run by hand, with no argument, checks the working tree
   as it is and refuses none of this: nothing leaves the machine, and
   development stays free. In both modes it refuses a `go.mod` below
   the module root, tracked or not, which would take a directory out
   of `./...`. Every step runs in an environment built from an
   allowlist of the variables that say where things are (`stepEnv`),
   with `GOFLAGS=-mod=readonly`. The
   tools are the ones mise installs below `HOME`: a tool that `mise
   which` resolves elsewhere stops the run, and so does a set
   `MISE_DATA_DIR` or `XDG_DATA_HOME`.
2. **The common shortcuts in tracked content.** A suppression
   directive, an excluded path, a lowered or disabled setting of
   `.golangci.yml`, a generated-code header, a file that no lint target
   builds or that `go list` skips, an `ignore` directive in `go.mod`,
   and an `[env]` table or a tool named by a path in a mise
   configuration. The tests make each of them fail the build, so the
   review sees it named.

One gap is named: a tree that is changed and restored while the steps
run is not detected, since the second judging compares HEAD and the
files at that moment, and not what the steps saw.

Deliberate sabotage written in tracked Go code, such as a `TestMain`
that exits 0, a deleted test or a helper that checks nothing, is
visible in the pull request's diff, and finding it is the review's job.
Two escapes act on `go run` before `tools/ci` starts, so no check
inside it can see them: an untracked `go.work`, in the tree or a
parent directory, and a `GOFLAGS` such as `-overlay` in the caller's
environment. They
need the pre-push hook to start `tools/ci` differently. The hook is on
the ask-first checks surface, so both are deferred until the hook next
changes, or until the sandbox-token question (Q25) is decided, which is
the plan's deferred-decisions row "whether agents push from a second
account", reopened by block O1 item d.
What the checks trust is the machine: the mise binary on the search
path, the tools installed below `HOME`, the Go caches, the git binary,
and the local refs, the remote-tracking default branch among them,
which decides with HEAD the denylist a push is judged by.

## Consequences

- A new Go dependency enters through the ask-first list, and testify
  is added to the list in [12 12.1](../spec/12-engineering.md#121-tech-stack).
  testify is MIT licensed (`LICENSE` in the module); no testify file is
  copied into the repository.
- The linter configuration is an ask-first surface (rule 8 of the
  documentation standard), so a change that loosens it needs the same
  approval line and review as any other change to it.
- A failing `NotContains` repeats the searched text up to 32668 bytes.
  If a test ever searches a large or sensitive text, it checks the
  condition with `assert.False(t, strings.Contains(...))` and a short
  message instead, and testifylint's `contains` checker will ask for
  `NotContains` again. That conflict is the trigger to reopen this
  point, with the case in hand.
- Upgrading golangci-lint can change what testifylint reports. The
  upgrade is a reviewed diff of `mise.toml` and `mise.lock`, and the
  findings it brings are fixed in the same change.
- The lint step runs four times, once per target. Measured on this
  repository on a linux/arm64 machine: about 17 seconds for the four
  with an empty golangci-lint cache, and under 2 seconds warm, against
  the one GOOS per run it replaces, which missed a file named for
  amd64 on an arm64 machine. A new target is one line in
  `lintTargets`, and the tests follow it.
- The environment of every step of `tools/ci fast` is an allowlist,
  with the module proxy off. A developer who moved the module cache
  with `go env -w GOMODCACHE=...` instead of the variable, or whose
  cache lacks a module of `go.sum`, sees the steps fail until the
  variable is set or `go mod download` has run; they fail, they do not
  pass with less checked.
- A new directive, such as `go:generate` or `go:embed` the day the
  code needs one, is a reviewed line in `allowedDirectives` of
  `tools/ci/lintconfig_test.go`.
- The fixture test and the lint step of `tools/ci fast` both run the
  golangci-lint and the go command that `mise which` resolves
  (`resolveLintTools`), so a machine without the pinned tools fails
  both, before any lint runs. A check that cannot run is a failure,
  not a skip. `mise which` installs nothing, measured with mise
  2026.10.3 and its `auto_install` settings on: against an empty data
  directory it exits 1 and writes nothing. A first run on a new
  machine needs `mise trust` for this repository and `mise install`,
  and the network that the install needs. Both runs happen in the
  environment of `lintEnv`, with `GOPROXY=off`. `gofmt`, `go vet` and
  `go test` run from the same pinned go directory, so the go command
  that tests is the one the linter runs.
- Two linters share the error-string rule. revive's `error-strings`
  reports a capitalized string or one that ends with punctuation, also
  in a top-level `var X = errors.New(...)`, but it skips a one-word
  capitalized string; staticcheck's ST1005 (a default linter) reports a
  capitalized string of two or more words. The fixture has a string of
  several words that ends with a full stop, which both report.
- A test that used `t.Fatal` for a precondition keeps stopping there
  with `require`, and a check that only reported keeps reporting with
  `assert`. The migration of the existing tests changes no check.
- The next test written starts from rules that a tool checks, and not
  from a file to imitate.
