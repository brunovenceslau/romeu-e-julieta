# Rename fastForward and drop from and to at the next bump of julieta-setup

Decided on 2026-10-28 with T177: `julieta setup` moves no branch
(REQ-0399), and its `--json` report keeps the format `julieta-setup/v1`
so a reader that already integrates with it keeps working (03 3.1,
REQ-0163, the protobuf rules). Each repo's `fastForward` now names the
state of the default branch against origin's, with `behind` where
`moved` was, and `from` and `to` stay, always empty. If the name is
wrong for readers, the cost is a misleading member name, not a broken
reader.

When the format is bumped to v2 for another reason, rename
`fastForward` (for example to `defaultBranch` state) and drop `from` and
`to`, whose names are then never used again.

Scope: this repository, the `julieta-setup` format; written here and in
the comments of `internal/cmd/julieta/setup/report.go`.

Reopen: a change to `julieta-setup` that is a new version under
REQ-0163.
