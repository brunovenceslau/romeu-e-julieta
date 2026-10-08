# 10. Testing strategy, CI, code style

Back to [index](../spec.md). Reader: implementers and reviewers of any
module. Type: reference. Tools are code: `tools/*`, `e2e/probes`,
`e2e/fakesbx`, workflows, schemas and kits are held to the same lint,
test, coverage and review rules as `internal/*`.

## 10.1 Test levels

| Level | Location | Runs where | Covers |
|---|---|---|---|
| Unit | `internal/**`, `tools/**`, `e2e/probes/**`, `e2e/fakesbx/**` `_test.go` | CI, dev | validators generated from `rules.go` (table + fuzz seeds committed), digests, widening-set and toolchain diffs, the generated state-machine tables (every row, every illegal pair), egress split, `termsafe`, shell quoter, sbx output parsers over every recorded sbx version, error-id table, doctor checks with `HOME` in a temp dir, the probe harness core and its sbx exec layer (against a helper binary re-executed from the test), the recorder's redaction, the fake sbx's placeholder normalizer, CI tools themselves (the forbidden-name matcher and its pushed-range walk over a fixture repository, `workflows` against fixture workflow files, `docs` against fixture Markdown files with one violation per case of rule 9, the linter configuration against a fixture package with one violation per checker of rule 14 in [ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) and one per testifylint checker that applies outside suites, invariants/mutate, acceptance against a recorded GitHub API fixture, `dora` against a recorded GitHub API fixture and a fixture repository, `pr` against recorded event payloads), the runtime ledger's tables ([13 13.10](13-runtime-ledger.md#1310-tests): the hostile spool, each field, entries and their races, the drain, the doctor checks on fixtures) |
| Golden | `internal/render/testdata/`, `internal/layout/testdata/`, `internal/gate/testdata/`, `internal/ledger/testdata/` | CI, dev | `sbxenv.yaml`, workspace files, `render.json`, gate diff text, herdr `layout.apply` requests (`layout up --dry-run`), handoff front matter, the two ledger views of a two-project fixture |
| Schema | `tools/ci schema` | CI | generated schemas equal the committed ones; examples and testdata validate |
| E2E (git + fake sbx) | `e2e/*_test.go`, tag `e2e` | CI (linux amd64, linux arm64, macOS Intel, macOS arm64) | romeu commands against a temp `$ROMEU_ROOT`; origins served by `git http-backend` behind `httptest` TLS (host settings `gitHosts[].caFile` points at the test CA; gitsafe has no test override); the sandbox daemon served by `git daemon` on `127.0.0.1`; journeys J2, J3b, J7, J10, J11 (scenario functions shared with the host suite), of which a journey that reaches a julieta call runs at the hybrid level below, on the Linux runners, while the macOS runners run the romeu commands that reach none; promotion fault injection; invariants marked E in [05](05-security.md), including the I27 hostile trees on the macOS runners |
| E2E (hybrid) | `e2e/hybrid_test.go`, tag `e2e` | CI on `ubuntu-26.04` and `ubuntu-26.04-arm` | the romeu-julieta contract: the fake sbx forwards `env exec` into the julieta container with `.romeu/bin` bind-mounted read-only at the host path; the journeys that reach a julieta call (julieta has no darwin build, 12 12.1); protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each stop `run` before `layout up`; a 32 KiB manifest round-trips (I30); a writable ledger view mount stops `run` the same way (I33) |
| E2E (container) | `e2e/container_test.go`, tag `e2e` | CI on `ubuntu-26.04` (amd64) and `ubuntu-26.04-arm` (arm64), native, no QEMU | julieta in the workload's Debian base: `setup` (PATH link, dispatcher, secondary clone, ff of default branch) and its no-op timing, `install` skip rule, `lock --check`, hooks dispatcher, chaining and recorded failures, memory (stamping, allowlist, import, verify incl. status), handoff (all kinds; SessionEnd and SessionStart in both orders, so the rule of 08 8.3 is tested in the order Q22 has not confirmed too), snapshot, salvage completeness cases, `layout up --dry-run` golden and `layout up` against the pinned herdr |
| Host | `e2e/host/*_test.go`, tag `host` | maintainer machines (Intel and Apple silicon) | the same scenario functions against real sbx for J1-J13; S3, S5, S8 confirmation; results as `probe-result.v1` per block B step (11) |
| Probes | `e2e/probes` | maintainer machines | [11](11-host-probes.md); `probe-result.v1` files in `docs/probes/` |

Level claims per invariant are those in the 05 table (I14, I16, I20,
I22, I23 are unit-level).

Additional required tests:

