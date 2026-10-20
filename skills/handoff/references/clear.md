<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# /handoff, before /clear

The sandbox and its unpushed work stay; only the context goes. Use these
five headings, each as a `## ` line, in this order:

```markdown kind=clear
## Goal
## State
## Decisions
## Next steps
## Open questions
```

- Goal: what the work is for, in a line or two.
- State: what is done, what is half done, the branches and open pull
  requests.
- Decisions: what was decided, with its date and time, so the next
  session does not reopen it.
- Next steps: the first thing to do, then the rest, in order.
- Open questions: what is unknown or waits on the operator.

Then write it with `julieta handoff write`, as the skill says.
