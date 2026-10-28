<!--
SPDX-FileCopyrightText: 2026 Bruno Venceslau
SPDX-License-Identifier: GPL-3.0-only
-->

# julieta memory

Read this when you need more than `julieta memory add`: to find, change or
close an entry, or when an entry went under the wrong repo.

## Find entries

Search before you add: an entry that already holds the thing is better
changed than repeated.

```sh
julieta memory list --query 'signing'
julieta memory list
julieta memory show <id> [<id>...]
```

`list` prints one line per entry, newest change first: the id, the
repo, the status, the type tag (`-` for an entry written before type
tags), the date of the last change and the title. `--query` matches an
entry whose title, body or a tag holds the text, ignoring case, so try a
word or two the entry would use before you conclude there is none. These
narrow the list, and combine:

```sh
julieta memory list --status open
julieta memory list --tag feedback --tag ci   # every tag given must hold
julieta memory list --since 2026-10-28        # changed on or after; a timestamp works too
julieta memory list --limit 20
julieta memory list --sort created            # newest added first
```

## Change or close an entry

Name what changes; an edit that names nothing is refused:

```sh
julieta memory edit <id> --title 'Signing waits on the host'
julieta memory edit <id> --status done
```

The tags you give with `--tag` replace the entry's tags, so give the
type tag again with them; an edit whose tags hold no type tag, or two,
is refused. An entry written before type tags (listed with the type
`-`) has none, so every edit of it, closing it too, names its tags and
gives it one:

```sh
julieta memory edit <id> --tag project --tag ci --status done
```

Close an entry once it no longer holds, so a later session does not act
on it.

## The wrong repo

Every command prints the repo it used. If that is not the repo you meant,
run it again with `--repo <dir>`:

```sh
julieta memory add --repo <dir> --title 'Signing waits on the host' --body 'Ask the operator.' --tag feedback
```
