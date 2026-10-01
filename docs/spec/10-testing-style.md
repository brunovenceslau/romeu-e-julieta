# 10. Testing strategy, CI, code style

Back to [index](../spec.md). Reader: implementers and reviewers of any
module. Type: reference. Tools are code: `tools/*`, `e2e/probes`,
`e2e/fakesbx`, workflows, schemas and kits are held to the same lint,
test, coverage and review rules as `internal/*`.

## 10.1 Test levels

| Level | Location | Runs where | Covers |
|---|---|---|---|
| Unit | `internal/**`, `tools/**`, `e2e/probes/**`, `e2e/fakesbx/**` `_test.go` | CI, dev | validators generated from `rules.go` (table + fuzz seeds committed), digests, widening-set and toolchain diffs, the generated state-machine tables (every row, every illegal pair), egress split, `termsafe`, shell quoter, sbx output parsers over every recorded sbx version, error-id table, doctor checks with `HOME` in a temp dir, the probe harness core and its sbx exec layer (against a helper binary re-executed from the test), the recorder's redaction, the fake sbx's placeholder normalizer, CI tools themselves (the forbidden-name matcher and its pushed-range walk over a fixture repository, `workflows` against fixture workflow files, the linter configuration against a fixture package with one violation per checker of rule 14 in [ADR 0001, the documentation standard](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md), invariants/mutate, acceptance against a recorded GitHub API fixture, `dora` against a recorded GitHub API fixture and a fixture repository, `pr` against recorded event payloads), the runtime ledger's tables ([13 13.10](13-runtime-ledger.md#1310-tests): the hostile spool, each field, entries and their races, the drain, the doctor checks on fixtures) |
| Golden | `internal/render/testdata/`, `internal/layout/testdata/`, `internal/gate/testdata/`, `internal/ledger/testdata/` | CI, dev | `sbxenv.yaml`, workspace files, `render.json`, gate diff text, herdr `layout.apply` requests (`layout up --dry-run`), handoff front matter, the two ledger views of a two-project fixture |
| Schema | `tools/ci schema` | CI | generated schemas equal the committed ones; examples and testdata validate |
| E2E (git + fake sbx) | `e2e/*_test.go`, tag `e2e` | CI (linux amd64, linux arm64, macOS Intel, macOS arm64) | romeu commands against a temp `$ROMEU_ROOT`; origins served by `git http-backend` behind `httptest` TLS (host settings `gitHosts[].caFile` points at the test CA; gitsafe has no test override); the sandbox daemon served by `git daemon` on `127.0.0.1`; journeys J2, J3b, J7, J10, J11 (scenario functions shared with the host suite); promotion fault injection; invariants marked E in [05](05-security.md), including the I27 hostile trees on the macOS runners |
| E2E (hybrid) | `e2e/hybrid_test.go`, tag `e2e` | CI on `ubuntu-latest` and `ubuntu-24.04-arm` | the romeu-julieta contract: the fake sbx forwards `env exec` into the julieta container with `.romeu/bin` bind-mounted read-only at the host path; protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each stop `run` before `layout up`; a 32 KiB manifest round-trips (I30); a writable ledger view mount stops `run` the same way (I33) |
| E2E (container) | `e2e/container_test.go`, tag `e2e` | CI on `ubuntu-latest` (amd64) and `ubuntu-24.04-arm` (arm64), native, no QEMU | julieta in the workload's Debian base: `setup` (PATH link, dispatcher, secondary clone, ff of default branch) and its no-op timing, `install` skip rule, `lock --check`, hooks dispatcher, chaining and recorded failures, memory (stamping, allowlist, import, verify incl. status), handoff (all kinds; SessionEnd then SessionStart as on `/clear`), snapshot, salvage completeness cases, `layout up --dry-run` golden and `layout up` against the pinned herdr |
| Host | `e2e/host/*_test.go`, tag `host` | maintainer machines (Intel and Apple silicon) | the same scenario functions against real sbx for J1-J13; S3, S5, S8 confirmation; results as `probe-result.v1` per block B step (11) |
| Probes | `e2e/probes` | maintainer machines | [11](11-host-probes.md); `probe-result.v1` files in `docs/probes/` |

Level claims per invariant are those in the 05 table (I14, I16, I20,
I22, I23 are unit-level).

Additional required tests:

- **Idempotency meta-test**: every mutating romeu and julieta command is
  run twice on unchanged inputs; the second run must write nothing
  except `lastRun` (filesystem and ref snapshot diff).
