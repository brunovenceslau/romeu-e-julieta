# A julieta command that prunes merged branches

Deleting a branch is ask-first in the sandbox payload, so merged local
branches and their worktrees pile up until a recreate. Decided on
2026-10-16: deletion moves to a deterministic command that refuses
what has not merged, a julieta command (04 4.3), for example
`julieta branch prune`. It is deferred: julieta has no git commands of
this kind yet.

What makes it simple:

- This repository allows the merge commit only; squash and rebase
  merges are off, and that stays (measured with `gh repo view`:
  mergeCommitAllowed true, squashMergeAllowed false,
  rebaseMergeAllowed false). A merged branch's head is then the
  second parent of a merge commit on the first-parent history of
  `origin/main`, so git alone proves the merge, with no GitHub API
  call. Being an ancestor of `origin/main` is not enough: a branch
  created at main with no commit of its own, or left behind it, is an
  ancestor too and has not merged.
- The command fetches first, never prunes the default branch or a
  checked-out branch, and a stale `origin/main` only makes it prune
  less.
- GitHub deletes the remote branch on merge (deleteBranchOnMerge
  true), so the command deletes only the local branch and its
  worktree, never a remote one.
- Salvage bundles every local branch (08 8.5); a branch whose head is
  in `origin/main` loses nothing when deleted. The future spec row
  states it.

Open when it reopens: the REQ text, whether a worktree that is dirty or
holds untracked files blocks the deletion (the default: it blocks), and the interim
script in the claude-home kit next to handoff_state.py, until julieta
ships the command.

Reopen: julieta gains its git commands, or the operator asks for the
interim script.
