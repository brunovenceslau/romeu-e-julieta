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
  gate on the host; nothing an agent wrote is ever executed on the host;
- nothing sandbox-only is lost when a sandbox is recreated or removed.

## 0.2 Users

| User | Needs | v1 support |
|---|---|---|
| Primary: a developer on a macOS host (Intel or Apple silicon) running Claude Code agents in Linux sandboxes (amd64 or arm64) | one command per project, no hand-maintained env files, nothing lost on sandbox recreate, no host code execution from agent output | full |
| Secondary: other developers adopting the public product | generic product, their own config repo, no personal values in code | the product is generic; onboarding documented; no support promise |
| Contributors | a codebase they can trust and extend without tribal knowledge | `ARCHITECTURE.md`, `CONTRIBUTING.md`, generators, error ids with fix hints ([12](12-engineering.md)) |
| Future: other agents | the core must stay agent-unaware | path kept open only (see non-goals) |
| Future: teams adopting AI-assisted development | the problems listed in [0.6](#06-teams-adopting-ai-assisted-development) | partial, and stated row by row in 0.6; no team feature in v1 |

## 0.3 Non-goals (v1)

- Adopting an external memory server; mounting the agent's own memory
  directory from the host.
- Signed or published kits (the publishing path is designed in
  [06](06-kits.md), not built).
- tmux and cmux renderers (the run-layout renderer interface exists;
  only herdr is implemented).
- herdr agent-team shims (Claude Code split-pane teammates).
- Linux or Windows hosts.
- Real support for agents other than Claude Code.
- A process supervisor (`ready`, `deps`, autorestart) in the run layout.
- Project rename, and repo rename or split inside a project, as
  commands (see J13 for the v1 manual path).

## 0.4 License

Decided by the maintainer:

- **GPL-3.0-or-later** for the product. The license text goes in
  `COPYING` at the repository root together with the first code change.
- **CC0-1.0** for `examples/` and `schemas/`, which users copy into
  their own config repos. `REUSE.toml` records the split.
- Every file carries an SPDX header (`SPDX-License-Identifier` plus
  `SPDX-FileCopyrightText`) or is covered by `REUSE.toml`; the repo is
  REUSE 3.3 compliant and `reuse lint` runs in `tools/ci`.

## 0.5 Scope justification

Every v1 feature traces to a success criterion, an invariant or a
journey; anything that does not is marked v1.1.

| Feature | Needed by |
|---|---|
| snapshot and salvage | "nothing sandbox-only is lost" (objective), J6, J10, I17; the security review requires it to be complete, not smaller |
| `romeu stop` | snapshot before stop (no loss), J3a restart path |
| `romeu adopt` | the only way to put a sandbox romeu did not record under management without removing it first (I26, no loss) |
| `romeu doctor` | I14, I22, I23 and the host checks in 04 (the same checks run in the preflight) |
| `romeu status` | J7, J10, J11 |
| `romeu handoff` | the sanctioned host read path of memory (I24) |
| `--timings` | S8 |
| coverage, mutation, fuzz gates | S6, S9 |
| attestations | S1 |
| `julieta pin check` | pin determinism (principle) |
| `julieta layout up --dry-run` | golden tests of the herdr requests |
| generators and `go generate` | the middleware principle; S10 |
| v1.1 | listing sbx's native egress approval queue in `status`; `romeu handoff --list` filters; Remote-SSH helper command; an explicit registry credential (Q23) |

## 0.6 Teams adopting AI-assisted development

v1 is built for one developer. The maintainer's premise is that the
people it should serve next are teams that adopt AI-assisted
development, and one report describes them at length: DORA and Google
Cloud, "The ROI of AI-assisted Software Development", v. 2026.1
(2026), linked from [dora.dev](https://dora.dev/ai/roi/report/). DORA
is a research program run by Google Cloud, and Google sells AI and
consulting. We label that and do not discount the report for it.

The report addresses technology leaders who must justify AI spending
to boards and finance (p.8), with teams of product, engineering, UX
and site reliability roles (p.55). It presents no new data of its own.
So each row below says how the report knows what it says: **measured**
is a survey or study the report cites, second-hand here and not read
by us; **argued** is the authors' own reasoning. Then, for each: what
in this spec addresses it, and what does not.

| Problem the report attributes to these teams | Page; how the report knows | What this spec has | Gap |
|---|---|---|---|
| A gain in productivity is perceived, and the return is unclear | p.18, measured: more than 80% of survey respondents perceived a gain; p.11-12, argued from three cited studies that disagree | nothing | the product measures no team's delivery. `tools/ci dora` ([12 12.10](12-engineering.md#1210-delivery-metrics)) measures this repository only |
| A verification tax: time spent reviewing generated code, which the authors call "the most immediate barrier to ROI" | p.9, 20, 33, 40; argued | partial. Checks that need no reviewer: julieta's dispatcher runs each repo's own hooks inside the sandbox (I21), and the host gate shows a reviewer the widening changes as one complete diff (I7, I29) | the product does not review code and does not account for the time or cost of verifying it |
| Software delivery instability rises with adoption | p.19, 25; the association is measured, +0.097 with an 89% credible interval of +0.067 to +0.129 (p.20, Figure 4); that the rise is temporary is argued | nothing | the product records no failure or rework of a team's releases |
| Friction and burnout do not fall | p.20, measured: both effects have intervals that cross zero | nothing | both are perceptions of people; the product sees none, and a survey is outside its scope |
| Individual gains pile up at manual review gates and brittle pipelines | p.13, 19; argued | partial. The host approval gate asks only for changes that widen what a sandbox can do, and shows them in one diff ([01](01-system-model.md)) | the queue of pull requests in front of a team's reviewers is untouched |
| Loss of familiarity with the codebase | p.33; argued | partial. Per-repo memory and handoff files keep what a session learned and survive the sandbox ([08](08-memory-handoff-salvage.md)) | nothing measures familiarity, and the documentation standard of this repository applies to this repository, not to a team's |
| A dip after adoption, of unknown depth and duration, that leadership reads as failure | p.8-10, 30; argued | nothing | no dated marker of an adoption or a change exists, so there is no before and after to compare |
| No baseline, so no attribution | p.4; argued | nothing | as the first row |
| "Shadow AI": people turn to unauthorized tools when sanctioned ones do not deliver | p.12; measured, from a study the report cites | partial. A sanctioned sandbox whose egress is derived from a catalog, with anything beyond it behind the host gate ([07](07-mise-egress.md)) | the report's cause is sanctioned tooling that fails to help. Nothing here shows that this product helps, so we do not claim it solves this |
| Weak context makes the agent produce bloat | p.41, 44; argued | partial. Memory, handoff and the skills that tell an agent to use them ([08](08-memory-handoff-salvage.md)) | the quality of a team's own documentation is the team's work |
| Low trust deepens the dip; teams need "a clear and communicated AI stance" | p.40; argued | nothing. The security model ([05](05-security.md)) is a reason to trust the tool, which is a different thing from an organization's stance | a stance is a policy of an organization toward its engineers, not a feature |
| The ongoing cost moves to governance | p.36; argued | nothing | as the verification tax: the cost is not accounted |

Five rows have a partial answer and seven have none. That is the
honest count for a product whose v1 serves one developer.

One thing we do not claim. The report's return-on-investment model
needs a team's own numbers: time saved per change, failures and
recoveries of its releases, the cost of its tools, dated markers of
what changed and when. The product does not let a team fill that model
from its own data. It would need data gathered across more than one
machine, and this spec has none. Recording those inputs is a
[deferred decision](../spec.md#deferred-decisions) with its trigger;
computing a return on investment is not planned at all.
