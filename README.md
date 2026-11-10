# Farewell, fun little thing

Deciding what not to build is the most important decision an
experienced engineer makes, and a fun hobby project is where it is
hardest: the building itself is the reward, so there is never an
obvious moment to stop. With agents doing the building, the fun never
runs out. This project showed me that in work like this the stop has
to come from a rule I set for myself, before I need it.

The finding is about agent loops. Agents are tireless at producing
specification and at approving it: a reviewer agent finds a gap, and
the gap becomes a requirement. None of them asks whether anyone uses
the thing yet. In a hobby project that question is mine to ask, so it
has to be a gate in the process, not a line in a plan. Meanwhile the
ground moved: Docker Sandboxes, still pre-1.0, shipped faster than a
wrapper around it could follow, and Claude Code's own memory made
julieta's unnecessary.

The useful parts did not stop with it: they moved to other projects
and run there every day. One signing key for commits, git hooks in
every repo, a pinned toolchain, the guard that keeps an agent from
merging or tagging, and a measurement discipline that writes the tool
version, the date and the command next to every claim.

Why it mattered: I now know from my own work, not from a blog post,
where agent-driven work goes wrong: it produces volume, approves volume,
and never asks for real use. When the only feedback is the system's own
(green pull requests, approving reviewers, a growing spec), every day
looks productive. The practice I carry forward: real use is a gate
before more spec, and the call to stop is made against usage, not
against progress.

Thank you for reading. Goodbye, fun little thing: you taught me more
than you ever shipped.

# romeu-e-julieta

**Status: paused on 2026-11-09 and archived. The repository is read-only;
no issue or pull request will be answered.** This page is the
retrospective: what the project set out to do, what was built, why it
stopped and where its pieces went.

## What it set out to do

romeu-e-julieta set out to organize development environments built on
Docker Sandboxes (`sbx`). **romeu** runs on the host: a deterministic,
auditable binary that turns reviewed project specs into one directory
tree and drives `sbx`, with no agent and no LLM ever running on the
host. **julieta** runs inside each sandbox: the pinned tools (through
mise), git hooks, memory, handoffs, salvage of unpushed work and the
terminal layout. The full design is in [docs/spec.md](docs/spec.md) and
[docs/spec/](docs/spec/).

## What was built

Everything below is merged to `main`, about six weeks of work from the
first commit on 2026-09-30. No release was ever tagged: `VERSION` reads
1.0.0, but the repository has no tags and
[CHANGELOG.md](CHANGELOG.md) has no section.

- **julieta** ([cmd/julieta](cmd/julieta/),
  [internal/cmd/julieta](internal/cmd/julieta/)): `setup`, `install`,
  `lock`, `status`, `spec validate`, `snapshot`, `salvage`,
  `memory add|check|edit|list|show`, `handoff list|show|write` and
  `hooks run|guard`.
- **The guard** (`julieta hooks guard`,
  [internal/cmd/julieta/hooks/guard.go](internal/cmd/julieta/hooks/guard.go)
  and [guard_rules.go](internal/cmd/julieta/hooks/guard_rules.go)): a
  Claude Code PreToolUse hook that refuses a pull request merge, a `v*`
  tag, a release, and `gh api` changes to rulesets or repository
  settings, so those stay the maintainer's actions.
- **Memory and handoffs**: per-repo entries with a session-start hook
  that prints the open handoff ([internal/memstore](internal/memstore/),
  [internal/memgit](internal/memgit/),
  [internal/handoff](internal/handoff/)), and the `julieta` and
  `handoff` skills ([skills/](skills/)).
- **Kits** for `sbx` ([kits/](kits/)): `julieta`, `julieta-claude`
  (julieta's hooks and skills, with Claude Code's own memory turned off
  in [managed-settings.json](kits/julieta-claude/managed-settings.json)),
  `git-ssh-sign`, `os-base`, plus the signed mise checksums in
  [kits/mise](kits/mise/) and the pins in [kits/pins.yaml](kits/pins.yaml).
