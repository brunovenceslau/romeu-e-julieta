# Session of 2026-10-21, T153, the Go bump and the work queue

Merged in this session, each with a code review before it opened,
`go run ./tools/ci all` after its last commit and CI green on both
hosts:

- #220 (c9fda531): the hooks test stdin race; its own record is "The
  hooks test stdin race and the procgroup Stdin handover"
  (01m5dvc9s9pj47gf9fh9k39766).
- #221 (955bb4bb): Go 1.27.3, for GO-2026-6809 in `crypto/x509`; the
  four sha256 values of `mise.lock` compared with the `.sha256` files of
  dl.google.com.
- #222 (d1804db0): the config repository's part of T112, two pending
  notes and the T078 tick; its record is "The config repository's part
  of T112 and the payload's rule pull requests"
  (01m5dzn7cr734xrxqc6vzcpgzb).
- #223 (e2b0f4f1): the plan of T153, after two reviews.
- #224 (7335c536): T153, after a code review, a security audit and a
  second round of each; its record is "T153, the gitsafe verbs of the
  snapshot bundle and the system hooks stage"
  (01m5ed3ahrd8b8e0p20606r97g).
- #225 (1b3094ee): T153's tick and the re-plan of T060.

Decided on 2026-10-21: the work queue and its rules while the operator
is away, in "Work authorized while the operator is away on 2026-10-21"
(01m5eg0qefcvwdx05q4vnqcwfe).

Written for other repositories, outside this one: a prompt for the
config repository (its part of T112, done there in its #1 and #2), and
three prompts for the payload's repository, not yet pasted: a recreated
sandbox runs mise and `tools/ci setup` by itself; the sandbox kit's part
of T112; the payload's checkable rules move from prose into mechanisms.

In flight at the end of the session: T060 and the status task, each a
builder in a worktree of its own; now.md names them.

Lessons:

- A finished check run in the working tree an edit is made in reads a
  dirty tree (`working tree` failed once on 2026-10-21): wait for the
  run, then edit.
- A stdlib vulnerability published after a green run turns every later
  pull request red on a step its diff does not touch; read the step's
  output before the diff.
- A deferred decision's trigger can fire in passing (GO-2026-6809 fired
  the scheduled pin-freshness run's); a review that reads the deferral
  list catches it.
