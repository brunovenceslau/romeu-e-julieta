# Session of 2026-10-22 and 2026-10-23, the M3 core: T021, T029, T031, T032, T033, T036, T040, T053

Done on 2026-10-23, in the pull request that carries this record. Each
item names its gate and its merge commit on main. Every feature pull
request below passed a code review before it opened, `go run ./tools/ci
all` on the committed tree and the hosted CI; those marked "security
audit" had a security audit too.

- #244 tick of twelve merged tasks and the session queue; merge
  3b7c2780.
- #245 T021, spec; security audit; merge 028e1f3a. #249 recorded its
  decisions; merge 5f2a1003.
- #246 T033, state; security audit; merge 028e1f3a.
- #247 T031, catalog; security audit; merge 805c6e4c.
- #250 T053, the mise driver; security audit (the checksum, nesting and
  dotted-key memory caps, and the index read in pre-commit); merge
  11df2d87.
- #251 T029, sbxdrv and the fake sbx; security audit; aligned with sbx
  v0.49.2 as observed on 2026-10-22, which is the floor; merge
  e98306a2. #254 its post-merge follow-up (approvals refused before the
  verb, the guard's positions pinned, I4 worded plainly); merge
  aeed4a77.
- #252 re-plan of T040; merge bc38a245.
- #253 T040, gate; merge 65141cba.
- #255 T036, egress; merge 8e0b2b5b. #256 recorded its decisions and
  the egress follow-ups; merge 535008f4.
- #257 T032, oci; code review before it opened, a focused review of the
  gate typing commit, `go run ./tools/ci all` on the tip, and the hosted
  CI; merge 2030e442.
- #248 pending notes on the subagent context reuse rule, from a
  parallel session; merge a19dc6bc. Listed as merged in the period.

Decisions of the session, each with its date:

- 2026-10-22: the queue of the session was at most eight tasks, critical
  path first: T021; T033, T031, T029, T032; T053; T040; T036 (next step
  1 of `now.md` of that date).
- 2026-10-22: pushes and pull requests were authorized for the rest of
  this session. Scope: this session only; the standing rule "Pushes
  and pull requests are asked first from 2026-10-22"
  (01m5gywvgc85vj8k4txzppjz6d) is unchanged.
- 2026-10-22: adding a dependency that `docs/spec` or `docs/plan`
  already names was approved. Scope: this session.
  github.com/BurntSushi/toml v1.6.0 entered with T053.
- 2026-10-22: for T053, a lock entry without a checksum is a finding
  for backends that can checksum, with a reviewed exception list;
  GITHUB_TOKEN is not passed to mise; the global and system mise config
  are isolated and MISE_PARANOID=1 is kept (recorded in #250 and in
  specs 04 4.3, 05 and 07).
- 2026-10-23: the sessions of the fake sbx stay `synthetic: true`;
  their shapes were checked against the help and `ls --json` of sbx
  v0.49.2.
- 2026-10-23: ADR 0018 is accepted (in #253).
- 2026-10-23: github.com/google/go-containerregistry v0.22.1 is
  approved as a dependency of T032, and CC-BY-SA-4.0 joins the license
  allowlist, for the LICENSE.docs of opencontainers/go-digest (in
  #257).
- 2026-10-23: the five open items of #253 (the text rule of the gate 2
  diff, the resolved recreate digest, all gate output on stderr, `y`
  approves in v1, the non-minimal diff in v1) were decided as the code
  does; 01 1.4, 03 3.12 and 04 4.1 were amended.
- 2026-10-23: the five open items of #255 and of #257 were decided as
  each pull request recommends (recorded in #256 and in the body of
  #257).
- 2026-10-23: the operator's to-do items go to `docs/handoff/pending`,
  not to an external reminder list. Scope: every repository and sandbox of the maintainer;
  written in the agent's memory for this repository; when
  romeu-e-julieta and verona replace the interim payload, it goes to
  the payload's rules.

Lessons:

- A decision that arrives after its pull request merged gets a small
  docs pull request.
- A scripted conflict resolution is followed by a grep for conflict
  markers before any push.
- Per-commit `go run ./tools/ci all` of a long replayed stack under a
  flaky network costs hours, so the hosted CI measures it.
