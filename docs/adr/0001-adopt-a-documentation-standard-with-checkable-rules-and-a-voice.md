# 1. Adopt a documentation standard with checkable rules and a voice

Date: 2026-09-30

## Status

Proposed

## Context

In Shakespeare's play, Romeo and Juliet do not die for lack of a plan.
Friar Laurence's plan was sound; it was written down; the letter that
explained it never reached the one reader who needed it. We would like
our documentation to fail less dramatically, so this record says what
every document in this repository must do, who checks it, and how.

romeu e julieta is a tool people are asked to trust with their host,
their credentials and their unpushed work. Its documentation carries
part of that trust: the README decides whether someone installs it, the
guides decide whether they recover from an error without reading Go,
and the ADRs decide whether the next contributor re-litigates a settled
question. The spec already holds these pages to a short standard
([12 12.8](../spec/12-engineering.md#128-contributor-docs-outlines)):
name the reader, one content type per page, self-contained sections,
copyable examples, concise prose, decisions in ADRs. That list is
correct, and none of it is enforced. Our first principle says anything
that must happen the same way every time is code, a gate or a hook,
never a model's memory or goodwill
([index](../spec.md#deterministic-where-it-can-be-the-model-where-it-adds-value)).
A documentation standard that lives only in a reviewer's head breaks
that principle.

The agents that write in this repository already apply three
documentation practices, each carried as a skill they load. We treated
the three as learning inputs, compared them rule by rule, and kept what
fits this project. We name each practice by its public origin, so the
standard survives a change of tools:

| Practice | Public origin | What we learned from it | Why it fits here |
|---|---|---|---|
| Technical writing for humans and AI agents | Mintlify's documentation guides ([mintlify.com/guides](https://www.mintlify.com/guides)) and Diataxis ([diataxis.fr](https://diataxis.fr)) | know the reader; one content type per page (tutorial, how-to, reference, explanation); the most common path first; sections that stand alone when retrieved; a 10-dimension review rubric | our guides are read by an operator mid-incident and by agents retrieving one section at a time; both need pages that answer one question without the page above |
| Decision records | Michael Nygard, "Documenting Architecture Decisions" (2011, [cognitect.com](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)), on the adr-tools layout by Nat Pryce ([github.com/npryce/adr-tools](https://github.com/npryce/adr-tools)) | record why, with the alternatives rejected; a record is never rewritten, only superseded; comments explain why, and traps point at the decision | the spec describes current state only (12 12.5), so the reasoning has to live somewhere that does not change when the spec does |
| Engineering documentation | common README, runbook and contributor-guide practice | a first run a reader finishes in minutes; runbook shape for operations (expected output, recovery per error); CONTRIBUTING and ARCHITECTURE outlines | the 12.8 outlines already follow it; this record makes the shape checkable |

For Go source we follow the Go project's own conventions, "Go Doc
Comments" ([go.dev/doc/comment](https://go.dev/doc/comment)) and the
error-string rule of "Go Code Review Comments"
([go.dev/wiki/CodeReviewComments](https://go.dev/wiki/CodeReviewComments)).

The comparison found gaps that none of the three practices covers:
error messages and CLI help, generated reference with drift detection,
executing documented commands, bidirectional supersede links, a
`SECURITY.md`, docs versioning per release, and citing where a borrowed
pattern came from. This record closes all but docs versioning (see
Consequences).

We learn from these sources; we do not copy them. Where a rule below
comes from one of them, the table above says which and why it fits.

### Alternatives considered

- **Adopt one practice as it is.** The Mintlify-based writing practice
  is the strongest single source, but it says nothing about ADR
  lifecycle, Go doc comments, generated reference or security policy.
  Adopting it alone leaves the gaps above open. Rejected.
- **Keep the six bullets in 12.8.** They are right and unenforced, and
  they live in a spec that describes current state rather than
  decisions, so a change to them has no record of why. Rejected; the
  bullets are folded into rules 1, 2, 4 and 9 and the rubric of rule
  18, with checks where a check is possible.
- **Write the standard as a section of `CONTRIBUTING.md`.**
  `CONTRIBUTING.md` is a how-to for contributors (12.8). A standard that
  can be reversed needs the ADR lifecycle: a status, a date and a
  superseding record. Rejected; `CONTRIBUTING.md` links here instead.
- **Adopt a large published style guide wholesale.** General style
  guides answer questions we do not have and leave the ones we do (ADR
  layout, generated reference, command execution) open. They are also
  too large to check mechanically. Rejected.

## Decision

We adopt one documentation standard for every document in this
repository and for text written about it elsewhere (commits, PR bodies,
issues, review comments, release notes). It has three parts: a lead
practice per artifact, twenty rules, and the resolution of every
conflict we found between the sources and the spec.

### Lead practice per artifact

| Artifact | Lead practice | Supporting |
|---|---|---|
| ADRs (`docs/adr/`) | Nygard's decision record (when, why, alternatives, lifecycle) on the adr-tools layout | the prose rules below; a generated index |
| `README.md` | the page pattern from Mintlify's guides: definition, when not to use it, then the shortest correct path | a first run finished in minutes; links to ADRs |
| Reference (`docs/reference/`) | generated from code (12 12.3), shaped as reference: tables, exact values, defaults | `tools/ci generated` drift check; never hand-edited |
| Guides (`docs/guide/`) | Diataxis how-to, one journey per page | runbook shape: expected output and recovery from each expected error id |
| The spec (`docs/spec.md`, `docs/spec/`) | written before the code and kept to current state | reviewed with the rubric of rule 18; it cites ADRs and ADRs cite it |
| `CONTRIBUTING.md`, `ARCHITECTURE.md` | the outlines in 12.8 | Diataxis how-to and explanation; diagrams as text |
| Go doc comments and Example tests | Go Doc Comments; comments say why | traps cite the ADR; `golangci-lint` |
| Commits, PRs, issues | Conventional Commits and the PR sections of 12 12.4 | the prose rules (rule 4) |
| Error messages and CLI help | none of the sources covers it: what failed, why, next action; Go error-string style | the error table with ids and fix hints (04) |
| `SECURITY.md` | the skeleton of rule 20 | - |

### The rules

**[check]** means a deterministic check fails the build; the step that
owns it is named. **[review]** means a reviewer, human or agent, judges
it. Where a rule could be a check and is not, that is a defect to fix,
not a permanent state.

1. **[check]** Every hand-written Markdown page names its Diataxis type
   and its primary reader in YAML front matter (`type:` one of
   `tutorial`, `how-to`, `reference`, `explanation`; `reader:` a phrase
   naming one reader), and has one type. Exempt, because the path
   already says what they are: `README.md` files, ADRs, a CHANGELOG, and
   generated pages. Owner: `tools/ci docs`.
2. **[review]** The reader's journey comes first: outcome,
   prerequisites, the shortest correct path; limits, alternatives and
   detail after, unless safety needs them earlier.
3. **[review]** Every claim is grounded in code, tests, the spec or
   measured behavior. Stale content is deleted, not caveated.
4. **[check]** Prose rules, repository-wide and in PR bodies and commit
   messages: no em dash character (U+2014), no hype words, no AI
   meta-commentary, no attribution footer, no paired
   `not only ... but also` construction. The word lists are committed
   data with a self-test; they skip code spans, so a rule can quote what
   it bans. The em dash check skips nothing.
   Owner: `tools/ci hygiene` for files, `tools/ci pr` for the PR body
   and the PR's commit messages.
5. **[check]** Markdown lint, link check and spell check. Relative links
   and anchors are checked offline in every run; external URLs are
   checked by a scheduled workflow, because `tools/ci all` stays
   offline (10 10.1). The spell check's word list is generated from the
   vocabulary table ([01 1.7](../spec/01-system-model.md#17-vocabulary))
   plus one committed list of other accepted words. Owner:
   `tools/ci docs`.
6. **[check]** ADRs use the adr-tools layout: `.adr-dir` contains
   `docs/adr`; files are `NNNN-kebab-title.md`; the first line is
   `# N. Title` and matches the filename; then a `Date: YYYY-MM-DD` line,
   then the sections `## Status`, `## Context`, `## Decision`,
   `## Consequences`, in that order. Alternatives go in
   `### Alternatives considered` inside Context. Owner:
   `tools/ci sequences`.
7. **[check]** ADR status is one of Proposed, Accepted, Deprecated,
   Superseded. A reversal is a new ADR whose Status section says
   `Supersedes [N. Title](NNNN-title.md)`, and the old ADR's Status
   section gains `Superseded by [M. Title](MMMM-title.md)`; the links
   must match in both directions. ADRs are never deleted, and numbers
   are contiguous and unique. The index `docs/adr/README.md` is
   generated from the titles and statuses. Owner: `tools/ci sequences`
   for links and numbers, `tools/ci generated` for the index.
8. **[review]** An ADR becomes Accepted only on the maintainer's
   explicit sign-off, quoted in the PR that changes its status. An
   agent never promotes its own record.
9. **[review]** Every decision that is expensive to reverse gets an
   ADR, with the alternatives considered and, for any borrowed pattern,
   its source and why it fits this project.
10. **[check]** Reference pages are generated from their single source
    (12 12.3), carry a generated-file header, and are never hand-edited;
    drift fails the build. Owner: `tools/ci generated`.
11. **[check]** Release notes are generated from Conventional Commit
    subjects (12 12.6); nobody writes a CHANGELOG by hand. Commit
    subjects are checked for the Conventional Commit form, and PR bodies
    for the sections of 12 12.4. Owner: `tools/ci pr`.
12. **[check]** `README.md` uses this heading order: what it is and who
    it is for; when not to use it; prerequisites; install and verify;
    first run; trust model; links to guides, reference and ADRs.
    Command and settings tables are links to the generated reference,
    not copies. Owner: `tools/ci docs`.
13. **[check]** Commands shown in `README.md` and `docs/guide/` are
    executed. Every command line in a `sh` block of those pages appears
    in the matching journey's scenario function (`e2e/scenarios`), so
    the CI suite runs it against the fake sbx or the host suite runs it
    against real sbx (10 10.1). A command that no suite runs fails the
    build. Owner: `tools/ci docs`.
14. **[check]** Every exported Go identifier has a doc comment; error
    strings follow Go style (lowercase, no trailing punctuation); the
    only accepted TODO form is `TODO(#<issue>)`. Owner: `golangci-lint`
    for the first two, `tools/ci hygiene` for the TODO form.
15. **[review]** Comments explain why, not what. A comment at a trap
    cites the ADR or lesson behind it. No commented-out code. Every
    package lives under `internal/`, so Example tests are written where
    a doc comment cannot show the call contract on its own.
16. **[review]** Error messages and CLI help say what failed, why, and
    what to do next; the fix hint in the error table (04) is the "next".
17. **[check]** Diagrams are text (Mermaid or ASCII) and appear only
    where prose cannot explain; no binary media under `docs/` or in
    `README.md`. The state-machine diagrams in `ARCHITECTURE.md` are
    generated (12 12.3). Owner: `tools/ci docs`.
18. **[review]** A docs review scores the page on ten dimensions, each 0
    to 2: audience fit, task success, content-type discipline, accuracy,
    structure, examples, terminology, AI retrievability, maintenance and
    style. A page ships at 16 of 20 or more with no zero. The review
    report names the audience, the type, the sources checked and the
    commands not run.
19. **[review]** **Docs have a voice.** A reader should feel a person
    with a point of view wrote the page: clear opinions where the
    project has them ("we do X because Y"), warmth toward the reader,
    memorable examples, a light touch of humor where it helps
    understanding, and the product's own imagery (Romeu, Julieta,
    Verona) used with restraint. Personality lives in choices - what to
    explain, which example, what to say plainly - never in adjectives,
    hype or filler, and never at the cost of accuracy, clarity or rules
    3-4. The docs review report says in one line whether the page has a
    voice, and `CONTRIBUTING.md` asks every contributor to write that
    way.
20. **[check]** `SECURITY.md` exists at the repository root and has
    these sections, in order: supported versions, how to report a
    vulnerability (a private channel, never a public issue), and
    response expectations (when the reporter hears back and what
    happens next). Owner: `tools/ci docs`.

This record is 0001. adr-tools' `adr init` would write a first record
titled "Record architecture decisions"; rules 6 to 9 make that decision
here, so a separate record would say the same thing twice. The spec does
not reserve the number for anything else.

### Conflicts resolved

| Conflict | Resolution | Why |
|---|---|---|
| ADR directory: `docs/decisions/` in one source, `docs/adr/` in the spec | `docs/adr/` with `.adr-dir` | the spec and adr-tools agree, and the decision-record source itself defers to an existing convention |
| ADR title: `# ADR-001: Title` vs `# N. Title` with `NNNN-kebab-title.md` | the adr-tools form | one scheme, the one the tooling writes |
| Section order: the spec listed Status, Date, then a top-level Alternatives considered section | adr-tools' template order: `Date:` line, Status, Context, Decision, Consequences; alternatives inside Context | the spec gave no reason for its order, and tools that read ADRs expect the template's |
| Status set: one source dropped Deprecated | Proposed, Accepted, Deprecated, Superseded | Nygard's post and the spec both keep Deprecated for a decision withdrawn without a replacement |
| Supersede link: the spec wrote `Supersedes: NNNN` | adr-tools' `Supersedes [N. Title](file)` in the Status section | the link is clickable, and it is what `adr new -s` writes |
| Screenshots allowed by one source, no media in another | text diagrams only (rule 17) | binary media rot silently and cannot be diffed or retrieved as text |
| Em dash used by two sources | banned everywhere, commits and PR bodies included | the spec already bans it (12 12.7) |
| Hand-written CHANGELOG in one source | generated release notes (rule 11) | the spec generates them (12 12.6); a hand-written file duplicates that source |
| Three README shapes | one order (rule 12), prerequisites before the first run | the first run needs a macOS host with sbx; a reader who lacks them must find out before step one |
| "No TODO comments" in one source | `TODO(#<issue>)` only | a TODO with an issue is tracked work; one without is a wish |
| Where lessons live | `docs/lessons.md` (12 12.7); a lesson that changes a decision also adds an ADR; a trap at a code site gets a comment citing the ADR or lesson | the spec already has a checked lessons file |
| Motivational tone in one source | the prose rules (rule 4) and the voice rule (rule 19) | personality comes from choices, not from enthusiasm |
| Page type: the spec pages state reader and type in their opening line | YAML front matter (rule 1) | a check reads front matter without parsing prose |

## Consequences

- Most of the standard becomes a failing check instead of a review
  comment. Reviews get shorter and spend their time on rules 2, 3, 9,
  15, 16, 18 and 19, which only judgment can cover.
- `tools/ci docs`, `tools/ci hygiene`, `tools/ci sequences` and
  `tools/ci pr` grow. Markdown lint, link check and spell check may need
  an external tool; it enters pinned through `mise.lock` like `reuse`,
  and a Go module dependency goes through the ask-first list. The
  implementation plan picks the tools; this record picks the checks.
- Rule 13 couples guides to scenario functions: changing a guide's
  commands can mean changing a scenario. We accept that cost, because a
  guide whose commands never ran is the letter that never arrived.
- The pages written before this record (`docs/spec.md`, `docs/spec/`,
  `docs/reviews/`) state reader and type in an opening line, not front
  matter. They move to front matter in the same change that lands the
  rule 1 check.
- The generated ADR index (rule 7) is a new generator in 12 12.3; it
  appears with `tools/new adr` in the `ci-bootstrap` task.
- Docs versioning per release (which docs a reader of v1.2 sees once
  v1.3 exists) is not decided here. The implementation plan decides it,
  and records the choice as an ADR if it is expensive to reverse.
- Rule 19 cannot be checked. A page can pass every check and still read
  like a form; the review line in rule 18 is the only guard, and we
  accept that.
- This record stays Proposed until the maintainer signs it off (rule 8).
  The spec cites it now; the implementation plan builds its checks only
  after it is Accepted.
