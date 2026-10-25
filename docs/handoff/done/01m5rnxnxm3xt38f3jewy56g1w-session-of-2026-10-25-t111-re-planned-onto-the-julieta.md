# Session of 2026-10-25: T111 re-planned onto the julieta-claude kit, tools/manifest and T079

Done from 2026-10-23 to 2026-10-25, after the checkpoint of
"Session of 2026-10-23, third part: the rebases after T042, the helper
text, A12 and T077" (01m5nedw9bac71zj6peq15kfzy). Each pull request
passed `go run ./tools/ci all` on the committed tree, the forbidden-names
gate on the tree and the body, and the hosted CI; those marked "review"
had a code review, "audit" a security audit.

- #275 docs: T111 drops its interim kit and uses the `julieta-claude`
  kit of T079, which moves to M1; `romeu init` defaults its root to
  `$HOME/Developer`; review; merge c6821ca7. Records "The default root
  of romeu init is HOME/Developer" (01m5nw2x0e32pvh43k5kpw5pjx) and
  "The bridge uses the julieta-claude kit and the skills of this
  repository" (01m5nw2x1dw37z6sc6tebk991e).
- #276 tools: `tools/manifest` encodes the bridge's `JULIETA_MANIFEST`
  (`encode`, `ulid`), under a named row of REQ-0971 with its `checks`
  approval line; T111's fourth edit turns the store's mount read-only
  until T115's recreate, and the source tag is `source-<14 digits>-<8
  hex>`; review (opened before it, then applied); merge 729ee776.
- #277 T079: the `julieta-claude` kit, `julieta hooks guard` on
  `mvdan.cc/sh/v3` v3.14.1 (BSD-3-Clause), the widened refusals of
  REQ-0598 and REQ-0547, the agent profile of Claude Code; review and
  audit; approval lines `kits`, `dependencies`, `spec`; merge 26d49d75.
- #278 docs: the pending notes "Name the sandbox GitHub App Frei
  Lourenco" (01m5rjz3n7588xgx1xhfn56wmk) and "Metrics of the
  development process from deterministic hooks, after v1"
  (01m5rjz3p5x27yebkayt1hjydb); merge 255847e2.
- #279 docs: the tick of T079; merge bef63eff.
- In the sandbox kit repository: its #285 removes the inbox skill from
  the sandbox agent payload (merge 4ff07b40) and its #286 adds the
  sitting M1 runbook and the open-items cut-over command (merge
  81f38236), both reviewed and audited; its issue #287 holds the
  findings, with one item left open there.
- Issue #272 (T079) closed by #277.

Decisions of the session, each with its date:

- 2026-10-23: wave 1 of the bridge is romeu-e-julieta and verona;
  T112's sandbox gate lands during the week of daily use.
- 2026-10-25: the guard reads shell with `mvdan.cc/sh/v3` instead of a
  hand-written splitter, and also refuses releases, merges through
  `gh api` and GraphQL, auto-merge and tag refs.
- 2026-10-25: `julietaBin` names the julieta binary in the bin mount,
  not the directory.
- 2026-10-25: "Work for another repository goes into an issue there"
  (01m5rnxnykc2nbn1d2dvyyxmsw).

Lessons:

- A background `cmd; echo rc=$?; tail` reports the exit of the last
  command: read the step lines, not the notification's exit code.
- `go run ./tools/ci plan --ready` reads the committed tree.
- A hand-written shell splitter missed `"$(...)"` inside double
  quotes; a refusal list over shell text needs a real parser.
- A token that turns into another account shows as 404 on private
  repositories and 403 on push; `gh api user` names the account.
