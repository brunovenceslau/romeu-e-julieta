# T077 waits for review on feat/t077-os-base, then its pull request

What is at stake: medium. T077 heads the critical path, and T038 and
T079 wait on it; until its pull request opens, nothing after it starts.

Scope: this repository.

The branch `feat/t077-os-base` holds T077 rebased onto main of
2026-10-23 (0121b247): the julieta kit, the os-base kit with tzdata and
zsh, `kits/embed.go` embedding julieta, os-base and the git-ssh-sign
descriptor, the go:embed lint allowance for `kits/embed.go`, the A12
result in the task file, and a new pending note for the open parts of
A12. It replaces the older branch `feat/t077-julieta-kit`, left as it
was.

Before the pull request opens:

- Run `go run ./tools/ci all` on the branch; the kit e2e passed after
  `deb.debian.org:80` was approved, the full run not yet.
- One code review, with a security focus on the os-base install step.
- A close review of the I7 placeholder guard change in `approve`'s test
  (T042's file, outside T077's owns).
- Squash the two kit commits, or keep them: the first alone fails
  `kits/embed_test.go`.
- Rebase onto main: the branch edits now.md and removes "T077 waits for
  the A12 measurement" (01m5gywvj5vj90yfedah9y670b).
- The body has the approval lines of `kits` and `checks`.

Reopen: the next session; remove this note in the pull request of T077.
