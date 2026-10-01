# 10. Testing strategy, CI, code style

Back to [index](../spec.md). Reader: implementers and reviewers of any
module. Type: reference. Tools are code: `tools/*`, `e2e/probes`,
`e2e/fakesbx`, workflows, schemas and kits are held to the same lint,
test, coverage and review rules as `internal/*`.

## 10.1 Test levels

| Level | Location | Runs where | Covers |
|---|---|---|---|
| Unit | `internal/**`, `tools/**`, `e2e/probes/**`, `e2e/fakesbx/**` `_test.go` | CI, dev | validators generated from `rules.go` (table + fuzz seeds committed), digests, widening-set and toolchain diffs, the generated state-machine tables (every row, every illegal pair), egress split, `termsafe`, shell quoter, sbx output parsers over every recorded sbx version, error-id table, doctor checks with `HOME` in a temp dir, the probe harness core and its sbx exec layer (against a helper binary re-executed from the test), the recorder's redaction, the fake sbx's placeholder normalizer, CI tools themselves (the forbidden-name matcher and its pushed-range walk over a fixture repository, `workflows` against fixture workflow files, the linter configuration against a fixture package with one violation per checker of rule 14 in [ADR 0001, the documentation standard](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md), invariants/mutate, acceptance against a recorded GitHub API fixture, `pr` against recorded event payloads) |
| Golden | `internal/render/testdata/`, `internal/layout/testdata/`, `internal/gate/testdata/` | CI, dev | `sbxenv.yaml`, workspace files, `render.json`, gate diff text, herdr `layout.apply` requests (`layout up --dry-run`), handoff front matter |
| Schema | `tools/ci schema` | CI | generated schemas equal the committed ones; examples and testdata validate |
| E2E (git + fake sbx) | `e2e/*_test.go`, tag `e2e` | CI (linux amd64, linux arm64, macOS Intel, macOS arm64) | romeu commands against a temp `$ROMEU_ROOT`; origins served by `git http-backend` behind `httptest` TLS (host settings `gitHosts[].caFile` points at the test CA; gitsafe has no test override); the sandbox daemon served by `git daemon` on `127.0.0.1`; journeys J2, J3b, J7, J10, J11 (scenario functions shared with the host suite); promotion fault injection; invariants marked E in [05](05-security.md), including the I27 hostile trees on the macOS runners |
| E2E (hybrid) | `e2e/hybrid_test.go`, tag `e2e` | CI on `ubuntu-latest` and `ubuntu-24.04-arm` | the romeu-julieta contract: the fake sbx forwards `env exec` into the julieta container with `.romeu/bin` bind-mounted read-only at the host path; protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each stop `run` before `layout up`; a 32 KiB manifest round-trips (I30) |
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
`tools/ci acceptance` and `tools/ci links` outside `all`; flaky tests
are fixed, never skipped.

## 10.2 CI

`.github/workflows/ci.yml` calls `go run ./tools/ci all`, which is also
what developers run locally before a PR; the tracked `.githooks/pre-push`
runs the `fast` subset.

**Every merge gate also runs locally.** A merge gate is a check whose
failure blocks a merge. Workflows hold no check logic of their own, and
`tools/ci workflows` enforces that with a closed grammar over
`.github/workflows/*.yml`. A key or a value outside it fails:

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

`tools/ci all` runs the `pr` step when `GITHUB_EVENT_NAME` is
`pull_request`, on the file that `GITHUB_EVENT_PATH` names. A local run
is `go run ./tools/ci pr <file>`, on a payload saved beforehand (with
`gh api`, for example); `pr` itself makes no network call. The title,
the body, the head ref and the base and head SHAs come from that file,
which is the event or the pull request object that is the event's
`pull_request` member. Commit messages, added lines and changed paths
come from git over base..head. A file with neither shape fails.

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
| hygiene | `tools/ci hygiene`: no U+2014; the prose rules and `TODO(#<issue>)` form of ADR 0001 (rules 4, 14); no personal absolute paths; the forbidden-name check over the path and content of each tracked file (below); scans `e2e/testdata/sbx/**` too | yes |
| pushed range | `tools/ci fast` with the pre-push hook's arguments: the forbidden-name check over the ref names and commits of a push (below) | in the hook only |
| sequences | `tools/ci sequences`: ADR numbers contiguous and unique; ADR layout and statuses per ADR 0001 (rules 6-7), the filename compared through the `slug` function `tools/new adr` uses; every `Supersedes` link in an ADR's Status section matches a `Superseded by` link in the target ADR and the reverse; every ADR that `docs/spec.md` or `docs/spec/` cites has status Accepted; spec section, invariant, probe, question and journey ids unique; every referenced id and journey range (for example "J1-J13" in a success criterion) exists | yes |
| vocabulary | `tools/ci vocabulary` (01 1.7) | yes |
| workflows | `tools/ci workflows`: the grammar above | yes |
| vulnerabilities | `govulncheck ./...` | |
| unit + golden + race | `go test -race -coverprofile=cover.out ./...` | |
| coverage | `tools/ci coverage` (S9) | |
| golden governance | `tools/ci golden`: fails if `-update` appears in CI invocations; lists every changed golden in the job summary for review | |
| schema | `tools/ci schema` | |
| e2e | `go test -tags e2e ./e2e/...` | |
| cross-build | darwin amd64/arm64 (`romeu`), linux amd64/arm64 (`julieta`) | |
| imports | `tools/ci imports` (I1, I2, I11, I24 call sites, 10.7) | |
| invariants | `tools/ci invariants` (every I-id has a guard tag and a test tag) | |
| mutation | `tools/ci mutate` on PRs touching a path in `.github/ask-first.yaml`; the scheduled `fuzz.yml` runs it too | |
| catalog | `tools/ci catalog` (explicit upload flags, key syntax) | |
| kits | `tools/ci kits` (one frontend pin; install steps <= 5 lines; every download has a sha256) | |
| mise | `tools/ci mise` (`julieta lock --check` logic on this repo's and the examples' locks) | |
| probes | `tools/ci probes` (11 11.4) | |
| license | `reuse lint` (REUSE 3.3) | |
| docs | `tools/ci docs` (S10; `--help` output vs `docs/reference/`; the checks ADR 0001 assigns to it: rules 1, 5 without external URLs, 12, 13, 17, 20) | |
| lessons | `tools/ci lessons`: every `docs/lessons.md` entry, read after the file's front matter, names an existing `tools/ci` subcommand or test name, or says "no check possible: <reason>" | |
| pr | `tools/ci pr` (pull requests only; inputs as described above; 12 12.4; ADR 0001 rules 4, 8, 11; the forbidden-name check, below) | |

