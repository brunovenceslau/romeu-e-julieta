# A scripted rebase on a full disk can commit conflict markers

On 2026-10-23 a loop that regenerated pages and continued a rebase
committed `<<<<<<<` markers when `go generate` failed for lack of disk
space.

Before any push, run
`git grep -n -e '^<<<<<<<' -e '^>>>>>>>' $(git rev-list origin/main..HEAD) -- .`
and resolve code conflicts by hand, not by script.
