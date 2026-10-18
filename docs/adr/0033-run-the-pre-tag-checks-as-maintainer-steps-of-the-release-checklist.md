# 33. Run the pre-tag checks as maintainer steps of the release checklist

Date: 2026-10-18

## Status

Proposed

## Context

The four completeness checks were planned to run once after v1.0.0 is
public, with no path for a failure then, and the comparison of the
candidate with the tag was checked only after the tag. Review round 8
raised this as item SC15 of [round 8](../reviews/round-8.md#decisions-for-the-operator). The decision supersedes
op-acceptance-after-release and op-probe-completeness of round 6.

### Alternatives considered

- **Run the checks after the release**, as planned. Not taken: a
  failure then has no path.
- **Add a step to the release workflow.** The proposal R8-10-44, not
  taken.
- **Make them maintainer steps of the release checklist.** Taken.

## Decision

Decided on 2026-10-08: the completeness checks join `tools/ci all` only
once the last task of the plan has landed, so that `all` does not turn
red before block B. The pre-tag `acceptance` run and the candidate
comparison are maintainer steps of the release checklist in
[12 12.2](../spec/12-engineering.md#122-development-commands-and-capability-map), outside `release.yml`. A failure
after the tag is a patch release.

## Consequences

- `release.yml` stays free of those checks, as
  op-acceptance-after-release asked.
- The check that a moved release tag equals the source digest of its
  attested provenance runs in the same place (item AR21).
