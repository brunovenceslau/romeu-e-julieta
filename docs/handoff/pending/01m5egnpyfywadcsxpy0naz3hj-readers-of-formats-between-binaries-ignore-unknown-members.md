# Readers of formats between binaries ignore unknown members

What is at stake: medium. Until it is built, the spec says any change to
a format's fields bumps its version (REQ-0163), and a newer julieta that
adds a member breaks an older romeu's strict reader of that format.

Scope: this repository.

Decided on 2026-10-21: formats evolve by the rules of protobuf ("Formats
evolve by the rules of protobuf", 01m5egnpxhheh7197j08zdr618).

To plan as one task, ask first on the `spec` and `decisions` surfaces:

- An ADR that records the rule, and REQ-0163 amended to say it.
- The list of formats one binary writes and another reads, measured
  from the code (`julieta-status`, `snapshot/heads.json` of T060, the
  handoff files, and any other), and each of their readers changed to
  ignore an unknown member, with a test that a reader accepts a member
  a newer writer adds.
- The readers of files a person writes stay strict, and a test holds
  that.

Reopen: when the task is planned, or when a format between binaries
gains a member before it is built.
