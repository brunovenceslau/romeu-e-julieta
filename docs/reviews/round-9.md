# Spec review round 9: the targeted re-audit of round 8

Reader: reviewers checking that every finding of round 9 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 8 is
in [round-8.md](round-8.md); the lenses are defined in
[lenses.md](lenses.md).

Round 9 is step 6 of "How stage A runs, and when the spec counts as
validated" in lenses.md: the targeted re-audit of the round-8 write
pass. Each lens read the sections that pass changed, checked the
round-8 findings it had raised against the new text, and reported what
was left, what regressed and what the new text added. lenses.md is
cited at its own commit, 29feb51.

The baseline is commit 8ecfbb2, the head of pull request #25 (the
round-8 write pass, branch `docs/review-round-8`), read from a detached
clone that nothing wrote to while the reviewers ran. It is not a commit
of the default branch, as step 1 of lenses.md asks for a full round:
the write pass is validated before it merges, so the re-audit reads the
pull request's head.

After the reviews, pull request #25 was rebased onto the default branch
at cecdd14, which brought in pull request #33 ("docs(spec): record that
the default-branch ruleset no longer restricts updates"); the rebased
head is d0bfdcc. The write pass of this round starts from d0bfdcc, and
each finding was checked against that text before it was applied. The
one finding the rebase closed, R9-01, says so in its row.

Every item below is a finding of a reviewer or a proposal of the
consolidation. Where a row needs the operator, the item is in
"Decisions for the operator" below with a recommendation, and the
decision is recorded next to it, with its date, when it comes.

Status, as in rounds 6 to 8: **A** = applied as proposed; **M** =
applied in a smaller or different form than proposed, as its row
states; **L** = stated as a limit, no mechanism added; **operator** =
not applied, because the change touches a decision record, an accepted
risk, the scope or a path outside `docs/`; the item is in "Decisions
for the operator".

The reports are kept in `round-9/reports/<lens>.json`, as the reviewers
wrote them, less one field: `report_file`, the report's path in the
reviewer's scratch directory, which names a host path and says nothing
about the spec.

## How the round was run

Seventeen reviewers, one per lens, L0 to L16, each in a fresh context,
read-only, in parallel on 8ecfbb2. L2 and L13 ran on `opus`, the rest on
`sonnet`. Each lens ran once: a re-audit carries no noise probe, since
the probe measures a lens set on a full read, and round 8 recorded it.
Every lens ran because every lens had findings that drove the round-8
delta. Each wrote one JSON report with `round8_checked`,
`round8_resolved_as_stated` and its findings, each of kind `unresolved`
(a round-8 finding the pass did not resolve as its row states),
`regression` (the pass broke what held) or `new` (a defect in text the
pass wrote or a seam it opened).

The reports hold 94 findings: 0 Blocking, 35 Required, 59 Advisory; 64
new, 26 unresolved, 4 regression. One consolidator merged them by
section and defect, kept every lens on its row, and checked each
Required finding against the baseline text. Three Required clusters were
downgraded to Advisory, each with its reason in its row; none was
dropped. The result is 88 clusters: 0 Blocking, 27 Required, 58
Advisory (3 of them parked for stage B).

### Per lens

`checked` is the number of round-8 findings the lens re-read against
the new text, and `resolved` how many of them read as their round-8 row
states. The checks are per lens, so a round-8 cluster several lenses
raised is counted once per lens.

| Lens | Model | round8_checked | resolved_as_stated | Findings (B / R / A) | new | unresolved | regression |
|---|---|---|---|---|---|---|---|
| L0 | sonnet | 33 | 31 | 4 (0 / 2 / 2) | 2 | 2 | 0 |
| L1 | sonnet | 26 | 25 | 3 (0 / 1 / 2) | 1 | 1 | 1 |
| L2 | opus | 48 | 46 | 9 (0 / 2 / 7) | 7 | 1 | 1 |
| L3 | sonnet | 28 | 28 | 6 (0 / 1 / 5) | 6 | 0 | 0 |
| L4 | sonnet | 37 | 36 | 4 (0 / 1 / 3) | 3 | 1 | 0 |
| L5 | sonnet | 33 | 29 | 7 (0 / 2 / 5) | 4 | 3 | 0 |
| L6 | sonnet | 33 | 31 | 8 (0 / 3 / 5) | 6 | 2 | 0 |
| L7 | sonnet | 30 | 28 | 5 (0 / 3 / 2) | 3 | 2 | 0 |
| L8 | sonnet | 29 | 25 | 8 (0 / 5 / 3) | 2 | 5 | 1 |
| L9 | sonnet | 23 | 21 | 5 (0 / 2 / 3) | 2 | 2 | 1 |
| L10 | sonnet | 28 | 27 | 4 (0 / 0 / 4) | 3 | 1 | 0 |
| L11 | sonnet | 33 | 31 | 2 (0 / 1 / 1) | 1 | 1 | 0 |
| L12 | sonnet | 22 | 21 | 4 (0 / 2 / 2) | 3 | 1 | 0 |
| L13 | opus | 66 | 64 | 12 (0 / 3 / 9) | 10 | 2 | 0 |
| L14 | sonnet | 19 | 19 | 2 (0 / 2 / 0) | 2 | 0 | 0 |
| L15 | sonnet | 23 | 22 | 7 (0 / 4 / 3) | 6 | 1 | 0 |
| L16 | sonnet | 17 | 16 | 4 (0 / 1 / 3) | 3 | 1 | 0 |
| all | | 528 | 500 | 94 (0 / 35 / 59) | 64 | 26 | 4 |

Severities in this table are the reviewers'. The consolidation's are in
the findings table below.

## Findings and resolutions

One row per cluster. The Required rows come first, then the three
downgraded rows, then the Advisory rows in lens order. Kind is the
cluster's: `unresolved` when any member is, else `regression` when any
member is, else `new`. Sections are cited as the spec cites itself.

### Required

