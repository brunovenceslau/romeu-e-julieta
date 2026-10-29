# Remove the deprecated members of julieta-setup at its next version

Decided on 2026-10-28 with T177: `julieta setup` moves no branch
(REQ-0399), and its `--json` report keeps the format `julieta-setup/v1`
so a reader that already integrates with it keeps working. By REQ-1310
(03 3.1), `fastForward`, `from` and `to` stay in v1, deprecated and
always empty, and the new member `branchState` carries the state of the
default branch. If this is wrong, the cost is three empty members in
every report until v2, not a broken reader.

When the format gets a new version for another reason, remove the three
members; their names are never used again.

Scope: this repository, the `julieta-setup` format; written in 03
(REQ-1310), 04 4.3 (REQ-0399) and the comments of
`internal/cmd/julieta/setup/report.go`.

Reopen: a change to `julieta-setup` that is a new version under
REQ-0163.