- **Concurrency**: two romeu processes (flock; second exits 1 with
  `RJ-101`); two julieta processes writing memory (store lock; no lost
  entry); snapshot racing a commit (bundle always valid).
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
- **TTY**: `approve` and `adopt` are driven through a `/dev/ptmx` pair
  created with `x/sys/unix` (test helper; no pty dependency).
- **S8 timing**: `romeu run --timings --json` with the fake sbx, median
  of 5 runs, asserted <= 2 s romeu overhead; `julieta setup` no-op in the
  container e2e, median of 5, asserted <= 3 s.
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
| job | the keys `name`, `runs-on`, `needs`, `strategy`, `permissions`, `timeout-minutes`, `steps` |
| `uses` step | the keys `name`, `uses`, `with`; `uses` is `<owner>/<repo>[/<path>]@<40 hex digits>`, a commit SHA |
| `run` step | the keys `name`, `run`, `env`; `run` is one line, `go run ./tools/ci <subcommand> [<argument>...]` or `go run ./tools/release <subcommand> [<argument>...]`; each word is made of ASCII letters, digits and `._/=:-`, or is `"$NAME"` |

So a workflow has no `if`, no `continue-on-error`, no `shell`, no
`container`, no job that calls another workflow, and no `${{ }}`
expression inside a `run` line: none of them is in the grammar. A
value a command needs from the event reaches it through `env`. A
workflow and a developer's shell therefore run the same code at the
same commit. `tools/ci` starts `golangci-lint`, `govulncheck` and
`reuse` through `mise exec`, so the versions locked in `mise.lock` are
the ones used in both places.

Three consequences of the grammar:

- A step that applies to some runners only (the hybrid and container
  e2e on Linux, the I27 hostile trees on macOS) is selected inside
  `tools/ci`, from the OS and architecture it runs on: there is no `if`
  to select it in the workflow.
- `ci.yml` lists `edited` among its `pull_request` types, so a change
  to a PR's title or body runs `pr` again; `tools/ci workflows` fails a
  `ci.yml` without it.
