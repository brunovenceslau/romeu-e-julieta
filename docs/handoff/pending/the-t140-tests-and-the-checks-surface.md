# The T140 tests and the checks surface

T140 added `tools/ci/internal/moduletree` and
`tools/ci/internal/identifiers` (#93). Their repository tests fail a
push whose spec and code disagree, but no glob of the `checks` surface
in `.github/ask-first.yaml` matches the two directories, so a change to
them needs no approval line. Adding both globs is a change to the
ask-first list, an ask-first surface itself.

The packages `tools/ci/internal/reqs` and `tools/ci/internal/format`
match no glob of `checks` either, so one decision can cover all four.

Recommended: add the globs to `checks`, in a pull request of its own.

Reopen: the operator's answer, or the next change to either package.
