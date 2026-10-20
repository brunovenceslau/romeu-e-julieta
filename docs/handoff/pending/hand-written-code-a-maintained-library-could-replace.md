# Hand-written code a maintained library could replace

What is at stake: each item is a parser, encoder or matcher whose tests
share its author's blind spots; a missed case is a bug in every caller.
Each swap is one task with its own dependency approval, and a swap that
does not fit leaves the code as it is
([ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md)).

Inventory of 2026-10-20, by reading the tree; the maturity of each
library was not checked online. In the order of ADR 0038, decision 5:

1. ULIDs, `internal/canon/ulid.go`: `github.com/oklog/ulid/v2` with a
   wrapper that lowercases on output and refuses uppercase (REQ-0258).
2. The ssh-agent identity listing, `internal/signing/signing.go`:
   `golang.org/x/crypto/ssh/agent`; `x/crypto` is in `go.mod` for
   `tools/kitpin` only (REQ-1049). It is on the `signing` surface (I34);
   the task reads whether its exact behavior is the requirement
   (ADR 0038, 3) before it swaps.
3. Atomic writes, eight rename sites outside tests:
   `internal/cmd/julieta/setup/state.go`, `internal/cmd/julieta/setup/link.go`,
   `tools/kitpin/internal/pin/pin.go`, `tools/ci/internal/hygiene/add.go`,
   `tools/ci/internal/setup/install.go`, `tools/release/internal/build/build.go`,
   `tools/planviewer/internal/view/view.go`, `e2e/internal/container/container.go`
   (not each read to confirm it is a temporary file and a rename):
   `github.com/google/renameio/v2`, or one shared helper copied from it.
4. The store lock T035 needs: `github.com/gofrs/flock`.
5. The schema generator, `tools/schemagen`: `github.com/invopop/jsonschema`
   (03-formats says "no dependency"; read again under ADR 0038, 3).
6. The glob matcher of `tools/ci/internal/askfirst`:
   `github.com/bmatcuk/doublestar/v4`, if its dialect fits 12 12.4.
7. Canonical JSON, `internal/canon/digest.go` and `internal/manifest`:
   an RFC 8785 library (`github.com/gowebpki/jcs`), if the form of
   REQ-0254 can become RFC 8785.
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

Reopen: each item as a task, in this order, once ADR 0038 is Accepted.
