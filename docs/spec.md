# Spec: romeu e julieta v1

Reader: the planner and implementers of romeu e julieta v1, and
reviewers of the design. Type: reference index; start here, then read
[01](spec/01-system-model.md).

Status: **draft v1 specification, review round 2 applied**. It is the
source of truth for the implementation plan and describes the current
state only. How it got here lives in [reviews/](reviews/round-2.md);
once the product has ADRs, every settled decision is an ADR and a spec
change that reverses one adds an ADR that supersedes it
([12 12.5](spec/12-engineering.md#125-decisions-and-history)).

romeu e julieta organizes development environments built on Docker
Sandboxes (`sbx`). **romeu** runs on the host: a deterministic,
auditable binary that turns reviewed project specs into one directory
tree and drives `sbx`. **julieta** runs inside each sandbox: tools (via
mise), git hooks, memory, handoff, salvage and the in-sandbox terminal
layout. No agent and no LLM ever runs on the host.

## Index

| # | File | Covers |
|---|---|---|
| - | this file | principles, success criteria, boundaries, open questions |
| 0 | [spec/00-scope.md](spec/00-scope.md) | objective, users, non-goals, license, scope justification |
| 1 | [spec/01-system-model.md](spec/01-system-model.md) | components, sources of truth, trust boundaries, gates, preflight, state machines, vocabulary |
| 2 | [spec/02-layouts.md](spec/02-layouts.md) | repositories and directory layouts, in-sandbox paths, multi-repo decision |
| 3 | [spec/03-formats.md](spec/03-formats.md) | file formats, validation rules, digests |
| 4 | [spec/04-cli.md](spec/04-cli.md) | romeu and julieta commands, flags, exit codes, error ids |
| 5 | [spec/05-security.md](spec/05-security.md) | hardened git, security invariants and their tests, ask-first surfaces, residual risks |
| 6 | [spec/06-kits.md](spec/06-kits.md) | kits v3 (local builds), workload, julieta delivery, product mixins, publishing path |
| 7 | [spec/07-mise-egress.md](spec/07-mise-egress.md) | mise integration, egress catalog and derivation |
| 8 | [spec/08-memory-handoff-salvage.md](spec/08-memory-handoff-salvage.md) | memory store, session hooks, handoff, snapshot and salvage |
| 9 | [spec/09-journeys.md](spec/09-journeys.md) | developer journeys, step by step |
| 10 | [spec/10-testing-style.md](spec/10-testing-style.md) | test levels, CI, fake-sbx contract, acceptance evidence, code style, module boundaries |
| 11 | [spec/11-host-probes.md](spec/11-host-probes.md) | host probes, the probe result format and its lifecycle |
| 12 | [spec/12-engineering.md](spec/12-engineering.md) | tech stack, dev commands, capability map, generators, middleware, contributor docs, text standard |

## Principles

### Deterministic where it can be, the model where it adds value

Anything that must happen the same way every time is code, a gate or a
hook - never a model's memory or goodwill: sequence identifiers,
timestamps, validations, schema and path checks, hashes, pins,
generated files, ordering, formatting, and every security invariant.
Agents do what they are best at: understanding intent, designing,
writing and reviewing specs and code, and judgment calls. Every place
where an agent is asked to "remember" something mechanical is a defect:
move it into romeu, julieta, a hook, a generator or a CI check, and let
the agent consume the result.

### Tools are code

Everything we write is held to production standard: the CLIs, and
equally `tools/*`, tests, CI workflows, schemas, kits, docs, and text
outside the repository (issues, PR bodies, review comments, commits,
release notes). There is no "just a script" exemption. Every change
leaves the area better than it was found, and each iteration records
what it taught as a lesson, and as a check when one is possible. The bar
is very high and pragmatic: quality is spent where it protects users,
security or future change. A decision that no longer fits is revisited
openly and superseded explicitly, never silently worked around.

### Mechanical middleware on the code that produces code

Before and after every change, the same machinery runs: generators
create the skeleton of a new ADR, invariant, probe, kit, command or
lesson with the next free id; `go generate` derives every table that has
a single source; a tracked pre-push hook and CI enforce the checks; the
PR body states which check or generator the change added or improved
(or why none). The before/after question "what would have caught this,
and does it exist now?" is a required field, not a habit
([12 12.4](spec/12-engineering.md#124-middleware-before-and-after-every-change)).

### Simple and explicit

Count concepts; prefer merging two into one over adding a third. Every
lifecycle is an explicit state machine with a transition table, one
name per state, illegal transitions refused, and table-driven tests
([01 1.6](spec/01-system-model.md#16-state-machines)). Every rule with
more than one consumer has exactly one source and the consumers are
generated from it. The result must be trusted by its users and inviting
to contributors: a newcomer reads `ARCHITECTURE.md`, runs one command
to scaffold what they add, and gets a precise error id with a fix hint
when something is wrong.

## Success criteria (measurable)

Evidence for each criterion is listed in `docs/acceptance.json` (schema
`acceptance.v1`, see [10 10.5](spec/10-testing-style.md#105-acceptance-evidence))
and verified by `go run ./tools/ci acceptance`, the last plan task.

| ID | Criterion | Evidence |
|---|---|---|
| S1 | v1.0.0 released: `romeu` darwin/amd64 + darwin/arm64, `julieta` linux/amd64 + linux/arm64, `checksums.txt`, build provenance attestations | release asset list; `gh attestation verify --signer-workflow <repo>/.github/workflows/release.yml` passes for each archive |
| S2 | CI on GitHub Actions green on the release commit (all jobs, all runners in 10.2) | workflow run id with conclusion `success` |
| S3 | local v3 kits (`julieta`, `julieta-claude`, `os-base`, `git-ssh-sign`) build and start a sandbox on both sandbox arches: linux/amd64 on an Intel Mac host and linux/arm64 on an Apple silicon Mac host | probe results `docs/probes/B2-<host>.json` from both hosts |
| S4 | the egress catalog covers every `backend:tool` in the reference config repo's projects | `julieta spec validate --catalog projects/*.yaml` reports 0 unknown, run in the config repo's CI |
| S5 | the `julieta-claude` mixin installs `skills/julieta` and `skills/handoff`, the SessionStart and SessionEnd hooks, and makes the agent's own memory dir non-writable | probe results `docs/probes/B2-<host>.json` |
| S6 | every security invariant in [05](spec/05-security.md) has a tagged guard and a tagged test that fails when the guard is disabled | `tools/ci invariants` (every id has both tags) and `tools/ci mutate` (each guard stub turns its test red) |
| S7 | every journey in [09](spec/09-journeys.md) passes: J2, J3b, J7, J10, J11 in CI with the fake sbx; J1-J13 on a real host | CI run; probe results `docs/probes/B3-<host>.json` |
| S8 | `romeu run <p>` on a running sandbox with unchanged locks: romeu's own overhead <= 2 s (median of 5, excluding time inside `sbx` subprocesses); `julieta setup` no-op <= 3 s (median of 5) | CI: `run --timings --json` with the fake sbx, and the timed container e2e; host confirmation in `docs/probes/B5-<host>.json` |
| S9 | coverage >= 80% statements across `internal/...`, `tools/...`, `e2e/probes/...` and `e2e/fakesbx/...`; >= 90% for `gitsafe`, `gate`, `spec`, `render`, `egress`, `termsafe`, `state`, `canon` | `tools/ci coverage` |
| S10 | docs: README, `ARCHITECTURE.md`, `CONTRIBUTING.md`, `docs/guide/*` per journey, generated `docs/reference/*` per command, file format, error id and exit code, `docs/adr/*` for the decisions in this spec | `tools/ci docs` and `tools/ci generated` |
| S11 | REUSE 3.3 compliant; every file has an SPDX header or a `REUSE.toml` entry | `reuse lint` inside `tools/ci` |

## Boundaries (for everyone who changes this repo)

- **Always**
  - Route every git call through `internal/gitsafe`, every sbx call
    through `internal/sbxdrv`, and every host output through the CLI
    writer, which escapes with `termsafe` by default.
  - Emit YAML and JSON only through marshallers; never `text/template`
    for structured files.
  - Validate every name and path field from a spec before use.
  - Tag every invariant guard and its test with the invariant id
    (`//romeu:invariant I<n> guard|test`) and change both in the same
    commit.
  - Scaffold with `go run ./tools/new ...` and regenerate with
    `go generate ./...`; the tracked pre-push hook runs
    `go run ./tools/ci fast`; run `go run ./tools/ci all` before opening
    a PR; review golden diffs.
  - English, Conventional Commits, no em dash character, SPDX headers;
    commit, PR, issue and review text follow the text standard
    ([12 12.7](spec/12-engineering.md#127-text-standard-and-lessons)).
  - Record what a change taught in `docs/lessons.md` and turn it into a
    `tools/ci` check when it can be one.
- **Ask first**
  - Any change to a path listed in `.github/ask-first.yaml`
    ([05 5.3](spec/05-security.md#53-ask-first-surfaces)); it covers the
    gates, gitsafe, termsafe, digests, signing, the catalog's upload
    flags, kits and pins, release and publishing paths, Go dependencies,
    CI actions, file format versions, and the exit-code and error table.
- **Never**
  - Add a code path in `romeu` that configures git hooks, runs mise,
    executes repository content, or opens a terminal pane on the host.
  - Pass `--auto-approve` to sbx.
  - Put a secret value, a host command, a user name or an absolute
    personal path in the product repo.
  - Delete a clone, a memory directory or a salvage ref from romeu.
  - Hand-edit a generated file.
  - Add a name listed in the `tools/ci hygiene` denylist.

## Open questions

Each has a stated default that the spec already follows. A question
leaves this table when it is settled: by a committed probe result
([11 11.4](spec/11-host-probes.md#114-probe-lifecycle)) or by a
maintainer decision, and in both cases it becomes an ADR.

| # | Question | Default taken (recommendation) | Settled by |
|---|---|---|---|
| Q2 | Workload kit: Docker's `sbx-kit-claude` by digest, or our own | **Docker's by digest**, with a host-settings allowlist of workload repositories | probes A12, B2 |
| Q4 | Same repo in two projects | **forbidden** in v1 | maintainer |
| Q6 | Project rename / repo rename or split | manual path (J13) | maintainer |
| Q7 | v3 milestone instability | frontend pinned by digest; probe A11 is go/no-go before kits are written; no v2 fallback | A11 |
| Q8 | Disabling Claude Code's own memory | `julieta-claude` makes the agent memory dir non-writable and the hooks check it; plus the setting if the pinned version has one | C3, B2 |
| Q9 | romeu distribution | GitHub release + checksums + mandatory attestation verify; Homebrew tap later | maintainer |
| Q10 | Host git floor | patched point release per series, initial floor 2.45.4 or newer; exact list verified in A1 | A1 |
| Q11 | Remote-SSH live view | documented journey only | A7 |
| Q12 | Salvage of ignored files and transcripts | ignored files included (1 GiB cap, spec excludes are acknowledged loss); transcripts excluded unless `--include-transcripts` | maintainer |
| Q13 | Per-project fine-grained tokens | `name@project` host-settings entries; sharing a command is allowed but written per project | maintainer |
| Q14 | Memory across machines | machine-local in v1 | maintainer |
| Q15 | Manifest delivery | `sbx env exec --env JULIETA_MANIFEST=...`, with a compatibility check | A4 |
| Q16 | herdr pre-1.0 churn | pinned by version + sha256; protocol number checked, fail closed | A13 |
| Q17 | Idle auto-stop vs `sync --from sandbox/...` | start the sandbox first when stopped | A8 |
| Q18 | Secrets change on a live sandbox | **recreate-class** (safe); switch the `apply` tag to live `sbx secret set --command` if A10 shows the next `env run` keeps it | A10 |
| Q19 | How sbx forwards the dedicated signing socket | romeu sets `SSH_AUTH_SOCK` to `signing.agentSocket` for the sbx calls of a project that uses `git-ssh-sign`; the refusal rule of [06 6.4](spec/06-kits.md#64-product-kits) is decided, only the forwarding mechanism is measured | A12, B2 |
| Q20 | Workload and kit capability discovery | `internal/oci` reads the descriptor annotation by digest over HTTPS; replaced by an sbx inspect command if A12 finds one | A12 |
| Q21 | Sandbox arch equals host arch | romeu selects `julieta-linux-<GOARCH>` from its own `GOARCH`; the compatibility check fails closed otherwise | A4 |
| Q22 | Claude Code hook order on `/clear` | SessionEnd fires before SessionStart; the handoff rules of [08 8.3](spec/08-memory-handoff-salvage.md#83-handoff) hold in either order, the probe confirms the tested order is real | C5, B3 |
| Q23 | Registry access for descriptors | anonymous HTTPS reads only, cached by digest; an explicit host-settings credential is added only if a private workload registry is needed | maintainer |