Runners: `ubuntu-latest`, `ubuntu-24.04-arm`, an Intel macOS runner (the
label is verified to exist and to report `x86_64` in the `ci-bootstrap`
task) and `macos-latest` (arm64).

Three subcommands are outside `all`:

- `tools/ci acceptance` needs the network (GitHub API). It runs in the
  final plan task and in `release.yml`, and its unit tests use a
  recorded API fixture.
- `tools/ci links` needs the network too: it fetches every external URL
  the docs cite (ADR 0001 rule 5). No workflow calls it in v1; a person
  runs it. Relative links and anchors stay in `tools/ci docs`, offline.
- `tools/ci fuzz` runs each fuzz target for a fixed time, longer than
  the short runs inside `go test`. The scheduled `fuzz.yml` calls it,
  and then `tools/ci mutate`.

### Forbidden names

`tools/ci/denylist.yaml` lists names this repository must not contain:
in a file, a path, a commit message, a ref name or the text of a PR.
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
- Text is matched one line at a time. A ref name is one line.

| Surface | Checked by | When |
|---|---|---|
| the path and the content of each tracked file at HEAD | `tools/ci hygiene` | in `fast` and in `all` |
| each pushed ref name; the message and the added lines of each pushed commit | `tools/ci fast`, called by the pre-push hook | before the push leaves the machine |
| the PR title, body and head ref name; the message and the added lines of each commit in base..head | `tools/ci pr` | on the pull request |

The hook is `exec go run ./tools/ci fast "$@"`. Git gives a pre-push
hook the remote's name and URL as arguments and, on stdin, one line per
pushed ref: local ref, local sha, remote ref, remote sha. Called with
those arguments, `fast` reads the lines and walks, for each, the
commits reachable from the local sha and not from the remote sha; for
a ref the remote does not have yet, the commits not reachable from any
remote-tracking ref of that remote. A line that deletes a ref adds no
commits and is skipped. Called without arguments, `fast` checks the
tracked files and no range. `pr` uses the same walk over base..head.
Reading the lines each commit adds, and not only the final tree, is
what catches a name that one commit adds and a later commit removes.

The limits, stated plainly:

- Outside a sandbox the hook is opt-in (12 12.4), and
  `git push --no-verify` skips it. `pr` is the backstop, and it runs
  after the push: a name pushed without the hook is on the remote
  before any check reads it.
- Matching works on bytes with ASCII lowercasing. A name written with
  look-alike characters of another script, or split across two lines,
  is not matched.
- The messages of annotated tags and git notes are not read.

An entry is written by `go run ./tools/ci hygiene add`. It reads the
name from stdin, one line with a space between segments; it takes no
name as an argument and prints none. It lowercases the name, writes the
segments through the matcher's own code, and before writing proves that
the matcher finds the name in a buffer in memory. A hand-computed entry
that is wrong matches nothing, and no test can see that, because the
name is not in the tree; that is why the command exists. The self-test
uses a made-up name with its own denylist: it plants the name written
together, with each separator and with a slash, and expects a failure
for each form except the slash.

What the hash buys is small, and we say so. It is a plain hash, not a
keyed one, so it hides nothing from someone who guesses a name and
hashes it. It only keeps the plaintext out of the tree, so that a
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
notes (12 12.6); `tools/release publish` creates the GitHub release.
The attestation is a `uses` step and each of the others is one `run`
step, so the file passes `tools/ci workflows`.

`ci-bootstrap` (first plan task), in this order:

1. One commit adds `tools/ci hygiene` with the denylist, `tools/ci
   fast` and `.githooks/pre-push`. Nothing can check the commit that
   adds the checker, so its author runs `go run ./tools/ci fast` on it
   and puts the output under the PR's Evidence. From the next push on,
   an enabled hook reads each pushed commit.
2. `tools/ci workflows` and a green workflow on the public repo,
   proving runners, labels and permissions.
3. The rest of the day-one middleware of
   [12 12.3](12-engineering.md#123-generators) and
   [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change),
   `tools/ci pr` included.

Between steps 1 and 3 the hook is the one check on commit messages and
ref names, and julieta's dispatcher does not exist yet, so the hook is
opt-in for each clone of that period.
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
operator keeps their own acceptance ledger (for example
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
| `gitsafe`, `spec`, `canon`, `catalog`, `memstore`, `handoff`, `salvage`, `shquote`, `termsafe`, `cli` | both | `sbxdrv`, `layout` |

## 10.8 Definition of done for implementing agents

The Always / Ask first / Never list in the
[index](../spec.md#boundaries-for-everyone-who-changes-this-repo)
applies. A task is done only when `go run ./tools/ci all` passes, its
invariant tags are in place, the generated files are current, and, for
host-facing behavior, the matching scenario function exists (it runs on
real hosts in block B).
