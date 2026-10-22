# The payload lost the subagent context reuse rule

What is at stake: medium. Without the rule, an orchestrator resumes a
subagent whose context is already half full, so its later work runs
with less room and a higher chance of compaction or lost detail; the
fix is one bullet of text.

Scope: every repository and sandbox of the maintainer. The rule
belongs in the payload the sandbox kit writes
(`kits/claude-home/spec.yaml` of the sandbox kit's repository,
rendered to `~/.claude/CLAUDE.md`), and later in the config
repository's personal kit.

Measured on 2026-10-22: `~/.sbx-kit/claude-home/bin/ctx-usage` still
backs "Reuse a subagent's context before spawning a new one" (ADR-0022
of the sandbox kit's repository, Decision 10: resume an agent only while
its usage is at or below 30% of its model's context window, otherwise
spawn a fresh one with a self-contained brief), but the bullet lived in
the retired house agreement and the current `~/.claude/CLAUDE.md` does
not carry it. In this repository's session of 2026-10-22, builders were
resumed at 40% to 53%.

To do: put the bullet back in the payload, naming `ctx-usage` as the
measurement, and decide whether a hook enforces it on SendMessage.

Reopen: the next session on the sandbox kit's repository.