- **Idempotency meta-test**: every converging romeu and julieta command
  is run twice on unchanged inputs, and the second run must leave
  every file's content and mode and every ref as they were, except
  `lastRun`. The diff compares content and mode, not modification
  times, so a rewrite with the same bytes passes. For each appending
  command in the list of [04 4.1](04-cli.md#41-conventions-both-binaries)
  the test asserts that the second run adds the one thing the list
  names and nothing else.
- **Concurrency**: two romeu processes (flock; second exits 1 with
  `RJ-101`); two julieta processes writing memory (store lock; no lost
  entry); snapshot racing a commit (bundle always valid); `romeu pull`
  succeeds while a `run` of the same project is attached, because
  `run` releases the lock before its final `exec` (04 4.1).
- **Runtime ledger**: the rows of
  [13 13.10](13-runtime-ledger.md#1310-tests), each with its stated
  outcome. Two seams are named there: the spool reader's `afterOpen`
  test hook, for a file that changes after it was opened, and a
  version table the test injects, since v1 has one event version. The
  racing ingests call the ingest function without `romeu.lock`; under
  the lock the second romeu exits with `RJ-101`, as the row above
  says.
- **Promotion fault injection**: a test hook kills romeu after each step
  of the promotion commit (01 1.6); every **P** command then finishes
  the promotion or refuses with exit 4, and a re-sync converges.
- **TTY**: `approve`, `adopt` and `tools/ci hygiene add` are driven through a `/dev/ptmx` pair
  created with `x/sys/unix` (test helper; no pty dependency).
- **S8 timing**: `romeu run --timings --json` with the fake sbx, median
  of 5 runs, asserted <= 2 s `romeuMs`; `julieta setup` no-op in the
  container e2e, median of 5, asserted <= 3 s. That second figure is
  wall time against local origins: the fetch of each repo is inside
  it and the network is not, which is the limit S8 states.
- **No network in the preflight**: every **P** command runs in the e2e
  with the registry and origins unreachable after `sync`, and succeeds.
- **Fuzz**: validators, `termsafe`, ref-name checks, YAML round-trip,
  descriptor grammar; short runs in every CI; long runs in the scheduled
  `fuzz.yml`, which calls `go run ./tools/ci fuzz`.

Rules: tests never touch the real `$HOME` (`HOME`, `XDG_*`,
`ROMEU_SETTINGS` in `t.TempDir()`); network only for the container
e2e's mise install (pinned, checksum-verified, cache keyed by the
fixture `mise.lock` sha256; download failures are reported as
infrastructure errors, not test failures), the host suite, and
`tools/ci acceptance`, `tools/ci links` and `tools/ci dora` outside
`all`; flaky tests
are fixed, never skipped.

## 10.2 CI

`.github/workflows/ci.yml` calls `go run ./tools/ci all`, which is also
what developers run locally before a PR; the tracked `.githooks/pre-push`
runs the `fast` subset.

**Every merge gate also runs locally.** A merge gate is a check whose
failure blocks a merge. Workflows hold no check logic of their own, and
`tools/ci workflows` enforces that with a closed grammar over
`.github/workflows/*.yml`. A key or a value outside it fails, and so
does a file in that directory with another name ending:

| Level | Allowed |
|---|---|
| file | the keys `name`, `on`, `permissions`, `concurrency`, `jobs`; `on` names the events `pull_request`, `push`, `schedule`, `workflow_dispatch`, with their filters |
| job | the keys `name`, `runs-on`, `needs`, `strategy`, `permissions`, `timeout-minutes`, `steps`; `runs-on` is `${{ matrix.os }}` or one of the four labels of Runners below, and so is each `os` of the matrix (`runs-on` as a list or a mapping fails); `permissions`, at the top and in a job, is a mapping whose values are `read` or `none`, and the file sets it at the top or in every job, so no scope is left to the default; `strategy` holds `matrix`, `fail-fast` and `max-parallel` only; the matrix is a written mapping with the keys `os` and `include` only, each `include` entry holds `os` and `mise_sha256` only (names matched exactly), and a `${{` in the value of an `include` entry fails (the `os` value is held to the four labels); with `runs-on: ${{ matrix.os }}` every runner of the matrix names an `os` |
| `uses` step | the keys `name`, `uses`, `with`; `uses` is `<owner>/<repo>[/<path>]@<40 hex digits>`, a commit SHA, the owner and the repository start with a letter or a digit, and so does each path segment or it starts with `_`, so no `.` or `..` segment, no `./` path of the repository (a local action) and no `docker://` image fits; `with` is a mapping that holds only the inputs listed for that action in `tools/ci` (the action is matched without case), each value a literal or one `${{ matrix.<key> }}` alone whose values are all literals; an `actions/checkout` step sets `persist-credentials: false`, written exactly so (the case of the action name is ignored, the case of the value is not) |
| `run` step | the keys `name`, `run`; `run` is one line, `go run ./tools/ci <subcommand> [<argument>...]` or `go run ./tools/release <subcommand> [<argument>...]`; each word is made of ASCII letters, digits and `._/=:-`, or is `"$NAME"` |

So a workflow has no `if`, no `continue-on-error`, no `shell`, no
`container`, no job that calls another workflow, and no `${{ }}`
expression inside a `run` line: none of them is in the grammar. Where
a value may hold an expression (`runs-on`, `with`,
`concurrency`, a `strategy` matrix), the text between the braces is
one context path: names of ASCII letters, digits, `_` and `-`, joined
by dots (`matrix.os`, `github.ref_name`). An operator, a function call
or a literal there fails, and so does `github.token` and any
`secrets.<name>` (the list of allowed secrets is empty; a secret joins
it with the operator's approval), whatever the case. An anchor, an
alias and a tag other than `!!str` on a key fail. No level holds `env`: its keys could make a
`run` step start other code (`BASH_ENV`, `LD_PRELOAD`, `PATH`,
`GOFLAGS=-toolexec`). A value a command needs from the event reaches
it as a variable the runner sets, a `"$NAME"` word such as
`"$GITHUB_REF_NAME"`. A
workflow and a developer's shell therefore run the same code at the
same commit. `tools/ci` runs `reuse` from a container image pinned by digest (the
license row), and runs `govulncheck`, `golangci-lint` and the go
command by the paths `mise which` resolves, never through `mise exec`:
with the tool not installed, `mise exec` warned and ran a program of the
search path, exit status 0 (measured with mise 2026.10.3). A path must
lie below the install directory of the tool, in the directory of the
version `mise.lock` locks (a tool locked at two different versions is an error; the same version twice is not), so the
versions locked in `mise.lock` are the ones used in both places.

Three consequences of the grammar:

- A step that applies to some runners only (the hybrid and container
  e2e on Linux, the I27 hostile trees on macOS) is selected inside
  `tools/ci`, from the OS and architecture it runs on: there is no `if`
  to select it in the workflow.
- `ci.yml` lists `edited` among its `pull_request` types, so a change
  to a PR's title or body runs `pr` again; `tools/ci workflows` fails a
  `ci.yml` without it.
- The grammar bounds keys, `run` lines, runner labels, the inputs of
  each action and expressions, not each value: the owner of a `uses`
  action, the filters under `on` and the values under `strategy` other
  than `matrix` are free. They are reviewed, not checked:
  `.github/workflows/**` is an ask-first surface (05 5.3).

`tools/ci all` runs the `pr` step when `GITHUB_EVENT_NAME` is
`pull_request`, on the file that `GITHUB_EVENT_PATH` names. A local run
is `go run ./tools/ci pr <file>`, on a payload saved beforehand (with
`gh api`, for example); `pr` itself makes no network call. The file is
the pull request object, or the event whose `pull_request` member is
that object, and `pr` reads seven fields of the object: `title`, `body`,
`head.ref`, `head.sha`, `base.sha`, `base.ref` and
`base.repo.default_branch`. A file with neither shape, or without one
of the seven, fails. The rest comes from git: the commits
reachable from `head.sha` and not from `base.sha`, read by the walk of
[Forbidden names](#forbidden-names). The changed paths, which decide
the approval lines, are the paths that differ between the merge base
of `base.sha` and `head.sha`, and `head.sha` itself, both names of a
rename included. So a merge of the default branch into the pull
request brings in no surface the pull request did not change.

GitHub Actions is the default runner. If the Actions quota runs out,
the maintainer may choose the local gates instead, provided they are
equivalent: the same subcommands at the same commit, on each OS and
architecture of the runner list below. A platform without a run means
the local gates are not equivalent and do not stand in. Build
provenance is the one thing a local run cannot produce, because the
attestation is signed with the Actions runner's identity: local gates
qualify a merge, and a release still needs Actions (S1, S2). How a
local-gate run is recorded is a
[deferred decision](../spec.md#deferred-decisions).

| Step | Command | In `fast` |
|---|---|---|
| format, vet | `gofmt -l` empty; `go vet ./...`, each the pinned tool, in the environment of the steps of `fast` (below) | yes |
| generated | `tools/ci generated`: `go generate ./...` changes no consumer of the table in [12 12.3](12-engineering.md#123-generators). It compares the working tree, tracked and untracked files by content, before and after the run, so a changed, a new and a removed generated file each fail and a tree with uncommitted work can be checked. A generated file that exists and is not tracked passes locally and fails in CI, whose checkout holds no untracked file | yes |
| lint | `golangci-lint run` over the whole module, `tools/` included; `.golangci.yml` enables the doc-comment, error-string and commented-out-code checkers of ADR 0001 (rule 14) and testifylint with `enable-all: true` (10.6), disables no linter and excludes no finding. A fixture test proves that each rule 14 checker and each testifylint checker that applies outside suites still reports (10.1), and tests read `.golangci.yml`, `go.mod`, the mise configuration files and the repository's Go files, tracked, not yet added and ignored, and fail on: a key of `.golangci.yml` outside a short list (so a `default` or `enable-all` key under `linters`, an exclusion or a disabled linter fails, while testifylint's `enable-all: true` is required), a second `.golangci.*` file, a directive outside the allowlist, which is `go:build` and `go:generate` (the one directive of the `generate` subcommand; the `tool:name` form of `isDirective` in `go/ast`, read in every line of every comment after the leading `/`, `*` and `unicode.IsSpace` runes are trimmed, golangci-lint's `nolint` word at the start of such a line, and the `line`, `extern` and `export` directives right after `//` or `/*`), a generated-code header, a file that builds with cgo off for none of the lint targets (build constraints with the tool tags the pinned go command reports for each target, file-name suffixes and an import of `"C"`, as `go/build` and `go/parser` read them), a file under `testdata`, `vendor`, a `_` or `.` directory or a nested `go.mod`, which `go list` skips, a `go.mod` directive other than `module`, `go` and `require` (an `ignore` takes a directory out of `./...`), a mise configuration table other than `[settings]` and `[tools]`, a setting other than `lockfile` (an `[env]` table among them), a tool in `[tools]` other than `go`, `golangci-lint` and `govulncheck` (the key `"go:golang.org/x/vuln/cmd/govulncheck"`), or a value other than the exact version `mise.lock` locks for it (a `path:` tool among them), and a `.tool-versions` file, which mise reads too. revive's `exported` reads only importable packages, so it skips package `main` and `_test.go` files (`File.IsImportable`, revive v1.17.0) and checks no doc comment in `tools/ci` today; no setting of the rule extends it to them. The linter sees only the files that build for the GOOS and GOARCH it runs under, so `tools/ci` starts it once for each lint target (linux and darwin, each on amd64 and arm64, one list that the tests read too), each as `<path> run --config .golangci.yml`, with the path of `golangci-lint` that `mise which` resolves and not through `mise exec`, which would add a mise `[env]` table, in the environment of the steps of `fast` (below) with the target's `GOOS` and `GOARCH` and `CGO_ENABLED=0` added; `mise which` installs nothing and fails when a tool is missing, so a new machine runs `go run ./tools/ci setup` first, which runs `mise trust`, `mise install` and `go mod download` | yes |
| unit | `go test -count=1 ./internal/... ./tools/...`, the pinned go command, in the environment of the steps of `fast` (below); `-count=1` because tests that scan the repository would otherwise pass from the test cache | yes |
| hygiene | `tools/ci hygiene`: no U+2014; the prose rules of ADR 0001 (rule 4); the `TODO(#<issue>)` form in Go files (rule 14); no personal absolute path (below); no tracked `go.work`, `go.work.sum` or `vendor/`; no tracked file named `PROLOGUE.md`, at any depth and in any case, because the user's agreement file lives outside product repositories; `.githooks/` holds exactly `pre-push`, tracked with mode 100755; the forbidden-name check over the path and content of each tracked file, which fails on a denylist that is missing or has no entry (below); scans `e2e/testdata/sbx/**` too | yes |
| pushed range | `tools/ci fast` with the pre-push hook's arguments: it first refuses a push it would not test as sent, then runs the forbidden-name check over the remote ref names and the commits of a push (below), and refuses a commit that adds or renames to a file named `PROLOGUE.md` even when a later commit removes it, with or without a denylist entry | in the hook only |
| sequences | `tools/ci sequences`: ADR numbers contiguous and unique; ADR layout and statuses per ADR 0001 (rules 6-7), the filename compared through the `slug` function `tools/new adr` uses; every `Supersedes` link in an ADR's Status section matches a `Superseded by` link in the target ADR and the reverse; every ADR that `docs/spec.md` or `docs/spec/` cites has status Accepted; every row of the index's Deferred decisions table has its three cells filled ([ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)); the ids below unique, and every referenced id and id range (for example "J1-J13" in a success criterion) defined | yes |
| vocabulary | `tools/ci vocabulary` (01 1.7) | yes |
| workflows | `tools/ci workflows`: the grammar above | yes |
| vulnerabilities | `govulncheck ./...`; it reads the Go vulnerability database over the network, so its result depends on the date | |
| unit + golden + race | `go test -race -count=1 -coverprofile=cover.out ./...`, with `cover.out` in a temporary directory, so no run leaves it in the working tree | |
| coverage | `tools/ci coverage` (S9), per package: each package below its threshold is a finding | |
| golden governance | `tools/ci golden`: fails if `-update` appears in CI invocations; lists every changed golden in the job summary for review | |
| schema | `tools/ci schema` | |
| e2e | `go test -tags e2e ./e2e/...` | |
| cross-build | darwin amd64/arm64 (`romeu`), linux amd64/arm64 (`julieta`) | |
| imports | `tools/ci imports` (I1, I2, I11, I24, the callers of the spool reader for I32, 10.7) | |
| invariants | `tools/ci invariants` (the guard and test tags found in the code pair up, and each tagged id is a row of 05 5.2; that each row has both is checked by `acceptance`, 10.5) | |
| mutation | `tools/ci mutate` on PRs touching a `.go` file on a path in `.github/ask-first.yaml`; the scheduled `fuzz.yml` runs it too | |
| catalog | `tools/ci catalog` (explicit upload flags, key syntax) | |
| kits | `tools/ci kits` (one frontend pin; install steps <= 5 lines; every download has a sha256: a download is a step line whose first word is `curl` or `wget`, and the same step has a `sha256sum -c` or `shasum -a 256 -c` line after it; a download line holds no `\|`, `;`, `&&` or `$(`, and the file `-c` reads is a kit file, not one the step downloaded. A download written with another tool is not seen by the check; the review of the `kits` surface covers it) | |
| mise | `tools/ci mise` (`julieta lock --check` logic on this repo's and the examples' locks) | |
| probes | `tools/ci probes` (11 11.4: the results that are committed, and the A5 golden hashes; that every probe has a result is checked by `acceptance`, 10.5) | |
| license | `reuse lint` (REUSE 3.3), from the image `fsfe/reuse:6.2.0` pinned by digest in `tools/ci`, with the working tree and the git directory (`git rev-parse --git-common-dir`) mounted read-only, each at its own path, and no network for the container, so that a linked worktree, whose git directory lies outside the tree, ignores the same files as a clone; on Linux only, since the check reads file content alone and the image is built for Linux. On macOS the output of `all` shows `--    license: not run on darwin`, neither `ok` nor `FAIL` | |
| docs | `tools/ci docs` (S10; the checks ADR 0001 assigns to it: rules 1, 5 without external URLs, 9 for the title at a first mention, 12, 13, 17, 20). The Markdown lint and the spell check of rule 5 check nothing until their tools are picked, a [deferred decision](../spec.md#deferred-decisions). The reference pages are compared with their source by `generated`, not here | |
| lessons | `tools/ci lessons`: every `docs/lessons.md` entry, read after the file's front matter, names an existing `tools/ci` subcommand or test name, or says "no check possible: <reason>" | |
| pr | `tools/ci pr` (pull requests only; inputs as described above; 12 12.4; ADR 0001 rules 4, 8, 11; the base branch, 12 12.9; the fix marker's form, 12 12.10; the forbidden-name check, below) | |

The command steps of `fast` (format, vet, lint, unit) run the pinned
tools by their paths: `gofmt` and the go command from the directory of
the go command that `mise which` resolves, and the `golangci-lint` it
resolves. `tools/ci` refuses a path that, once its symbolic links are
resolved, is not below the directory where mise installs that tool, so
a mise configuration that names a tool by a path fails the run. Each
step runs in an environment built from nothing, not from the caller's:
the variables that say where things are (`PATH`, with that go
directory first, `HOME`, `TMPDIR`, the XDG and mise directories,
`GOPATH`, `GOCACHE` and `GOMODCACHE`), then `GOENV=off`,
`GOTOOLCHAIN=local`, `GOWORK=off`, `GOPROXY=off` and
`GOFLAGS=-mod=readonly`. So no `GOFLAGS` of the caller (a `-run` that
selects no test, or build tags), `go env -w` file or other variable of
the caller changes what a step checks. Some unit
tests need what the steps need: mise on the search path with the
pinned tools installed and this repository trusted, a module cache
that already holds the modules of `go.sum` (no step fetches a module,
since the steps run with `GOPROXY=off`; `go mod download` fills the
cache, and `go run ./tools/ci setup` runs it), and the `.git` directory of a
clone, because they list the repository's files with `git ls-files`.

A personal absolute path, for `hygiene`, is `/Users/<name>/` or
`/home/<name>/` where `<name>` is one path segment other than `agent`
(the sandbox user in this spec's examples) and is not written as a
`<placeholder>`.

The ids `sequences` reads, in `docs/spec.md` and `docs/spec/`:

| Id | Form | Defined by |
|---|---|---|
| invariant | `I<n>` | the first cell of a row in [05 5.2](05-security.md#52-invariants-and-their-tests) |
| success criterion | `S<n>` | the first cell of a row in the index's criteria table |
| question | `Q<n>` | the first cell of a row in the index's Open questions |
| journey, variant | `J<n>`, `J<n><lowercase letter>` | a `## J<n>` heading in [09](09-journeys.md); a variant is defined where that heading names it |
| probe | `A<n>`, `B<n>`, `C<n>` | the first cell of a row in a table of [11](11-host-probes.md) |
| section | `<n>.<m>` | a `## <n>.<m>` heading of file `<n>` |

A reference is another occurrence of an id as a whole word, inside a
code span too; `X<a>-X<b>` stands for each id from a to b. A section is
referenced only as a link anchor, which the link check of `tools/ci
docs` resolves. Uniqueness and existence are checked; only ADR numbers
must be contiguous.

Runners: `ubuntu-26.04` (x64), `ubuntu-26.04-arm` (arm64), `macos-26`
(arm64) and `macos-26-intel` (x64). The arm64 and Intel labels are
verified to exist, and every label to report, by `uname -m`, as `aarch64`
(`ubuntu-26.04-arm`), `arm64` (`macos-26`) and `x86_64` (`ubuntu-26.04`
and `macos-26-intel`), on the first run of `ci.yml` (T002). Labels are
pinned, never `*-latest`: `tools/ci workflows` fails a `runs-on` or a
matrix `os` that is not one of the four. `tools/ci all` reports the
architecture of the Go binary next to `uname -m`, and fails when they
differ (`x86_64` is `amd64`; `aarch64` and `arm64` are `arm64`). On macOS it also reads `sysctl -n sysctl.proc_translated`, which `uname -m` cannot show (a binary under Rosetta sees the architecture it emulates), and fails when it is `1`. They are bumped deliberately; the images are listed at
<https://github.com/actions/runner-images>.

Four subcommands are outside `all`:

- `tools/ci acceptance` needs the network (GitHub API). It runs in the
  final plan task, after the v1.0.0 release (10.5), and in no
  workflow; its unit tests use a recorded API fixture.
- `tools/ci links` needs the network too: it fetches every external URL
  the docs cite (ADR 0001 rule 5). No workflow calls it in v1; a person
  runs it, and the final plan task runs it once, next to
  `tools/ci acceptance`. Relative links and anchors stay in
  `tools/ci docs`, offline.
- `tools/ci fuzz` runs each fuzz target for a fixed time, longer than
  the short runs inside `go test`. The scheduled `fuzz.yml` calls it,
  and then `tools/ci mutate`.
- `tools/ci dora` needs the network (GitHub API): it computes the
  delivery metrics of [12 12.10](12-engineering.md#1210-delivery-metrics).
  It is a report and no merge gate: nothing fails on a number.
  `release.yml` calls it, anyone runs it by hand, and its unit tests
  use a recorded API fixture.

### Forbidden names

`tools/ci/denylist.yaml` lists names this repository must not contain:
in a file, a path, a commit message, a commit's author or committer
identity, a ref name or the text of a PR.
This page does not write them and calls them the denylist entries.
There is one definition of a match and one function that applies it;
`hygiene`, `fast` and `pr` all call that function.

- An entry is an ordered list of one or more segments. A segment is a
  length and the sha256 of a piece of the name, with ASCII letters
  lowercased. The name is not in the file.
- A line matches an entry when, with its ASCII letters lowercased, it
  contains the segments in order, and between two consecutive segments
  there are zero or more characters that are neither ASCII letters,
  ASCII digits nor a slash. The matcher hashes each substring of the
  first segment's length; on a hit it skips such characters and hashes
  the next segment's length, and so on. A line matches when at least
  one start position leads to all the segments in order: after a hit
  whose continuation fails, the matcher goes on from the next
  position.
- So an entry with one segment matches as a substring, inside a path or
  a longer word too. An entry with two segments matches its halves
  written together, or joined by a hyphen, an underscore, a dot or
  spaces. It does not match them joined by a slash: that shape is a
  registry or repository path of another product, such as the upstream
  workload image of [06](06-kits.md), and not the name.
- A two-segment entry also matches two unrelated words that happen to
  be its halves and stand next to each other; such a line is reworded.
- Text is matched one line at a time. A ref name, a path, and a name
  with its email are each one line.
- A hit is reported by its location: the file and line, the commit and
  the field, or the PR field. The matched text and its line are not
  printed, because CI logs are public and the output is pasted under
  Evidence. A path that is printed is quoted, in Go's ASCII-quoted
  form, when it is not valid UTF-8 or holds a character that does not
  print; a withheld path stays withheld.

| Surface | Checked by | When |
|---|---|---|
| the path and the content of each tracked file at HEAD | `tools/ci hygiene` | in `fast` and in `all` |
| the name each pushed ref gets on the remote; of each pushed annotated tag: the name it was created with, its tagger and its message; of each pushed commit: the message, the author and committer names and emails, the paths it adds or renames to, the lines it adds, and the name, tagger and message of each tag it embeds in a `mergetag` header | `tools/ci fast`, called by the pre-push hook | before the push leaves the machine |
| the PR title, body and head ref name; the same four readings of each commit in base..head | `tools/ci pr` | on the pull request |

The hook is `exec go run ./tools/ci fast "$@"`. Git gives a pre-push
hook the remote's name and URL as arguments and, on stdin, one line per
pushed ref: local ref, local sha, remote ref, remote sha. Called with
those arguments, `fast` reads the lines, checks the remote ref name of
each, and walks the commits reachable from the local sha and not from
the remote sha. For a ref the remote does not have yet, or a remote
sha that the local repository does not hold, it walks the commits not
reachable from any remote-tracking ref of that remote. A push to a URL
that is no configured remote has no such ref, and `fast` stops it
before the walk: the default branch whose denylist judges the push
(below) is not known. A merge commit
is compared with its first parent. A line that deletes a ref adds no
commits and is skipped, name included, so a ref with a forbidden name
can be deleted. A git command that fails stops the push, and so does a
pushed ref that leads to no commit, such as a tag on a blob or a tree:
both end with exit status 2. The objects are read as the push sends
them: replace refs are ignored, and grafts (`info/grafts`) are
honoured by the reader and by `git push` alike, so the two agree.
Called without
arguments, `fast` runs the `In fast` steps and no range. `pr` uses the
same walk over base..head.

The steps run on the working tree, and the review sees only commits,
so with the hook's arguments `fast` judges the commit the push sends,
and refuses to run any check, with exit status 2, when:

- a pushed tip that does not delete a ref, followed through an
  annotated tag to its commit, is not HEAD: the operator checks out
  the ref and pushes again;
- a tracked file differs from HEAD: staged, added with `git add -N`, or
  modified in the working tree;
- an untracked or ignored file is one the go command or the checks
  read: a Go file, `go.work`, `go.work.sum`, a root `vendor`
  directory, a mise configuration, a `.tool-versions` file, or a nested
  repository.

The fix is to commit such a change or remove it. The steps take
minutes, and the working tree can change while they run, so after the
last step `fast` judges it again, the `go.mod` rule below included, and
stops the push with exit status 2 when HEAD has moved or one of these
refusals now holds. Hygiene and the range then read the commit judged,
by its id, and not HEAD again. The git commands of
this check run without `GIT_DIR`, `GIT_INDEX_FILE`, `GIT_WORK_TREE`,
`GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, the other
variables of `git rev-parse --local-env-vars` and every `GIT_CONFIG`
variable, so they read the repository's own index, and with
`GIT_OPTIONAL_LOCKS=0`, so `git status` writes nothing. Called without
arguments, `fast` refuses none of this and checks the working tree as it
is. In both modes it refuses a `go.mod` below the module root, tracked
or not.
Reading what each commit adds, and not only the final tree, is what
catches a line or a path that one commit adds and a later commit
removes.

A push is judged by the union of two denylists: the one at HEAD, the
commit judged, and the one at the remote's default branch as this
clone knows it, the commit of `refs/remotes/<remote>/HEAD`, which
`git fetch` creates and `git remote set-head <remote> --auto` sets.
An entry that the pushed commit removes therefore stays in force until
its removal reaches the default branch through review. A push to no
configured remote, a missing `refs/remotes/<remote>/HEAD` and a
denylist there that cannot be read stop the push with exit status 2,
and the message says to run `git fetch <remote>`, or, for a remote with
no commit yet, that no fetch can succeed. In a brand-new repository,
with no default branch yet, every hooked push therefore fails closed
until the default branch exists on the remote and is fetched here, and
the operator makes the first push of a new repository with
`--no-verify`. A default branch
that holds no denylist adds nothing, since the denylist itself arrives
there by a pull request. A finding says which of the two lists holds
the name. Entries that exist only at a commit inside the range, and
neither at HEAD nor at the default branch, are not applied. The local
refs and the git binary are trusted, as the toolchain is
([ADR 0007, Threat model](../adr/0007-adopt-testify-assert-and-require-in-tests.md#threat-model)).
The hook knows only the denylist this clone holds: an entry pushed
from another clone applies once it is fetched here.

The limits, stated plainly:

- Outside a sandbox the hook is opt-in (12 12.4), and
  `git push --no-verify` skips it. `pr` is the backstop, and it runs
  after the push: a name pushed without the hook is on the remote
  before any check reads it. A branch pushed that way and not opened as
  a PR is read by no check.
- Matching works on bytes with ASCII lowercasing. It does not match a
  name written with look-alike characters of another script, with a
  zero-width character inside it, split across two lines, written as an
  escape or an entity, or encoded (base64, a UTF-16 file).
- Git notes, issues and review comments are not read: no check sees
  their text. Of a pushed annotated tag the hook reads the name it was
  created with, its tagger and its message, signature lines included;
  `pr` reads no tag ref. A tag that a merge commit embeds in a
  `mergetag` header, as `git merge` of a signed tag does, is read the
  same way, by the hook and by `pr`. The other headers of a commit are
  not read: its signature, `encoding`, and any extra header. Nor are
  the headers of a tag other than `tag` and `tagger`. Git writes the
  `author`, `committer`, `object`, `tag` and `tagger` headers once
  each, so an object that repeats one stops the push. An identity is
  read up to its email when git's date and zone follow it, and whole
  otherwise.
- An identity is matched as a line, so a contributor whose name or
  email holds an entry as a substring cannot commit under that
  identity. That is a cost of the check, not an oversight.
- `hygiene` proves that the denylist has an entry, not that it has the
  right ones.

An entry is written by `go run ./tools/ci hygiene add`. The maintainer
runs it on the host, in a plain clone and a terminal outside any
sandbox and any agent session, because an agent must not hold the
names, and a process inside a sandbox shares the agent's user. It reads the name from the terminal with echo off,
one line with a space between segments, and refuses to start when stdin
is not a terminal; it takes no name as an argument and prints none, so
the name is in no argument list and no shell history. It refuses a
segment that holds a character other than an ASCII letter or digit, so
no segment starts or ends with a separator. It lowercases the name,
writes the
segments through the matcher's own code, and before writing proves that
the matcher finds the name in a buffer in memory. A hand-computed entry
that is wrong matches nothing, and no test can see that, because the
name is not in the tree; that is why the command exists. The self-test
uses a made-up name with its own denylist: it plants the name written
together, with each separator and with a slash, and expects a failure
for each form except the slash.

What the hash buys is small, and we say so. It is a plain hash, not a
keyed one, and each segment's length is in the file, so a short segment
is recovered by trying each string of that length, in seconds on a
laptop. The hash only keeps the plaintext out of the tree, so that a
search of the repository does not find it. The maintainer decided on
the plain hash knowing this
([round 3](../reviews/round-3.md#maintainer-decisions)).

### Release and bootstrap

`release.yml` on tag `v*` runs `tools/ci all` on each runner. Then
`tools/release build` builds the julieta linux binaries, embeds them,
the kits and the catalog into romeu, builds romeu for darwin, and
writes the archives and `checksums.txt`;
`actions/attest-build-provenance` attests them; `tools/release verify`
**verifies** each archive with
`gh attestation verify --signer-workflow .../release.yml`, starting
`gh` through `mise exec` (12 12.1);
`tools/release notes` generates the release
notes (12 12.6); `tools/release publish` creates the GitHub release,
published and never a draft, and marks it a prerelease when the tag
has a suffix after the patch number (`v1.0.0-rc.1`);
`tools/ci dora --attach "$GITHUB_REF_NAME"` adds the delivery metrics to it
(12 12.10), with the tag read from the runner's `"$GITHUB_REF_NAME"`.
The attestation is a `uses` step and each of the others is one `run`
step, so the file passes `tools/ci workflows`. `tools/ci acceptance`
is not among them: its evidence names the release and the run of the
tagged commit, which exist only when this workflow has ended (10.5).
A tag of the form `v<major>.<minor>.<patch>` is a release; the same
with a suffix is a prerelease, which is what the release candidate of
block B is ([11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)).

The sandbox's token (the operator's) can push a `v*` tag
([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)), so
the pull request that adds `release.yml` decides how releases come from
`main` only, and one constraint is fixed now: the release workflow file
must not come from the tagged commit
([Deferred decisions](../spec.md#deferred-decisions)). Releasing from
`main` only blocks a stray tag and not the token, which merges through
the API, so a second rule is fixed now: the credential that signs
release artifacts is held outside GitHub and out of the sandbox's reach,
signing is an operator step, and `release.yml` may not hold that
credential; the attestation above is GitHub's keyless provenance record,
an extra record and not the signing credential
([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1), "Release
signing").

`ci-bootstrap` (first plan task), in this order:

1. One PR adds `tools/ci hygiene` with `tools/ci/denylist.yaml`,
   `tools/ci fast` and `.githooks/pre-push`. Its author can neither
   write the denylist entries nor push it past a hook that is not
   enabled yet, so the maintainer block below finishes and pushes it.
2. `tools/ci workflows` and a green workflow on the public repo,
   proving runners, labels and permissions.
3. The day-one middleware of
   [12 12.3](12-engineering.md#123-generators) and
   [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change):
   `tools/new adr`, the `go generate` wiring with `tools/ci generated`
   (the ADR index and CODEOWNERS are its first two generators),
   `tools/ci sequences`, `tools/ci pr` and `.github/ask-first.yaml`.
   `pr` lands whole, the form check of a `Fixes-release:` trailer
   included, so a trailer written before the metrics tool exists is
   well formed. Each other `tools/new` kind and generator row lands in
   the first PR of the module that owns its first input, and
   `tools/ci dora` lands with `ci-release` (12 12.2).

**The maintainer block.** The parts of `ci-bootstrap` that only the
maintainer can do are one sitting around step 1 and one call after
step 2. The maintainer runs every item. The output of each goes under
the Evidence of the PR it belongs to; no check reads that text, so it
is **[review]**.

The sitting starts with item a, before the sandbox that writes step 1
receives its token, or at the latest before item b: until the rulesets
exist, nothing refuses that token a push to the default branch, a push
of a `v*` tag or a merge. Once they exist, of the four tries only the
push is refused (item d), a control against an agent that does not edit the ruleset or the repository settings (05 5.4). Nothing of step 1 is pushed before the
sitting. The agent that wrote step 1 does not push its branch. The
maintainer fetches the branch from the sandbox into a plain clone on
the host and reads its diff there before checking it out, because
items b and c run code and a hook of that branch on the host, outside
any sandbox and any agent session.

- a. Set the repository up and save the API's answer for each setting.
  Two rulesets. On the default branch: changes arrive by pull request,
  with one approving review and a code-owner review; an approval is
  dismissed when a new commit is pushed, and the most recent
  reviewable push must be approved; a merge commit is the only merge
  method; force pushes and deletion are refused; updates to the
  default branch are restricted to the bypass actor, so a merge by any
  other account is refused whether or not it is approved (a GitHub
  behaviour that item d measured against the sandbox's token: it does
  not hold for a merge through the API,
  [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)). On
  tags matching `v*`: creation, update and deletion are refused for
  everyone except the bypass actor. The administrator role is the
  bypass actor of both, and the operator's token holds that role. Two repository settings, which bind a
  bypass actor too: squash merging and rebase merging off, so every
  merge is a merge commit (12 12.9); and private vulnerability
  reporting on, the channel `SECURITY.md` names (12 12.8). If GitHub
  refuses a branch ruleset on a repository whose default branch does
  not exist yet (not tested against GitHub), the maintainer creates
  the repository with an empty initial commit first.

  As measured on 2026-10-08, the default-branch ruleset requires no
  review: no approving review, no code-owner review, no approval of
  the most recent push, and a push dismisses nothing; that requirement
  was removed. The rest of this item is as measured when it was set
  up. The rules that lean on the code-owner review (05 5.4, 12 12.4,
  12 12.9, ADR 0001 rule 8) stand as written until the approval
  decision in the [Deferred decisions](../spec.md#deferred-decisions)
  table is made.
- b. In that clone, run `go run ./tools/ci hygiene add` once for each
  forbidden name, and commit `tools/ci/denylist.yaml`.
- c. Enable the hook in that clone (`git config core.hooksPath
  .githooks`), push the branch, so the hook reads the bootstrap
  commits themselves (their messages, identities, paths and ref name),
  and open step 1's PR.
- d. From inside the sandbox, where the one credential is the
  operator's token, try four things with throwaway payloads and read
  what each does. Each push is tried from a clean working tree with HEAD
  at the commit it pushes, so that `fast` passes and a refusal comes from
  GitHub, not from the hook. The result is a measurement and not a gate: the
  sandbox acts with the operator's account, and
  [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1) states
  the rule once. After the four tries the maintainer checks from their
  own session what took effect: the default branch head, the tag list,
  the throwaway pull request, and each ruleset's JSON against the answer
  saved in item a, whatever the sandbox printed. The maintainer undoes
  what can be undone (the tag deleted, the merge reverted, the ruleset
  restored; an empty commit stays and harms nothing), and then closes
  the throwaway pull request and deletes its branch. Measured on
  2026-10-07 ([ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md));
  the evidence is kept outside this repository:

  | Try | Result |
  |---|---|
  | push an empty commit to the default branch | refused; a control only against an agent that does not edit the ruleset or the repository settings (an edit is inferred, not measured) |
  | merge a throwaway pull request through the API, without a bypass request | accepted |
  | create and delete the tag `v0.0.0-try` | accepted, through the bypass |
  | update the default-branch ruleset with its own unchanged body | accepted; the ruleset did not change |

  Of the four tries, only the direct push was refused.
- e. After step 2: add the CI jobs of the green run to the
  default-branch ruleset as required status checks, and save the API's
  answer.

Between steps 1 and 3 the hook is the one check on commit messages,
identities and ref names, and julieta's dispatcher does not exist yet,
so the hook is opt-in for each clone of that period. Nothing checks PR
text or the approval line in that period, the line for the `checks`
surface included; the maintainer's review of those PRs is the guard.
If Actions cannot run, the local gates above stand in, and the
criterion's evidence in `docs/acceptance.json` is the recorded local
run, marked interim; S2 still requires a real green run for release.

## 10.3 Fake sbx fidelity contract

| Rule | Detail |
|---|---|
| Source of truth | block A records real `sbx <cmd> --help` output and every argv/stdout/stderr/exit the probes produce into `e2e/testdata/sbx/<sbx-version>/*.jsonl`, as ordered sessions per scenario |
| Redaction | the recorder replaces the home path, user name, host name and root path with placeholders, and each U+2014 with `-`, so recorded help text passes `hygiene` and rule 4 of ADR 0001 still skips nothing (no parser matches on that character); it keeps stdout only for an allowlist of read commands whose output the parsers need; it never records secret or credential output (`sbx secret` values, tokens); a unit test plants a fake token and a home path and asserts neither reaches the file; `tools/ci hygiene` scans the recordings |
| Replay only | `e2e/fakesbx` accepts only argv shapes present in a recording (placeholders for names, paths, domains); an unrecorded shape fails the test with the argv. For `env exec` at the hybrid level, the argv before `--` is matched against the recording and the command after it is forwarded into the julieta container, not replayed |
| Stateful replay | each scenario's sessions replay as a state machine keyed by the prior mutating calls (for example `sbx ls` answers differently after `env run` and after `env rm`), so a call out of the recorded order fails. A scenario names the recorded session it starts from, which is how a sandbox that was removed, stopped or created outside romeu is replayed. On create the fake writes the recorded `remote.sandbox-<name>` stanza into the primary clone's config, as sbx does (01 1.2) |
| Parsers | `sbxdrv` parsers run over every recorded version |
| Shared scenarios | CI and host suites call the same scenario functions in `e2e/scenarios`; only the sbx binary differs |
| Floor bumps | raising the sbx floor requires a new recording set |
| Known argv | taken from recordings, for example `sbx policy rm network --sandbox <s> --resource <host>` |
| Cannot model | VM boot timing, virtiofs semantics, real network policy enforcement, credential injection, kit builds, host-command prompts; these are covered only by the host suite and probes |

## 10.4 Determinism seams and golden governance

- Injected clock, ULID entropy source and version stamp in every package
  that writes time, ids or versions; tests fix all three.
- Golden files change only with `-update` locally; CI never runs it and
  lists changed goldens, and `tools/ci pr` requires them listed under
  the PR's Evidence. A golden, for that check, is a file under a
  directory the Golden row of 10.1 names, and `pr` requires the
  repository path of each changed one, verbatim, in the Evidence
  section. The `sbxenv.yaml` goldens carry the sha256 recorded
  by probe A5; `tools/ci probes` flags a golden whose hash moved since
  its probe (re-probe needed).

## 10.5 Acceptance evidence

`docs/acceptance.json` (schema `acceptance.v1`): one entry per success
criterion (S1-S11) with `evidence[]` items of kind `ci-run`
(repository, run id, job, conclusion), `file` (path, sha256), `command`
(argv, exit, output sha256), `probe` (probe-result id, verdict), `repo`
(full name, expected `archived` flag) or `interim`. The final plan
task commits the file after the v1.0.0 release, so the S1 and S2 items
name the release and the CI run of the tagged commit, which exist by
then. No release workflow reads the file, so a later release does not
verify it again.

`tools/ci acceptance [--file <path>]` verifies each item mechanically:
files exist with the hash; probe verdicts are `pass`; CI runs are
`success` and repo flags match, through the GitHub API (a `ci-run`
names its repository, because S4's run is in the config repo); a
`command` item is run again, its argv from the repository root, and
its exit status and the sha256 of its stdout are compared.

On the product's own file it also checks four things that are
complete only at the end of the plan. Checking them on each PR would
keep `all` red while the plan is under way, so they are checked here,
once:

- every row of 05 5.2 has a guard tag and a test tag (S6);
- every probe named in the "Settled by" column of the index's Open
  questions has a committed result, from both hosts when the probe is
  arch-sensitive (11 11.4);
- every `## J<n>` heading of 09 is named by the `journey:` of at least
  one page under `docs/guide/` (S10);
- each block B result counts for v1.0.0: between the candidate's
  commit, which every block B result records, and the v1.0.0 tag, the
  only paths that differ are under `docs/` or are Markdown files at
  the repository root (11 11.2); results of one host that name
  different candidates fail.

`--file` validates any `acceptance.v1` file, so an operator keeps
their own acceptance file (for example `julieta memory verify` per
repo, `romeu status --json`, repositories that must be archived) in
their config repo and verifies it from a product checkout; the product
names no such repository. With `--file` a `command` item is not run:
that file is written where agents author (01 1.1), and its argv would
run on the machine of whoever verifies it. Its shape is validated and
the output lists it as recorded, not verified. The product's own run
is the final plan task.

## 10.6 Code style

- Standard Go layout; small single-purpose packages; no `utils`.
- Errors carry an id from the error table (`cli.Error{ID: "RJ-301"}`),
  which fixes the exit code; `main` maps them; tests assert ids.
- `context.Context` first for anything that runs a subprocess; every
  subprocess has a timeout.
- No global state except the embedded catalog, kits and binaries.
- No `text/template` for structured output; no `sh -c` anywhere in
  romeu.
- Every lifecycle is a transition table in `internal/state`, never
  scattered conditionals.
- SPDX header in every file; doc comments on every exported identifier;
  one-line "why" comments where a rule exists for security, plus the
  invariant tag.

```go
// SPDX-License-Identifier: GPL-3.0-only

// Fetch copies refs from a sandbox's git daemon into a romeu-owned
// namespace. It never uses a remote name, so remote config (prune,
// tag options) from the repository cannot apply.
//
//romeu:invariant I5 guard
func (g *Git) Fetch(ctx context.Context, repo, url string, dst RefPrefix) error {
	if err := dst.Validate(); err != nil { // allowlisted namespaces only
		return fmt.Errorf("fetch %s: %w", repo, err)
	}
	refspec := "+refs/heads/*:" + string(dst) + "*"
	_, err := g.run(ctx, repo, "fetch", "--no-tags", "--no-prune",
		"--no-recurse-submodules", "--no-write-fetch-head", url, refspec)
	return err
}
```

### Tests

Tests use the `assert` and `require` packages of
`github.com/stretchr/testify`, as package functions, and no other
package of it (no `suite`, `mock` or `http`, no `assert.New`).
testifylint enforces most of what follows with `enable-all: true`. A
unit test, `TestTestStyle`, reads every `_test.go` file and every file
of a package whose name ends in `test` (as `gittest`), and fails on an
import of `suite`, `mock` or `http` and on a reference to `assert.New`,
`require.New` or `reflect.DeepEqual`.

- `require` for a precondition and for every error assertion (`Error`,
  `NoError`, `ErrorIs`, `ErrorAs`, `ErrorContains`): a setup step, a
  value a later line depends on, a length before an index.
- `assert` for an independent check, such as an exit code, an output
  line or a final comparison, so that one run lists every problem.
  Inside a goroutine other than the test's a test uses `assert`,
  because `require` stops the test with `FailNow`, which only the
  goroutine that runs the test may call.
- The expected value comes first: `assert.Equal(t, want, got)`.
- The precise assertion: `Contains` and `NotContains` for text, `Len`
  and `Empty` for size, `True` and `False` for a boolean, `Equal` for a
  comparison. A test has no `reflect.DeepEqual` and no hand-written
  `if got != want`.
- A message only when it adds information: a label for a fixture, or
  the output that explains the failure. A test that logs its output
  on failure keeps doing so.
- A table test gives each case a `t.Run` that does not depend on
  another: `go test -run 'TestName/case'` runs it alone and it passes.
- A lint finding is fixed in the code: no directive outside the
  allowlist of rule 8 of
  [ADR 0007, adopt testify assert and require in tests](../adr/0007-adopt-testify-assert-and-require-in-tests.md)
  (`go:build`, and `go:generate` for the `generate` subcommand), so no `nolint`, `revive:disable`, `lint:ignore`
  or line directive, and no exclusion and no lowered setting in
  `.golangci.yml`.

## 10.7 Module boundaries (enforced by `tools/ci imports`)

| Package | May be imported by | Must not import |
|---|---|---|
| `sbxdrv`, `render`, `gate`, `state`, `egress`, `signing` | `cmd/romeu`, `tools/ci` | `mise`, `hooks`, `layout` |
| `mise`, `hooks`, `layout`, `agent/*`, `memstore/write`, `ledger/emit` | `cmd/julieta` only | `sbxdrv` |
| `gitsafe`, `spec`, `canon`, `catalog`, `oci`, `memstore`, `handoff`, `salvage`, `ledger`, `shquote`, `termsafe`, `cli` | both | `sbxdrv`, `layout` |

Names are packages under `internal/`. The second column says which
binary may link a package, so a package of the last row imports none
of the second: romeu links it. That is why the writer of the memory
store and julieta's emit and drain are packages of their own, and why
`salvage` takes the agent profile as an argument (08 8.5). `oci` is
shared because `julieta pin` resolves and checks digests with the
same guarded client (I28).

## 10.8 Definition of done for implementing agents

The Always / Ask first / Never list in the
[index](../spec.md#boundaries-for-everyone-who-changes-this-repo)
applies. A task is done only when `go run ./tools/ci all` passes, its
invariant tags are in place, the generated files are current, and, for
host-facing behavior, the matching scenario function exists (it runs on
real hosts in block B).