| Cluster | Section | Lenses | Kind | Defect | Verification | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R9-01** | 05 5.4 (approval-line row); 12 12.4 (merge row); ADR 0009 (Context, decision 1); 10 10.2 (maintainer block, items a and e); index Deferred decisions | L0 (L0-r9-1) | unresolved (L0-10) | 05 5.4, the 12 12.4 merge row and ADR 0009 say the default-branch ruleset requires CI green, while item a of 10 10.2 lists no status check, item e (add them) is To do, and the index row "required status checks without the administrator bypass" says they are not in place. | holds: 05 5.4 approval-line row, 12 12.4 merge row and ADR 0009 Context say "requires CI green"; 10 10.2 item a names no check and item e reads "To do". Closed by the rebase onto cecdd14: pull request #33 and the rebased item a read a required-status-checks rule that lists the four `all on ...` checks (as read on 2026-10-09, [the note on the maintainer block](../notes/maintainer-block-of-ci-bootstrap.md)), so "requires CI green" holds as written | none for the defect. The write pass moves the required-checks clause from item a to item e, which set it up, and says that only saving the API's answer is left; the index Deferred row whose trigger this fired is D4 | M: closed by the rebase onto cecdd14; only the move of the required-checks clause from item a to item e applies |
| **R9-02** | 10 10.2 (grammar row "release publish job"; Release and bootstrap steps 1 and 4); 12 12.6 | L0 (L0-r9-2), L7 (L7-9r-1), L9 (L9-R9-1), L13 (L13-R9-3) (high-signal) | unresolved (R8-10-1) | Only the `tools/release publish` and `tools/release verify` steps may hold `GH_TOKEN`, but `tools/release build` reads the latest scheduled `fuzz.yml` run and `tools/release notes` reads merged pull requests, authors and closing issues, both GitHub API reads. | holds: the grammar row names publish and verify only; step 1 and 12 12.6 describe the two reads | the row admits `GH_TOKEN` on the `build`, `notes`, `verify` and `publish` steps of that job and no other; step 1 and 12 12.6 name the token each read uses; one `tools/ci workflows` fixture per admitted and refused step (approval lines: `checks`, `release`) | A |
| **R9-03** | 10 10.2 (Release and bootstrap step 1; e2e row); index S6 | L6 (L6-r9-1); L9 (L9-R9-1, its dry-run clause) | new | The fuzz refusal of `tools/release build` does not exempt `--dry-run`, which the e2e step of every `all` run calls, so a red or missing scheduled run, or no network, fails every pull request's e2e. | holds: step 1 has no exemption; the e2e row builds with `tools/release build --dry-run` | the refusal applies to a build that is not `--dry-run`; the dry-run build reads no network; the refusal's three cases (success, not success, no run found) are fixtures of `tools/release` (the plan's acceptance line is parked for B) | A |
| **R9-04** | 10 10.2 (Release and bootstrap; the publish job) | L7 (L7-9r-2) | new | The publish job "runs no test and no `tools/ci` step" and takes no tool or cache from the runner, yet `tools/release build` runs with `GOPROXY=off` and needs the pinned go and a filled module cache, which only `tools/ci setup` provides. | holds: 10 10.2 says setup is the step that runs `go mod download`; the publish job runs none | name the one exception: the publish job runs `go run ./tools/ci setup` first, in the setup environment of 10 10.2 (`GOPROXY` and `GOSUMDB` set), and then the release steps in the steps' environment | A |
| **R9-05** | J1 step 2; index S1; 10 10.2 (Verification contract) | L7 (L7-1r) | unresolved (L7-1) | The contract lists the commit the tag names and hosted runners only, but J1 step 2 and S1 show `gh attestation verify` with `--repo` and `--signer-workflow` alone, so a user following J1 accepts any run of `release.yml`. | holds: J1 step 2 and S1 show those two flags only | read the pinned `gh`'s `attestation verify --help` and write the flags for the commit and the hosted-runner clauses into the `commands.yaml` entry; J1 step 2 and S1 show the same command and say where the tag's commit is read | A |
| **R9-06** | 01 1.4 (Gate 1); 01 1.5; 05 5.2 I7; 04 4.2 | L1 (L1-r9-1), L13 (L13-R9-2) (high-signal) | regression (R8-01-16) | 01 1.4 says every command that invokes sbx exits 3 on a gate 1 mismatch, with only `status` and `doctor` excepted, while 01 1.5, the I7 test and the 05 5.4 row let the protective commands print it and go on; `approve --toolchain` is not excepted either. | holds: 01 1.4 and the protective paragraph of 01 1.5 read as stated | one sentence in 01 1.4: every command that invokes sbx exits 3 on a mismatch, except `approve --toolchain` (gate 1 itself), `status` and `doctor` (they report it), and the protective commands of 01 1.5 (they print it and go on) | A |
| **R9-07** | 01 1.4; 01 1.5 step 5; 01 1.6; 04 4.2 (run step 2); 04 4.4 (RJ-308 to RJ-310); 03 (opening); 10 10.3 (Tested window) | L13 (L13-R9-1), L15 (L15-R9-3), L4 (L4-r9-3, half b) (high-signal) | new | One condition, an sbx output that does not decode or lacks a field, has three ids and two exits: `sbx-output-unparsed` (RJ-309, exit 2) in 01 1.4 and 10 10.3, `sbx-unknown` (RJ-308, exit 1) in 01 1.6, `upstream-shape` (RJ-310, exit 2) in 03. | holds: the five places read as stated | one rule in 03 (opening), cited by the others: an upstream output that does not parse or lacks a read field is `upstream-shape` (exit 2), naming the upstream; `sbx-unknown` stays only for an sbx call that exits non-zero; `sbx-output-unparsed` is retired as RJ-309, marked retired in the table, and its users cite RJ-310 | A |
| **R9-08** | 03 (opening); 07 7.5 step 1; 04 4.4 (RJ-325) | L15 (L15-R9-4), L2 (L2-r9-4) (high-signal) | new | 03 lists `mise.lock` among both the strictly parsed formats and the tolerantly decoded upstream outputs ("unknown fields are allowed"), and 07 7.5 makes an unknown top-level key a sync error. | holds: `mise.lock` is named in both lists of 03, and 07 7.5 step 1 reads as stated | `mise.lock` leaves the tolerant list: it is agent-writable and drives egress, so its top level is strict as 07 7.5 says, with the reason in 03; its TOML limits sit beside the YAML ones; `lock-unparsed` stays for it | M: mise.lock leaves the tolerant list and its top level is strict; its TOML limits are stated as no duplicate keys or tables and the file cap, the only limits the text can name without a new bound |
| **R9-09** | 01 1.6 (Generation); 01 1.7 (open generation); 01 1.5 step 6; J6 steps 3 and 5 | L4 (L4-r9-1) | new | The new state `removing` has no row for a sandbox still present (a failed `sbx env rm`, or a kill before it), and the 01 1.7 definition of an open generation excludes `removing`, so step 6 refuses the operator's own sandbox with `unknown-sandbox`. | holds: the table has `removing` rows for success and for an absent sandbox only; 01 1.7 says "open or salvaging" | a row: `removing`, `sbx env rm` fails or the sandbox is found present, to `removing`, exit 1, and a rerun of `rm` resumes at `sbx env rm`; 01 1.7 counts `removing` as open for step 6; I17 gains the fault case; J6 steps 3 and 5 pass through `removing` | M: open generation counts removing for preflight step 6 only, since counting it everywhere would make run step 2 treat a removing generation with no sandbox as lost |
| **R9-10** | 08 8.3 (Resume); 08 8.2 | L5 (L5-r9-1) | new | The gap line "the session ended without /handoff" fires whenever the newest facts file is newer than the latest narrative, which is also true after `/handoff` then `/clear`, because SessionEnd writes a facts file at `/clear`. | holds: the Resume row and the SessionEnd row read as stated, and 8.3 itself says SessionEnd fires at `/clear` | the gap line prints only when the newest facts file's git facts differ from the facts stamped in the latest narrative (new commits, a moved branch, a changed dirty count); goldens for handoff then `/clear` (no line) and a facts-only end with new commits (the line) | M: the gap line compares the git facts of the facts file and the narrative, the form the row states |
| **R9-11** | 08 8.2 (SessionStart row); 04 4.3 (`julieta setup`) | L5 (L5-r9-2) | unresolved (R8-04-43) | `julieta setup` records `default-branch-moved` for "the next SessionStart", but the SessionStart row, the one home of that output, lists its warnings without it and no rule says when it stops. | holds: the 08 8.2 row's list of warnings has no such item | add it to the list, shown by the first SessionStart after the move and then cleared | A |
| **R9-12** | 01 1.4 (Gate 2, the ungated list); 03 3.2; 05 5.2 I7 | L6 (L6-r9-2) | new | `go generate` fails on a field that is on neither the gate tags nor the ungated list, which "today holds `sandboxOptions`", while 03 3.2 gives `run` no gate and I7 names `run` among the fields without the tag. | holds: 01 1.4 names `sandboxOptions` only; 03 3.2 has `run` with gate `-` | `run` joins the list in 01 1.4, with its reason (pane commands run only inside the sandbox), and 03 3.2 says so; the three places name the same members | A |
| **R9-13** | 11 11.2 (block B opening); 10 10.3 (Tested window) | L6 (L6-r9-3) | unresolved (R8-10-13) | Block B's replay of every recorded argv shape against the candidate's sbx has no probe id, no result file, no verdict and no outcome for a difference, and covers mutating shapes with no scope. | holds: one sentence in 11 11.2, no id | the replay is probe B6, `kind: check`: an observation per shape and its comparison, a fail naming the shape, read-only shapes and the harness's own throwaway sandbox only | A |
| **R9-14** | 03 (opening); 04 4.4; 12 12.5 | L3 (L3-R9-1), L8 (L8-R9-7), L13 (L13-R9-10) (high-signal) | new | 12 12.5 says the spec names no plan task, but 03 names T021 and 04 4.4 names T008; and 04 4.4 becomes a link once the table lands while the plan has a test read its rows. | holds: both ids are in the text; the T008 test is in plan.md | each id becomes its event ("until the error table of 04 4.1 lands in `internal/cli`", "the pull request that lands a format's generator"); 04 4.4 says the table is then the source and the section a link. The T008 test is parked for B | A |
| **R9-15** | 04 4.1 (Error ids); ADR 0001 rule 13; 10 10.2 (docs row) | L8 (L8-R9-1) | unresolved (L8-2) | 04 4.1 matches fix hints and `--help` examples with the matcher of ADR 0001 rule 13, which reads only `README.md` and `docs/guide/` against `commands.yaml` journeys, so the named mechanism cannot read a hint. | holds: rule 13 reads as the finding says | 04 4.1 and the docs row name the real check: a test over the generated command definitions that parses each hint and example (command, subcommand, flags) and fails on an unknown one, as 12 12.3 does for SKILL.md; the pointer to rule 13 goes | A |
| **R9-16** | 04 4.1 (Warnings); 04 4.2 (doctor checks); 01 1.5 step 5; 04 4.4 | L8 (L8-R9-3) | unresolved (L8-11) | Doctor checks are rows "by slug and with no id", yet the five **pre** checks (`sbx-global-secret`, `sbx-broad-allow`, `home-symlink`, `auto-trust`, `vscode-trust`) fail a mutating command with exit 2, and every error needs an id. | holds: 04 4.2 marks the five **pre**; 04 4.4 has an id only for `signing-socket` | the five get ids in 04 4.4 (next numbers, exit 2, raised by `doctor` and preflight step 5); a doctor check that stops a command is an error row, and a warning stays id-less | A |
| **R9-18** | the index; 01 1.7; 04 4.1; 07 7.4; 10; 11 11.1 | L8 (L8-R9-5) | regression | First mentions of a record do not follow the link form of ADR 0001 rule 9 in six pages, so the T006 matcher fails on main when it lands. | holds, by sample: 04 4.1 and 07 mention ADR 0001 and ADR 0008 bare at first mention; 11 names ADR 0008 bare | one mechanical sweep of the first mentions; the rule 9 matcher, when it lands, is the proof | A |
| **R9-20** | 10 10.5; 11 11.4; 12 12.2 (release checklist step 3) | L9 (L9-R9-3) | new | `acceptance --pre-tag` runs four checks, which leave out two that read only the repository: a `default-overturned` result with no `resolvedBy`, and a block A result recorded at another sbx or pin. | holds: 10 10.5 lists four; the two are in 11 11.4 | the pre-tag run adds both, and 10 10.5 lists six | A |
| **R9-21** | 00 0.5; 10 10.2 (`sequences` row) | L11 (L11-r9-1) | new | 00 0.5 says `tools/ci sequences` checks that every command has a scope row (SC6, decided on 2026-10-08), but the `sequences` row of 10 10.2 has no such rule. | holds: the row lists ADR, deferred-row, id and plan rules only | the `sequences` row gains the rule both ways (every command of 04 4.2 and 4.3 has a row in 00 0.5, and every row names a command), with its fixtures; T004's acceptance line is parked for B, and the spec half lands before T004 | M: every row names a command is narrowed to rows written as a command, since 00 0.5 also holds feature rows |
| **R9-22** | 12 12.3 (`tools/new`); 04 4.1 (Error ids) | L12 (L12-R9-1) | new | ADR numbers, lesson numbers and RJ ids take "the next free number", so two branches from one base take the same one, both pass on their own, and `sequences` goes red on main after the second merge. | holds: 12 12.3 and 04 4.1 read as stated; nothing reruns a branch against the new base | one rule in 12 12.3: a pull request that adds a numbered record is rebased on the current base and its checks run again before the merge | A |
| **R9-24** | index Deferred decisions (narrowing the development sandbox's GitHub token); ADR 0009 decision 18 | L2 (L2-r9-1) | unresolved (R8-index-5, DR5) | The trigger "the second final handoff ... counted by hand from the handoff record those sessions keep" names no file and no mark of a final handoff, so nobody can count it, and the session checkpoints since 2026-10-07 may already reach two. | holds: the row names no file; round-8.md holds checkpoint sections of 2026-10-08 (and "continued") and 2026-10-09 | the row names the record (the "Session checkpoint" sections of `docs/reviews/round-<n>.md`, then the handoff files once julieta runs in the development sandbox), the mark of a final one, and the one command that counts them; the count is taken now. Needs the operator (D2) | operator (D2; decided on 2026-10-09 and applied) |
| **R9-25** | 06 6.4 (signing rule); J1 step 5; 05 5.4 (Verified signature row); 11 11.2 B2 | L2 (L2-r9-2) | new | The mitigation rests on `ssh-add -c`, whose confirmation needs an askpass program that macOS does not ship, so the first signed commit may fail and the operator may drop `-c`. Inferred, not measured. | holds as inferred: no probe exercises `-c`; B2 checks `ssh-add -L` only | B2 records, on both hosts, a commit in the sandbox with the key added by `-c`; J1 step 5 names the askpass prerequisite; until B2 has run, the 05 5.4 row says "confirm-on-use where an askpass is installed (B2)". Rewording the accepted risk needs the operator (D3) | operator (D3; decided on 2026-10-09 and applied) |
| **R9-26** | 04 4.1 (Error ids, the details) | L14 (L14-r9-1) | new | The details of a failed subprocess (its argv and the first 4 KiB of its stderr) are called safe to paste into an issue because romeu handles no secret, but I25 covers what romeu writes, not what git or sbx print, which can hold a token in a remote URL or a home path. | holds: 04 4.1 reads as stated | stderr in the details passes the recorder's redaction of 10 10.3; the paste claim covers argv, status, step and versions; home paths and host names are named as not covered | A |
| **R9-27** | 03 (opening, Versions); 04 4.4 | L14 (L14-r9-2) | new | A record romeu cannot decode is reported "by id" by `status` and `doctor`, but 04 4.4 has no row for an undecodable own record: `format-newer` is for a newer version, `upstream-shape` for other tools' output. | holds: no row fits; "by id" can be read as the record's id, which still leaves the error with none | one row, `record-unreadable`, exit 2, raised by every reader of a versioned format, with its fix hint; 03 cites it | A |
| **R9-28** | 11 11.3 (`host.upstream`); 11 11.4 | L15 (L15-R9-1) | new | `host.upstream` records "the frontend pin for A11 to A13", while the 11.4 table re-runs A12 for the workload digest and A13 for herdr, so the pin check of 11.4 cannot fire for those bumps. | holds: the field rule and the table differ as stated | `host.upstream` is a map keyed by pin (frontend, workload, herdr, mise, agent), filled for every probe the 11.4 table lists under that pin; the check compares each key | A |
| **R9-29** | 11 11.4; 10 10.3 (Tested window) | L15 (L15-R9-2) | new | `acceptance` fails when a block A result's `sbxVersion` differs from the floor, while widening the window records A4, A5 and A9 again at a newer sbx without raising the floor, with one result file per probe per host. | holds: 11 11.4 and 10 10.3 read as stated; 11 11.3 names one file per probe per host | `acceptance` checks that each block A result's `sbxVersion` is in the tested window, and that A4, A5 and A9 have a result at its newest version; results of other versions sit at `docs/probes/<id>-<host>-<sbxVersion>.json` | A |
| **R9-30** | 04 4.2 (`root-indexed`); 08 8.5; 05 5.4; J14 | L16 (L16-r9-1) | new | `doctor` warns when `$ROMEU_ROOT` is not excluded from Time Machine, and nothing tells the owner that excluding it removes memory, handoffs and unpushed clones from every backup. | holds: the 04 4.2 row and 08 8.5 read as stated | the warning stays (it keeps indexers off agent-written trees); its hint, the README data section and J14 add that an excluded root needs another copy of memory and unpushed work | M: the warning stays; its hint, the README data item, J14 step 3 and 08 8.5 add that an excluded root needs another copy of memory and unpushed work |

### Downgraded to Advisory

| Cluster | Section | Lenses | Kind | Defect | Why downgraded | Resolution | Status |
|---|---|---|---|---|---|---|---|
| **R9-17** | 04 4.1; 04 4.4; 12 12.3 | L8 (L8-R9-4, Required) | new | The "recovered in" column of the error table is defined in 12 12.3 and 12 12.8, not in 04 4.1, which specifies the table. | the task that builds the table implements the 12 12.3 error-table row too, which defines the column, so no implementer is left without it; the T008 and T094 lines are plan text, parked for B | 04 4.1 lists the column and says whether `--json` carries it | A |
| **R9-19** | 12 12.6; index Deferred decisions (docs versioning) | L8 (L8-R9-2, Required) | unresolved (L8-3) | The index row says `tools/release` writes a link to the docs at the release tag into the notes; 12 12.6 does not. | the rule is stated, in the index row that holds it; what is missing is its echo in 12 12.6, and the T088 line is plan text, parked for B | one sentence in 12 12.6: the notes open with the link to `docs/` at the tag | A |
| **R9-23** | 02 2.1 (docs tree); 03 3.13; 05 5.3; 12 12.4 | L12 (L12-R9-2, Required), L13 (L13-R9-12, Advisory) (high-signal) | new | `docs/grants.yaml`, which `tools/ci pr` reads, is in no tree, schema list or formats-without-schema list, and the grantable surfaces are listed in both 05 5.3 and 12 12.4. | the reading of the file is in 12 12.4, which T005 implements whole; the missing plan task and acceptance cases are plan text, parked for B | 02 2.1 and 03 3.13 list the file with its reason; 05 5.3 points at 12 12.4 for the grantable surfaces | A |

### Advisory

Advisory findings are applied by default (lenses.md, step 4). The
Defect cell keeps the reviewer's first sentence; the report holds the
rest.

| Finding | Section | Kind | Defect | Resolution | Status |
|---|---|---|---|---|---|
| L0-r9-3 | 05 5.3 ask-first surfaces | unresolved | The round-8 row (status M) put `internal/sbxdrv` on a new surface and `skills/**` on the kits surface, but the write pass added only the `spec` surface, and 05 5.3 does not say the two are pending. | one sentence in 05 5.3 that `internal/sbxdrv` and `skills/**` join a surface in their own tooling pull request, with that pull request as the trigger; the surfaces themselves wait for it (approval line: `spec`) | M: the two paths are named as pending, with their tooling pull request as the trigger; the surfaces wait for it |
| L0-r9-4 | ADR 0006 | new | The spec defers the runtime ledger, but ADR 0006 is still Accepted and speaks of 'a v1 event', 'Rejected for v1' and 'v1 defers'. | a status note on ADR 0006 pointing at the Deferred row of the ledger, in the next decision-record pass (approval line: `decisions`) | operator (D5; decided on 2026-10-09 and applied) |
| L1-r9-2 | 02 2.3 | unresolved | The one-sentence statement of the supported daily setup (edit and debug inside the sandbox; host clones are for reading in Restricted Mode) is not in 02 2.3 or anywhere else. | apply the change the finding proposes | A |
| L1-r9-3 | 04 4.2 (run step 2); 04 4.4 | new | The loss block that run prints before exiting 1 names the handoff command and J10 step 3 but not the command that creates the new sandbox (romeu run <name>), and the exit 1 has no row in the error table although 4.1 gives every error an id and a fix hint. | apply the change the finding proposes | A |
| L2-r9-3 | 07 7.4 (GitHub rate limit row) | regression | The pointer added for L2b-17 says that, until the token-narrowing row fires, the project's github secret 'is the operator's own token'. | apply the change the finding proposes | A |
| L2-r9-5 | ADR 0009 (Consequences, the pre-push hook bullet) | new | The record says that until PR #34 lands, 'the pull request's diff review is the only guard' against an untracked go.work or vendor/ and a caller's GOFLAGS. | in the record that next amends ADR 0009 (approval line: `decisions`) | operator (D5; decided on 2026-10-09 and applied) |
| L2-r9-6 | 02 2.3 (host tree); 01 1.3 boundary D; 04 4.2 (doctor, root-indexed) | new | The indexer control is a .metadata_never_index file inside each <name>-env/, and doctor checks only that the file exists. | apply the change the finding proposes | A |
| L2-r9-7 | 11 11.1 (A16) | new | A16 reads the token's type 'from its prefix' and its scopes 'from the x-oauth-scopes header'. | apply the change the finding proposes | A |
| L2-r9-8 | 03 3.4 (Rules: secret-in-argv) | new | The best-effort token shapes list ghp_, gho_, ghs_ and github_pat_, but leave out ghu_ and ghr_, GitHub's other two token prefixes, which the same refusal would catch at no cost. | apply the change the finding proposes | A |
| L2-r9-9 | 01 1.2 ('Agent output that reaches the host' row) | new | The row, edited in this pass, still says agent output is 'stored in romeu namespaces or as ledger entries', while SC1 deferred the ledger in the same pass, so v1 has no such store. | apply the change the finding proposes | A |
| L3-R9-2 | 03 (opening), 03 3.6, 03 3.8, 04 4.1 | new | 04 4.1 now defines an error id as RJ-<nnn> with a separate slug, but 03 calls the slugs 'the error id' (format-newer, upstream-shape, manifest-size). | apply the change the finding proposes | A |
| L3-R9-3 | index: Deferred decisions, 12 12.7, 02 2.1 | new | The resolution of L3-18 makes the issue label 'deferred' the one channel for about 15 human-observed triggers, but nothing creates the label: the issue forms are two (a bug, a catalog gap) and no plan task or check mentions the label. | apply the change the finding proposes | A |
| L3-R9-4 | 12 12.3, 04 4.1 | new | The command-definitions row of 12.3 lists 'the P marker and the appending list of 04 4.1' as generated consumers, while the same section says a hand-written page is never a generate target; 04 4.1 still carries both lists by hand. | 04 4.1 says its two lists are design input that the generated command pages replace, the rule 03 states for its tables | M: 04 4.1 says its lists are design input, and the 12 12.3 command-definitions row no longer lists them as generated consumers |
| L3-R9-5 | index: Deferred decisions, 12 12.2 (release checklist) | new | The deferred tables are 'read at each release-candidate tag' (index) and the dark-code listing is read at that tag too, but the release checklist of 12.2 has no step for either. | apply the change the finding proposes | A |
| L3-R9-6 | plan (How this page is kept), 12 12.2 (Re-planning), index: Deferred decisions | new | 12.2 says the plan states no counts, while the plan's 'How this page is kept' still says its counts and traceability rows are kept true by hand; and a deferred row states 'the 19 fixtures' and 'testifylint v1.6.4' in prose, hand-kept facts of the class L3-2 and L3-33 named. | the spec half (the deferred row drops the count and the version for a pointer to the pin file); the plan paragraph is parked for B | A |
| L4-r9-2 | 03 3.13; 08 8.4 | unresolved | The resolution said to add a 03 section and a schema for snapshot/heads.json with a version. | `schema: snapshot/v1` in 08 8.4, since 03 says every format the product owns names its version; 03 3.13 keeps its reason for having no schema file | A |
| L4-r9-3 | 01 1.6 (salvaging row); 04 4.1 Subprocesses; 01 1.5 step 5 vs 04 4.4 | new | Two seams in the write pass: (a) the row 'the sandbox half fails (julieta exits 1, a timeout, a full disk)' names a timeout while 04 4.1 says the salvage exec has no timeout and stops only on an interrupt, so a hung sandbox half has no way to reach the row; (b) an sbx ls --json that does not parse exits 2 sbx-output-unparsed at preflight step 5, but 1.6 and run step 2 say any answer that does not parse exits 1 sbx-unknown, two ids and two exit codes for one condition. | half (a) only: an interrupted salvage exec takes the failure row and the row drops "a timeout"; half (b) is R9-07 | A |
| L4-r9-4 | 08 8.1; 08 8.4 | new | A snapshot write that fails on a full disk is now recorded as a hook failure and keeps the previous bundle, but nothing says that the failed write removes its own .tmp-<ULID> file; only julieta setup removes older leftovers. | apply the change the finding proposes | A |
| L5-r9-3 | 10 10.2 (docs step), 12 12.3 | unresolved | The resolution put the check of SKILL.md command lines, flags included, and of the headings the handoff skill names against handoff write's validator into the 10 10.2 docs step. | apply the change the finding proposes | A |
| L5-r9-4 | 08 8.1 (skill rule), plan.md T059, T078, T079 | unresolved | The skill rule of 08 8.1 lacks the sentence the free-text-note resolution asked of the julieta skill (something no event id expresses goes into the handoff, and the agent opens an issue labelled deferred naming the row), and the plan tasks still carry no acceptance rows for the gap line, hook-failure output, flag and heading checks of T078, the PreToolUse refusals, the compact and resume sources or agent-hooks-missing. | the 08 8.1 sentence; the plan rows are parked for B | A |
| L5-r9-5 | 08 8.2 | new | 'A hook always exits 0 . | apply the change the finding proposes | A |
| L5-r9-6 | 06 6.4 (PreToolUse hook) | new | The new PreToolUse hook refuses gh pr merge, a v* tag push and ruleset or settings gh api calls, but the spec does not say what the agent sees on a refusal or what it should do next. | apply the change the finding proposes | A |
| L5-r9-7 | 08 8.5 step 1, 08 8.1 (v1 backend) | new | Salvage 'waits for the julieta store lock', but the lock is per memory dir (locks/<dir>.lock) and is described as held by memstore/write's Put; it is unstated that handoff write --facts takes it or which dir's lock salvage waits on, so the SessionEnd race is only closed if the hook happens to hold a lock. | apply the change the finding proposes | A |
| L6-r9-4 | 10 10.1 (S8 timing); S8 | unresolved | The julieta setup no-op bound of 3 s is still asserted in CI on shared hosted runners (median of 5), and no text says what a timing red does, in a spec that forbids skipping flakes; only the romeu half moved to B5. | apply the change the finding proposes | A |
| L6-r9-5 | S9; 10 10.2 (coverage row) | new | S9 names five package prefixes while the coverage row says every package go list ./. | apply the change the finding proposes | A |
| L6-r9-6 | S6 (Evidence); 10 10.2 (Release step 1) | new | The mutate evidence is the latest scheduled fuzz.yml run on main with no bound on its age or its commit, and GitHub stops scheduled workflows in a repository with no activity for 60 days, so a stale success can stand for a release commit whose guards changed since. | apply the change the finding proposes | A |
| L6-r9-7 | spec.md Deferred decisions (a mutation stub per guard site); 05 5.2 | new | The row's trigger, an invariant with a second guard site that a mutate run left unobserved, cannot be seen by a mutate run: one stub per id passes as soon as one tagged test goes red. | apply the change the finding proposes | A |
| L7-6r | 10 10.5; 05 5.4 token row | new | The tag-commit check sits in `tools/ci acceptance`, which needs the network, is outside `all`, is run by hand, and whose acceptance.json records v1.0.0 only, so no event runs it for any later release; the 05 5.4 row says a moved tag goes undetected until it runs, with no trigger for running it. | one step of the release checklist of 12 12.2 runs the tag-commit check for every release after v1.0.0 | M: release checklist step 7 runs the check; 10 10.5 and the 05 5.4 token row cite it |
| L7-17r | ADR 0001 rule 12; 12 12.8 | unresolved | 12 12.8 now puts the license statement in the README row, but ADR 0001 rule 12's checked heading list has no License heading, so the statement lands under the trust model heading and not where the lens asks, early on the page. | 12 12.8 names the heading under which the license sentence sits, near its start; ADR 0001 rule 12 is unchanged | M: 12 12.8 names the first README heading and moves the license sentence under it; ADR 0001 rule 12 is unchanged |
| L8-R9-6 | 10 10.2 (docs row); 03 (opening); plan.md T021 | unresolved | The resolution says the tools/ci docs header-row check is dropped. | apply the change the finding proposes | A |
| L8-R9-8 | index S10; 10 10.5; 12 12.8 | unresolved | The completeness checks now keep one guide per journey after v1.0.0, but S10 and 10 10.5 do not say that the expected output of a guide step is checked by nothing: rule 13 compares command words only, and the CI scenarios run J2, J3b, J6, J7, J10 and J11 only. | apply the change the finding proposes | A |
| L9-R9-2 | index: Deferred decisions (preamble) | unresolved | The resolution applied a reader-after-v1 sentence ('after the review rounds end, the table is read at each release, as a step of the release PR whose Evidence names the rows whose event happened'); the preamble says only 'the tables are read at each release-candidate tag', and rc tags end with v1.0.0. | apply the change the finding proposes | A |
| L9-R9-4 | 10 10.2 (the maintainer block) | new | The split into 'ran' and 'to do' reuses the letter d: item d 'Four tries with the sandbox's token' ran, and the to-do 'd. | apply the change the finding proposes | A |
| L10-r9-1 | 11 11.1 A13 | unresolved | Round 8 applied L10-8 'as proposed', which included dropping cmux from the examples. | apply the change the finding proposes | A |
| L10-r9-2 | 12 12.8 README row; 03 3.5 | new | The README prerequisite and 3.5 say the catalog 'covers the reference config repo's stack'. | apply the change the finding proposes | A |
| L10-r9-3 | 03 opening (Versions) | new | The new rule says user-authored formats change only by a new schema version announced in the release notes, and that the bumping release reads the previous version. | apply the change the finding proposes | A |
| L10-r9-4 | 05 5.4 row 'protective command'; spec.md Q13 | new | Product-facing text carries review-record ids and a maintainer-instance default. | apply the change the finding proposes; this removes the clause R8-index-33 added; the 05 5.4 split between product and development rows carries what it said | A  |
| L11-r9-2 | index (the page table, row 06); 06 | unresolved | The page table of the index still describes 06 as 'kits v3 (local builds), workload, julieta delivery, product kits, publishing path', but 06 now has sections 6.1 to 6.5 and no publishing path; the resolution of R8-index-4 and R8-06-5 was to move only the pointer. | apply the change the finding proposes | A |
| L12-R9-3 | 02 2.1 (tools/ci entry); 10 10.2 | unresolved | The tree now gives tools/ci one package per step, but says the steps are 'registered in one dispatcher table'. | apply the change the finding proposes | A |
| L12-R9-4 | 12 12.2 (module table and the directory-to-module test) | new | The new sentence says a package module owns internal/<id> and that a tools/ci test holds the tree, this table and the package list to the same set of directories. | apply the change the finding proposes | A |
| L13-R9-4 | index: Deferred decisions (the runtime ledger row; the record of each gate run row); ADR 0001 (Where lessons live); 12 12.7 | new | The write pass moved lessons to one file each under docs/lessons/ (12 12.7, 12 12.3, 10 10.2 lessons row, 01 1.7, the index Boundaries), but two Reopened-by cells it wrote still name docs/lessons.md, and ADR 0001's Where lessons live row says docs/lessons.md with no amendment in ADR 0009. | two cells, and the amendment of ADR 0001's lessons row in the next decision-record pass (approval line: `decisions`) | M: the two index cells only; the ADR 0001 row is D5 |
| L13-R9-5 | 04 4.1 (Exit codes); 04 4.3 (julieta spec validate); index S4 | new | Exit 6 now means the check ran and found what it checks for, and spec validate --catalog exits 6 on an unknown backend:tool, while the same command exits 2 (usage or precondition, invalid spec) when the check it exists for finds a schema or rule error, so one command reports its findings with two codes and 2 again carries two meanings. | apply the change the finding proposes | A |
| L13-R9-6 | 01 1.7; 04 4.2 (run step 3); 04 4.1 (Warnings); 04 4.4 | unresolved | R8-01-33 asked for the row recreate needed with Not: stale for the recreate digest; the row exists with an empty Not column, run step 3 still prints recreate-pending with the text 'stale: <fields> changed', and the one condition (the recreate digest differs from the generation's) carries two slugs, the warning recreate-pending on a running sandbox and the error recreate-needed (RJ-305) on a stopped one, beside status's 'recreate needed'. | apply the change the finding proposes | A |
| L13-R9-7 | 04 4.2 (How romeu run reaches the run layout, step 2); 01 1.6 (Generation); J10 step 2 | unresolved | 01 1.6 now keeps a lost record that salvage --from-host wrote for the generation and writes one only otherwise, but 04 run step 2 still imports into a new salvage id and records result: lost unconditionally, so the two texts still allow two lost records for one loss. | apply the change the finding proposes | A |
| L13-R9-8 | 11 11.1 (A12, A15); 01 1.2 (Where data lives); 06 6.2 | new | What of the workload's volumes survives sbx env rm is measured by two probes: A12 records the volumes and what survives sbx env rm and a recreate, A15 runs sbx env rm and lists the volumes; 01 1.2 cites A15 and 06 6.2 cites A12, and A15's Feeds cell does not name 06 6.2 or 01 1.2. | apply the change the finding proposes | A |
| L13-R9-9 | ADR 0009 decision 5; 10 10.1 (Rules); index Q27 | new | ADR 0009 decision 5 says tools/ci all is offline except the vulnerabilities step, while 10 10.1, the list the index row on re-reading the rulesets points to, names the license step's image pull inside all as well; and Q27's default says the workload base image and the herdr download join 10 10.1's rule, while 10 10.1 ends 'Nothing else in a test or a step reads it'. | 10 10.1 becomes the one list; the ADR 0009 clause in the next decision-record pass (approval line: `decisions`) | M: the spec half only (10 10.1 is the one list); the ADR 0009 clause is D5 |
| L13-R9-11 | 07 7.5; 02 2.2; 03 3.5 | new | 02 2.2 and 07 7.5 have the config project list api.github.com in egress.extra for julieta pin check --workflows, while derivation step 4 adds the meta and base sets, which hold api.github.com for every project, and step 7 already gates it as upload: true, so one host is reached two ways. | apply the change the finding proposes | A |
| L15-R9-6 | 08 8.5 step 5 | new | `agent-path-missing` makes a salvage incomplete for any profile path that does not exist, but a sandbox whose agent never created a todos or scratch directory legitimately lacks it; the text cannot tell drift from never-created. | apply the change the finding proposes | A |
| L15-R9-7 | 02 2.6 | unresolved | The resolution said to date the 'documented sbx behavior' with an sbx version and a doc date and to name A3 as the probe that re-confirms it. | apply the change the finding proposes | M: no sbx version or doc date is recorded for the documented behaviour, so 02 2.6 says it is as documented when written and re-confirmed by A3 at the version A1 records |
| L16-R9-2 | 09 J14 step 4 | unresolved | J14 lists what the operator removes by hand but omits the keychain entries named in host settings and the signing agent socket key, which the store table of 01 1.2 marks 'kept / kept'; it also lists .attic only under 'stays on purpose' with no pointer to J7 step 4 as the way to delete it with its secret-bearing salvage payloads. | apply the change the finding proposes | A |
| L16-R9-3 | 09 J7 step 4 | new | The condition 'once status shows nothing unpushed for the project' cannot be observed: retire moves the project record to the state attic and the same step says status and doctor do not read the attics. | apply the change the finding proposes | A |
| L16-R9-4 | 10 10.1 (Host row); 08 8.5 | new | 10 10.1 says the host suite runs 'J1-J13' while 09, S7 and B3 say J1-J14, so the leaving scenario has no stated home in the test levels; and 08 8.5 says payloads 'stay until the owner deletes them' without saying whether doctor, status or salvage verification tolerate a deleted salvage/<gen>/ dir of a live project. | apply the change the finding proposes | A |

## Parked for stage B

Findings on `plan.md` alone, kept for the plan review with the halves of
R9-03, R9-14, R9-17, R9-19, R9-21, R9-23, L3-R9-6 and L5-r9-4 that name
plan tasks:

- **L6-r9-8** (plan.md (T007, pr task, T002, T088)): Several round-8 test mechanisms of this lens have no acceptance line in any plan task: the pr rule that removes no last guard or test tag, the zero-test failure of a go test step, the TestTestStyle ban on t.Skip and testing.Short, and the positive controls of I2, I3, I14 and I16 (parked items plan-26 to plan-28 cover only the stub, the absent package and the host vet).
- **L9-R9-5** (plan.md, Operator blocks, 'The rehearsal in O5'): The text still calls the O5 rehearsal 'the first run of the product against real sbx', but the smoke steps now put it on real sbx at C4 and C7.
- **L15-R9-5** (plan.md T020): T020 (implements 11 11.4) lists no acceptance row for the new checks: a result whose recorded upstream differs from its current pin, a guide page naming a probe id its `affects` does not list, and an A5 result outside the tested window.

## Decisions for the operator

Everything the write pass did not apply, in one block: a Required
finding that changes a decision record or an accepted risk, a trigger
that has fired, and the halves of Advisory findings that belong to a
decision record. Each item names the finding, the options and the
consolidation's recommendation; the recommendation is not a decision.
The decision, when taken, is recorded beside the item with its date,
and in the text it changes.

- **D1 (R9-01), the ruleset sentence of ADR 0009.** Closed by the
  rebase onto cecdd14, with no decision needed: the default-branch
  ruleset, as read on 2026-10-09, has a required-status-checks rule
  that lists the four `all on ...` checks, so "requires CI green" in
  05 5.4, 12 12.4 and ADR 0009 holds. What that reading fired is D4.
  Answered on 2026-10-09: closed, as stated.
- **D2 (R9-24), the token-narrowing trigger.** The index row
  "narrowing the development sandbox's GitHub token" counts "the second
  final handoff ... counted by hand from the handoff record those
  sessions keep", and names no file and no mark of a final handoff, so
  nobody can count it. Measured on d0bfdcc: `grep -n '^## Session
  checkpoint' docs/reviews/round-*.md` finds two sections since
  2026-10-07, in round-8.md (2026-10-08 and 2026-10-09); whether a
  checkpoint is a final handoff is what the row does not say. Options:
  (a) the row names the record (the "Session checkpoint" sections of
  `docs/reviews/round-<n>.md`, then the handoff files once julieta runs
  in the development sandbox), the mark of a final handoff and the
  command that counts them, and the count is taken now; if it is two or
  more, the review the row asks for is held and its date written in the
  row; (b) drop the handoff count from the trigger and keep its other
  events, with checkpoint C7 as the backstop; (c) leave the row as it
  is, which keeps it failing the shape of criterion 3. Recommendation:
  (a), and the same wording in the record that next amends decision
  18 of ADR 0009 (the triggers of ADR 0008's deferred item 1).
  Approval lines: `spec` (the index), `decisions` (the record).
  Decided on 2026-10-09: option (a). The index row names the record (a `## Session checkpoint (<date>)` heading in `docs/reviews/round-<n>.md`, then a `--final` handoff file), the command that counts them, and the count taken on 2026-10-09: three, so the trigger has fired and the review is due; decision 6 of ADR 0010 carries the record half.
- **D3 (R9-25), the `ssh-add -c` mitigation.** The mitigation of the
  Verified-signature row of 05 5.4, and of 06 6.4 and J1 step 5, rests
  on `ssh-add -c`, whose confirmation needs an askpass program that
  macOS does not ship; inferred, not measured. Options: (a) B2 records,
  on both hosts, a commit in the sandbox with the key added by `-c`;
  J1 step 5 names the askpass prerequisite; until B2 has run, the
  05 5.4 row reads "confirm-on-use where an askpass is installed (B2)";
  (b) keep the row as written and add only the B2 measurement;
  (c) replace `-c` by another confirmation. Recommendation: (a): it
  states only what is known, and B2 settles it on both hosts. Approval
  line: `spec`.
  Decided on 2026-10-09: option (a), applied in the 05 5.4 row, 06 6.4, J1 step 5 and probe B2.
- **D4, the triggers that the 2026-10-09 ruleset reading fired.** The
  index Deferred row "required status checks without the administrator
  bypass" is reopened by "the jobs of the green run are added as
  required checks", and deferred item 4 of ADR 0008 ("Required checks
  without the bypass") by "item e of the maintainer block". As read on
  2026-10-09 the rule lists the four `all on ...` checks and the
  administrator role is still the bypass actor, so both triggers appear
  to have fired. Options: (a) reopen the decision in a dedicated item:
  whether the required checks bind the administrator role too, decided
  with what it costs the operator's own merges, and recorded in a
  decision record that amends ADR 0008's deferred item 4 and replaces
  the index row; (b) rewrite both triggers to a later event (for example
  the token narrowing), recorded the same way; (c) leave both, which
  leaves a fired trigger with nothing reopened. Recommendation: (a).
  Approval lines: `spec` (the index row), `decisions` (the record).
  Decided on 2026-10-09: option (a). ADR 0010, decision 1, records the decision as reopened with its options open, and the index row says so; the choice is the next decision block's item.
- **D5, the decision-record halves of Advisory findings.** An accepted
  record is not rewritten (12 12.5), so these wait for one amending
  record: a status note on ADR 0006 pointing at the Deferred row of the
  runtime ledger (L0-r9-4); the pre-push hook sentence of ADR 0009's
  Consequences, replaced by the guard that holds (L2-r9-5); ADR 0001's
  "Where lessons live" row, which still names `docs/lessons.md`
  (L13-R9-4); ADR 0009 decision 5, which should say that `all` reads
  the network only where 10 10.1 lists (L13-R9-9). The spec halves of
  L13-R9-4 and L13-R9-9 are applied. Options: (a) one record in the
  next decision-record pass carrying the four, with D2 and D4 if they
  are decided by then; (b) one record each. Recommendation: (a).
  Approval line: `decisions`.
  Decided on 2026-10-09: option (a), as decisions 2 to 5 of ADR 0010, which ADRs 0001, 0006, 0008 and 0009 now name in a `Superseded in part by` line.
- **D6 (ship9-te-1), the packages S9 measures.** S9 lists `cmd/...`,
  but `coverTrees` in `tools/ci/coverage.go` holds `internal`, `tools`,
  `e2e/probes` and `e2e/fakesbx`, so a package under `cmd/` is not
  measured. Options: (a) add `cmd` to `coverTrees` with a test, a
  tooling pull request on the `checks` surface; (b) drop `cmd/...` from
  S9, which changes a success criterion. Recommendation: (a). Approval
  line: `checks`.
  Decided on 2026-10-09: option (a), done in PR #37, "fix(ci): hold cmd packages to the coverage floor (S9)".

## Exit criteria of stage A

The five criteria of lenses.md, "How stage A runs, and when the spec
counts as validated", judged on 8ecfbb2:

1. **No Blocking open; every Required fixed or declined with its
   decision and date: not met.** 0 Blocking. 27 Required clusters are
   open (8 unresolved from round 8, 2 regressions, 17 new), none fixed
   and none declined.
2. **Coverage: met for the pages, unproven for the criteria and
   journeys, carried from round 8.** A re-audit reads only the changed
   sections and its reports carry no coverage table, so the round-8
   measurement stands: nineteen reports, no coverage key missing, every
   page with a finding or an explicit "nothing for this lens" from at
   least two lenses (the thinnest page, 13, had a finding from fourteen
   reports); measured over finding text only, J7 had one lens (L12), S2
   and J5 two each ([round-8.md](round-8.md#coverage)).
3. **The open questions and deferred decisions of the index have a
   valid shape: not met.** 99 rows: 24 open questions and 75 deferred
   decisions (36 in the product, 11 with the runtime ledger, 28 in how
   this repository is run). Every row has all its cells filled, measured
   by a script over the tables. Four rows have a trigger that cannot be
   observed as written: the token narrowing (R9-24), a mutation stub per
   guard site (L6-r9-7), the runtime ledger and the record of each gate
   run (L13-R9-4, both name `docs/lessons.md`, which the pass replaced by
   `docs/lessons/`). The preamble's reading after v1.0.0 (L9-R9-2) and
   the `deferred` label that nothing creates (L3-R9-3) are Advisory.
   Duplicates were not measured beyond the reviewers' reading.
4. **The three measurements: met, carried from round 8.** The `listed`
   to `own` ratio per lens (96 : 540 across the round), the clusters
   only L0 raised, and the overlap of the L2 and L13 noise-probe pairs
   are recorded in [round-8.md](round-8.md#measurements). This round
   ran no noise probe and records no origin per finding.
5. **The targeted re-audit raises no new Required finding: not met.**
   It raised 19 Required clusters that are new or regressions (17 new,
   2 regressions), from 26 Required findings of kind `new` or
   `regression` in the reports.

Validated for T004 onward: no. 0 Blocking; 27 Required open (8
unresolved, 2 regressions, 17 new), 0 declined; coverage carried from
round 8 (pages met, criteria and journeys unproven); 4 of 99 index rows
fail the shape; measurements carried; the re-audit raised 19 new or
regressed Required.

What is left before T004, after the write pass below:

- the operator's answers to D2 to D6; R9-24 and R9-25 are the two
  Required clusters still open, and R9-24's row is the one index row of
  criterion 3 the pass did not fix (the other three, L6-r9-7 and the two
  cells of L13-R9-4, are fixed);
- a targeted re-audit of the pass, round 10, by the lenses whose
  findings drove it, reading only the sections it changed; clean
  verdicts are carried forward. R9-21 is the one cluster whose section
  T004 itself implements.

## Write pass (2026-10-09)

The pass ran on branch `docs/review-round-9`, stacked on pull request
#25 at d0bfdcc, in one scratch clone. Seven build nodes ran one after
another, never at the same time, each applying the clusters of one page
group on every page their resolutions name, and each running
`go run ./tools/ci fast` before its commit. Pages 01, 04, 05 and 10,
which hold the gates, the command contract, the security model and the
release path, ran on `opus`; the other groups on `sonnet`.

| Group | Pages it led | Model | Commit subject |
|---|---|---|---|
| 01 | 01, with 02, 03, 04, 05, 09, 10, 11 | opus | docs(spec): apply the round-9 resolutions of the system model |
| 04 | 04, with 03, 07, 08, 10, 12 | opus | docs(spec): apply the round-9 resolutions of the command line |
| 05 | 05 and the index, with 02, 03, 04, 08, 09, 10, 12 | opus | docs(spec): apply the round-9 resolutions of the security model |
| 10 | 10 and 11, with the index, 02, 05, 08, 09, 12 and plan block O2 | opus | docs(spec): apply the round-9 resolutions of testing and release |
| 03 | 02, 03, 07, with 08, 12 | sonnet | docs(spec): apply the round-9 resolutions of formats and egress |
| 08 | 06, 08, 09, with 11 | sonnet | docs(spec): apply the round-9 resolutions of memory, handoff and journeys |
| 12 | the index, 11, 12, and the first-mention sweep of R9-18 | sonnet | docs(spec): apply the round-9 resolutions of the index and engineering |

Group 10 also applied three findings of the review of the pull request
#25 rebase delta: the required-status-checks clause moved from item a
of the maintainer block (10 10.2) to item e, which set it up, and item
e now says the rule exists, as read on 2026-10-09, and that only saving
the API's answer under Evidence is left; item a was reflowed; and plan
block O2 says the same as item e. What that reading fired is D4.

Resolved: 27 Required clusters, 19 A, 6 M and 2 operator (R9-24, R9-25),
R9-01 of the M closed by the rebase; 3 downgraded clusters, all A; 52
Advisory rows, 43 A, 7 M and 2 operator (L0-r9-4 and L2-r9-5, both in
D5). Nothing under `tools/**`, `.github/**`, `.githooks/**` or
`docs/adr/**` changed; where a resolution describes a check that does
not exist yet, the spec states it as pending.

Seen by the groups and left for round 10: 11 11.2 gives B6 no time
estimate beside "about 90 minutes per host for B1 to B5"; the five
doctor checks given ids by R9-16 are exit 2 when they stop a command,
while `doctor` itself reports them through exit 6, which their rows do
not spell out; the R9-02 job permissions (`actions: read`,
`pull-requests: read`, `issues: read`) are inferred, not read from
GitHub's documentation.

The ship gate's round 1 on 42cc664 was NO-GO. Its must-fix findings
ship9-cr-1 with ship9-sa-5, ship9-sa-1, ship9-sa-2 and ship9-sa-3, and
its capped findings ship9-sa-4, ship9-sa-6, ship9-cr-2 to ship9-cr-5
and ship9-te-2, were applied in one write pass, the commit "docs(spec):
apply the round-9 ship-gate fixes"; ship9-te-1 is D6. Two fix texts
were adjusted where the spec showed them wrong: the `sbxdrv` surface
dependency sits on the first plan task that adds a file under
`e2e/fakesbx` (T015), not T016; and the inferred release permissions
are confirmed by the first `release.yml` run only, since a `--dry-run`
build reads no network. The test cases of ship9-te-3 (the
`root-indexed` row of 04 4.2) and ship9-te-4 (the `removing` and
`salvaging` transitions, in I17 of 05 5.2) were added in the same
commit. ship9-te-5 is in the pending list below.

The ship gate's round 2 on e93387c was NO-GO. Its must-fix findings
ship9-cr-6 and ship9-cr-7, and its capped findings ship9-cr-8 to
ship9-cr-10, ship9-sa-7, ship9-sa-8 and ship9-te-6, were applied in
one write pass, the commit "docs(spec): apply the round-9 ship-gate
round-2 fixes". `removal-pending` (RJ-336) is raised by preflight step 6
for any mutating command, `salvage --from-host` included since it is
marked **P** in 04 4.2, except `rm`, `recreate` and `retire`, and
exits 2.

The ship gate's round 3 on 43f2ccd was GO, with ship9-cr-11 left open
after the round budget of three rounds.

Pending, one line each:

- ship9-cr-11 (code-reviewer, Optional, open after round 3): in the
  recovery text of 01 1.6, in 01 1.7 and in the RJ-336 row of 04 4.4,
  "any mutating command but `rm`, `recreate` and `retire`" becomes "any
  **P** command (04 4.2) but `rm`, `recreate` and `retire`", because
  `adopt` and `init` write state but never run step 6.
- ship9-te-5 (test-engineer): a `tools/lenses` check that each report
  under `docs/reviews/round-*/reports/` is well formed: valid JSON with
  the fields of "What a reviewer returns" in lenses.md.
- `TestLintConfigReportsEachChecker` fails with "parallel golangci-lint
  is running" when another `tools/ci` runs at the same time (captured
  2026-10-09). The fix isolates the lock, for example a per-test
  `GOLANGCI_LINT_CACHE` or `TMPDIR`, never a skip or a retry; it is a
  `checks` tooling pull request.

## Session checkpoint (2026-10-09)

Branch and pull request state is not recorded here; the next session
measures it with `handoff_state.py`.

### Done

- Pull request #34, "fix(hooks): start tools/ci from pre-push with
  GOWORK=off and GOFLAGS=-mod=readonly", is a draft. Its ship gate was
  GO in round 2, and CI is green on four platforms.
- Pull request #25 was rebased onto cecdd14, at d0bfdcc, and pushed.
  Its ship gate ran rounds 1 to 3 and was GO in round 3. The rebase
  delta was reviewed and approved. CI is green.
- Round 9 ran: seventeen lenses, then the consolidation.
- On branch `docs/review-round-9`: the write pass of this page, then
  ship gate rounds 1 to 3, GO in round 3.
- That branch was pushed as pull request #36 at dc76fec.
- Decision D6 was done in pull request #37, "fix(ci): hold cmd packages
  to the coverage floor (S9)".
- The answers of 2026-10-09 were applied on this branch: D2 and D3 in
  the spec, D4 and D5 in ADR 0010, AR22 closed.

### Answered on 2026-10-09

Every recommendation of the decision block above was accepted on
2026-10-09, and each item carries its line. AR22, the fifth ruleset try
of round-8.md: closed as no longer needed, because pull request #33
read the ruleset through the API on 2026-10-09; item g of the
maintainer block (10 10.2) is removed and the 05 5.4 token row says the
claim stays inferred. G5 of pull request #25 is applied there, as the
`approvals` surface. This branch was rebased onto pull request #25 at
3f29a9c.

### Open for the operator

- **N2 of pull request #34**, a system-wide mise config that reaches
  the hook's `tools/ci`; it waits for the operator.
- **The decision ADR 0010 reopened**, required checks without the
  administrator bypass, as an item of the next decision block.
- **The token-narrowing review**, whose count reached three on
  2026-10-09 (D2).
- **Round 10, the targeted re-audit** of this pass, a fan-out of about
  seventeen reviewers, once this branch's pull request carries the
  answers above.
- **The R8-05-3 tooling pull request** (the `sbxdrv` surface and
  `skills/**` on `kits`, 05 5.3), after #25 lands, before plan T015.
- **The merges of #34, #25 and this branch's pull request**, each once
  its approval lines are ticked and its CI is green.

### Next steps

- Push this branch and open its draft pull request, stacked on #25.
- Round 10.
- The declared validation of the spec for T004 onward, if round 10
  raises no new Required finding.
- The stage-B items of this page and of round-8.md.
