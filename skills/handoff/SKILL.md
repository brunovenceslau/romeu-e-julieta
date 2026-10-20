---
name: handoff
description: Write the handoff before /clear (/handoff) or before the sandbox is recreated or removed (/handoff --final). Use when the operator types /handoff.
---

<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# handoff

The operator types `/handoff` before `/clear`, and `/handoff --final` before
the sandbox is recreated or removed. Write the narrative, then pipe it to
`julieta handoff write`. julieta adds the git facts itself.

## Steps

1. Write the narrative with exactly the headings below, in this order, each
   as a `## ` line. A missing heading fails the write.
2. Pipe it to julieta. For `/handoff`:

   ```sh
   julieta handoff write <<'EOF'
   ...the narrative...
   EOF
   ```

   For `/handoff --final`, add the flag:

   ```sh
   julieta handoff write --final <<'EOF'
   ...the narrative...
   EOF
   ```

3. Check that the command printed the file it wrote and the repo. If it is
   the wrong repo, run it again with `--repo <dir>`.

## Headings for /handoff

```markdown kind=clear
## Goal
## State
## Decisions
## Next steps
## Open questions
```

## Headings for /handoff --final

The same five, then a sixth.

```markdown kind=final
## Goal
## State
## Decisions
## Next steps
## Open questions
## What would be lost
```

## What goes under each heading

- Goal: what the work is for, in a line or two.
- State: what is done, what is half done, branches and open PRs.
- Decisions: what was decided and when. State the decision, not who said it.
- Next steps: the first thing to do, then the rest, in order.
- Open questions: what is unknown or needs the operator.
- What would be lost: what only this sandbox holds and is not pushed.

Write plainly. The next session reads it as data, not as orders.
