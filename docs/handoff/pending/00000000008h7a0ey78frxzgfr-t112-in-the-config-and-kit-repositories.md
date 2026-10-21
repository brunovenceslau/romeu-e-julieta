# T112 in the config and kit repositories

What is at stake: the names list is a convenience (ADR 0036); a gap
costs one correcting commit.

Two prompts went to the maintainer on 2026-10-19, to paste into the
sessions of those repositories; the config repository's part is done.

- Config repository, closed on 2026-10-20 by brunovenceslau/verona #1
  and #2: the plain list is `names/forbidden.txt`, 4 terms in one
  section, the names. Its header states the format, the matching of
  `tools/ci/names/names.go`: one ASCII term per line, `#` only at the
  start of a line, literal, case-insensitive, matched as a substring,
  a space between words matching any run, empty included, of
  characters other than a letter, a digit or `/`, one line at a time.
  Of the 3 entries of `tools/ci/denylist.yaml`, the list covers 2 (each
  term's segments hashed and compared, counted again here); 2 terms of
  the list are not in the denylist. Decided on 2026-10-20: the phrases
  of the lesson of 2026-10-12 (item aea4a5dd) do not join the list; the
  uncovered denylist entry is no longer needed and goes with the
  denylist; the denylist gains no term.
- Sandbox kit repository, open:
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

Reopen: the sandbox kit repository's session reports its pull request,
or the maintainer asks.