- The grammar bounds keys and `run` lines, not each value: the scopes
  under `permissions`, the owner of a `uses` action, and `with` and
  `env` values are free. They are reviewed, not checked:
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
[Forbidden names](#forbidden-names); the changed paths are the paths
those commits touch, both names of a rename included.

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
| format, vet | `gofmt -l` empty; `go vet ./...` | yes |
| generated | `tools/ci generated`: `go generate ./...` leaves no diff in any consumer of the table in [12 12.3](12-engineering.md#123-generators) | yes |
| lint | `golangci-lint run` over the whole module, `tools/` included; `.golangci.yml` enables the doc-comment, error-string and commented-out-code checkers of ADR 0001 (rule 14), and a fixture test proves it still does (10.1) | yes |
| unit | `go test ./internal/... ./tools/...` | yes |
| hygiene | `tools/ci hygiene`: no U+2014; the prose rules of ADR 0001 (rule 4); the `TODO(#<issue>)` form in Go files (rule 14); no personal absolute path (below); no tracked `go.work`, `go.work.sum` or `vendor/`; `.githooks/pre-push` tracked with mode 100755; the forbidden-name check over the path and content of each tracked file, which fails on a denylist that is missing or has no entry (below); scans `e2e/testdata/sbx/**` too | yes |
| pushed range | `tools/ci fast` with the pre-push hook's arguments: the forbidden-name check over the remote ref names and the commits of a push (below) | in the hook only |
| sequences | `tools/ci sequences`: ADR numbers contiguous and unique; ADR layout and statuses per ADR 0001 (rules 6-7), the filename compared through the `slug` function `tools/new adr` uses; every `Supersedes` link in an ADR's Status section matches a `Superseded by` link in the target ADR and the reverse; every ADR that `docs/spec.md` or `docs/spec/` cites has status Accepted; every row of the index's Deferred decisions table has its three cells filled ([ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)); the ids below unique, and every referenced id and id range (for example "J1-J13" in a success criterion) defined | yes |
| vocabulary | `tools/ci vocabulary` (01 1.7) | yes |
| workflows | `tools/ci workflows`: the grammar above | yes |
| vulnerabilities | `govulncheck ./...` | |
| unit + golden + race | `go test -race -coverprofile=cover.out ./...` | |
| coverage | `tools/ci coverage` (S9) | |
| golden governance | `tools/ci golden`: fails if `-update` appears in CI invocations; lists every changed golden in the job summary for review | |
| schema | `tools/ci schema` | |
| e2e | `go test -tags e2e ./e2e/...` | |
| cross-build | darwin amd64/arm64 (`romeu`), linux amd64/arm64 (`julieta`) | |
| imports | `tools/ci imports` (I1, I2, I11, I24 call sites, I32 call sites, 10.7) | |
| invariants | `tools/ci invariants` (every I-id has a guard tag and a test tag) | |
| mutation | `tools/ci mutate` on PRs touching a `.go` file on a path in `.github/ask-first.yaml`; the scheduled `fuzz.yml` runs it too | |
| catalog | `tools/ci catalog` (explicit upload flags, key syntax) | |
| kits | `tools/ci kits` (one frontend pin; install steps <= 5 lines; every download has a sha256) | |
| mise | `tools/ci mise` (`julieta lock --check` logic on this repo's and the examples' locks) | |
| probes | `tools/ci probes` (11 11.4) | |
| license | `reuse lint` (REUSE 3.3) | |
| docs | `tools/ci docs` (S10; `--help` output vs `docs/reference/`; the checks ADR 0001 assigns to it: rules 1, 5 without external URLs, 12, 13, 17, 20) | |
| lessons | `tools/ci lessons`: every `docs/lessons.md` entry, read after the file's front matter, names an existing `tools/ci` subcommand or test name, or says "no check possible: <reason>" | |
| pr | `tools/ci pr` (pull requests only; inputs as described above; 12 12.4; ADR 0001 rules 4, 8, 11; the base branch, 12 12.9; the fix marker's form, 12 12.10; the forbidden-name check, below) | |

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

Runners: `ubuntu-latest`, `ubuntu-24.04-arm`, an Intel macOS runner (the
label is verified to exist and to report `x86_64` in the `ci-bootstrap`
task) and `macos-latest` (arm64).

Four subcommands are outside `all`:

- `tools/ci acceptance` needs the network (GitHub API). It runs in the
  final plan task and in `release.yml`, and its unit tests use a
  recorded API fixture.
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
  the next segment's length, and so on.
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
  Evidence.

| Surface | Checked by | When |
|---|---|---|
| the path and the content of each tracked file at HEAD | `tools/ci hygiene` | in `fast` and in `all` |
| the name each pushed ref gets on the remote; of each pushed commit: the message, the author and committer names and emails, the paths it adds or renames to, and the lines it adds | `tools/ci fast`, called by the pre-push hook | before the push leaves the machine |
| the PR title, body and head ref name; the same four readings of each commit in base..head | `tools/ci pr` | on the pull request |

The hook is `exec go run ./tools/ci fast "$@"`. Git gives a pre-push
hook the remote's name and URL as arguments and, on stdin, one line per
pushed ref: local ref, local sha, remote ref, remote sha. Called with
those arguments, `fast` reads the lines, checks the remote ref name of
each, and walks the commits reachable from the local sha and not from
the remote sha. For a ref the remote does not have yet, or a remote
sha that the local repository does not hold, it walks the commits not
reachable from any remote-tracking ref of that remote. A merge commit
is compared with its first parent. A line that deletes a ref adds no
commits and is skipped, name included, so a ref with a forbidden name
can be deleted. A git command that fails stops the push. Called without
arguments, `fast` runs the `In fast` steps and no range. `pr` uses the
same walk over base..head.
Reading what each commit adds, and not only the final tree, is what
catches a line or a path that one commit adds and a later commit
removes.

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
- The messages of annotated tags and git notes are not read.
- `hygiene` proves that the denylist has an entry, not that it has the
  right ones.

An entry is written by `go run ./tools/ci hygiene add`. The maintainer
runs it, in a terminal outside an agent session, because an agent must
not hold the names. It reads the name from the terminal with echo off,
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
`gh attestation verify --signer-workflow .../release.yml`;
`tools/ci acceptance` runs; `tools/release notes` generates the release
notes (12 12.6); `tools/release publish` creates the GitHub release;
`tools/ci dora --attach "$TAG"` adds the delivery metrics to it
(12 12.10), with the tag passed through `env`.
The attestation is a `uses` step and each of the others is one `run`
step, so the file passes `tools/ci workflows`.

`ci-bootstrap` (first plan task), in this order:

1. One PR adds `tools/ci hygiene` with `tools/ci/denylist.yaml`,
   `tools/ci fast` and `.githooks/pre-push`. Its author can neither
   write the denylist entries nor push it past a hook that is not
   enabled yet, so the maintainer block below finishes and pushes it.
2. `tools/ci workflows` and a green workflow on the public repo,
   proving runners, labels and permissions.
3. The rest of the day-one middleware of
   [12 12.3](12-engineering.md#123-generators) and
   [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change),
   `tools/ci pr` and `tools/ci dora` (12 12.10) included.

**The maintainer block.** The parts of `ci-bootstrap` that only the
maintainer can do are one sitting, before step 1 is pushed, and one
call after step 2. The output of each item goes under the Evidence of
the PR it belongs to.

- a. In the branch of step 1, run `go run ./tools/ci hygiene add` once
  for each forbidden name, and commit `tools/ci/denylist.yaml`.
- b. Enable the hook in that clone (`git config core.hooksPath
  .githooks`) and push the branch, so the hook reads the bootstrap
  commits themselves: their messages, identities, paths and ref name.
- c. Create two rulesets and save the API's answer for each. On the
  default branch: changes arrive by pull request, with one approving
  review and a code-owner review; force pushes and deletion are
  refused. On tags matching `v*`: creation, update and deletion are
  refused. The maintainer is the one bypass actor of both.
- d. With the sandbox's token, try four things and save the four
  refusals: a push to the default branch, a push of a `v*` tag, the
  merge of step 1's PR, and a change to a ruleset. A try that is not
  refused is [Q25](../spec.md#open-questions).
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
| Redaction | the recorder replaces the home path, user name, host name and root path with placeholders; it keeps stdout only for an allowlist of read commands whose output the parsers need; it never records secret or credential output (`sbx secret` values, tokens); a unit test plants a fake token and a home path and asserts neither reaches the file; `tools/ci hygiene` scans the recordings |
| Replay only | `e2e/fakesbx` accepts only argv shapes present in a recording (placeholders for names, paths, domains); an unrecorded shape fails the test with the argv |
| Stateful replay | each scenario's sessions replay as a state machine keyed by the prior mutating calls (for example `sbx ls` answers differently after `env run` and after `env rm`), so a call out of the recorded order fails |
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
  the PR's Evidence. The `sbxenv.yaml` goldens carry the sha256 recorded
  by probe A5; `tools/ci probes` flags a golden whose hash moved since
  its probe (re-probe needed).

## 10.5 Acceptance evidence

`docs/acceptance.json` (schema `acceptance.v1`): one entry per success
criterion (S1-S11) with `evidence[]` items of kind `ci-run` (run id,
job, conclusion), `file` (path, sha256), `command` (argv, exit, output
sha256), `probe` (probe-result id, verdict), `repo` (full name, expected
`archived` flag) or `interim`. `tools/ci acceptance [--file <path>]`
verifies each item mechanically (files exist with the hash, probe
verdicts are `pass`, CI runs are `success` and repo flags match via the
GitHub API). `--file` validates any `acceptance.v1` file, so an
operator keeps their own acceptance file (for example
`julieta memory verify` per repo, `romeu status --json`, repositories
that must be archived) in their config repo and verifies it from a
product checkout; the product names no such repository. The product's
own run is the final plan task.

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
// SPDX-License-Identifier: GPL-3.0-or-later

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
		"--no-recurse-submodules", url, refspec)
	return err
}
```

## 10.7 Module boundaries (enforced by `tools/ci imports`)

| Package | May be imported by | Must not import |
|---|---|---|
| `sbxdrv`, `oci`, `render`, `gate`, `state`, `egress`, `signing` | `cmd/romeu`, `tools/ci` | `tools`, `hooks`, `layout` |
| `tools`, `hooks`, `layout`, `agent/*` | `cmd/julieta` only | `sbxdrv` |
| `gitsafe`, `spec`, `canon`, `catalog`, `memstore`, `handoff`, `salvage`, `ledger`, `shquote`, `termsafe`, `cli` | both | `sbxdrv`, `layout` |

## 10.8 Definition of done for implementing agents

The Always / Ask first / Never list in the
[index](../spec.md#boundaries-for-everyone-who-changes-this-repo)
applies. A task is done only when `go run ./tools/ci all` passes, its
invariant tags are in place, the generated files are current, and, for
host-facing behavior, the matching scenario function exists (it runs on
real hosts in block B).
