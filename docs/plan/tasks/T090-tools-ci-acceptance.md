---
id: T090
title: "`tools/ci acceptance`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T088
  - T133
after: []
ask_first:
  - checks
  - formats
sitting: none
---

# T090 - `tools/ci acceptance`

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. It needs T133 for the S6 tag check.
The text below is the one written before the re-plan; `owns` is written
when the task reopens.

- Implements: REQ-0002 (index), REQ-0003 (index), REQ-0004 (index),
  REQ-0005 (index), REQ-0011 (index), REQ-0475 (05 5.2), REQ-0772 (10
  10.1), REQ-0871 (10 10.2), REQ-0913 (10 10.2), REQ-0923 (10 10.2),
  REQ-0944 (10 10.5), REQ-0946 (10 10.5), REQ-0947 (10 10.5), REQ-0948
  (10 10.5), REQ-0949 (10 10.5), REQ-0950 (10 10.5), REQ-0951 (10 10.5),
  REQ-0952 (10 10.5), REQ-0953 (10 10.5), REQ-0954 (10 10.5), REQ-0955
  (10 10.5), REQ-0979 (11 (opening)), REQ-1012 (11 11.2), REQ-1028 (11
  11.4), REQ-1029 (11 11.4), REQ-1033 (11 11.4), REQ-1034 (11 11.4),
  REQ-1035 (11 11.4), REQ-1069 (12 12.1), REQ-1095 (12 12.2), REQ-1099
  (12 12.2), REQ-1111 (12 12.3), REQ-1121 (12 12.3).
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - Against a recorded API fixture, each evidence kind is verified as 10
    10.5 says.
  - On the product's own file the six end-of-plan checks of 10 10.5 run,
    one fixture each that fails it, and a seventh: every step row of 10
    10.2 has a registered check. They join `all` only in the last task,
    T102, so `all` stays green while the plan is under way (10 10.5).
  - It fails while a `default-overturned` result has no `resolvedBy`;
    when a block A result's `sbxVersion` is outside the tested window of
    10 10.3, when A4, A5 or A9 has no result at the window's newest
    version, or when a recorded `upstream` key differs from its current
    pin (11 11.4); and when block B results of one host name different
    candidates (11 11.2).
  - `acceptance --pre-tag` runs the six checks of 10 10.5 offline
    against the commit about to be tagged, else it exits non-zero; it is
    step 3 of the release checklist of 12 12.2, which block O7 runs.
  - For each release, `acceptance` checks that the tag's commit equals
    the source digest of the release's attested provenance, read through
    the API; `--tags` runs that check alone; fixtures against the
    recorded API: an unmoved tag passes, a moved tag fails (10 10.5; 12
    12.2 step 7).
  - `schemas/acceptance.v1.json` is generated.
- Verify: `go test ./tools/ci/... -run '^TestAcceptance'`, which lists
  the tests by name.
