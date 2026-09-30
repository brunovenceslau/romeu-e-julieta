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
