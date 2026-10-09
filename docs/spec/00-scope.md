# 0. Scope

Back to [index](../spec.md). Reader: anyone deciding what v1 includes.
Type: explanation.

## 0.1 Objective

Give a developer who works with coding agents in Docker Sandboxes one
organizer for all their environments:

- agents, inside a sandbox, author and maintain **project specs** in a
  config repo (repos, kits, sandbox options, secret names, run layout);
- `romeu` **materializes** those specs into one host tree
  (`$ROMEU_ROOT`, default `$HOME/dev`): plain clones (never submodules),
  generated `sbxenv.yaml`, per-repo memory directories, generated VS Code
  workspace files;
- `romeu run <project>` opens the project's sandbox, with its tools
  installed, straight into its run layout;
- anything that widens what a sandbox can do goes through an approval
  gate on the host; romeu runs nothing an agent wrote on the host unless
  the operator approved it at that gate. This is a property of romeu at
  run time; the checks a maintainer of this repository runs on the host
  are a risk of how the repository is developed
  ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1));
- nothing that salvage covers is lost when a sandbox is recreated or
  removed: the recorded repos (worktrees, local refs, stashes, ignored
  files within the cap) and the agent's state paths
  ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction));
  anything else in the sandbox is not kept. A sandbox that dies
  unplanned keeps what its last snapshot holds, its committed work
  ([08 8.4](08-memory-handoff-salvage.md#84-snapshot-after-every-commit)).

## 0.2 Users

| User | Needs | v1 support |
|---|---|---|
| Primary: a developer (the operator, [01 1.7](01-system-model.md#17-vocabulary)) on a macOS host (Intel or Apple silicon) running Claude Code agents in Linux sandboxes (amd64 or arm64) | one command per project, no hand-maintained env files, nothing that salvage covers lost on sandbox recreate (0.1), no unreviewed host code execution from agent output; prerequisites listed once in [12 12.8](12-engineering.md#128-contributor-docs-outlines) | full |
| Secondary: other developers adopting the public product | generic product, their own config repo, no personal values in code | generic by design (no personal values in code); v1 acceptance runs only with the maintainer's config repo and `examples/`, so a fresh adopter's first run is not tested (a [deferred decision](../spec.md#deferred-decisions)); onboarding documented; no support promise |
| Contributors | a codebase they can trust and extend without tribal knowledge | `ARCHITECTURE.md`, `CONTRIBUTING.md`, generators, error ids with fix hints ([12](12-engineering.md)) |
| Future: other agents | the core must stay agent-unaware | path kept open only (see non-goals) |
| Future: teams adopting AI-assisted development | the problems listed in [0.6](#06-teams-adopting-ai-assisted-development) | none; see 0.6 |

The organization that owns the code is not a user v1 serves; what the
tool stores about a repository, and where, is listed in
[08](08-memory-handoff-salvage.md).

## 0.3 Non-goals (v1)

- Adopting an external memory server; mounting the agent's own memory
  directory from the host.
- Signed or published kits (a
  [deferred decision](../spec.md#deferred-decisions)).
- The runtime ledger (designed in [13](13-runtime-ledger.md), not
  built; a [deferred decision](../spec.md#deferred-decisions)).
- tmux and cmux renderers. The run layout format
  ([03 3.2](03-formats.md#run-layout-run)) names no
  multiplexer; herdr is the one renderer, and the renderer interface is
  designed with the second one.
- herdr agent-team shims (Claude Code split-pane teammates).
- Linux or Windows hosts.
- Real support for agents other than Claude Code.
- A process supervisor (`ready`, `deps`, autorestart) in the run layout.
- Project rename, and repo rename or split inside a project, as
  commands (see J13 for the v1 manual path).

## 0.4 License

Decided by the maintainer:

- **GPL-3.0-only** for the product: version 3 of the GPL only, with no
  "or any later version" option. The license text goes in `COPYING` at
  the repository root together with the first code change.
- **CC0-1.0** for `examples/` and `schemas/`, which users copy into
  their own config repos. `REUSE.toml` records the split; the CC0-1.0
  text joins `LICENSES/` with the first file under either directory,
  since `reuse lint` fails on a license text that no file uses.
- Every file carries an SPDX header (`SPDX-License-Identifier` plus
  `SPDX-FileCopyrightText`) or is covered by `REUSE.toml`; the repo is
  REUSE 3.3 compliant and `reuse lint` runs in `tools/ci`.
- Each release archive contains `COPYING` and a third-party notices
  file that `tools/release build` generates from the module graph; the
  `license` step of `tools/ci` fails a module whose license is not on an
  allowlist compatible with GPL-3.0-only
  ([10 10.2](10-testing-style.md#102-ci)).
- A personal kit copied from a product kit stays GPL-3.0-only; to start
  a kit without that, copy the example kits under `examples/kits/`
  (CC0-1.0).

## 0.5 Scope justification

Every v1 feature traces to a success criterion, an invariant, a
journey or a need the maintainer named for v1; anything that does not
is a row of [Deferred decisions](../spec.md#deferred-decisions). The
table has one row per command of [04](04-cli.md), with the need it
serves on the first day, and `tools/ci sequences` checks that every
command has a row; the rows after the commands are features that are
not commands.

| Feature | Needed by |
|---|---|
| `romeu init` | J1 step 3: the first command on a new machine writes host settings and clones the config repo |
| `romeu sync` | J1 step 6, J2, J8, J11: the one path from a reviewed spec to the host tree, through gate 2 (I7) |
| `romeu approve --toolchain` | gate 1 in J1 step 4 and J9 |
| `romeu run` | J1 step 7, J3: one command per project (0.2) |
| `romeu adopt` | the only way to put a sandbox romeu did not record under management without removing it first (I26, no loss) |
| `romeu stop` | snapshot before stop (no loss), J3a restart path |
| `romeu salvage` | J6 step 3: nothing that salvage covers is lost (0.1), J10, I17; the security review requires it to be complete, not smaller |
| `romeu rm` | J6: a sandbox removed only after its salvage |
| `romeu recreate` | J6, J9, J13: a recreate-class change applied with salvage first |
| `romeu retire` | J7: a project that leaves the config repo leaves the host tree without losing work |
| `romeu status` | J7, J10, J11 |
| `romeu doctor` | I14, I22, I23 and the host checks in 04 (the same checks run in the preflight) |
| `romeu pull` | J4: an agent's work reviewed on the host before it is pushed |
| `romeu handoff` | the sanctioned host read path of memory (I24) |
| `romeu version` | J9: the installed release, which a bug report names |
| `julieta setup` | J3: the project's tools installed when its sandbox starts (0.1) |
| `julieta install` | J8: a tool bump installed inside the sandbox |
| `julieta lock` | J8: every tool locked for both sandbox arches (pin determinism) |
| `julieta hooks run` | I21: each repo's own hooks run inside the sandbox |
| `julieta memory add\|list\|show\|edit` | the per-repo memory of [08 8.1](08-memory-handoff-salvage.md#81-memory-store), which survives the sandbox |
| `julieta memory check` | I24: every memory dir mounted and within the layout allowlist |
| `julieta handoff write`, `julieta handoff show` | J5, J6: a session's narrative and facts reach the next session |
| `julieta handoff list` | resuming a session in a new sandbox: seeing which handoffs the repository has before choosing which one to resume |
| `julieta snapshot` | J4, J10: unpushed work bundled into memory after every commit |
| `julieta salvage` | J6 step 3: the sandbox half of salvage |
| `julieta layout up`, `julieta pane run` | J1 step 7, J3: the run layout; `layout up --dry-run` gives the golden tests of the herdr requests |
| `julieta pin workload` | J9: a workload bump resolved to a digest that covers both sandbox arches |
| `julieta pin check` | pin determinism (principle) |
| `julieta spec validate` | J2 step 1: a spec checked before its pull request; S4 |
| `julieta status` | J4 step 3 and `romeu status`: a sandbox's repo state, read from the host |
| `julieta version` | the compatibility check of the julieta that romeu delivers to a sandbox (Q21) |
| the `git-ssh-sign` product kit | the maintainer signs every commit and will not run an agent that cannot |
| coverage, mutation, fuzz gates | S6, S9 |
| attestations | S1 |
| generators and `go generate` | the middleware principle; S10 |

Memory written before v1 is imported once, by a script in the
operator's config repo, and not by the product.

## 0.6 Teams adopting AI-assisted development

v1 serves one developer and has no team feature. The maintainer's
premise is that teams adopting AI-assisted development come next; the
analysis of the DORA and Google Cloud report behind that premise is in
[the DORA report note](../notes/dora-roi-report.md). Recording the
inputs of the report's return model is a
[deferred decision](../spec.md#deferred-decisions) with its trigger.
