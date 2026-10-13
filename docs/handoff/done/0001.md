# Session of 2026-10-13: stage-B write pass, re-plan, gopls pin, task marks and T137

What merged this session, each with the gate it passed.

- Pull requests #45 (stable REQ ids, `tools/ci reqs`, the stage-B write
  pass) and #43 (the re-plan, rebased) merged as stack #46, merge commit
  5e2be26. Gates: #45 test-engineer round 3 GO at dece74f; #43
  code-reviewer rounds 1 to 3, APPROVE at 3ed83bf. CI green.
- Pull request #44 (gopls pin) merged at b9378bb.
- Pull request #47 (marks T104, T121 and T122 merged) merged at b1700c9.
  Gate: ship gate GO, then a targeted re-audit GO at bf7a86c.
- Stack #50 merged at 1bb5c62, CI green on 4 legs including macOS:
  - #48, the procgroup ask-first surface and `internal/egress` on
    `catalog`; REQ-0305 rewritten and split into REQ-0305, REQ-1234 and
    REQ-1235; the second-signal deferred row closed; T039, T105, T036
    and T137 plan updates. Gate GO round 3 at 50ec159.
  - #49, T137 `internal/procgroup`. Gate GO round 3 at c76e04e, plus the
    stacked gate GO.

Decided on 2026-10-13, as standing records under decisions/:
the-procgroup-surface-and-egress-on-the-catalog-surface, t105-tool-timeouts-procgroup-switch-and-three-prs,
re-plan-od-14-at-t111-od-7-for-every-risk-row-prefix-capture. Items the gates left open are under
pending/.

Closed in passing: the 2026-10-01 spec sweep item was superseded per
OD-16 of the re-plan.

Lessons for the process:

- A verifier brief forbids pattern kills (pkill, killall) and requires
  exact pids, and test helpers carry a self-timeout: a round-2 persona's
  pkill ended another persona's run.
- When a stream copy moves out of os/exec into our own code, compare it
  line by line against os/exec's source: the missing pipe close of
  exec.go (line 614) was a regression found only at the gate.
- Whether a static tool closes a class: the `merged:`/`pr:` pairing and
  the fate-table match are candidates for `tools/ci plan` (pending item
  "check-that-merged-and-pr-come-as-a-pair").
