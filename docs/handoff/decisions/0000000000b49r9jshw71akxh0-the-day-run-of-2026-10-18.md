# The day run of 2026-10-18

Decided on 2026-10-18, for the run that starts after GitHub stack #136
merged. The rules of "the-overnight-run-of-2026-10-18" still hold
(stacking with no limit, a new dependency at its newest version once its
license is read, any other ask-first step skipped and recorded, merging
stays the operator's), and these answers are added.

- Order: the record pull request of stack #136 (Done record 36), T132, one decisions pull request, T010, T125
  after its re-plan, T139, T123, T012, the four items below, then the
  tasks T125 unblocks, stacked.
- REQ-0971: option (a) of "tools-new-imports-internal-cli", one
  sentence that lets `tools/new` import `internal/cli`.
- The lint set of T139: option A with the `tidy` rule, the agreed
  default of item 16 of Decisions of the re-plan; sitting M0 no longer
  holds it.
- T125: the three recommendations of "t125-three-design-questions" (a
  committed version file that `tools/release` also reads, the fixture
  schema written in a test, a re-plan that gives T125 the pinned lists).
- T123: the six records are drafted; the operator reads them in the
  pull request, and the merge is the approval.
- T012: mise's minisign public key is read from mise's official
  release, and the pull request states its fingerprint, which the
  operator confirms out of band before the merge. If the network
  policy blocks the download, the `sbx policy` command is handed over
  and the run goes on.
- Items of now.md done as pull requests of their own: the globs of the
  `checks` surface, the cobra completion fix, I29 widened to
  Default_Ignorable_Code_Point, and `golang.org/x/text` in REQ-1049
  with the NFC and NFD check of the tree walk.
- The label `deferred` (T124) is created through the API.
- When every next step waits on the operator, small pending notes that
  are not ask-first are done, one pull request each.
