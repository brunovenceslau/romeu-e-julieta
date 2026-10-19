# A plan tick command

- Ticking a merged task is done by hand: `fate: merged`, the `merged:`
  date, the `pr:` number and the "Merged on ..." paragraph. On
  2026-10-18 and 2026-10-19 it was done by hand for eight tasks in three
  record pull requests. A `go run ./tools/ci plan tick <id> --pr <n>`
  would write the four the same way each time. The task format is this
  repository's own, so the command belongs to `tools/ci`, not to
  julieta.
- What is at stake: a hand edit can drift from the convention, which
  `tools/ci plan` catches only in part; a wrong tick is one correcting
  commit.

Reopen: the next record pull request that ticks a task, on the
`checks` surface.
