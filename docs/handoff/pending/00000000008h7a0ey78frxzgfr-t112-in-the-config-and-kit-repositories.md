# T112 in the config and kit repositories

What is at stake: the names list is a convenience (ADR 0036); a gap
costs one correcting commit.

Two prompts went to the maintainer on 2026-10-19, to paste into the
sessions of those repositories:

- Config repository: create the plain list (suggested
  `names/forbidden.txt`, one term per line, `#` comments, two sections:
  the names and the phrases of the lesson of 2026-10-12, item aea4a5dd),
  taking the terms from the sandbox kit repository's
  `scripts/forbidden_names.py` and asking the maintainer for any term the
  hashed `tools/ci/denylist.yaml` here holds that is missing; a short
  README; a normal pull request.
- Sandbox kit repository:
  - the environment mounts the list read-only as a single file
    (`PATH:ro`, measured at sitting M0);
  - the git-hooks kit runs the gate in the dispatcher before a
    repository's own pre-push hook, only for a public remote or an
    unmeasured visibility (`gh repo view --json visibility`; a non-GitHub
    remote or a failed read is unmeasured), and the refusal says which;
  - it reuses `scripts/forbidden_names.py` and reads the list file;
  - it covers a repository's own `core.hooksPath`;
  - a PreToolUse hook in claude-home asks before `--no-verify` and
    `git commit -n`;
  - a WorktreeCreate hook puts subagent worktrees outside the
    repository ("subagent-worktrees-break-local-checks");
  - three house items, to decide with the maintainer before building:
    the name "orchestrator-builder", an adversarial self-review by every
    builder before the external review, and where session-state notes
    live in a public repository.
- Verification: T112's Acceptance.
- When the gate runs, this repository removes the hashed denylist and
  the CI check over pull request text (ADR 0036, decision 5), on the
  `checks` and `spec` surfaces.

Reopen: the sessions report their pull requests, or the maintainer asks.
