---
id: T005
title: "`tools/ci pr`"
milestone: M6
module: ci-bootstrap
fate: deferred
owns: []
needs:
  - T003
after:
  - T181
ask_first:
  - checks
  - approvals
  - ask-first
  - dependencies
  - spec
sitting: none
---

# T005 - `tools/ci pr`

Re-plan (2026-10-12): deferred to M6; the sections and trailers are
friction before a product exists; the sections are read in review until
then, and the grant file of 12 12.4 is a recorded intent until `pr`
reads it. Its first slice is the approval-line and grant part, run in
the base-defined pr job of 12 12.4: acceptance items (a), (b), (c) and
(e) below and the pr workflow file, from its first pull request.
Reopened by: the first merged pull request missing a section the review
wanted, or missing an approval line it needed (that first slice may land
first); or before the first release-candidate tag. The text below is the
one written before the re-plan; `needs` lists its earlier dependencies
with a folded id replaced by the id that absorbed it. When it reopens,
`owns` is the pr workflow file, `tools/ci/internal/workflows/**`,
`.github/pull_request_template.md`, `docs/spec/12-engineering.md` and
`docs/spec/05-security.md` (the two pages 12 12.4 assigns this task an
edit of, rule 6 of `docs/plan.md`), and `ask_first` is the forecast
above.

