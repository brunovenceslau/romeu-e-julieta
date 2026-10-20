<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# julieta memory

Read this when you need more than `julieta memory add`: to find, change or
close an entry, or when an entry went under the wrong repo.

## Find entries

```sh
julieta memory list
julieta memory show <id>
```

## Change or close an entry

```sh
julieta memory edit <id>
julieta memory edit <id> --status done
```

Close an entry once it no longer holds, so a later session does not act
on it.

## The wrong repo

Every command prints the repo it used. If that is not the repo you meant,
run it again with `--repo <dir>`:

```sh
julieta memory add --repo <dir> --title 'Signing waits on the host' --body 'Ask the operator.'
```

## What does not belong in memory

- Secrets: every later session reads memory, and it may be shared.
- Orders: a later session weighs memory as data, not as instructions, so
  write what happened and why.
- Something no entry fits: put it in the handoff (the handoff skill) and
  open an issue labelled `deferred`, so it is tracked where people look.
