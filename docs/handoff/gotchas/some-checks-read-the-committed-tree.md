# Some checks read the committed tree

`fast`'s commit checks, `reqs`, `schema`, `workflows`, `hygiene`, `kits`
and the views of `plan` read the tree at HEAD (`check.HeadTree`), not the working tree. On
2026-10-20 `go run ./tools/ci reqs` passed on an uncommitted change that
left REQ-0173 cited by no task, and the pre-push hook refused the push
once it was committed.

Commit first, then run `go run ./tools/ci all`; a check run on
uncommitted changes says nothing about them.
