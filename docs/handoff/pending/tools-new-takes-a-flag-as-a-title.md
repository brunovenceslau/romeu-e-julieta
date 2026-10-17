# tools/new takes a flag as a title

`go run ./tools/new adr --help` wrote `docs/adr/0014-help.md`, a record
titled `--help`, instead of printing the usage (found on 2026-10-17,
the file removed before any commit). A title that starts with `-`
should be refused with the usage and exit 2.

Reopen: the next change to tools/new.
