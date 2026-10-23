# Session of 2026-10-23, second part: T041, T037, T046 and the decisions PR

Done on 2026-10-23, in the pull request that carries this record, after
the checkpoint of #258. Each item names its merge commit on main. Every
pull request below passed a code review before it opened, `go run
./tools/ci all` on the committed tree and the hosted CI; those marked
"security audit" had a security audit too.

- #259 T041, `romeu init` and the scenario skeleton; security audit
  (fixed: a fail-closed default-branch check, a symlinked or
  group-writable env dir, identity comparison of paths, a no-replace
  settings commit, the CGI path check); the e2e also run on macOS by
  hand; merge 43ac3fe3.
- #260 T037, render (`sbxenv.yaml`, `render.json`, workspace files,
  `shquote`, `tools/ci golden`, the I11 imports rule exempting cobra by
  name); security audit (hostIP always 127.0.0.1, the repo-dir pattern
  in render, a short-list note of golden, a runtime guard refusing
  `-update` under CI); merge 191e6109.
- #261 T046, the preflight; code review with a security focus; the
  macOS race job found a real bug (a path that does not exist yet was
  not resolved through `/var` to `/private/var`), fixed and tested;
  merge 9d7d88b7.
- #262 docs: the decisions of 2026-10-23, the ticks of T037, T041 and
  T046, and tasks T164 to T169; code review; merge 045abac2.

Decisions of the session, each with its date:

- 2026-10-23: the only supported pair of host and sandbox is a Mac on
  Apple silicon with a linux/arm64 sandbox (01m5n79k7zb9yvw4bneb3pcv96).
- 2026-10-23: T170 measures whether `macos-26` runs a Linux container
  before the Mac pair is placed (01m5n79k97khetavqr1pzq4ymf).
- 2026-10-23: the credential helper of `romeu init` answers only the
  config repo's host (01m5n79kanj94f68cwybw96ym1).
- 2026-10-23: gate 1 (T042): a declined prompt exits 1 with
  `toolchain-declined` (RJ-355), its row added to 04 4.4 inside the code
  pull request; the lock is held across the prompt (REQ-0284); a damaged
  project record blocks approval; promotion recovery is not run until it
  exists; `golang.org/x/mod/semver` v0.41.0 orders prereleases in
  `sbxdrv.Release.Compare`; both prompts (gate 1's `Confirm` and gate
  2's `Ask`) drop typed-ahead input before the question is written,
  never after, approved inside #263.
- 2026-10-23: opening a pull request and pushing to an open one need no
  approval in this session ("Pushes to the branch of an open pull
  request need no approval in the session of 2026-10-23",
  01m5n7net1v4sgs10t0nvwvyka), replacing the narrower record of the
  same day.
- 2026-10-23: dependencies approved for this session:
  `github.com/tailscale/hujson` (pseudo-version
  v0.0.0-20260727124030-b80ff77dac4f, no tagged release) and
  `golang.org/x/net` v0.60.0 (publicsuffix) with #261, and
  `golang.org/x/mod` v0.41.0 with #263. Scope: this session.

Pending items added: "Apply the per-host credential helper text to 03
3.4 and 05 REQ-0464 after T164 merges" (01m5n79kc411gxevqamd508scw) and "Correct the Intel Mac
in 00 0.2 and REQ-0868" (01m5n79kdbtk9dpq7y87sfkkaq).

Lessons:

- A builder told of an approval by another agent refuses to add a
  dependency; the orchestrator adds an approved dependency itself.
- A terminal-driven test types only after it reads the question; an
  answer typed ahead is dropped by the gate's input flush. The e2e of
  T042 hung on this and the harness deadline did not end the run.
- macOS `/var` is a link: compare a path that does not exist yet after
  resolving its existing part; a TMPDIR that is a link reproduces it on
  Linux.
- `go clean -cache` during another builder's `all` fails its race step
  once ("package internal/chacha8rand is not in std"); clean between
  runs.
- The brief guard matches some romeu command names as words of other
  task kinds; reword the brief.
- A row of the spec that names a runner is not a job: the e2e row named
  macOS and `ci.yml` never ran it (T170 adds the check).
- `tools/ci` never runs fuzzing; the slow steps of `all` are race
  (about 1 to 2.5 minutes) and e2e (about 1 minute).
