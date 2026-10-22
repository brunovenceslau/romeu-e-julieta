# Formats evolve by the rules of protobuf

Decided on 2026-10-21, while T154 added a member to `julieta-status/v1`.
Scope: this repository, for every format the product writes.

- Adding a member is backward compatible and does not change a format's
  version, as adding a field is in protobuf.
- Removing or renaming a member, or changing its type or meaning, is a
  new version; a removed member's name is never used again.
- A reader of a format one binary writes and another reads (for example
  `julieta-status`, `snapshot/heads.json`, the handoff files) ignores a
  member it does not know. A reader of a file a person writes (the
  project spec, host settings) stays strict, since an unknown member
  there is most often a typing error. (The manifest was listed here by
  mistake: romeu writes it. See the last paragraph.)

Built by T162 on 2026-10-22: REQ-0163 says it, and the readers of the
formats between binaries skip an unknown member
([ADR 0041, readers of formats between binaries ignore unknown members](../../adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md)).
The manifest is written by romeu and read by julieta. The maintainer
decided on 2026-10-22 (T163, ADR 0041) that julieta's reader skips an
unknown member of `manifest.v1` too, with one exception: a member an
older julieta could not safely ignore (one that restricts, denies or
changes what julieta must do) bumps the protocol number, and the window
of REQ-0219 refuses a julieta that is too old.
