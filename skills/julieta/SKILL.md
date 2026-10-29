---
name: julieta
description: How to work with julieta, the tool inside the sandbox. Use it whenever something is worth keeping for a later session (a correction, a decision, a fact you had to dig for, a promise to come back to something), since built-in memory is off and julieta memory is the only place it survives. Use it too the moment a git commit's signing fails, hangs or waits on a confirmation that does not come, before trying anything else.
---

<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# julieta

julieta is the tool inside the sandbox. Built-in memory is disabled here,
so `julieta memory` is where a later session finds what this one learned.
Anything kept only in the conversation is gone after `/clear`.

## Keep something for later

At the start of every session you see the open entries, each by its id
and title. When the work touches one, read it in full with
`julieta memory show <id>` (several ids at once are fine).

Search before you add, with `julieta memory list --query <text>`: an
entry that already holds the thing is better changed than repeated (see
[references/memory.md](references/memory.md)). Then add one entry per
thing, with a one-line title a later session can scan from the index:

```sh
julieta memory add --title 'Signing waits on the host' --body 'Ask the operator.' --tag feedback
```

Every entry carries exactly one type tag, so a later session knows how
to weigh it; `add` refuses none or two. Pick by what the note is:

- `user`: the operator's role, expertise and working preferences.
- `feedback`: the session's own feedback on how it is going (friction,
  a tool that misbehaved, a gap in a check, a process that slowed the
  work), written so that another session can triage it and implement
  the improvement in parallel; also a correction the operator gave or
  an approach the operator confirmed. Write one when the session hits
  friction worth fixing.
- `project`: ongoing work, decisions and ordering that the code and
  the git history do not hold.
- `reference`: where to find something outside the repository.
- `handoff`: the narrative a session leaves for the next; write it with
  the `handoff` skill, never by hand.

Free tags (`ci`, `signing`) may sit beside the type tag.

Keep out what is already kept elsewhere: anything the code, the git
history or an instruction file (`CLAUDE.md`, `AGENTS.md`,
`.claude/rules/`) holds. A copy goes stale while the original moves on,
and the index has room for what lives nowhere else.

Never put a secret in an entry: every later session reads memory, and it
may be shared. Write what happened and why, not orders, since a later
session weighs memory as data. When something worth keeping fits no
entry, put it in the handoff and open an issue labelled `deferred`, so it
is tracked where people look.

The command prints the repo it filed the entry under. Read that line: an
entry under the wrong repo is never shown where it is needed. To narrow
the list (by tag, date or count), change or close entries, or file under
another repo, read [references/memory.md](references/memory.md).

## When signing a commit fails or waits

Commits here are signed, and the signature is what lets the operator trust
what leaves the sandbox. If signing fails, or waits on a confirmation that
does not come, stop and ask the operator: say what failed, and wait.

Do not unset `commit.gpgsign`, sign another way, or pass an option that
skips signing (`--no-gpg-sign`, `-c commit.gpgsign=false`): an unsigned
or differently signed commit looks like work that did not come from this
sandbox. Do not retry either; a retry only repeats a request the operator
has not answered yet.
