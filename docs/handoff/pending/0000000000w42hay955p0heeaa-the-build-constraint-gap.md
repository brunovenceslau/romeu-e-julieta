# The build-constraint gap

- From the security audit of T130: a build constraint such as
  `//go:build ignore` at the top of a `_test.go` file drops its tests
  from every run without a word. The directive scan of
  tools/ci/internal/lint/config_test.go allows `go:build` with any
  expression; allow-listing the expressions (today `unix` only) closes
  it, on the `checks` surface.

Reopen: the operator's choice of the next task, or the next change to
tools/ci/internal/lint.
