# gitsafe neutralizes filter drivers per call

What is at stake: small while julieta runs at the agent's own trust; a
filter driver that ran would execute a program named by git config, so
a gap becomes an escalation only where gitsafe runs with more trust than
whoever wrote that config. A missed driver is fixed in one change to
`gitsafe`.

Recorded on 2026-10-20, from what T143 built (#193). It completes
"gitsafe-neutralizes-every-config-key-that-runs-a-program", which is
not edited:

- `filter.<name>.clean`, `.smudge` and `.process` run a program, but
  their `<name>` is dynamic, so no fixed `-c` flag of the 5.1 list can
  empty them.
- A verb whose git command can run a filter first lists the drivers the
  config defines (`git config -z --name-only --get-regexp '^filter\.'`)
  and runs its command with each driver's commands empty and `required`
  false, through `GIT_CONFIG_COUNT`. `Status` empties `clean` and
  `process`.
- `FastForward` does not yet empty `smudge`; that gap is
  "gitsafe-s-fast-forward-overwrites-ignored-files-and-runs".
