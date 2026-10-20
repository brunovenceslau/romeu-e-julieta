# 38. Search for a maintained library before writing one

Date: 2026-10-20

## Status

Proposed

## Context

REQ-1049 (12 12.1) names the Go dependencies as a "complete v1 list",
each addition ask-first. The list was fixed before anyone searched for
what the code would need, so a builder who meets a job a library
already does reads the list as a reason to write it, and "fewer
dependencies" becomes the whole argument. The one general statement of
that policy is the reason ADR 0015 quotes, "fewer dependencies shrink
the host trust surface"; no record weighs it against the cost of
writing the code.

That cost has already shown once. The command line was first written
on the standard library's `flag`, and each review round found another
piece a framework holds, until
[ADR 0015, adopt spf13/cobra for the command line of romeu and julieta](0015-adopt-spf13-cobra-for-the-command-line-of-romeu-and-julieta.md)
moved both binaries to cobra.

An inventory of 2026-10-20 found about a dozen more places where the
code does by hand what a maintained library does, most with no reason
recorded beyond the list: the ULIDs of `internal/canon` (T017 says
"from the standard library alone"), the ssh-agent identity listing of
`internal/signing` while `golang.org/x/crypto` is already in `go.mod`,
the schema generator of `tools/schemagen`, the glob matcher of the
ask-first list, eight copies of an atomic write, the canonical JSON of
`internal/canon` and its second copy in `internal/manifest`, the cover
profile reader, the license identification of `tools/ci`, the minisign
verifier of `tools/kitpin`, and the Markdown renderer of
`tools/planviewer`. The list is in the pending note
"hand-written-code-a-maintained-library-could-replace".

Most of these are parsers, encoders and matchers. The code and its
tests come from the same author, here mostly an agent, so a case the
author did not think of is missing from both, and the checks pass. A
maintained library carries the cases other users found. An agent is
also quick to write a plausible implementation and its validator from
first principles, which makes the hand-written path look cheaper than
it is.

The Go proverb "a little copying is better than a little dependency"
does not cover this. It weighs copying a small piece of proven code
against importing a module for it; the code above was neither copied
nor imported, but written from the spec and the author's memory.

What is at stake: if this decision is wrong one way, dependencies grow
without need and each one widens what a compromised module could reach
on the host; a dependency comes out again in one pull request, and the
ask-first `dependencies` surface still sees each one enter. If it is
wrong the other way, as the closed list is, hand-written parsers keep
landing with tests that share their blind spots, and the cost grows
with every caller; that is harder to undo than a dependency.

### Alternatives considered

- **Keep the closed list.** The state before this record; the inventory
  and ADR 0015 show how it fails.
- **Let a builder add any dependency.** Not taken: the host trust
  surface and the GPL-3.0 license constraint still need a maintainer's
  look at each module that enters `go.sum`.
- **Search first, record the search, and keep the addition
  ask-first.** Taken, with copying as the middle way of the proverb.

## Decision

Decided on 2026-10-20:

1. REQ-1049 lists the dependencies in use, not a complete v1 list.
   Adding one stays an ask-first change of the `dependencies` surface.
2. **[review]** Before a task writes code that parses or encodes a
   format, implements a published algorithm or protocol, or does a job
   that is not specific to this product (locking, atomic writes,
   globbing, schema generation, Markdown), its builder searches for a
   maintained library and records the search in the pull request body
   under **Prior art**: the candidates, each one's license against
   GPL-3.0-only, its maintenance and its transitive modules, and the
   choice. The choice is one of three:
   - import the library;
   - copy a small, proven piece of it (or of another maintained
     project) into the repository, with its license, its origin and
     version in a header comment, and its upstream tests where they
     exist, as the proverb means;
   - write it by hand, under 3.
3. **[review]** Writing such code by hand needs one of these reasons,
   stated under **Prior art**: a requirement no candidate meets, after
   a thin wrapper over it was considered; a security surface whose
   exact behavior is the requirement (`internal/gitsafe`,
   `internal/procgroup`, `internal/termsafe`); or no candidate that
   passes the license or maintenance look, and nothing small enough
   to copy. "One dependency fewer" is
   not a reason on its own.
4. A requirement that seems to force hand-written code is first met
   with a wrapper. For example, the lowercase ULIDs of REQ-0258 are a
   wrapper over a ULID library that lowercases on output and refuses
   uppercase on input.
5. The code the inventory found is replaced one task at a time, each
   with its own dependency approval, starting with the ones with the
   least risk and the most callers: the ULIDs, the ssh-agent listing
   and the atomic writes. A place where the library does not fit keeps
   its code, and its reason is recorded under 3.
6. A builder brief for code covered by 2 names this rule.
7. `tools/ci pr` gains a check: a pull request that adds a Go package
   carries a **Prior art** line, which holds the search or the words
   `none: <reason>`, as the **Middleware** line does. It is a change on
   the `checks` surface with its own approval.

## Consequences

- REQ-1049 in 12 12.1 drops "complete v1 list". The deliberate
  hand-written choices of 12 12.1 (no pty library, the OCI manifest
  reads, the ssh-agent listing, the schema generator) are each read
  again under 3 when their task of 5 comes up; until then they stand.
- T035 waits for this record, so its store lock and atomic writes are
  chosen under 2.
- The pull request template and REQ-1179 gain the **Prior art**
  section when the check of 7 lands.
- The rule for builder briefs belongs to the sandbox payload as well;
  the pending note names it for the payload's repository.
