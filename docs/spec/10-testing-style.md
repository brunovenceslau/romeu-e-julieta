# 10. Testing strategy, CI, code style

Back to [index](../spec.md). Reader: implementers and reviewers of any
module. Type: reference. Tools are code: `tools/*`, `e2e/probes`,
`e2e/fakesbx`, workflows, schemas and kits are held to the same lint,
test, coverage and review rules as `internal/*`.

## 10.1 Test levels

| Level | Location | Runs where | Covers |
|---|---|---|---|
| Unit | `internal/**`, `tools/**`, `e2e/probes/**`, `e2e/fakesbx/**` `_test.go` | CI, dev | validators generated from `rules.go` (table + fuzz seeds committed), digests, widening-set and toolchain diffs, the generated state-machine tables (every row, every illegal pair), egress split, `termsafe`, shell quoter, sbx output parsers over every recorded sbx version, one fixture per decoded upstream shape (the tolerant decoding of [03](03-formats.md), opening), error-id table, doctor checks with `HOME` in a temp dir, each warn and fail row asserted by its slug in the `--json` output and not only by the exit status, the probe harness core and its sbx exec layer (against a helper binary re-executed from the test), the recorder's redaction, the fake sbx's placeholder normalizer, CI tools themselves (the forbidden-name matcher and its pushed-range walk over a fixture repository, `workflows` against fixture workflow files, `docs` against fixture Markdown files with one violation per case of rule 9, the linter configuration against a fixture package with one violation per checker of rule 14 in [ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) and one per testifylint checker that applies outside suites, invariants/mutate, acceptance against a recorded GitHub API fixture, `pins` against one fixture per source, `pr` against recorded event payloads) |
| Golden | `internal/render/testdata/`, `internal/layout/testdata/`, `internal/gate/testdata/`, `internal/handoff/testdata/` | CI, dev | `sbxenv.yaml`, workspace files, `render.json`, gate diff text, herdr `layout.apply` requests (`layout up --dry-run`), handoff front matter, the SessionStart print whole and cut at its bound ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)) |
| Schema | `tools/ci schema` | CI | generated schemas equal the committed ones; examples and testdata validate |
| E2E (git + fake sbx) | `e2e/*_test.go`, tag `e2e` | CI (linux amd64, linux arm64, macOS Intel, macOS arm64) | romeu commands against a temp `$ROMEU_ROOT`; origins served by `git http-backend` behind `httptest` TLS (host settings `gitHosts[].caFile` points at the test CA; gitsafe has no test override); the sandbox daemon served by `git daemon` on `127.0.0.1`; journeys J2, J3b, J6, J7, J10, J11 (scenario functions shared with the host suite), of which a journey that reaches a julieta call runs at the hybrid level below, on the Linux runners, while the macOS runners run the romeu commands that reach none; promotion fault injection; invariants marked E in [05](05-security.md), including the I27 hostile trees on the macOS runners |
| E2E (hybrid) | `e2e/hybrid_test.go`, tag `e2e` | CI on `ubuntu-26.04` and `ubuntu-26.04-arm` | the romeu-julieta contract: the fake sbx forwards `env exec` into the julieta container with `.romeu/bin` bind-mounted read-only at the host path; the journeys that reach a julieta call (julieta has no darwin build, 12 12.1), J6 among them, with `julieta salvage` forwarded through `env exec` and the agent replaced by a fixture process; protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each stop `run` before `layout up`; a 32 KiB manifest round-trips (I30) |
| E2E (container) | `e2e/container_test.go`, tag `e2e` | CI on `ubuntu-26.04` (amd64) and `ubuntu-26.04-arm` (arm64), native, no QEMU | julieta in the workload's Debian base: `setup` (PATH link, dispatcher, secondary clone, ff of default branch) and its no-op timing, `install` skip rule, `lock --check`, hooks dispatcher, chaining and recorded failures, at the pinned mise no request to its versions host and a refused configuration path outside `MISE_TRUSTED_CONFIG_PATHS` ([04 4.3](04-cli.md#43-julieta-sandbox)), memory (stamping, allowlist), handoff (all kinds; SessionEnd and SessionStart in both orders, so the rule of 08 8.3 is tested in the order Q22 has not confirmed too; SessionStart wired for every source the pinned agent fires), snapshot, the salvage completeness cases, which are the one list in the Test cell of I17 ([05 5.2](05-security.md#52-invariants-and-their-tests)), and three more salvage cases: a fixture process with the agent's process name that ignores SIGTERM is killed after the wait and the manifest records the kill; untracked files count against the salvage cap; a nested repository gets the dirty-tree and stash capture of a top-level one; `layout up --dry-run` golden and `layout up` against the pinned herdr |
| Host | `e2e/host/*_test.go`, tag `host` | maintainer machines (Intel and Apple silicon) | the same scenario functions against real sbx for J1-J13; S3, S5, S8 confirmation; results as `probe-result.v1` per block B step (11) |
| Probes | `e2e/probes` | maintainer machines | [11](11-host-probes.md); `probe-result.v1` files in `docs/probes/` |

Level claims per invariant are those in the 05 table: I14, I20, I22
and I23 are unit-level, and I16 is unit-level for its scan and E-level
for the snapshot diff of 05 5.2.

Additional required tests:

