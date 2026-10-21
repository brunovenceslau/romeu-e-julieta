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
  project spec, host settings, the manifest) stays strict, since an
  unknown member there is most often a typing error.

REQ-0163 and the strict readers between binaries still say otherwise
until "Readers of formats between binaries ignore unknown members"
(01m5egnpyfywadcsxpy0naz3hj) is built.
