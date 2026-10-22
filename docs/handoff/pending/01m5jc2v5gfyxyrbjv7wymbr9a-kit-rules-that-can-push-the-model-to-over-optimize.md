# Kit rules that can push the model to over-optimize

What is at stake: medium. A rule that rewards doing more than the task
asks, such as a boy scout rule (leave the code better than you found
it), can widen a diff past its task, cost review time and context, and
mix unrelated changes into one pull request; each wording is easy to
change.

Scope: every repository and sandbox of the maintainer. The rules live
in the payload the sandbox kit writes (`kits/claude-home/spec.yaml` of the
sandbox kit's repository, rendered to `~/.claude/CLAUDE.md`), and later in the config
repository's personal kit.

To do: review the kit's rules for any that can push the model to
over-optimize or widen scope, the boy scout rule first. For each: the
rule, where it lives, the behavior it can cause, and a narrower
wording. The spec and plan of this repository may be used, and a
subagent for the read. Not found by a search of `~/.claude`,
`~/.sbx-kit` and `docs/` of this repository on 2026-10-22, so it is
likely in the kit's source.

Reopen: the next session on the sandbox kit's repository, or any
session that has it cloned.
