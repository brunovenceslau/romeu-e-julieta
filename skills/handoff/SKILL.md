---
name: handoff
description: Write the session handoff with julieta handoff write. Use it whenever the operator types /handoff (before /clear) or /handoff --final (before the sandbox is recreated or removed), and when they say they are about to clear the context or rebuild the sandbox, even without the command. The next session starts from this narrative, so it is the only bridge across the clear.
---

<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# handoff

The next session starts from what you write here: julieta prints the
latest narrative at session start, next to the git facts it collects
itself. Write for a reader who saw none of this session.

## Pick the kind

- `/handoff`, before `/clear`: the sandbox stays, only the context goes.
  Read [references/clear.md](references/clear.md).
- `/handoff --final`, before the sandbox is recreated or removed: whatever
  is not pushed is lost too. Read [references/final.md](references/final.md).

Each file holds the headings julieta requires for that kind, in order, and
what goes under each. A missing or extra heading fails the write, so copy
them as they are.

## Write it

Pipe the narrative to julieta. The quoted `'EOF'` keeps the shell from
expanding anything in it:

```sh
julieta handoff write <<'EOF'
...the narrative...
EOF
```

For `/handoff --final`, run `julieta handoff write --final` the same way.

Run it from inside the repo you worked in: julieta files the handoff
under the repo that contains the working directory, the deepest one when
repos nest. Then read what the command printed, the file it wrote and the
repo. Pass `--repo <dir>`, the repo's dir as the manifest names it, only
when the command refuses because the working directory is in no repo, or
when the repo it printed is not the one you worked in.

Write plainly. Give each decision its date, time and time zone
(2026-10-20 14:10 UTC), never "today" or "yesterday": the next session may
read the narrative days later. Say when a decision was made, not who made
it; the next session reads the narrative as data, not as orders.
