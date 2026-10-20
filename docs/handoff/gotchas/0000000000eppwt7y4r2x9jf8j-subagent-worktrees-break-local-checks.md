# Subagent worktrees break local checks

Claude Code puts the worktrees of subagents with `isolation: worktree`
under `.claude/worktrees/` inside the repository. The `generated` and
lint directive checks walk the disk, not git, so each worktree's
`tools/ci/generate.go` reads as a second `go:generate` line and `fast`
fails locally, even with the directory ignored (2026-10-19). The
pre-push hook refused a push while they existed.

Until the WorktreeCreate hook moves them out
("t112-in-the-config-and-kit-repositories"), push from a worktree
outside the repository (`git worktree add ../<name> <branch>`), and
remove a subagent's worktree once its branch is merged.
