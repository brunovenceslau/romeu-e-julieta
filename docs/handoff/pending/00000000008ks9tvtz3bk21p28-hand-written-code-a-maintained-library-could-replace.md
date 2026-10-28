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
   Planned on 2026-10-23 as T166, with the question of `null` for a
   pointer: a member with no value is omitted (REQ-1273, 03 3.7). Kept
   by T166 (#PRNUM) on 2026-10-28, measured against
   `github.com/invopop/jsonschema` v0.14.0: without references it
   recurses without end on the run layout's tree type, and with them
   every named struct moves to `$defs`, which changes every schema and
   the paths the keywords are placed at; a `TextMarshaler` such as the
   ULID comes out as an array of integers; and the refusals the
   generator holds (`time.Duration`, a type with its own JSON methods,
   the `string` option, a JSON name used twice) would stay a walk of
   their own. Most of the hand-written code would stay beside three new
   modules. Reopen when a library maps encoding/json/v2's rules (embed,
   omitzero, text marshalers) and references only a recursive type.
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
13. Found on 2026-10-20 while reading the JSON callers, maintenance and
    license not checked online: the Dockerfile instruction parser and
    shell word splitting of `tools/ci/internal/kits` (`splitScript`,
    `unquote`; `mvdan.cc/sh/v3/syntax` or buildkit's parser, on the
    `checks` surface); the RFC 8288 Link header parser `nextLink` of
    `tools/kitpin/internal/pin/frontend.go`
    (`github.com/tomnomnom/linkheader`, on the `kits` surface);
    `yamlQuote` of `tools/new/task.go`, which `go.yaml.in/yaml/v3`,
    already a dependency, can do; the git object parsing of
    `tools/ci/internal/drift` and `tools/ci/pushed` (go-git plumbing, low
    priority).

Kept with a stated reason (ADR 0038, 3): `internal/gitsafe`,
`internal/procgroup` and `internal/termsafe` (security surfaces whose
exact behavior is the requirement), and the container e2e harness
(REQ-0798's split of infrastructure errors from test failures, which no
candidate meets).

Reopen: each remaining item as a task, in this order. Next is item 5,
the schema generator (T166).
