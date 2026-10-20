# Hand-written code a maintained library could replace

What is at stake: each item is a parser, encoder or matcher whose tests
share its author's blind spots; a missed case is a bug in every caller.
Each swap is one task with its own dependency approval, and a swap that
does not fit leaves the code as it is
([ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md)).

Inventory of 2026-10-20, by reading the tree; the maturity of each
library was not checked online. In the order of ADR 0038, decision 5:

1. Done by T145 (#201) on 2026-10-20: the ULIDs of `internal/canon`
   now come from `github.com/oklog/ulid/v2` (REQ-0258).
2. Swapped by T147 on 2026-10-20: the ssh-agent identity listing of
   `internal/signing` comes from `golang.org/x/crypto/ssh/agent`; its
   exact behavior was read as not the requirement (ADR 0038, 3).
3. Done by T146 (#202) on 2026-10-20: the atomic writes come from
   `github.com/google/renameio/v2`; the two sites kept by hand, with
   their reasons, are in T146.
4. Decided on 2026-10-20: not a swap. The store lock of T035 stays on
   `golang.org/x/sys/unix` flock; no task.
5. The schema generator, `tools/schemagen`: `github.com/invopop/jsonschema`
   (03-formats says "no dependency"; read again under ADR 0038, 3).
6. The glob matcher of `tools/ci/internal/askfirst`:
   `github.com/bmatcuk/doublestar/v4`, if its dialect fits 12 12.4.
7. Done by T149 on 2026-10-20: the canonical JSON of REQ-0254 is
   RFC 8785, from `jsontext.Value.Canonicalize` of the standard library,
   in `internal/canon` and `internal/manifest`; no module was added.
8. The cover profile reader, `tools/ci/internal/coverage`:
   `golang.org/x/tools/cover`.
9. License identification, `tools/ci/internal/license`:
   `github.com/google/licensecheck`.
10. The minisign verifier, `tools/kitpin/minisign`: `aead.dev/minisign`.
11. The Markdown renderer of `tools/planviewer/template.html`: `marked`,
    vendored with its license.
12. Smaller: the git version compare of `internal/gitsafe/floor.go`, the
    mutation runner `tools/mutants`.

Kept with a stated reason (ADR 0038, 3): `internal/gitsafe`,
`internal/procgroup` and `internal/termsafe` (security surfaces whose
exact behavior is the requirement), and the container e2e harness
(REQ-0798's split of infrastructure errors from test failures, which no
candidate meets).

Reopen: each remaining item as a task, in this order. Next is item 5,
the schema generator.
