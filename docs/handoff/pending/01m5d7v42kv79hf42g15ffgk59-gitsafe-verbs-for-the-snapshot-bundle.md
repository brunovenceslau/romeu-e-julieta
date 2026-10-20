# gitsafe verbs for the snapshot bundle

What is at stake: medium; T060 (`julieta snapshot`) cannot be built, and
T111, the cut-over of M1, needs it.

Scope: this repository.

Found on 2026-10-20 when T060 was started, by reading the code:
REQ-0688's bundle holds tags, notes, the current generation's
`refs/salvage/<generation>/*`, each worktree HEAD and each reflog-only
commit as `refs/salvage-reflog/<branch>/<n>`, but `internal/gitsafe`
lists only `refs/heads` (`Branches`), reads no reflog, and
`BundleCreate` takes only existing refs; `CreateRef` is create-only and
limited to `refs/salvage/` in sandbox mode, and git refuses a bundle of
a raw object id ("Refusing to create empty bundle", git 2.53.0). A
`gitsafe` task (ask first) adds: a ref listing over allowlisted
namespaces, a reflog read, and a bundle of name and object id pairs
built without writing refs into the user's repository (for example in a
temporary bare repository that borrows the objects through
alternates), with the empty-bundle case handled.

T060 also needs the store lock and atomic write that T059 exports
(#213), and its `disabled` acceptance item touches `setup`, `status` and
the SessionStart print, outside its owns.

Reopen: when that `gitsafe` task is planned; then T060 is re-planned on
it.
