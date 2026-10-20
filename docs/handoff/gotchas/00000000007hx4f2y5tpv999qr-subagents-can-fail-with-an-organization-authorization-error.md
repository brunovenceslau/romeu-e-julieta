# Subagents can fail with an organization authorization error

On 2026-10-13 every subagent of a six-agent run stopped with HTTP 403
`oauth_org_not_allowed` (subscription access disabled for Claude Code)
while the orchestrating session kept working. Work a subagent had begun
stayed in its worktree, uncommitted.

Before a fan-out, launch one small probe agent and read its result.
After a failure, measure each worktree (git status and log) before
relaunching: some agents commit before they stop.
