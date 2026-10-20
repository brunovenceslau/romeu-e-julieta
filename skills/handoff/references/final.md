<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# /handoff --final, before the sandbox goes

The sandbox is about to be recreated or removed, so anything only it holds
is lost. The handoff itself is not: julieta writes it to the repo's memory
dir, which lives on the host and is only mounted into the sandbox, so the
next sandbox of the project reads it. Use these six headings, each as a `## ` line, in this order:

```markdown kind=final
## Goal
## State
## Decisions
## Next steps
## Open questions
## What would be lost
```

- Goal: what the work is for, in a line or two.
- State: what is done, what is half done, the branches and open pull
  requests.
- Decisions: what was decided, with its date and time, so the next
  session does not reopen it.
- Next steps: the first thing to do, then the rest, in order.
- Open questions: what is unknown or waits on the operator.
- What would be lost: what only this sandbox holds and is not pushed
  (commits, stashes, untracked files, local state), each by name, so the
  operator can save it before the sandbox goes.

Then write it with `julieta handoff write --final`, as the skill says.
