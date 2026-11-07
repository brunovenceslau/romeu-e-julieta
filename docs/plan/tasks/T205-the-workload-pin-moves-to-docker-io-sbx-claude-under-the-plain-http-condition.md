---
id: T205
title: "The workload pin moves to docker.io/sbx/claude under the plain-HTTP condition"
milestone: M4
module: kitpin
fate: new
owns:
  - docs/notes/workload-sbx-claude.md
  - docs/spec.md
  - docs/spec/05-security.md
  - docs/plan/tasks/T208-the-workload-pin-move-to-docker-io-sbx-claude.md
  - docs/plan/tasks/T209-the-per-sandbox-deny-of-the-workload-s-plain-http-egress-pair.md
  - docs/plan/tasks/T214-sync-refuses-a-spec-agent-other-than-the-pinned-workload-s-handle.md
needs: []
after:
  - T055
ask_first:
  - spec
sitting: none
---

# T205 - The workload pin moves to docker.io/sbx/claude under the plain-HTTP condition

Goal: the measurement and the decision that the pin move of T208 waits
for: whether a per-sandbox deny on the two plain-HTTP host and port
pairs the `docker.io/sbx/claude` workload declares beats the allow rule
sbx provisions from its descriptor, and survives a recreate under the
same name.

Added on 2026-11-06 (scope this repository): Q2 of the index, as
decided on 2026-11-06, the plain-HTTP row of 05 5.4 (REQ-1374) and 06
6.2. Measured that day from both descriptor annotations: `sbx/claude`
is a Docker Hardened Image with Claude Code 2.1.318 and docker-ce
started by the annotation `com.docker.sandboxes.start-docker=true`,
declares no `volume@1`, and declares `deb.debian.org:80` and `dhi.io:80`
in its runtime egress, which sbx provisions as a rule romeu cannot
remove and which `os-base` refused on purpose on 2026-10-23;
`sbx-kit-claude` is on frontend m.11 with Claude Code 2.1.314, five
volumes, Go and Node. Neither provides `deb/tzdata` or `deb/zsh`, so
`os-base` stays. The same measurement is probe A9's once block A runs
(T019, T026), which re-records it with the harness.

1. The measurement, by hand on the maintainer's host and recorded in
   the note this task owns: with a sandbox of the `sbx/claude` workload,
   `sbx policy deny network --sandbox <p> deb.debian.org:80` and the
   same for `dhi.io:80`, then a request to each from inside, then the
   same after an `sbx env rm`
   and `sbx env run -d --clone` of the same name; the argv, the output
   and the sbx version recorded (sbx 0.49.1 says a CIDR deny beats a
   domain allow; a host and port deny is unmeasured).
2. The decision the measurement settles, recorded in Q2 and in the 05
   5.4 row: the deny holds and survives, so T209 makes `run` step 4
   apply and verify it and T208 moves the pin after T209; or the deny
   does not hold, and the row stands as accepted with its trigger
   (Docker serving the two hosts over 443, or the first finding that
   reaches the host through them), and T208 moves the pin after this
   task; or the maintainer chooses to wait for 443, and T208 stays
   open with that trigger.

Amended on 2026-11-07 (as decided on 2026-11-07, decision P with C1-14
and C1-15): no romeu call swaps a container any more, so item 1 and
the goal no longer name a swap: the deny is measured fresh and after
`sbx env rm` and `sbx env run -d --clone` under the same name; whether
it survives an
in-place apply is measured when the in-place apply's Deferred row
fires (T210). Item 1 gains three readings, all by hand in the same
sitting and recorded in the note:

3. A9's deny beside a portless `sbx policy allow network --sandbox <p>
   deb.debian.org` (an `egress.extra` entry, 07 REQ-1316): whether the
   port 80 request is still refused (REQ-1374).
4. A19, by hand with A9 (REQ-1386): inside a sandbox of the
   `sbx/claude` workload, the uid and capabilities the engine runs
   with, the group of its socket, what the agent user reaches of it,
   and `docker run hello-world` as the agent user. A19's own probe text
   makes it likely that the agent user reaches the socket, so the
   likely outcome is not the first branch of item 5.
5. The workload user and its home inside the sandbox, against the
   `/home/agent` that romeu hard-codes (V2.39), and whether the
   descriptor declares `docker-auth` or `host-port` (V1.9's note).

Item 2's decision now has two conditions, and each branch names the
fates it writes, all in this task's pull request (V2.12):

- The plain-HTTP condition (REQ-1374): (a) the deny holds and survives
  the recreate: T208 gains `needs: T209`; (b) it does not: T209 takes
  `fate: removed`, `milestone: none`, with the reason, and the 05 5.4
  row stands as accepted with its trigger; (c) the maintainer waits for
  443: T208 takes `fate: deferred`, `milestone: M6`, with that trigger,
  and T209 is removed or deferred with the same trigger.
- The engine condition (REQ-1386): (a) A19 shows the agent user cannot
  reach the engine's socket: the pin may move; (b) it can: this task's
  pull request rewrites the engine row of 05 5.4 to accept that
  elevation by name with its trigger and amends Q33, or (c) the pin
  waits: T208 takes `fate: deferred`, `milestone: M6`, with the trigger
  the maintainer names, and T209 takes the same fate, milestone and
  trigger, so no deny is built for a pin that waits (RP-9, amended on
  2026-11-07).
- Wherever T208 takes `fate: deferred`, T214, which comes after it,
  takes the same fate, milestone and trigger. This task's pull request,
  not T208's, rewrites the engine row and amends Q33 on branch (b):
  the plan's reading of REQ-1386's "the pull request that moves it"
  (RP-20).

T208 moves the pin only where both conditions allow it. `needs: T055`
becomes `after: T055`: the link is the file overlap on
`docs/spec/05-security.md`, and the measurement uses nothing T055
builds (V2.11); T126, which owns `docs/spec.md` too, comes after this
task.

- Implements: REQ-1374 (05 5.4), REQ-0576 (06 6.2), REQ-1386 (05 5.4).
- Operator: yes: the measurements of items 1, 3, 4 and 5 on the host,
  and the approval line for `docs/spec.md` and
  `docs/spec/05-security.md` (ask-first `spec`) for the decision's edit
  of Q2, Q33 and the two rows.
- Acceptance:
  - The note records each command, its output and the sbx version,
    and the outcome of each measurement (the deny fresh, after a
    recreate under the same name, and beside a portless allow; A19's
    readings; the workload user and home; `docker-auth` and
    `host-port`), and names the decision taken with its date (amended
    on 2026-11-07).
  - Q2, Q33 and the 05 5.4 rows of REQ-1374 and REQ-1386 say which
    branch of each condition was taken, and T208 and T209 carry the
    `needs` and fates that branch writes.
- Verify: `go run ./tools/ci static` (the note and the spec edit pass
  hygiene, prose and `reqs`; no test is added).