Re-plan (2026-10-29): the first slice is planned as T180 (the reader,
run locally) and T181 (the base-defined pr job), decided on 2026-10-29,
scope this repository, because the `formats` surface (#314) landed with
no check behind it; the approval line becomes a checklist the
maintainer ticks (T180). They take from the acceptance below the
checklist grammar, `TestPRApprovalLine`, the grants, the changed paths,
the payload read, the `head.sha` fetch, the pr workflow file and its
`tools/ci workflows` rows, the hostile and head-job fixtures, and
invariants (a), (b), (c) and (e), and cite the requirement ids those
lines implement; this task keeps citing them too, so its Implements
line and the OverCap ratchet stay as they are. Those lines below stay
as the record, and T180 and T181 supersede them (the plain approval
line of the table becomes the checklist). This task keeps the rest,
REQ-0541 and invariant (d), the 05 5.4 rows on `tools/ci pr` (05 is
owned by T167 while T180 and T181 run), the template's other hints,
and stays deferred with its trigger. The operator item below, the
required check and the edit of open pull request bodies, moves to
T181.

Re-plan (2026-10-29, later the same day): the 05 5.4 row that says the
approval line is typed by the PR's author, and the index row on the
signed approval record, move to T182, so 05 does not wait for this
task to state the checklist of T180.

- Implements: REQ-0022 (index), REQ-0032 (index), REQ-0122 (02 2.1),
  REQ-0262 (03 3.13), REQ-0476 (05 5.2), REQ-0479 (05 5.2), REQ-0483 (05
  5.2), REQ-0484 (05 5.2), REQ-0497 (05 5.2), REQ-0537 (05 5.3),
  REQ-0538 (05 5.3), REQ-0540 (05 5.3), REQ-0541 (05 5.3), REQ-0558 (05
  5.4), REQ-0774 (10 10.1), REQ-0813 (10 10.2), REQ-0815 (10 10.2),
  REQ-0816 (10 10.2), REQ-0817 (10 10.2), REQ-0818 (10 10.2), REQ-0819
  (10 10.2), REQ-0854 (10 10.2), REQ-0884 (10 10.2), REQ-0890 (10 10.2),
  REQ-0942 (10 10.4), REQ-1079 (12 12.2), REQ-1083 (12 12.2), REQ-1130
  (12 12.4), REQ-1131 (12 12.4), REQ-1132 (12 12.4), REQ-1133 (12 12.4),
  REQ-1134 (12 12.4), REQ-1135 (12 12.4), REQ-1136 (12 12.4), REQ-1137
  (12 12.4), REQ-1138 (12 12.4), REQ-1139 (12 12.4), REQ-1141 (12 12.4),
  REQ-1144 (12 12.4), REQ-1145 (12 12.4), REQ-1146 (12 12.4), REQ-1147
  (12 12.4), REQ-1148 (12 12.4), REQ-1149 (12 12.4), REQ-1150 (12 12.4),
  REQ-1151 (12 12.4), REQ-1152 (12 12.4), REQ-1153 (12 12.4), REQ-1154
  (12 12.4), REQ-1155 (12 12.4), REQ-1156 (12 12.4), REQ-1157 (12 12.4),
  REQ-1158 (12 12.4), REQ-1159 (12 12.4), REQ-1160 (12 12.4), REQ-1161
  (12 12.4), REQ-1162 (12 12.4), REQ-1163 (12 12.4), REQ-1166 (12 12.5),
  REQ-1167 (12 12.5), REQ-1178 (12 12.7), REQ-1179 (12 12.7), REQ-1189
  (12 12.9), REQ-1194 (12 12.9), REQ-1196 (12 12.10), REQ-1197 (12
  12.10).
- Operator: item e of 10 10.2: add the pr job to the required checks and
  record under this pull request's Evidence the `head_sha` of its check
  run and whether the ruleset blocks then allows a merge, or that it
  cannot be required. The pull request that changes `pr` lists the open
  pull requests in its body; after the merge the maintainer edits each
  one's body, or closes and reopens it, so the fixed `pr` runs, and
  names each new run under the Evidence of the next pull request (05
  5.4).
- Ask-first, as forecast before the re-plan: `checks`.
- Acceptance:
  - Each line below is a fixture: a recorded event payload and a fixture
    repository (10 10.1).
  - The approvals checklist grammar is applied at the start of this
    task: one `> [!IMPORTANT]` alert, one `> - [ ] Approval: <surface
    id> - <what is asked>` line per surface, a ticked line the grant;
    which event a tick fires is measured on a throwaway pull request
    before `ci.yml` is wired.
  - A body without one of Why, What changed, Evidence, Lessons, or
    without a well-formed Middleware line, fails.
  - A Lessons section that is empty or holds neither a lesson nor the
    word `none` fails.
  - A pull request whose range adds a directory holding a non-test
    `.go` file, outside `testdata/` and other than generated code,
    without a non-empty Prior art section (the search, or `none:
    <reason>` with a reason of decision 3 of ADR 0038) fails; one fixture
    each for a new package, a test-only directory and a generated one.
  - A touched ask-first surface without its approval line fails, with
    the list read at the base and at the head and joined; a line that
    names an unknown id fails; a line that starts with `Approval: ` and
    is not an approval line fails as a malformed approval line, not as a
    surface without a line. `TestPRApprovalLine` holds one case per row
    of the table below, titled with the row's case title verbatim, and
    each outcome is the one 12 12.4 states.

    | Case title | Body lines (diff touches `decisions`) | Outcome |
    |---|---|---|
    | a valid line | `Approval: decisions - amend rule 8` | passes |
    | a lowercase prefix | `approval: decisions - amend rule 8` | fails:
    no line for `decisions` |
    | an uppercase id | `Approval: Decisions - amend rule 8` | fails:
    malformed |
    | an en dash separator | `Approval: decisions` U+2013 `amend rule 8`
    | fails: malformed |
    | an empty phrase | `Approval: decisions - ` | fails: malformed |
    | a straight quote around the phrase | the phrase in U+0022 | fails:
    malformed |
    | a straight quote inside the phrase | U+0022 after the first word |
    fails: malformed |
    | a left curly quote alone | U+201C before the phrase | fails:
    malformed |
    | a right curly quote alone | U+201D after the phrase | fails:
    malformed |
    | a look-alike quote | the phrase in U+00AB and U+00BB | fails:
    malformed |
    | an apostrophe | U+2019 inside the phrase | passes |
    | a leading no-break space | U+00A0 before the phrase | fails:
    malformed |
    | a zero-width space | U+200B before the phrase | fails: malformed |
    | a byte order mark | U+FEFF before the phrase | fails: malformed |
    | a leading combining mark | U+0301 before the phrase | fails:
    malformed |
    | CR LF line ends | a valid line ending in CR LF | passes |
    | a line inside an HTML comment | a valid line between `<!--` and
    `-->` | fails: no line for `decisions` |
    | an unclosed comment | a valid line after `<!--` with no `-->` |
    fails: no line for `decisions` |
    | a duplicate id | two valid lines for `decisions` | fails:
    duplicate |
    | an untouched known surface | a valid line, and one for `kits` |
    passes |
    | a refused line next to a valid one | a valid line, and one with a
    quoted phrase | fails: malformed |
    | an unknown id | a valid line, and one for `nosuch` | fails:
    unknown id |
    | an id with a double hyphen | `Approval: a--b - x`, and a valid
    line | fails: unknown id |
    | a surface removed in the head | the head's list drops `decisions`;
    a valid line | passes, by the union |
    | a surface removed in the head, no line | as above, without the
    line | fails: no line for `decisions` |
  - A grant phrase `granted at <sitting> for <task id>` passes on a
    grantable surface only when `docs/grants.yaml` at the base commit
    holds that grant for that surface and task; fixtures: present
    passes, absent fails, present only at the head fails, the phrase on
    `decisions` fails.
  - A changed golden whose repository path is not written verbatim under
    Evidence fails (10 10.4).
  - A title or a one-parent commit subject outside the Conventional
    Commit form fails; a merge commit's subject is not checked.
  - A prose-rule violation (ADR 0001 rule 4) in the title, in the body
    or in a commit message fails, one fixture per place.
  - A base other than the default branch fails.
  - A walk from `base.sha` to `head.sha` that finds no commit fails (10
    10.2; round 18, ship gate round 1, F12).
  - A local run with `--title` and `--body` makes no network read (10
    10.2; round 18, ship gate round 1, F12).
  - `pr` fetches `head.sha` without credentials only in the pr job and
    only when the object is missing; a fixture with the object present
    makes no network read.
  - The changed paths are those between the merge base of `base.sha` and
    `head.sha`, and `head.sha`, both names of a rename included: a
    fixture that merges the default branch into the head touches no
    surface; `**` matches zero segments in a fixture.
  - A malformed `Fixes-release:` trailer fails. After the first release
    tag a pull request with a commit of type `fix` and no Fixes-release
    line fails; before it the same passes; `Fixes-release: none` passes.
  - A pull request that removes the last guard tag or the last test tag
    of an invariant id whose row 05 5.2 keeps fails.
  - The forbidden-name check runs over the title, the body, the head ref
    name and the four readings of each commit.
  - A payload with neither accepted shape, or without one of the seven
    fields, fails.
  - The pr workflow file is written: `on: pull_request_target` with
    `branches: [main]` and the types `opened, synchronize, reopened,
    edited`, `permissions: contents: read`, no secret and no
    `github.token`, one checkout of `github.sha` with
    `persist-credentials: false`, and its mise step with
    `persist_github_token: false`; `tools/ci workflows` gains the pr job
    row of 10 10.2 with one fixture per refusal (`pull_request_target`
    in a second file, the event without `branches: [main]`, a permission
    other than `contents: read`, a step naming a secret or
    `github.token`, a checkout of a ref other than `github.sha`, one of
    `pull_request.base.sha`, a checkout without `persist-credentials:
    false`, the file without `edited`); the `edited` rule moves from
    `ci.yml` to that file.
  - This pull request replaces `tools/ci pr`, `go run ./tools/ci pr` and
    `tools/ci/askfirst.go` in 12 12.4, 12 12.2, 05 5.3 and the
    `approvals` glob with the names it chose (12 12.4), and changes the
    PR template's Lessons hint to `A lesson this work taught, or the
    word none.` and its surfaces pointer to `.github/ask-first.yaml`.
  - The leading-space and `##[` cases are measured once on a throwaway
    workflow run, and 12 12.4 drops "as remembered of the runner's code
    and not measured" in the same pull request.
  - The invariant of 05 5.3 holds: no code that a pull request changes
    judges that pull request. This task decides where the code of `pr`
    lives and how it reads the head, and shows each of these (round 11,
    R11-01 to R11-03, decided on 2026-10-09; round 12, R12-02, R12-05
    and R12-06):
    - (a) every input `pr` reads from the head is parsed by code under
      paths on `approvals`, its YAML and TOML readers included (R11-02);
    - (b) the code `pr` builds imports only the standard library and
      packages under paths on `approvals`, and the `approvals` globs of
      `.github/ask-first.yaml` name those packages' directories
      (R11-01). A test holds it: it reads each package of `go list -deps
      -test` with its `Standard` field and its `Dir`, once for each lint
      target of 10 10.2 from the same list, so an import behind a build
      constraint of another target is read too, takes the standard
      library from `Standard`, and matches the `Dir` of every other
      package to an `approvals` glob; a fixture package that imports a
      path outside the globs turns it red, and so does a fixture with a
      module path without a dot under a local `replace` (round 13,
      L2-r13-2, which replaces the dot rule of R12-06), whose removal is
      reading the standard library by that dot rule instead of
      `Standard`, and a fixture whose outside import builds only for
      darwin, whose removal is listing one target (round 14, R14-11);
    - (c) `pr`, built from the default branch, applies the version-pin
      rules of `mise.toml` and `go.mod` of 10 10.2 to the head (R11-03),
      and refuses a head that holds a tracked `go.work`, `go.work.sum`
      or `vendor/`, the hygiene rule of 10 10.2 applied from the base,
      since the go command reads those files from the checkout and the
      rule that refuses them runs from the head's `tools/ci`; one
      fixture each, and one for a head that edits a `go.work` already
      tracked on the base, and removing the path pattern of its file
      turns each red (R12-02; round 13, R13-04);
    - (d) the mise toolchain and the `go` line of `go.mod` are the
      grantable inputs left under the job, each listed in 05 5.4;
    - (e) every committed `.github/ask-first.yaml` and
      `docs/grants.yaml` gives `pr` and the generator of 12 12.3 the
      same surface set (R12-05). When each has its own reader, a test
      holds it with fixtures: two files with the same ids and different
      globs fail, two files where one holds an id the other lacks fail,
      identical files pass, and removing the glob comparison or the id
      comparison turns the matching fixture red; when T005 takes one
      reader, the test asserts that both call it, and a second reader
      inlined in either turns it red (round 13, L6-r13-2; round 14,
      R14-12).
  - Each fixture 12 12.4 names is an acceptance line here, and the
    removal written after it turns it red (R12-06):
    - the hostile fixture repository (a submodule, a textconv driver, an
      external diff driver and hooks) against each git call `pr` makes
      on head objects: removing any one flag of 12 12.4 from its call;
    - "pr escapes a workflow command in a PR body", with a leading
      space, a leading U+00A0 and a `##[` case: removing the fixed
      prefix or the `##[` escape;
    - a head job outside the pr job's file whose check name equals the
      pr job's: removing the name comparison;
    - a head job outside the pr job's file whose `name` holds `${{`, one
      with a literal expression and one with a matrix value: removing
      the `${{` refusal;
    - a head version of the pr job's file that names an event other than
      `pull_request_target`: removing the event check.
- Verify: `go test ./tools/ci/... -run '^TestPR'`, which runs the
  fixture lines; `go run ./tools/ci pr <payload file>` on a saved
  payload.
