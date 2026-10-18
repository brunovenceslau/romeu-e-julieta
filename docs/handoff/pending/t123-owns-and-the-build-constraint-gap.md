# T123 owns, and the build-constraint gap

- T123 (the records of the product decisions): its acceptance takes
  each row out of the table of 12 12.5 and edits the task files of
  T035, T040 and T077, but `owns` names only `docs/adr/0015-*.md` to
  `0021-*.md`. A plan fix adds those pages (rule 6 of docs/plan.md).
  Its records are product decisions to read before they leave Proposed.
- From the security audit of T130: a build constraint such as
  `//go:build ignore` at the top of a `_test.go` file drops its tests
  from every run without a word. The directive scan of
  tools/ci/internal/lint/config_test.go allows `go:build` with any
  expression; allow-listing the expressions (today `unix` only) closes
  it, on the `checks` surface.

Reopen: the operator's choice of the next task, or the next change to
tools/ci/internal/lint.
