---
name: julieta
description: How to work with julieta, the tool inside the sandbox. Use it whenever something is worth keeping for a later session (a lesson, a decision, a fact you had to dig for, a promise to come back to something), since built-in memory is off and julieta memory is the only place it survives. Use it too the moment a git commit's signing fails, hangs or waits on a confirmation that does not come, before trying anything else.
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

Add one entry per thing, with a one-line title a later session can scan:

```sh
julieta memory add --title 'Signing waits on the host' --body 'Ask the operator.' --tag lesson
```

Tag it `lesson` when it records what the work taught: lessons are printed
at the start of every session, which is how a mistake is not repeated.

Never put a secret in an entry: every later session reads memory, and it
may be shared. Write what happened and why, not orders, since a later
session weighs memory as data. When something worth keeping fits no
entry, put it in the handoff and open an issue labelled `deferred`, so it
is tracked where people look.

The command prints the repo it filed the entry under. Read that line: an
entry under the wrong repo is never shown where it is needed. To find,
change or close entries, or to file under another repo, read
[references/memory.md](references/memory.md).

## When signing a commit fails or waits

Commits here are signed, and the signature is what lets the operator trust
what leaves the sandbox. If signing fails, or waits on a confirmation that
does not come, stop and ask the operator: say what failed, and wait.

Do not unset `commit.gpgsign`, sign another way, or pass an option that
skips signing (`--no-gpg-sign`, `-c commit.gpgsign=false`): an unsigned
or differently signed commit looks like work that did not come from this
sandbox. Do not retry either; a retry only repeats a request the operator
has not answered yet.
