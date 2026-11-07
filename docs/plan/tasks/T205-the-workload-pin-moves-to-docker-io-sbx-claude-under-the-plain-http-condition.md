---
id: T205
title: "The workload pin moves to docker.io/sbx/claude under the plain-HTTP condition"
milestone: M4
module: kitpin
fate: new
owns:
  - docs/notes/workload-sbx-claude.md
needs:
  - T055
after: []
ask_first:
  - spec
sitting: none
---

# T205 - The workload pin moves to docker.io/sbx/claude under the plain-HTTP condition

Goal: the measurement and the decision that the pin move of T208 waits
for: whether a per-sandbox deny on the two plain-HTTP host and port
pairs the `docker.io/sbx/claude` workload declares beats the allow rule
sbx provisions from its descriptor, and survives a swap and a recreate.

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
   same after a container swap (A15's call) and after an `sbx env rm`
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

- Implements: REQ-1374 (05 5.4), REQ-0576 (06 6.2).
- Operator: yes: the measurement of item 1 on the host, and the
  approval line for `docs/spec.md` and `docs/spec/05-security.md`
  (ask-first `spec`) for the decision's edit of Q2 and the row.
- Acceptance:
  - The note records each command, its output and the sbx version,
    and the outcome of the three measurements (fresh, after a swap,
    after a recreate), and names the decision taken with its date.
  - Q2 and the 05 5.4 row say which branch of item 2 was taken, and
    T208 and T209 carry the matching `needs`.
- Verify: `go run ./tools/ci static` (the note and the spec edit pass
  hygiene, prose and `reqs`; no test is added).
