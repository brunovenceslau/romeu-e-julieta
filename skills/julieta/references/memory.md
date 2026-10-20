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

Name what changes; an edit that names nothing is refused:

```sh
julieta memory edit <id> --title 'Signing waits on the host'
julieta memory edit <id> --status done
```

The tags you give with `--tag` replace the entry's tags, so repeat
`--tag lesson` when you change the tags of a lesson, or it stops being
printed at session start.

Close an entry once it no longer holds, so a later session does not act
on it.

## The wrong repo

Every command prints the repo it used. If that is not the repo you meant,
run it again with `--repo <dir>`:

```sh
julieta memory add --repo <dir> --title 'Signing waits on the host' --body 'Ask the operator.'
```
