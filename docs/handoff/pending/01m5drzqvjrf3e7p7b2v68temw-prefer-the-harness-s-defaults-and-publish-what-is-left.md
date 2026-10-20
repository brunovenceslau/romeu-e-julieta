# Prefer the harness's defaults and publish what is left

What is at stake: medium. Each customization of the harness is code to
keep in step with it; one the harness already covers drifts from it.

Scope: this repository and the config repository. Here, a decision
record once the open questions below are answered; in the config
repository, its personal kit (T114, REQ-0608), through "Rules to move
when the interim payload is retired" (00000000001v2n0qea13kr8yvy).

Decided on 2026-10-20: romeu-e-julieta and the config repository use the
features the harness exposes by default as far as they go, keep their
own customizations as few as possible, and publish each customization
once it is validated.

Ways to hold it, proposed:

- The **Prior art** section of the pull request template also names the
  harness feature that covers the change, and why it is not enough.
- Every code review brief asks the same of a diff that adds a hook, a
  skill, a setting or a kit.
- A CI check that asks for that line when the diff touches `skills/`,
  `kits/` or hooks, only if the first pull requests show the question is
  forgotten (the `checks` surface).

Candidates to review under it, none decided:

- The agent's built-in memory is turned off and replaced by `julieta
  memory` (`docs/adr/0021-keep-memory-as-plain-per-repo-files.md:14`,
  `docs/spec/06-kits.md:125`).
- `julieta handoff`: the harness's `/compact` and `--resume` cover part
  of `/clear`, not a recreated sandbox.
- The payload's hooks (the brief guard among them), when they move.

Open for the operator:

1. Does "harness" mean Claude Code alone, or the sbx sandbox too?
2. What counts as validated? Proposed: CI green and used in one real
   session without a correction.
3. Where is a customization published? Proposed: a public plugin
   marketplace repository, for what the payload and the config
   repository hold today.

Reopen: when the operator answers; then a decision record here and a
line for T114.
