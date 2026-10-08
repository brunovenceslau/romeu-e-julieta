# 1. Adopt a documentation standard with checkable rules and a voice

Date: 2026-09-30

## Status

Accepted

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
question. Before this record the spec held these pages to a short list
of its own, six bullets in
[12 12.8](../spec/12-engineering.md#128-contributor-docs-outlines),
which now points here instead: name the reader, one content type per
page, self-contained sections, copyable examples, concise prose,
decisions in ADRs. That list was correct, and none of it was enforced.
Our first principle says anything
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
- **Keep the six bullets in 12.8.** They were right and unenforced, and
  they lived in a spec that describes current state rather than
  decisions, so a change to them had no record of why. Rejected; the
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
- **Generate the guide's command blocks, or the command table, from
  the scenario functions.** That would reuse the generator and drift
  check of 12 12.3 and need no matcher (rule 13). But a guide is a
  hand-written page, and a generator that writes blocks into it has to
  inject text between markers in a file people edit, which is the
  opposite of rule 10's model (a generated file is never hand-edited).
  Rejected; one committed table that both the guides and the scenarios
  read keeps each file either written or generated, never both.

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
| Reference (`docs/reference/`) | generated from code (12 12.3), shaped as reference: tables, exact values, defaults | `tools/ci generated` drift check; front matter emitted by the generator; never hand-edited |
| Guides (`docs/guide/`) | Diataxis how-to, one journey per page, named in front matter | runbook shape: expected output and recovery from each expected error id |
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

1. **[check]** A hand-written page names its Diataxis type and its
   primary reader in YAML front matter (`type:` one of `tutorial`,
   `how-to`, `reference`, `explanation`; `reader:` a phrase naming one
   reader), and has one type. A page under `docs/guide/` also names its
   journey (`journey: J<n>`, the id of a `## J<n>` section of
   [09](../spec/09-journeys.md)); a value that is not such an id
   fails. The check covers a path set:
   `ARCHITECTURE.md`, `CONTRIBUTING.md`, `SECURITY.md` and the Markdown
   files under `docs/`, minus `docs/adr/` and `docs/reviews/` (ADRs and
   review rounds are historical records, and the path says what they
   are) and minus `docs/reference/` (generated; the generator emits the
   same front matter, and rule 10 checks it). Markdown outside that set
   belongs to another consumer and is out of scope: the root
   `README.md` (rule 12), `SKILL.md` files (the agent runtime reads
   their front matter) and `.github/` templates (GitHub copies them into
   a PR body as they are). Owner: `tools/ci docs`.
2. **[review]** The reader's journey comes first: outcome,
   prerequisites, the shortest correct path; limits, alternatives and
   detail after, unless safety needs them earlier.
3. **[review]** Every claim is grounded in code, tests, the spec or
   measured behavior. Stale content is deleted, not caveated.
4. **[check]** Prose rules, repository-wide and in PR titles, PR bodies
   and commit messages: no em dash character (U+2014), no hype words, no AI
   meta-commentary, no attribution footer, no paired
   `not only ... but also` construction. The word lists are committed
   data with a self-test, in `tools/ci/prose.yaml`. A listed word or
   phrase matches as whole words, with ASCII case ignored, outside code
   spans and fenced blocks, so a rule can quote what it bans. The em
   dash check skips nothing.
   Owner: `tools/ci hygiene` for files, `tools/ci pr` for the PR
   title, the PR body and the PR's commit messages.
5. **[check]**, external URLs excepted. Markdown lint, link check and
   spell check. Relative links and anchors are checked offline in every
   run; text inside a code span or a fenced block is not a link, so a
   rule can show a link form. The spell check reads
   one accepted-words list; its format and path are fixed in the plan
   task that picks the tool. External URLs are the exception:
   `tools/ci all` stays offline (10 10.1), so they are fetched by
   `tools/ci links`, which a person runs and the final plan task runs
   once. That half of the rule is not a gate in v1, and between those
   runs nothing reminds anyone; Consequences says what reopens it.
   Owner: `tools/ci docs`; `tools/ci links` for
   external URLs.
6. **[check]** ADRs use the adr-tools layout: `.adr-dir` contains
   `docs/adr`; files are `NNNN-kebab-title.md`, and the generated
   `README.md` of rule 7 is the one other file there; the first line is
   `# N. Title`; then a `Date: YYYY-MM-DD` line, then the sections
   `## Status`, `## Context`, `## Decision`, `## Consequences`, in that
   order. Alternatives go in `### Alternatives considered` inside
   Context. The title matches the filename through one function: the
   filename is `NNNN-` plus `slug(Title)`, where `slug` lowercases the
   ASCII letters of the title, turns each run of characters other than
   ASCII letters and digits into one hyphen, and trims hyphens from
   both ends.
   `tools/new adr` takes the title and writes the number, the date, the
   title line and the filename (12 12.3), so nobody types them, and
   `tools/ci sequences` calls the same `slug`, which lives under
   `tools/ci/` and so on the `checks` surface. The date is checked for
   its shape and no further: a commit date is set by its author, so it
   proves nothing about the day. Owner: `tools/ci sequences`.
7. **[check]** ADR status is one of Proposed, Accepted, Rejected,
   Deprecated, Superseded. A Proposed record the maintainer declines
   becomes Rejected, stays in the repository, and gains one line in its
   Status section saying why. A reversal is a new ADR whose Status
   section says
   `Supersedes [N. Title](NNNN-title.md)`, and the old ADR's Status
   section gains `Superseded by [M. Title](MMMM-title.md)`; the links
   must match in both directions. ADRs are never deleted, and numbers
   are contiguous and unique. The index `docs/adr/README.md` is
   generated from the titles and statuses. Owner: `tools/ci sequences`
   for links and numbers, `tools/ci generated` for the index.
8. **[check]** An ADR leaves Proposed on the maintainer's explicit
   sign-off, and changes after that only with it. The mechanism is the
   ask-first list: `docs/adr/**` and `.adr-dir` are one of its surfaces
   ([05 5.3](../spec/05-security.md#53-ask-first-surfaces)). A PR that
   touches an ADR in any way (a new record, a status change, any other
   edit) therefore needs an approval line for that surface in its
   body, in the one form
   [12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change)
   defines, and a code-owner review before it merges. The intent is
   that an agent does not promote or rewrite a record on its own. The
   approval line is typed by the PR's author, so `tools/ci pr` proves
   it is there, not who said it. The code-owner review and the merge
   are what an author cannot type. The default-branch ruleset requires
   them; 12 12.4 says who sets it up, and
   [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1)
   says what the review is worth while the project has one account.
   The code of these checks is an ask-first surface too, so a PR that
   switches one off needs the same line and review.
   Whether the decision is sound stays a review judgment (rule 9).
   Owner: `tools/ci pr` for the approval line; the default-branch
   ruleset for the review.
9. **[review]** Every decision that is expensive to reverse gets an
   ADR, with the alternatives considered and, for any borrowed pattern,
   its source and why it fits this project. A new record's title starts
   with a verb, not with an acronym or a name, so that the lowercased
   first letter of the check below reads correctly. **[check]** For
   each record, the first `ADR` plus that record's four-digit number in
   a file starts an inline link to the record's file under `docs/adr/`,
   whose text is that mention, a comma, a space and the record's title
   with its first letter lowercased (a line break and the indentation
   after it count as one space); a later mention may be the number
   alone. A first mention in a heading, in front matter or inside
   another link's text fails, and so do a number with no record and a
   plural form (`ADRs` plus a four-digit number): name each record. The
   check reads the Markdown files of rule 1's path set, `README.md` and
   the records in `docs/adr/`, skips code spans and fenced blocks
   throughout, and exempts a record's own number in its own file.
   Owner: `tools/ci docs`.
10. **[check]** Reference pages are generated from their single source
    (12 12.3), carry a generated-file header and the front matter of
    rule 1, both written by the generator, and are never hand-edited;
    drift fails the build. Owner: `tools/ci generated`.
11. **[check]** Release notes are generated from Conventional Commit
    subjects (12 12.6); nobody writes a CHANGELOG by hand. Commit
    subjects and the PR title are checked for the Conventional Commit
    form (a squash merge can make the title a subject), and PR bodies
    for the sections of 12 12.4. Owner: `tools/ci pr`.
12. **[check]** `README.md` uses this heading order: what it is and who
    it is for; when not to use it; prerequisites; install and verify;
    first run; trust model; links to guides, reference and ADRs. The
    check compares the page's level-2 headings with an ordered list in
    `tools/ci/headings.yaml`; a heading that is missing, extra or out of
    order fails. The plan task that writes the page's skeleton fixes
    the strings. **[review]** Command and settings tables are links to
    the generated reference, not copies. Owner: `tools/ci docs`.
13. **[check]** The `romeu` and `julieta` commands shown in `README.md`
    and `docs/guide/` are command lines the scenario functions run. A
    guide page names its journey in
    front matter (rule 1); `README.md` belongs to J1. The commands have
    one committed source, `e2e/scenarios/commands.yaml`: scenario id to
    ordered command lines. A scenario id is a journey id (`J<n>`), or a
    variant that 09 names inside a journey, written as the journey id
    and one lowercase letter (J3a and J3b inside J3). The
    scenario functions read their command lines from that table.
    In those pages every fenced block names its language, one of `sh`,
    `text`, `yaml`, `json`, `toml` and `mermaid`; `tools/ci docs` fails
    a block with another language or none, `bash` and `console`
    included. An `sh` block holds commands.
    The check reads each line of an `sh` block as text: it interprets
    no quotes and no shell syntax. It splits the line into parts at
    `&&`, `||`, `;` and `|`, and each part into words at ASCII
    whitespace. A command word is `romeu` or `julieta`, or a word that
    ends in one of them after a character other than an ASCII letter, a
    digit, `-`, `_` or `.` (`./romeu`, `bin/julieta`, `$(romeu`). In a
    part that has a command word, the words from the first command word
    to the end of the part, with that word read as the bare name, must
    equal an entry of the page's journey or of one of its variants,
    word by word, or the build fails. Inside an entry, a `<placeholder>`
    token matches one or more characters that are not whitespace, so
    the entry word `projects/<name>.yaml` matches `projects/foo.yaml`.
    So a prompt, an assignment or `sudo` before the command does not
    hide it. A command inside quotes or `$( )`, or followed by a
    comment, is found and then equals no entry: the check fails closed,
    and these pages show commands bare.
    A table entry that no scenario function reads fails the scenario
    package's own test, which drives each function against a recording
    runner and executes nothing. Reading is not running: the CI suite
    runs J2, J3b, J7, J10 and J11 against the fake sbx, and the host
    suite runs each journey against real sbx on maintainer machines
    (10 10.1). A command of a journey outside that CI list is exercised
    by the host suite and not in CI. The maintainer decided to keep
    that alternative, because CI is offline and has no real sbx.
    Three limits. A line that names a binary as a file and does not
    run it (`mv romeu ...`, `chmod u+x romeu`, `| grep julieta`, a
    path that ends in `cmd/romeu`) has a command word and equals no
    entry, so it fails; such lines go in a `text` block. A block in
    another language is output or data and is
    not matched, so a command shown there is not checked. A part with
    no command word (the checksum and attestation commands
    of install and verify, for example) is outside this check: it needs
    the network and a published release, and `release.yml` runs that
    verification (10 10.2). Owner: `tools/ci docs`.
14. **[check]** Every exported Go identifier has a doc comment; error
    strings follow Go style (lowercase, no trailing punctuation); no
    commented-out code; the only accepted TODO form in a Go file is
    `TODO(#<issue>)`. Owner: `golangci-lint` for the first three,
    `tools/ci hygiene` for the TODO form. The three checkers are
    switched on in the linter configuration, and a unit test runs the
    linter with that configuration over a fixture holding one violation
    of each, so a configuration that stops reporting them fails the
    build (10 10.1).
15. **[review]** Comments explain why, not what. A comment at a trap
    cites the ADR or lesson behind it. Every package lives under
    `internal/`, so Example tests are written where
    a doc comment cannot show the call contract on its own.
16. **[review]** Error messages and CLI help say what failed, why, and
    what to do next; the fix hint in the error table (04) is the "next".
17. **[check]** Diagrams are text (Mermaid or ASCII): a file under
    `docs/` has the extension `.md`, `.json` or `.yaml`, and no page of
    rule 1's path set, nor `README.md`, holds Markdown image syntax or
    an `<img` tag. The state-machine diagrams in `ARCHITECTURE.md` are
    generated (12 12.3). **[review]** A diagram appears only where
    prose cannot explain. Owner: `tools/ci docs`.
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
    happens next). The check is the one of rule 12: the level-2
    headings against the list for this file in
    `tools/ci/headings.yaml`. Owner: `tools/ci docs`.

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
| Status set: one source dropped Deprecated, and none gave a declined proposal a status | Proposed, Accepted, Rejected, Deprecated, Superseded | Nygard's post and the spec both keep Deprecated for a decision withdrawn without a replacement; without Rejected, a declined record that is never deleted would read as pending forever. MADR's template ([adr.github.io/madr](https://adr.github.io/madr/)) has the same status |
| Supersede link: the spec wrote `Supersedes: NNNN`; adr-tools' `adr new -s` writes the link forms `Supercedes` and `Superceded by`, misspelled ([src/adr-new](https://github.com/npryce/adr-tools/blob/master/src/adr-new)) | `Supersedes [N. Title](file)` and `Superseded by [M. Title](file)` in the Status section, correctly spelled (rule 7) | the link is clickable and checked in both directions. We keep adr-tools' link shape and not its spelling; that is a recorded divergence, so a record written by `adr new -s` itself fails `tools/ci sequences` until the word is corrected |
| Screenshots allowed by one source, no media in another | text diagrams only (rule 17) | binary media rot silently and cannot be diffed or retrieved as text |
| Em dash used by two sources | banned everywhere, commits and PR text included | the spec's Boundaries banned it before this record ([index](../spec.md#boundaries-for-everyone-who-changes-this-repo)). We keep the ban because one dash form makes the check a byte search that skips nothing (rule 4) |
| Hand-written CHANGELOG in one source | generated release notes (rule 11) | the spec generates them (12 12.6); a hand-written file duplicates that source |
| Three README shapes | one order (rule 12), prerequisites before the first run | the first run needs a macOS host with sbx; a reader who lacks them must find out before step one |
| "No TODO comments" in one source | `TODO(#<issue>)` only | a TODO with an issue is tracked work; one without is a wish |
| Where lessons live | `docs/lessons.md` (12 12.7); a lesson that changes a decision also adds an ADR; a trap at a code site gets a comment citing the ADR or lesson | the spec already has a checked lessons file |
| Motivational tone in one source | the prose rules (rule 4) and the voice rule (rule 19) | personality comes from choices, not from enthusiasm |
| Page type: the spec pages state reader and type in their opening line | YAML front matter (rule 1) | a check reads front matter without parsing prose |
| Which pages the front matter check covers | a path set (rule 1): the root contributor pages and `docs/`, minus ADRs, review rounds and generated reference | "every Markdown page" would put front matter into files another consumer parses: a PR template GitHub copies into each PR body, and `SKILL.md` files the agent runtime reads |
| Executing documented commands: an earlier draft of this standard ran each one in CI against the built binaries | the CI suite against the fake sbx, or the host suite against real sbx (rule 13) | CI is offline and has no sbx (10 10.1). J1 and the other host-only journeys cannot run there, and a rule that only CI can satisfy would leave their guides unexecuted or unwritten. The maintainer decided to keep the host-suite alternative for that reason. The cost: a host-only command runs on maintainer machines in block B, not on each PR |

## Consequences

- Most of the standard becomes a failing check instead of a review
  comment. Reviews get shorter and spend their time on rules 2, 3,
  15, 16, 18 and 19 and the review halves of rules 9, 12 and 17, which
  only judgment can cover.
- `tools/ci docs`, `tools/ci hygiene`, `tools/ci sequences` and
  `tools/ci pr` grow, and `tools/ci links` is new. Markdown lint, link
  check and spell check may need
  an external tool; it enters pinned through `mise.lock` like `reuse`,
  and a Go module dependency goes through the ask-first list. The
  implementation plan picks the tools; this record picks the checks.
- Rule 8 puts `docs/adr/**` and `.adr-dir` on the ask-first list, and
  the list also gains the code of the checks themselves (`tools/ci/**`,
  `.githooks/**`, the linter configuration). The maintainer decided
  both. The cost: a PR that adds a Proposed record needs an approval
  line and a code-owner review like any other ADR change.
- Rule 13 couples guides to scenario functions through
  `e2e/scenarios/commands.yaml`: changing a guide's commands means
  changing that table, and so the scenario. We accept that cost,
  because a guide whose commands never ran is the letter that never
  arrived.
- The spec pages written before this record (`docs/spec.md`,
  `docs/spec/`) state reader and type in an opening line, not front
  matter. They move to front matter in the same change that lands the
  rule 1 check. The review rounds in `docs/reviews/` stay as they were
  written: like ADRs they are history, and rule 1 exempts them.
- The generated ADR index (rule 7) is a new generator in 12 12.3; it
  appears with `tools/new adr` in the `ci-bootstrap` task.
- Three things are deferred, not decided, each with what holds until
  then and the event that reopens it, in the spec's
  [Deferred decisions](../spec.md#deferred-decisions) table: a
  scheduled workflow that checks external links (rule 5), generating
  part of the spell-check word list from the vocabulary table (rule 5),
  and docs versioning per release (which docs a reader of v1.2 sees
  once v1.3 exists).
- Rule 19 cannot be checked. A page can pass every check and still read
  like a form; the voice line that rule 19 adds to the rule 18 review
  report is the only guard, and we accept that.
