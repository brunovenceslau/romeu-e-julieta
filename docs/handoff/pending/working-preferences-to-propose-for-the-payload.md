# Working preferences to propose for the payload

Session preferences that lived only in the sandbox's agent memory, to
be proposed for the house payload in a PR of their own:

- Operator decisions go out as a quiz (options, the recommended one
  first, grouped in one block), each preceded by a plain explanation:
  what the thing is, what breaks and what the user sees, with file and
  line.
- A resume prompt is handed over in the conversation and in the
  reminders store, never as a comment on a pull request.
- Each piece of work goes out as a stack: the pull request that does
  the change, and on top of it the one that records it done (the task
  file's `fate`, `merged:` and `pr:`, the Done record, now.md), opened
  together; the operator merges the stack with a merge commit (asked
  for on 2026-10-17).
- Pull requests merge with a merge commit; squash and rebase merges are
  off, and the remote branch is deleted on merge.
- While CI runs, start the next safe task of the highest value; waiting
  idle beats filler work.

Reopen: the next payload PR.
