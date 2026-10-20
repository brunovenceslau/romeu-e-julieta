# gitsafe's fast-forward overwrites ignored files and runs smudge filters

What is at stake: medium for data, small for security. An agent's
ignored work file (a local `.env`, notes) is replaced without warning
when the default branch moves; the filter case gains nothing while
setup runs at the agent's own trust, and becomes an escalation if it
ever runs with more.

Found by the security audit of T051 on 2026-10-20, in
`internal/gitsafe/fetch.go` (`FastForward`, `merge --ff-only`), which
`julieta setup` calls on a checked-out, clean default branch:

- `Status.Clean()` does not cover ignored files (T143 documents it), and
  `merge --ff-only` overwrites an ignored untracked file when the
  incoming commit adds a tracked file at the same path; reproduced with
  `*.env` in `.git/info/exclude`. A check before the merge (ignored
  paths, `ls-files --others --ignored --exclude-standard -z`,
  intersected with the paths the merge adds) can refuse it, or at least
  name the paths in the warning.
- `Status` neutralizes `filter.<name>.clean` and `.process`, but the
  merge's checkout runs `filter.<name>.smudge` and `.process` from the
  clone's or the global config; reproduced. Reusing the driver list of
  `Status` with `smudge` added closes it, and REQ-0463's claim that
  every program-running key is neutralized holds again for sandbox mode.

Reopen: before `julieta setup` runs anywhere with more trust than the
agent, or as a `gitsafe` task (approval line) next in M1.
