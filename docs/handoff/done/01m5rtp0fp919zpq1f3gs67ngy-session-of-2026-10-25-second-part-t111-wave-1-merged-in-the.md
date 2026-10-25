# Session of 2026-10-25, second part: T111 wave 1 merged in the sandbox kit repository; T038 to be rebuilt from main

Done on 2026-10-25, after the checkpoint of "Session of 2026-10-25: T111
re-planned onto the julieta-claude kit, tools/manifest and T079"
(01m5rnxnxm3xt38f3jewy56g1w). Each pull request and issue number below
is of the sandbox kit repository.

- #291 merged as 98bba2e7: T111 wave 1, edits 1 to 3 of that
  repository's `docs/julieta-by-hand-runbook.md` in the romeu-e-julieta
  and verona definitions. The read-only mount of the shared bin
  directory `$HOME/.local/share/julieta/bin`; the memory
  mount `$HOME/Developer/<project>-env/memory/<project>`;
  `JULIETA_MANIFEST` written by this repository's `tools/manifest`
  `encode`; the `julieta-claude` kit pinned at
  26d49d7539ddbbbbed77a9f6ea08d735cbbfa046 in object form, with
  `julietaBin` set to
  `$HOME/.local/share/julieta/bin/julieta-linux-arm64`
  (`uname -m` read as aarch64 inside a sandbox of that host on
  2026-10-25). Gates: a code review and a security audit before the pull
  request (no Critical or High finding; the findings were fixed in its
  second commit), local CI 29 of 29, hosted CI green.
- #292 merged as 82c1108d: the runbook rebuilds julieta under a `.next`
  name and renames it over the old one (item 20 of #287,
  which had been closed without it, was reopened on 2026-10-25 and is
  closed by #292); step 5 uses the object form with `julietaBin` the
  binary path; step 2's undo says the PreToolUse guard fails closed;
  step 3 says a planted `.git` can reach a host git. Gates: local CI
  29 of 29, hosted CI green. Main of that repository was green at
  82c1108d after both merges.
- Manifests written for the wave: the generation and base of each
  project are in the body of #291, for the runbook's Record;
  the bases are romeu-e-julieta d4290531a42fc571bcd4e666847d88d8c57892d4
  and verona 3b12330fe2558929619852553196ac947bbe606c.
- #294 opened: a test that the bin directory stays disjoint
  from every read-write mount (an audit finding of Low severity, left
  for later).

Decisions, each with its date:

- 2026-10-25: T038 ignores the branch `feat/t038-render-kits` entirely
  and is rebuilt from main without consulting it. This closes the
  pending item 01m5rq1wpvm2vxfgq7ns3zjwjk "The branch
  feat/t038-render-kits needs a full review or a rebuild", whose file
  is removed in this pull request. The branch stays on the remote;
  deleting it is ask-first.
- 2026-10-25: the shared bin directory of sitting M1 is
  `$HOME/.local/share/julieta/bin`, that is
  `$XDG_DATA_HOME/julieta/bin`. The XDG base directories name no
  directory for an app's binaries; the linux binaries are data the
  sandboxes run rather than host executables; and `~/.local/bin` would
  expose every host tool to the mount.
- 2026-10-25: the edits of a T111 wave are written by hand for wave 1
  (the decision 01m5rtp0gnajhg915n01gk08g8 "The edits of a T111 wave
  are written by hand in one pull request for wave 1").
- 2026-10-25: memory storage stays per repository for now; the open
  question is the pending item 01m5rtp0hjczzy9f66ez7rxvna "Memory per
  repository and per project, and where it is kept durably".

New pending items: 01m5rtp0jj3cajgrpsxra2vp0a "T111's Acceptance says
every wrapper exits 0 with the bin mount absent" and
01m5rtp0kfzjckbdct0y77531j "The julieta-claude kit's root install step
acts on the agent-owned ~/.claude/skills".
