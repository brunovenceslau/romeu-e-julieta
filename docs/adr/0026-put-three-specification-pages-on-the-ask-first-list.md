# 26. Put three specification pages on the ask-first list

Date: 2026-10-18

## Status

Proposed

## Context

The ask-first list names the surfaces on which a change needs an
approval line
([05 5.3](../spec/05-security.md#53-ask-first-surfaces)). Review round 8
found (item AR13 of [round 8](../reviews/round-8.md#decisions-for-the-operator)) that `docs/spec/**` was on no surface,
while `tools/ci vocabulary` and `sequences` read its tables and page 05
holds the security model. The decision adds to the round 3 decision on
which documents are surfaces. Four clusters of findings proposed three
forms.

### Alternatives considered

- **A three-file specification surface.** Taken.
- **A data-file form:** the two inputs under `tools/ci`, rendered into
  the specification. The fallback if the operator wanted no
  specification surface. Not taken.
- **No surface for the specification.** The state before the decision.

## Decision

Decided on 2026-10-08: the specification surface is three files, the
index (`docs/spec.md`), the system model page (`docs/spec/01-system-model.md`)
and the security page (`docs/spec/05-security.md`). The path of the
generated denylist is named outside `tools/ci`. Confirmed on 2026-10-09
in the ship gate of the pull request that adds the surface, which carries
three approval lines, one for `ask-first`, one for `spec` and one for the
`decisions` surface.

## Consequences

- A change to one of the three pages needs an approval line of the
  `spec` surface.
- Writing the surface needs an `ask-first` approval line for
  `.github/ask-first.yaml`.
- The other pages of the specification stay free of the surface.