- **romeu** ([cmd/romeu](cmd/romeu/),
  [internal/cmd/romeu](internal/cmd/romeu/)): `init`, `approve`, `sync`,
  `status`, `doctor` and `upgrade` exist in the code.
- **The CI tool** ([tools/ci](tools/ci/)): 25 checks, one entry point
  (`go run ./tools/ci all`, with the jobs `fast`, `static`, `race` and
  `e2e`), listed in [docs/reference/checks.md](docs/reference/checks.md),
  including a hashed forbidden-name check and the approval-line check of
  [.github/ask-first.yaml](.github/ask-first.yaml).
- **The plan viewer** ([tools/planviewer](tools/planviewer/)), below.

## What was specified but not built

The plan ([docs/plan.md](docs/plan.md)) holds 217 task files in
[docs/plan/tasks](docs/plan/tasks/). By the `fate` field of their front
matter on 2026-11-10: 110 merged, 47 open (36 `new`, 11 `kept`), 39
`deferred`, 7 `left-v1`, 9 `folded` into other tasks and 5 `removed`.
The specification holds 1409 requirement ids (`REQ-NNNN`) and went
through 18 rounds of agent review ([docs/reviews/](docs/reviews/)).

## Why it stopped

On 2026-11-09 the project was paused in favor of Claude Code's own
memory and persisted state. julieta's memory and handoffs existed to
carry state across sessions and sandbox rebuilds; Claude Code's auto
memory, persisted on the host, now does that without a separate tool.
The parts that proved useful every day were small enough to live in
the sandbox configuration itself, and the host side wrapped a
dependency that was still changing under it.

## Where each piece went

These moved into the maintainer's sandbox configuration, outside this
repository:

- **The guard**: ported from the Go hook above to a fail-closed Python
  PreToolUse hook in the maintainer's sandbox payload.
- **Memory and handoffs**: Claude Code auto memory, persisted on the
  host.
- **The environment layout**: kept in the maintainer's sandbox
  configuration instead of a tree rendered by romeu.
- **julieta's old memory** is kept read-only; nothing writes to it.

## What is worth reusing

The ideas that held up, each with its record in [docs/adr/](docs/adr/)
(status as recorded; a `Proposed` record was never adopted):

- Deterministic where it can be: anything that must happen the same way
  every time is code, a hook or a check, never an agent's memory
  ([docs/spec.md](docs/spec.md), Principles).
- A documentation standard with checkable rules and a voice:
  [ADR 0001](docs/adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)
  (Accepted).
- Decide at the last responsible moment and record the trigger that
  reopens a decision:
  [ADR 0005](docs/adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)
  (Accepted).
- State what is at stake and the scope of every decision record:
  [ADR 0037](docs/adr/0037-state-what-is-at-stake-in-every-decision-record.md)
  and [ADR 0039](docs/adr/0039-state-the-scope-of-every-decision-record.md)
  (both Accepted).
- Search for a maintained library before writing one:
  [ADR 0038](docs/adr/0038-search-for-a-maintained-library-before-writing-one.md)
  (Accepted).
- Plain per-repo memory files:
  [ADR 0021](docs/adr/0021-keep-memory-as-plain-per-repo-files.md)
  (Accepted).
- The re-plan for early daily use:
  [ADR 0014](docs/adr/0014-re-plan-v1-for-early-daily-use-task-files-registration-by-file-build-order-dogfood-before-recording.md)
  (Proposed), worth reading next to the farewell below.

The code is GPL-3.0-only, and `examples/` and `schemas/` are CC0-1.0
([LICENSES/](LICENSES/)).

## The plan viewer

`go run ./tools/planviewer` writes `plan-viewer.html` at the repository root (git ignores it) and prints its path. The page is one file with no network use: each task of `docs/plan/tasks` is a node, colored by status, with its dependencies as edges. Run it again to refresh it. `go run ./tools/planviewer -ready` prints the ready ids, the same as `go run ./tools/ci plan --ready`.