- **Idempotency meta-test**: every converging romeu and julieta command
  is run twice on unchanged inputs, and the second run must leave
  every file's content and mode and every ref as they were, except
  `lastRun`. The diff compares content and mode, not modification
  times, so a rewrite with the same bytes passes. The test reads which
  commands converge and which append from the side-effect fields of
  the command definitions ([12 12.3](12-engineering.md#123-generators)),
  and for each appending command it asserts that the second run adds
  the one thing the definition names and nothing else.
- **Concurrency**: two romeu processes (flock; second exits 1 with
  `RJ-101`); two julieta processes writing memory (store lock; no lost
  entry); snapshot racing a commit (bundle always valid); a commit
  racing salvage's capture (the worktree is captured again once, then
  skipped as `worktree-changing`, 08 8.5); `romeu pull` succeeds while
  a `run` of the same project is attached, because `run` releases the
  lock before its final `exec` (04 4.1).
- **Promotion fault injection**: a test hook kills romeu after each step
  of the promotion commit (01 1.6); every **P** command then finishes
  the promotion or refuses with exit 4, and a re-sync converges. A
  candidate that no longer verifies after the kill restores the
  previous digests and reports `interrupted-promotion-abandoned`
  (exit 4), never drift.
- **Destructive-command fault injection**: the case of I17's Test cell
  ([05 5.2](05-security.md#52-invariants-and-their-tests)): a test hook
  kills romeu after each step of `salvage`, `rm`, `recreate` and
  `retire`, and the next command converges.
- **Error ids**: every id of [04 4.4](04-cli.md#44-error-ids) that is not
  retired is asserted by at least one test; an id that cannot be is in
  a committed list of exemptions, each with its reason.
- **Git keys**: one test holds the plant list of I3 and the list of
  host git keys `doctor` warns on equal to `gitsafe`'s list of
  neutralized keys, all three read from one Go slice.
- **TTY**: `approve`, `adopt` and `tools/ci hygiene add` are driven through a `/dev/ptmx` pair
  created with `x/sys/unix` (test helper; no pty dependency).
- **S8 timing**: `julieta setup` no-op in the container e2e, median of
  5, asserted <= 3 s. That figure is wall time against local origins:
  the fetch of each repo is inside it and the network is not, which is
  the limit S8 states. romeu's share of `romeu run` is measured on real
  hosts by B5 ([11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)),
  not in CI.
- **No network in the preflight**: every **P** command runs in the e2e
  with the registry and origins unreachable after `sync`, and succeeds.
- **Fuzz**: validators, `termsafe`, ref-name checks, YAML round-trip,
  descriptor grammar, and every strict decoder of a document read from
  a sandbox or a mount (julieta's `--json` output read over
  `sbx env exec`, `snapshot/heads.json`, the memory entry, the handoff
  file and the `mise.lock` parser); short runs in every CI; long runs in
  the scheduled `fuzz.yml`, which calls `go run ./tools/ci fuzz`.

Rules:

- Tests never touch the real `$HOME` (`HOME`, `XDG_*`,
  `ROMEU_SETTINGS` in `t.TempDir()`).
- The network is read by the container e2e's mise install (pinned,
  checksum-verified, cache keyed by the fixture `mise.lock` sha256),
  the host suite, `tools/ci acceptance`, `tools/ci links` and
  `tools/ci pins`, which are outside `all`, and, inside `all`, by the
  `vulnerabilities` step and by the `license` step's pull of its pinned
  image (10.2). Nothing else in a test or a step reads it.
- The container e2e restores its mise cache with `actions/cache`, pinned
  by SHA like every action (10.2); `release.yml` restores no cache.
- A precondition of the hybrid or container level that cannot be met
  (the Docker daemon, the image digest, the herdr pin, the mise
  download) fails the job with an exit status of its own that names it
  as an infrastructure error, never as a skip; a digest or checksum
  mismatch is always a test failure.
- Flaky tests are fixed, never skipped: `TestTestStyle` (10.6) fails on
  a call to `t.Skip`, `t.SkipNow`, `t.Skipf` or `testing.Short`.

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
| file | the keys `name`, `on`, `permissions`, `concurrency`, `env`, `jobs`; `on` names the events `pull_request`, `push`, `schedule`, `workflow_dispatch`, with their filters, and `pull_request_target` only as the pr job row below allows; `env` is required and holds exactly the mise environment of `tools/ci` (`miseEnv` in `tools/ci/misefiles.go`), in its order, each value a YAML string |
| job | the keys `name`, `runs-on`, `needs`, `strategy`, `permissions`, `timeout-minutes`, `steps`; `runs-on` is `${{ matrix.os }}` or one of the four labels of Runners below, and so is each `os` of the matrix (`runs-on` as a list or a mapping fails); `permissions`, at the top and in a job, is a mapping whose values are `read` or `none`, and the file sets it at the top or in every job, so no scope is left to the default; `strategy` holds `matrix`, `fail-fast` and `max-parallel` only; the matrix is a written mapping with the keys `os` and `include` only, each `include` entry holds `os` and `mise_sha256` only (names matched exactly), and a `${{` in the value of an `include` entry fails (the `os` value is held to the four labels); with `runs-on: ${{ matrix.os }}` every runner of the matrix names an `os` |
| `uses` step | the keys `name`, `uses`, `with`; `uses` is `<owner>/<repo>[/<path>]@<40 hex digits>`, a commit SHA, the owner and the repository start with a letter or a digit, and so does each path segment or it starts with `_`, so no `.` or `..` segment, no `./` path of the repository (a local action) and no `docker://` image fits; `with` is a mapping that holds only the inputs listed for that action in `tools/ci` (the action is matched without case), each value a literal or one `${{ matrix.<key> }}` alone whose values are all literals; an `actions/checkout` step sets `persist-credentials: false`, written exactly so (the case of the action name is ignored, the case of the value is not) |
| `run` step | the keys `name`, `run`; `run` is one line, `go run ./tools/ci <subcommand> [<argument>...]` or `go run ./tools/release <subcommand> [<argument>...]`; each word is made of ASCII letters, digits and `._/=:-`, or is `"$NAME"` |
| pr job | one workflow file whose `on` names `pull_request_target` and no other event holds the one job that runs `go run ./tools/ci pr`; the file and the job set `permissions` to `contents: read` and nothing else, no step names a secret or `github.token`, its checkout is of the base commit, and the head is read only as git objects fetched by `head.sha`, never checked out to be built or run; `tools/ci workflows` refuses `pull_request_target` in any other file, with a fixture per refusal. Under that event GitHub runs the workflow file of the default branch, with a token that may write and with secrets, and warns against building or running pull request code ([events that trigger workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows), [secure use](https://docs.github.com/en/actions/reference/security/secure-use), read on 2026-10-09), which is why every one of these limits is in the grammar |
| release publish job | in `.github/workflows/release.yml` only, one job, the publish job of [Release and bootstrap](#release-and-bootstrap), may set `contents: write`, `id-token: write` and `attestations: write` in its job `permissions`, and only its `run` steps of `tools/release publish` and `tools/release verify` may hold `env` with the one key `GH_TOKEN: ${{ github.token }}`; `tools/ci workflows` refuses each of these in any other file, job or step, with a fixture per refusal |

So a workflow has no `if`, no `continue-on-error`, no `shell`, no
`container`, no job that calls another workflow, and no `${{ }}`
expression inside a `run` line: none of them is in the grammar. Where
a value may hold an expression (`runs-on`, `with`,
`concurrency`, a `strategy` matrix), the text between the braces is
one context path: names of ASCII letters, digits, `_` and `-`, joined
by dots (`matrix.os`, `github.ref_name`). An operator, a function call
or a literal there fails, and so does `github.token` outside the
release publish job row and any
`secrets.<name>` (the list of allowed secrets is empty; a secret joins
it with the operator's approval), whatever the case. An anchor, an
alias and a tag other than `!!str` on a key fail. No job or step holds
`env`, but for the release publish job row, and the one at the top of the file holds nothing but the mise
environment: another key could make a `run` step start other code
(`BASH_ENV`, `LD_PRELOAD`, `PATH`, `GOFLAGS=-toolexec`). That table
reaches every mise run of the workflow, the mise action's and the one
of the mise shim that starts go, and `go run ./tools/ci setup` fails a
hosted run whose environment does not hold it. A value a command needs from the event reaches
it as a variable the runner sets, a `"$NAME"` word such as
`"$GITHUB_REF_NAME"`. A
workflow and a developer's shell therefore run the same code at the
same commit. `tools/ci` runs `reuse` from a container image pinned by digest (the
license row), and `tools/ci` and `tools/release` run `govulncheck`,
`golangci-lint`, `gh` and the go command by the paths `mise which`
resolves, never through `mise exec`: with the tool not installed,
`mise exec` warned and ran a program of the search path, exit status 0
(measured with the pinned mise). A path must
lie below the install directory of the tool, in the directory of the
version `mise.lock` locks (a tool locked at two different versions is an error; the same version twice is not), so the
versions locked in `mise.lock` are the ones used in both places.

Three consequences of the grammar:

- A step that applies to some runners only (the hybrid and container
  e2e on Linux, the I27 hostile trees on macOS) is selected inside
  `tools/ci`, from the OS and architecture it runs on: there is no `if`
  to select it in the workflow. The steps each runner class runs are a
  table in `tools/ci` with a unit test of its own, and a step whose
  `go test -json` output reports zero tests fails, so a selection that
  runs nothing is red.
- The pr job's workflow lists `edited` among its `pull_request_target`
  types, so a change to a PR's title or body runs `pr` again;
  `tools/ci workflows` fails that file without it.
- The grammar bounds keys, `run` lines, runner labels, the inputs of
  each action and expressions, not each value: the owner of a `uses`
  action, the filters under `on` and the values under `strategy` other
  than `matrix` are free. They are reviewed, not checked:
  `.github/workflows/**` is an ask-first surface (05 5.3). The inputs
  listed in `tools/ci` cover `actions/checkout`, the mise action,
  `actions/cache` (the container e2e's mise cache, 10.1) and
  `actions/attest-build-provenance`.

The `pr` step runs in its own job, the pr job of the grammar, from the
base commit's code, on the file that `GITHUB_EVENT_PATH` names; `tools/ci
all` does not run it. A local run
is `go run ./tools/ci pr <file>`, on a payload saved beforehand (with
`gh api`, for example), or, before the pull request exists,
`go run ./tools/ci pr --title <title> --body <file>`, which reads the
local base..HEAD range in place of a payload; `pr` itself makes no
network call. The file is
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
| generated | `tools/ci generated`: `go generate ./...` changes no consumer of the table in [12 12.3](12-engineering.md#123-generators). It compares the working tree, tracked and untracked files by content, before and after the run, so a changed, a new and a removed generated file each fail and a tree with uncommitted work can be checked. It then requires each file that `generate` writes to be tracked in the index as a regular file (mode 100644, so not a link and not executable) whose blob is the bytes `generate` gives, and to match no ignore rule, because the comparison above cannot see a generated file that is ignored, untracked or a link to a copy: the merged tree would not hold it. `generate` reads and writes through the root of the repository (`os.Root`), so a link cannot send it outside. It judges the index, so in `fast` and `all` it runs before every step that runs code of the change (`unit`, `race`), a test holds that order, and the mise it starts reads `mise.toml` alone (the mise environment, below) | yes |
| lint | `golangci-lint run` over the whole module, once per lint target; its rules are in [Lint](#lint) below | yes |
| unit | `go test -count=1 ./internal/... ./tools/...`, the pinned go command, in the environment of the steps of `fast` (below); `-count=1` because tests that scan the repository would otherwise pass from the test cache | yes |
| hygiene | `tools/ci hygiene`; its rules are in [Hygiene](#hygiene) below | yes |
| pushed range | `tools/ci fast` with the pre-push hook's arguments: it first refuses a push it would not test as sent, then runs the forbidden-name check over the remote ref names and the commits of a push (below), and refuses a commit that adds or renames to a file whose name is on the never-tracked list (below) even when a later commit removes it | in the hook only |
| sequences | `tools/ci sequences`: ADR numbers unique; ADR layout and statuses per ADR 0001 (rules 6-7), the `Superseded in part by` form included, the filename compared through the `slug` function `tools/new adr` uses; every `Supersedes` link in an ADR's Status section matches a `Superseded by` link in the target ADR and the reverse; every ADR that `docs/spec.md` or `docs/spec/` cites has status Accepted; every row of the index's Deferred decisions tables has its three cells filled ([ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)); the ids below unique, and every referenced id and id range (for example "J1-J13" in a success criterion) defined; in `docs/plan.md`, task ids contiguous, each "Depends on" naming an existing lower task id or block, and each "Implements" citing an existing section ([12 12.2](12-engineering.md#122-development-commands-and-capability-map)) | yes |
| vocabulary | `tools/ci vocabulary` (01 1.7) | yes |
| workflows | `tools/ci workflows`: the grammar above | yes |
| vulnerabilities | `govulncheck ./...`; it reads the Go vulnerability database over the network, so its result depends on the date. It is the one step of `all` that reads the network for its result: a local `all` without the network is not the equivalent run, and its output says so. It runs before `unit` (below) | |
| build the tagged suites | `go vet -tags e2e ./e2e/...` and `go vet -tags host ./e2e/host/...` on every runner, so a suite that does not build fails here and not at block B | |
| unit + golden + race | `go test -race -count=1 -coverprofile=cover.out ./...`, with `cover.out` in a temporary directory, so no run leaves it in the working tree; it runs code of the change, which can game its own result (below) | |
| coverage | `tools/ci coverage` (S9), per package: every package `go list ./...` reports, `cmd/` included, and a package absent from the profile counts as 0 percent; each package below its threshold is a finding, and the thresholds are starting values that are only raised; it reads the profile that the race run's code of the change wrote, so a test can game it (below) | |
| golden governance | `tools/ci golden`: fails if `-update` appears in CI invocations; lists every changed golden in the job summary for review | |
| schema | `tools/ci schema` (10.1); it also checks that each YAML file under `examples/` carries the `$schema` hint of the release that ships it ([03 3.2](03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1)) | |
| e2e | `go test -race -count=1 -tags e2e ./e2e/...`, `-count=1` for the reason the unit row gives; the romeu it runs is built by `tools/release build --dry-run`, the one way julieta is embedded ([02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)) | |
| cross-build | darwin amd64/arm64 (`romeu`), linux amd64/arm64 (`julieta`) | |
| imports | `tools/ci imports` (I1, I2, I11, I24, 10.7) | |
| invariants | `tools/ci invariants` reads the ids from the table of 05 5.2: the guard and test tags found in the code pair up, and each tagged id is a row of that table. That each row has both is checked by `acceptance` (10.5); that a pull request removes no last guard or test tag of a row it keeps is checked by `pr` | |
| mutation | `tools/ci mutate` on PRs touching a `.go` file on a path in `.github/ask-first.yaml`, or a file that holds a guard tag of an invariant; the scheduled `fuzz.yml` runs it too | |
| catalog | `tools/ci catalog` (explicit upload flags, key syntax) | |
| kits | `tools/ci kits` (one frontend pin; install steps <= 5 lines; every download has a sha256: a download is a step line whose first word is `curl` or `wget`, and the same step has a `sha256sum -c` or `shasum -a 256 -c` line after it; a download line holds no `\|`, `;`, `&&` or `$(`, and the file `-c` reads is a kit file, not one the step downloaded. A download written with another tool is not seen by the check; the review of the `kits` surface covers it). On a change to the pin file, it verifies the committed signature of mise's `SHASUMS256.txt` offline and checks the pinned hashes against that file ([07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress)) | |
| mise | `tools/ci mise` (`julieta lock --check` logic on this repo's and the examples' locks) | |
| probes | `tools/ci probes` (11 11.4: the results that are committed, and the A5 golden hashes; that every probe has a result is checked by `acceptance`, 10.5) | |
| license | `reuse lint` (REUSE 3.3), from the `fsfe/reuse` image pinned by digest in `tools/ci` ([12 12.1](12-engineering.md#121-tech-stack)), with the working tree and the git directory (`git rev-parse --git-common-dir`) mounted read-only, each at its own path, and no network for the container, so that a linked worktree, whose git directory lies outside the tree, ignores the same files as a clone; it runs before `unit` (below); on Linux only, since the check reads file content alone and the image is built for Linux. On macOS the output of `all` shows `--    license: not run on darwin`, neither `ok` nor `FAIL`. The same step reads the licenses of the module graph and fails a module whose license is not on an allowlist compatible with GPL-3.0-only ([00 0.4](00-scope.md#04-license)); `tools/release build` writes the third-party notices file of each archive from that graph | |
| docs | `tools/ci docs` (S10; the checks ADR 0001 assigns to it: rules 1, 5 without external URLs, 9 for the title at a first mention, 12, 13, 17, 20). It also fails `ARCHITECTURE.md` when an invariant id of 05 5.2 or a module id of 12 12.2 does not appear in it; an error id whose "recovered in" cell is empty or names an anchor that does not resolve (12 12.8); and a spec page holding a table whose header row equals a generated reference table's ([03](03-formats.md), opening). The Markdown lint and the spell check of rule 5 check nothing until their tools are picked, a [deferred decision](../spec.md#deferred-decisions). The reference pages are compared with their source by `generated`, not here | |
| lessons | `tools/ci lessons`: every lesson under `docs/lessons/` ([12 12.7](12-engineering.md#127-text-standard-and-lessons)), read after its front matter, names an existing `tools/ci` subcommand or test name, or says "no check possible: <reason>" | |
| pr | `tools/ci pr` (pull requests only; inputs as described above; 12 12.4; ADR 0001 rules 4, 8, 11; the base branch, 12 12.9; the fix marker, 12 12.10; the tags of 05 5.2, above; the forbidden-name check, below) | |

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
the caller changes what a step checks. Every mise command `tools/ci`
starts (`mise --version`, `mise trust`, `mise install` and `mise which`) runs at the top
of the tree with the mise environment besides:
`MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml`,
`MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none`, `MISE_ENV` set and
empty, `MISE_AUTO_ENV=false`, `GOENV=off` and `GOTOOLCHAIN=local`, so
mise reads `mise.toml` as its one configuration file of the tree, and
`mise.lock`, and runs nothing that another mise file of a change holds
(the threat model in `tools/ci/misefiles.go` says what stays trusted).
That environment is measured with one version of mise, the `version`
input of the mise action in `ci.yml`, so `fast`, `all` and `setup`
refuse, before any other mise command, a mise whose `mise --version`
names another.

The command steps of `all` are those of `fast` and `race`, in that
order, with `vulnerabilities` and `license`, which read the tree and
run none of its code, between `lint` and `unit`: no step that judges
the tree runs after the first step that runs the change's tests, so a
test cannot rewrite a file such a step reads (a test that prepended an
SPDX header to a file without one, while `unit` ran, passed `license`
when it ran after, as measured). That is all the order
guarantees. The tests run in the same process tree and account as
`tools/ci`, so an attack on the `tools/ci` process itself (ptrace, root
on a hosted runner, the same account) is out of its reach: what holds
is the read-only token permissions of `ci.yml`, and a job of their own
for the tests is a
[deferred decision](../spec.md#deferred-decisions). `race` and
`coverage` measure code of the change, which can game its own result
(a test can write the cover profile itself); review, by working
agreement, is what reads for that. Before mise and the steps run, `all`
reads HEAD, the index and every file of the working tree that git does
not ignore, by its mode and content, and its `working tree` check fails
when the steps leave any of them changed; a pre-push run of `fast`
refuses the push in that case.

Some unit tests need what the steps need: mise on the search path with the
pinned tools installed and this repository trusted, a module cache
that already holds the modules of `go.sum` (no step fetches a module,
since the steps run with `GOPROXY=off`; `go mod download` fills the
cache, and `go run ./tools/ci setup` runs it), and the `.git` directory of a
clone, because they list the repository's files with `git ls-files`.

`setup` runs `mise trust`, `mise install` and `go mod download` in the
environment of the steps, built from nothing, with
`GOPROXY=https://proxy.golang.org`, `GOSUMDB=sum.golang.org` and
`GOENV=off` in place of the steps' `GOPROXY=off`; a test shows that a
caller's `GONOSUMDB` and `GOFLAGS` do not reach it, so a caller's
settings cannot fill the module cache unverified.

What a run prints:

- Every failure line names the step, a rule id, the location and a fix
  line addressed to whoever pushed, agent, contributor or maintainer.
  For a forbidden-name hit the fix line reads "reword this line; after
  one refused rewrite, or if it is your own identity, hand the location
  to the maintainer, who re-authors the commit"; for a mise version it
  prints the exact version and the install command; for a prose hit in
  an older commit message, the rebase instruction.
- Each step prints one progress line and its duration. `fast` targets
  three minutes on a warm cache, and `tools/ci` warns when it takes
  longer; a budget for the hosted legs is a
  [deferred decision](../spec.md#in-how-this-repository-is-run).

A red leg that the change did not cause (an advisory, a flake, an
upstream outage) is fixed on `main` by one pull request, owned by
whoever sees it first; open pull requests rebase after it and are not
fixed one by one. Gating each merge on the merged result (a merge
queue) is a repository setting on the ruleset, and v1 does not use it.

The steps of `fast`, with `workflows`, `generated`, `sequences` and
`pr`, land with `ci-bootstrap`
([12 12.2](12-engineering.md#122-development-commands-and-capability-map));
every other step lands in the first pull request of the module that
owns its first input, as a `tools/new` kind does.

A personal absolute path, for `hygiene`, is `/Users/<name>/` or
`/home/<name>/` where `<name>` is one path segment other than `agent`
(the sandbox user in this specification's examples) and is not written as a
`<placeholder>`.

The ids `sequences` reads, in `docs/spec.md` and `docs/spec/`:

| Id | Form | Defined by |
|---|---|---|
| invariant | `I<n>` | the first cell of a row in [05 5.2](05-security.md#52-invariants-and-their-tests) |
| success criterion | `S<n>` | the first cell of a row in the index's criteria table |
| question | `Q<n>` | the first cell of a row in the index's Open questions |
| journey, variant | `J<n>`, `J<n><lowercase letter>` | a `## J<n>` heading in [09](09-journeys.md); a variant is defined where that heading names it |
| probe | `A<n>`, `B<n>`, `C<n>` | the first cell of a row in a table of [11](11-host-probes.md) |
| error | `RJ-<nnn>` | the first cell of a row in [04 4.4](04-cli.md#44-error-ids) |
| section | `<n>.<m>` | a `## <n>.<m>` heading of file `<n>` |

A reference is another occurrence of an id as a whole word, inside a
code span too; `X<a>-X<b>` stands for each id from a to b. A section is
referenced as a link anchor, which the link check of `tools/ci docs`
resolves, or in plain text as `<n> <n>.<m>` ("10 10.2"), which
`sequences` resolves against the headings of file `<n>`. Uniqueness and
existence are checked; of these, only the task ids of the plan must be
contiguous.

Runners: `ubuntu-26.04` (x64), `ubuntu-26.04-arm` (arm64), `macos-26`
(arm64) and `macos-26-intel` (x64). The arm64 and Intel labels are
verified to exist, and every label to report, by `uname -m`, as `aarch64`
(`ubuntu-26.04-arm`), `arm64` (`macos-26`) and `x86_64` (`ubuntu-26.04`
and `macos-26-intel`), on the first run of `ci.yml`. Labels are
pinned, never `*-latest`: `tools/ci workflows` fails a `runs-on` or a
matrix `os` that is not one of the four. `tools/ci all` reports the
architecture of the Go binary next to `uname -m`, and fails when they
differ (`x86_64` is `amd64`; `aarch64` and `arm64` are `arm64`). On macOS it also reads `sysctl -n sysctl.proc_translated`, which `uname -m` cannot show (a binary under Rosetta sees the architecture it emulates), and fails when it is `1`. They are bumped deliberately; the images are listed at
<https://github.com/actions/runner-images>. A retirement notice for a
label, or a leg that fails on a changed image, replaces the label by a
reviewed change, which names any coverage that moves to the host suite;
S2 means the runner list at the release commit. The release takes no
tool from the image beyond the shell and git, and its run records the
image version.

Four subcommands are outside `all`:

- `tools/ci acceptance` needs the network (GitHub API). Its four
  completeness checks (10.5) read only the repository, and they join
  `all` once the plan's last task has landed
  (10.5); before the v1.0.0 tag the maintainer runs it with `--pre-tag`
  as a step of the release checklist of
  [12 12.2](12-engineering.md#122-development-commands-and-capability-map),
  and no workflow runs it. Its unit tests use a recorded API fixture.
- `tools/ci links` needs the network too: it fetches every external URL
  the docs cite (ADR 0001 rule 5). No workflow calls it in v1; a person
  runs it, and the final plan task runs it once, next to
  `tools/ci acceptance`. Relative links and anchors stay in
  `tools/ci docs`, offline.
- `tools/ci fuzz` runs each fuzz target for a fixed time, longer than
  the short runs inside `go test`. The scheduled `fuzz.yml` calls it,
  and then `tools/ci mutate`.
- `tools/ci pins` needs the network: it reports the pin freshness of
  [12 12.1](12-engineering.md#121-tech-stack). It is a report and no
  merge gate, since its result depends on what upstream published that
  day. No workflow calls it in v1; a person runs it. Its unit tests use
  one fixture per source, and a test shows that `all` does not start
  it.

### Lint

- `golangci-lint run` covers the whole module, `tools/` included.
- `.golangci.yml` enables the doc-comment, error-string and
  commented-out-code checkers of ADR 0001 (rule 14) and testifylint with
  `enable-all: true` (10.6); it disables no linter and excludes no
  finding.
- A fixture test proves that each rule 14 checker and each testifylint
  checker that applies outside suites still reports (10.1).
- revive's `exported` reads only importable packages, so it skips
  package `main` and `_test.go` files (`File.IsImportable` in the
  pinned revive) and checks no doc comment in `tools/ci` today; no
  setting of the rule extends it to them.
- The linter sees only the files that build for the GOOS and GOARCH it
  runs under, so `tools/ci` starts it once for each lint target (linux
  and darwin, each on amd64 and arm64, one list that the tests read
  too), each as `<path> run --config .golangci.yml`, with the path of
  `golangci-lint` that `mise which` resolves and not through
  `mise exec`, which would add a mise `[env]` table, in the environment
  of the steps of `fast` with the target's `GOOS` and `GOARCH` and
  `CGO_ENABLED=0` added.
- `mise which` installs nothing and fails when a tool is missing, so a
  new machine runs `go run ./tools/ci setup` first.

Tests read `.golangci.yml`, `go.mod`, the mise configuration files and
the repository's Go files, tracked, not yet added and ignored, and fail
on:

- a key of `.golangci.yml` outside a short list, so a `default` or
  `enable-all` key under `linters`, an exclusion or a disabled linter
  fails, while testifylint's `enable-all: true` is required;
- a second `.golangci.*` file;
- a directive outside the allowlist, which is `go:build` and, at one
  site only, `go:generate`: the one directive `go:generate go run .
  generate` of `tools/ci/generate.go`, exactly once in that file and in
  no other, judged on the raw lines that `go generate` reads, in every
  `.go` file below the root but `.git`, and checked again by
  `tools/ci generated` before it runs `go generate`. A directive is the
  `tool:name` form of `isDirective` in `go/ast`, read in every line of
  every comment after the leading `/`, `*` and `unicode.IsSpace` runes
  are trimmed; golangci-lint's `nolint` word at the start of such a
  line; and the `line`, `extern` and `export` directives right after
  `//` or `/*`;
- a generated-code header;
- a file that builds with cgo off for none of the lint targets (build
  constraints with the tool tags the pinned go command reports for each
  target, file-name suffixes and an import of `"C"`, as `go/build` and
  `go/parser` read them);
- a file under `testdata`, `vendor`, a `_` or `.` directory or a nested
  `go.mod`, which `go list` skips;
- a `go.mod` directive other than `module`, `go` and `require` (an
  `ignore` takes a directory out of `./...`);
- a mise configuration table other than `[settings]` and `[tools]`, or
  a setting other than `lockfile` (an `[env]` table among them);
- a tool in `[tools]` outside the list of
  [12 12.1](12-engineering.md#121-tech-stack), or a value other than the
  exact version `mise.lock` locks for it (a `path:` tool among them);
- a `.tool-versions` file, which mise reads too.

### Hygiene

`tools/ci hygiene` fails on:

- a U+2014 character;
- a break of the prose rules of ADR 0001 (rule 4);
- a `TODO` in a Go file without the `TODO(#<issue>)` form (rule 14);
- a personal absolute path (10.2, above);
- a tracked `go.work`, `go.work.sum` or `vendor/`;
- a tracked mise file other than `mise.toml` and `mise.lock`, by their
  exact names: every other path where the pinned mise, started at the
  top of the tree, reads configuration (the globs of the `dependencies`
  surface in 05 5.3, held to the list in `tools/ci/misefiles.go` by a
  test, whose threat model says what this stops and what it does not),
  matched in any case, as a case-insensitive file system reads them;
- a tracked path with a byte outside printable ASCII (0x20 to 0x7E),
  since a fold beyond ASCII, an accent composed or decomposed by the
  file system, or a control byte lets a name stand for another one;
- a tracked entry with a mode other than 100644 or 100755 (a symbolic
  link or a submodule), since either leads a path to content no rule
  reads by that path;
- a tracked file whose name is on the never-tracked list of
  `tools/ci/denylist.yaml`, at any depth and in any case: names of
  files that belong outside product repositories, such as a person's
  own working agreement;
- a `.githooks/` that holds anything but `pre-push`, tracked with mode
  100755;
- a forbidden name in the path or content of a tracked file, the
  recordings under `e2e/testdata/sbx/**` included, and a denylist that
  is missing or has no entry ([Forbidden names](#forbidden-names)).

### Forbidden names

`tools/ci/denylist.yaml` lists names this repository must not contain:
in a file, a path, a commit message, a commit's author or committer
identity, a ref name or the text of a PR.
This page does not write them and calls them the denylist entries.
The list holds names of other projects this repository must not
mention. A false positive is reported in an issue, and the maintainer
adjusts the entry.
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

The hook is `exec env <the mise environment> go run ./tools/ci fast
"$@"`, the mise environment being the variables of the steps'
mise commands (above), so the mise shim that starts go reads
`mise.toml` alone. Git gives a pre-push
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
  directory, a file mise reads when it starts in the directory that
  holds it or above (the mise list of the hygiene row, in any case), or
  a nested repository.

The fix is to commit such a change or remove it. The steps take
minutes, and the working tree can change while they run, so after the
last step `fast` judges it again, the `go.mod` rule below included, and
stops the push with exit status 2 when HEAD has moved or one of these
refusals now holds. Hygiene, `workflows` and the range judge the commit
judged, by its id, as read before the first step and before mise runs:
a step runs code of the change, which could rewrite the working tree,
the index, HEAD or an object of the store, and `git cat-file` does not
hash what it reads. That order closes this file channel only; an
attack on the `tools/ci` process itself is out of its reach, as the step order of `all` above says. The git commands of
this check run without `GIT_DIR`, `GIT_INDEX_FILE`, `GIT_WORK_TREE`,
`GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, the other
variables of `git rev-parse --local-env-vars` and every `GIT_CONFIG`
variable, so they read the repository's own index, and with
`GIT_OPTIONAL_LOCKS=0`, so `git status` writes nothing. Called without
arguments, `fast` refuses none of this: its steps run on the working
tree as it is, and hygiene and `workflows` judge the commit HEAD names,
read before the first step, as `all` does. In both modes it refuses a `go.mod` below the module root, tracked
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
for each form except the slash. The real denylist also holds one
synthetic entry, written by `hygiene add`, whose plaintext is committed
under `testdata`; hygiene's tests assert that the matcher finds it
through the real file, so a change to the matcher that stops the real
entries matching fails a test.

What the hash buys is small, and we say so. It is a plain hash, not a
keyed one, and each segment's length is in the file, so a short segment
is recovered by trying each string of that length, in seconds on a
laptop. The hash only keeps the plaintext out of the tree, so that a
search of the repository does not find it. The maintainer decided on
the plain hash knowing this
([round 3](../reviews/round-3.md#maintainer-decisions)).

### Release and bootstrap

`release.yml` releases a `v*` tag. A tag of the form
`v<major>.<minor>.<patch>` is a release; the same with a suffix is a
prerelease, which is what the release candidate of block B is
([11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last)).
How the workflow is started, so that its own file does not come from
the tagged commit, is a
[deferred decision](../spec.md#in-how-this-repository-is-run).
The workflow has two kinds of job:

- The test jobs run `tools/ci all` on the commit the tag names, one
  job per runner of the list above, with the read permissions of
  `ci.yml`.
- One publish job, on `ubuntu-26.04`, needs all of them and runs no
  test and no `tools/ci` step. It holds the permissions and the token
  of the release publish job row of the grammar, and it builds, so no
  archive passes from one job to another. Its steps, in order:
  1. `tools/release build` refuses to build when the latest scheduled
     `fuzz.yml` run on `main` is not `success`. It cross-compiles the
     julieta linux binaries, embeds them, the kits and the catalog into
     romeu, builds romeu for darwin, and writes the archives, each with
     `COPYING` and the third-party notices file, and `checksums.txt`.
     The build contract: `-trimpath`, `CGO_ENABLED=0`, a fixed build id
     and the commit time as the build date; archives with fixed
     modification times, owners and order; the environment of the
     `tools/ci` steps, built from nothing. Nothing compares the bytes
     of a second build with the first.
  2. `actions/attest-build-provenance` attests the archives and
     `checksums.txt`.
  3. `tools/release verify` checks each archive against the
     verification contract below, with the `gh` that `mise which`
     resolves ([12 12.1](12-engineering.md#121-tech-stack)).
  4. `tools/release notes` generates the release notes
     ([12 12.6](12-engineering.md#126-release-notes)).
  5. `tools/release publish` creates the release as a draft, uploads
     every asset and publishes it, in one call, and marks it a
     prerelease when the tag has a suffix after the patch number
     (`v1.0.0-rc.1`).

A published release never changes; a fix is a new version. GitHub's
immutable releases setting (item f of the maintainer block) refuses a
changed asset after publication. The release takes no tool from the
runner image beyond the shell and git, and it restores no cache.

**Verification contract.** One list, from which J1 step 2, J9, B1, the
README and `tools/release verify` derive: the repository; the signer
workflow `release.yml`; the commit the tag names; hosted runners only.
The ref that started the workflow joins the list when the deferred
decision above is made. The commands that check it are written once,
in `e2e/scenarios/commands.yaml`. The releases carry `checksums.txt`
and the keyless build provenance attestation, and no other signature;
what the attestation proves, and what it does not, is in
[05 5.4](05-security.md#risks-of-running-romeu), "Release proof".

`tools/ci acceptance` is not a step of the workflow: its pre-tag run
and the comparison of the candidate's commit with the tag are
maintainer steps of the release checklist of
[12 12.2](12-engineering.md#122-development-commands-and-capability-map),
and its evidence names the release and the run, which exist only when
this workflow has ended (10.5).

The development sandbox's token (the operator's) can push a `v*` tag
([05 5.4](05-security.md#risks-of-how-this-repository-is-developed)), so
the pull request that adds `release.yml` decides how releases come from
`main` only, and one constraint is fixed now: the release workflow file
must not come from the tagged commit. Releasing from `main` only blocks
a stray tag and not the token, which merges through the API.

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
   included, so a trailer written before the first release is well
   formed. Each other `tools/new` kind and generator row lands in the
   first PR of the module that owns its first input (12 12.2).

**The maintainer block.** The parts of `ci-bootstrap` that only the
maintainer can do: one sitting around step 1 and one call after step
2. The output of each item goes under the Evidence of the PR it belongs
to; no check reads that text, so it is **[review]**. Items a to d ran,
and what each did and measured, with its date, is in
[the note on the maintainer block](../notes/maintainer-block-of-ci-bootstrap.md).
What they set up, and holds now:

- a. Two rulesets. On the default branch: changes arrive by pull
  request; a merge commit is the only merge method; force pushes and
  deletion are refused; a required-status-checks rule lists the four
  `all on ...` checks. Updates are no longer restricted to the bypass
  actor (removed, as read on 2026-10-09), so, read from the rules and
  not tried, an account with write access can merge a pull request
  whose required checks are green. On tags matching `v*`: creation, update and deletion
  are refused for everyone except the bypass actor. The administrator
  role is the bypass actor of both, and the operator's token holds that
  role. The merge requirements, and what they are worth, are in
  [05 5.4](05-security.md#risks-of-how-this-repository-is-developed).
  Two repository settings, which bind a bypass actor too: squash
  merging and rebase merging off, so every merge is a merge commit
  (12 12.9); and private vulnerability reporting on, the channel
  `SECURITY.md` names (12 12.8).
- b. The denylist entries, written with `go run ./tools/ci hygiene add`
  in a plain clone on the host ([Forbidden names](#forbidden-names)).
- c. The pre-push hook, enabled in that clone.
- d. Four tries with the sandbox's token; of the four, only the direct
  push to the default branch was refused (05 5.4).

The host runs `tools/ci`, `hygiene add` included, only from a commit
whose diff since the last host run the maintainer has read, or from the
last released tag
([05 5.4](05-security.md#risks-of-how-this-repository-is-developed)).
For a branch an agent wrote, the maintainer fetches it from the sandbox
into a plain clone on the host and reads its diff there before checking
it out.

To do, each a maintainer step that saves the API's answer under
Evidence:

- d. A fifth try, at the next sitting, only with the operator present
  and confirming live and the revert step ready: set the default-branch
  ruleset's enforcement to disabled with the sandbox's token, read the
  body back, and restore it. The result is recorded in 05 5.4 and in
  the next decision record. To do.
- e. After step 2: add the CI jobs of the green run to the
  default-branch ruleset as required status checks. As read on
  2026-10-09, the rule lists the four `all on ...` checks (the note on
  the maintainer block); saving the API's answer under Evidence is still
  to do. When the task that builds `pr` lands, its job joins the
  required checks, the same way.
- f. Before the first release candidate: turn on GitHub's immutable
  releases setting. To do.

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
| Redaction | the recorder keeps real values only for the harness's own sandboxes and env dirs and writes typed placeholders for every other row of a listing, and it replaces the home path, user name, host name and root path with placeholders; git config, trust settings and key listings go through typed parsers, so an email, a `url.insteadOf` host or a foreign global rule never reaches a recording; it replaces each U+2014 with `-`, so recorded help text passes `hygiene` and rule 4 of ADR 0001 still skips nothing (no parser matches on that character); it keeps stdout only for an allowlist of read commands whose output the parsers need; it never records secret or credential output (`sbx secret` values, tokens); a unit test plants a fake token, a home path, an email, an internal `url.insteadOf` host and a foreign global rule and asserts none reaches the file; `tools/ci hygiene` scans the recordings |
| Replay only | `e2e/fakesbx` accepts only argv shapes present in a recording (placeholders for names, paths, domains); an unrecorded shape fails the test with the argv. For `env exec` at the hybrid level, the argv before `--` is matched against the recording and the command after it is forwarded into the julieta container, not replayed |
| Stateful replay | each scenario's sessions replay as a state machine keyed by the prior mutating calls (for example `sbx ls` answers differently after `env run` and after `env rm`), so a call out of the recorded order fails. A scenario names the recorded session it starts from, which is how a sandbox that was removed, stopped or created outside romeu is replayed. On create the fake writes the recorded `remote.sandbox-<name>` stanza into the primary clone's config, as sbx does (01 1.2) |
| Parsers | `sbxdrv` parsers run over every recorded version |
| Shared scenarios | CI and host suites call the same scenario functions in `e2e/scenarios`; only the sbx binary differs |
| Tested window | the sbx versions with a recording set under `e2e/testdata/sbx/` are the tested window, named in [12 12.1](12-engineering.md#121-tech-stack) and `SECURITY.md`. An sbx outside it is the `sbx-untested` warning of `status` and `doctor`, and gate 1 names the window ([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine)); an sbx output romeu cannot parse, or one that lacks a field, is its own error, `sbx-output-unparsed` (exit 2, [04 4.4](04-cli.md#44-error-ids)), and no state transition follows it. Each recording set and each probe result names its sbx version and capture date. Block B replays every recorded argv shape against the candidate's sbx and compares the output shape; a new sbx minor release or a floor bump records A4, A5 and A9 again before the window widens, and raising the floor requires a new recording set |
| Known argv | taken from recordings, for example `sbx policy rm network --sandbox <s> --resource <host>`. The argv shapes and sessions block A must record are the table of [11 11.1](11-host-probes.md#recorded-sessions); an argv builder with no row there is an error of the fake, found by a unit test before block A runs |
| Cannot model | VM boot timing, virtiofs semantics, real network policy enforcement, credential injection, kit builds, host-command prompts; these are covered only by the host suite and probes. A recording anyone makes with the probe harness (`--only`), through the recorder's redaction, may be attached to an issue; the maintainer commits it after reading it, and a fix for one of these classes replays it ([12 12.9](12-engineering.md#129-delivery-practices)) |

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
(argv, exit, output sha256), `probe` (probe-result id, verdict) or
`interim`. The final plan task commits the file after the v1.0.0
release, so the S1 and S2 items name the release and the CI run of the
release commit, which exist by then. The file records v1.0.0 only and
is not rewritten; the criteria of a later release are a spec change
made with that release.

`go run ./tools/ci acceptance` verifies each item mechanically: files
exist with the hash; probe verdicts are `pass`; CI runs are `success`,
through the GitHub API (a `ci-run` names its repository, because S4's
run is in the config repo); a `command` item is run again, its argv
from the repository root, and its exit status and the sha256 of its
stdout are compared. For each release it also checks that the tag's
commit equals the source digest in the release's attested provenance,
so a `v*` tag moved to another commit is found.

It also checks four things that are complete only at the end of the
plan. They read only the repository, and they join `all` once the
plan's last task has landed: before then they would keep `all` red
while the plan is under way, and from then on every pull request keeps
them true.

- every row of 05 5.2 has a guard tag and a test tag (S6);
- every probe id of [11](11-host-probes.md) has a committed result,
  from both hosts when the probe is arch-sensitive (11 11.4);
- every `## J<n>` heading of 09 is named by the `journey:` of at least
  one page under `docs/guide/` (S10);
- each block B result counts for v1.0.0: between the candidate's
  commit, which every block B result records, and the v1.0.0 tag, the
  only paths that differ are under `docs/` or are Markdown files at
  the repository root (11 11.2); results of one host that name
  different candidates fail.

Before the v1.0.0 tag the maintainer runs
`go run ./tools/ci acceptance --pre-tag`, which runs the four checks
against the commit about to be tagged, as a step of the release
checklist of
[12 12.2](12-engineering.md#122-development-commands-and-capability-map).
A check that fails after a tag is fixed by a new patch version.

## 10.6 Code style

- Standard Go layout; small single-purpose packages; no `utils`.
- Errors carry an id from the error table (`cli.Error{ID: "RJ-301"}`),
  which fixes the exit code; `main` maps them; tests assert ids. A
  table row that no test raises fails `go test ./internal/cli/...`; a
  row measurable only on a real host (`kit-build-failed`, B2) is listed
  as exempt with its reason.
- `context.Context` first for anything that runs a subprocess; every
  subprocess has a timeout, except the salvage exec, which stops only
  on an interrupt ([04 4.1](04-cli.md#41-conventions-both-binaries)).
- No global state except the embedded catalog, kits and binaries.
- No `text/template` for structured output; no `sh -c` anywhere in
  romeu.
- Every lifecycle of host state is a transition table in
  `internal/state`, never scattered conditionals.
- SPDX header in every file; doc comments on every exported identifier;
  one-line "why" comments where a rule exists for security, plus the
  invariant tag.

```go
// SPDX-FileCopyrightText: 2026 Bruno Venceslau
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
import of `suite`, `mock` or `http`; on a reference to `assert.New`,
`require.New` or `reflect.DeepEqual`; on a call to `t.Skip`,
`t.SkipNow`, `t.Skipf` or `testing.Short`; and on a `TestMain` that
does not end in `os.Exit(m.Run())`.

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
  (`go:build`, and `go:generate` at its one site, the directive of `tools/ci/generate.go`), so no `nolint`, `revive:disable`, `lint:ignore`
  or line directive, and no exclusion and no lowered setting in
  `.golangci.yml`.

## 10.7 Module boundaries (enforced by `tools/ci imports`)

| Package | May be imported by | Must not import |
|---|---|---|
| `sbxdrv`, `render`, `gate`, `state`, `egress`, `signing` | `cmd/romeu`, `tools/ci` | `mise`, `hooks`, `layout` |
| `mise`, `hooks`, `layout`, `agent/*`, `memstore/write` | `cmd/julieta` only | `sbxdrv` |
| `gitsafe`, `spec`, `canon`, `catalog`, `oci`, `manifest`, `memstore`, `handoff`, `salvage`, `shquote`, `termsafe`, `cli` | both | `sbxdrv`, `layout` |

Names are packages under `internal/`; a command package under
`internal/cmd/romeu/` or `internal/cmd/julieta/` follows the row of its
binary. A package under `internal/` that no row names fails
`tools/ci imports`, so a new package is placed in a row before it
builds. The second column says which binary may link a package, so a
package of the last row imports none of the second: romeu links it.
That is why the writer of the memory store is a package of its own, and
why `salvage` takes the agent profile as an argument (08 8.5). `oci` is
shared because `julieta pin` resolves and checks digests with the
same guarded client (I28).

## 10.8 Definition of done for implementing agents

The Always / Ask first / Never list in the
[index](../spec.md#boundaries-for-everyone-who-changes-this-repo)
applies. A task is done only when `go run ./tools/ci all` passes, its
invariant tags are in place, the generated files are current, a new or
changed error id has a reviewed fix hint, a changed journey has its
guide page updated in the same pull request, and, for host-facing
behavior, the matching scenario function exists and its
recording-runner test passes in CI (ADR 0001 rule 13); its block B run
is the acceptance.

Some steps of `all` cannot run inside an agent's sandbox:

| Step | Needs |
|---|---|
| `license` | Docker, and a pull of the pinned `reuse` image |
| `e2e` at the hybrid and container levels | Docker, the pinned workload base image, and the mise and herdr downloads |
| `vulnerabilities` | the Go vulnerability database |

The sandbox's network policy allows the hosts these need as one set,
listed beside this table when the first of them lands. A step that
cannot run is reported as `not run: <step>, <reason>`, and the task is
done only when CI runs it.
