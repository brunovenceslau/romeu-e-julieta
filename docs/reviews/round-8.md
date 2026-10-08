# Spec review round 8: seventeen lenses on the specification

Reader: reviewers checking that every finding of round 8 was handled,
and maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 7 is
in [round-7.md](round-7.md); the lenses are defined in
[lenses.md](lenses.md).

Round 8 is stage A of the protocol in lenses.md, "How stage A runs, and
when the spec counts as validated": the first round read through the
lens set L0 to L16, with the noise probe on L2 and L13. The baseline is
commit 2cf36fa of the default branch, read from a detached export that
nothing wrote to while the reviewers ran. lenses.md is cited at its own
commit, as its step 1 asks: both parts of every brief, the Role
paragraph of the first and the lens examples of the second, came from
lenses.md at commit 0845a3c; its current commit bd67143 differs from
0845a3c only in two citations of L15 and one sentence of brief step 4.
This page is the consolidation's
draft: it records the findings as clusters, proposes a resolution per
cluster, and sends to the operator, in one block, everything the
consolidation cannot decide.

Every item below is a finding of a reviewer or a proposal of the
consolidation. None is a maintainer decision, and the maintainer has
not ruled on any of them. Where a row says "recommend", the
recommendation is the consolidation's, and the operator's answer is
recorded next to it when it comes.

The principle the proposals were judged by, as in rounds 6 and 7:
prefer deleting, merging or stating a limit over adding machinery. A
fix is a few sentences; a mechanism waits for its trigger.

Status: **A** = apply as proposed; **M** = apply in a smaller or
different form than proposed, as stated; **L** = state as a limit, no
mechanism added; **operator** = needs the operator, because the change
touches a decision record (`docs/adr`), an accepted risk (05 5.4, or
the ask-first surfaces of 05 5.3) or the scope (00 0.3, 00 0.5, a
success criterion, the v1 command set). An operator row carries the
consolidation's recommendation and the id of its item in "Decisions
for the operator" below.

## How the round was run

Nineteen reviewers, one per lens plus a second independent reader of L2
and L13 (L2b and L13b, the noise probe), each in a fresh context,
read-only, in parallel on 2cf36fa, with the two-part brief of lenses.md.
Model per report, as the reports record it: `fable` for L0, L2, L2b, L3,
L6, L11, L12, L13 and L13b; `opus` for L1, L4, L5, L7, L8, L9, L10, L14,
L15 and L16. Each wrote one JSON report in the shape of "What a reviewer
returns": 636 findings in all (54 Blocking, 289 Required, 293 Advisory;
96 listed, 540 own; 56 parked for stage B).

Consolidation ran in four steps, each by a separate agent reading only
the reports and the spec export:

1. Clustering, one agent per page (the index, 00 to 13, adr, plan):
   findings that name the same defect merged into one cluster with every
   lens and finding id kept, the cluster's severity the highest among
   its members, and a proposed resolution. The 636 findings became 503
   clusters, named `R8-<page>-<n>`. A finding that cites several pages
   sits on each page's cluster (57 findings sit on more than one; 748
   finding memberships in all; 30 of the 57 are L3's and 26 L11's, whose
   findings cross pages by nature), so one decision can appear under
   several ids; the Decisions section merges those.
2. Measurement: the per-lens counts, the coverage matrix and the
   missing-viewpoint answers grouped by theme.
3. The regression check: every cluster against the resolutions of
   rounds 1 to 7, asking whether an earlier round resolved the same
   defect and whether the proposed change reopens something an earlier
   round closed.
4. This draft, then a completeness critic (every finding id in a
   cluster, every cluster in this table).

The regression check changed the table in two ways, both marked in the
rows. Six clusters the page consolidators had marked A or M are routed
to the operator, because the proposed change reverses a maintainer
decision of an earlier round (R8-00-7, R8-01-21, R8-06-1, R8-10-4,
R8-10-44, R8-11-7). Another 68 rows (the 63 marked "Regression check",
the two conflict rows and the three amended ones) carry an amended form
or a note, so that the fix does not undo an earlier fix or so that the
record names what it supersedes; three of them change status (R8-04-24,
R8-10-40, R8-10-52). The Regressions measurement lists them.

The completeness critic read the draft before this one, and its record,
`round-8/critic.json`, describes that draft: verdict "gaps", 203 M, 91
operator, SC10 as Advisory, and five defects, each settled here. First,
R8-06-24 carried the section 12 12.5 only while its finding L3-20 also
cites 06 6.4: the row's section now reads 06 6.4; 12 12.5. Second, SC10
was Advisory while R8-08-22, its page-08 form, stood Required and M:
SC10 is Required and R8-08-22 is routed to the operator, the one status
change written back to its cluster file. Third, four findings that sit
on several clusters had one cluster at the operator and another resolved
A, M or L with no cross-reference between them (L3-24, L11-7, L3-30,
L3-26): the resolved rows R8-00-9, R8-01-11, R8-03-36, R8-index-40 and
R8-adr-3 now point at SC12, SC8, SC15 and DR1, and those items name
them. Fourth, the Regressions paragraph gave 74 and 68 without their
derivation: it now states 63 + 2 + 3 and the 65 rows that carry the
mark. Fifth, the count of findings only L0 raised, the measurement as
lenses.md names it, was not reported beside the cluster count: it is
now, and is 17 as well.

Counts of the table: 470 stage A clusters (60 Blocking, 252 Required,
158 Advisory), resolved 136 A, 199 M, 39 L and 96 operator (85 before
the regression check, 91 before the completeness check, and 92 before
the gate review of this page routed R8-index-22, R8-00-3, R8-09-2 and
R8-10-10 to the operator as SC16 and SC17); 33 clusters parked for stage
B (10 Required, 23 Advisory) in their own table. Of the stage A
clusters, 119 were raised by two or more lenses independently and are
marked high-signal (28 Blocking, 74 Required, 17 Advisory); 5 parked
clusters are high-signal too.

Deviations from lenses.md. Three, recorded so that the lens page can
take them in or refuse them. (a) Fourteen Advisory rows went to the
operator (R8-01-20, R8-03-35, R8-04-34, R8-04-35, R8-04-37, R8-05-6,
R8-05-15, R8-10-33, R8-10-39, R8-10-44, R8-adr-12, R8-adr-20, R8-adr-21,
R8-adr-22, in the items SC8 to SC11, SC15, AR3, AR5, AR11, AR16, AR20,
AR22, DR1 and DR8), where step 4 of lenses.md sends Blocking and
Required findings to the operator and applies Advisory ones by default:
each changes the scope, a decision record or an accepted risk, which the
legend above reserves to the operator whatever the severity. The rule
applied: they are decided with the rest of the block, and one left
unanswered is applied as its recommendation says when the write pass
lands. (b) Clustering ran as seventeen per-page agents on `opus`, not as
the one consolidator on the strongest tier that step 3 names, because
nineteen reports do not fit one context; the cost is the 57 findings
that sit on more than one page's cluster, merged by hand in this table
and in the Decisions section. (c) Three amendments of lenses.md are
pending, since that page is another pull request: step 3 should read
"one agent per page, then one merge"; the round 8 row of "Rounds that
used these lenses" should name this page and its baseline 2cf36fa; and
the `tools/lenses` command should carry a link, anchor and
section-citation checker and a schema check of the reports under
`round-8/reports/`, since nothing under `tools/ci` checks either today.

## Findings and resolutions

One row per cluster, in page order (the index, 00 to 13, adr, plan). The
Cluster cell carries the id, the severity, the sections as the spec
cites itself, and the defect in one sentence; the Lenses cell names
every lens whose finding is in the cluster. The finding ids behind each
cluster, the scenario, the proposed text change and its cost are in the
consolidation's `clusters-<page>.json` files under [round-8/](round-8/),
beside the nineteen reports (`round-8/reports/<lens>.json`), the
per-lens statistics, the regression check and the completeness check;
the page keeps the defect and the resolution. The Status column of this
page is the record: the `resolution` field of a cluster file is the
consolidation's first proposal. The regression check changed nine of
them here without writing back to the files (the six rows it routed to
the operator and the three it amended, each marked in its row), while
the completeness check's one change (R8-08-22, M to operator, SC10) and
the gate review's four (R8-index-22, R8-00-3, R8-09-2, R8-10-10) were
written back, so the files count 90 operator clusters and the table 96
rows.

| Cluster | Lenses | Status | Resolution |
|---|---|---|---|
| **R8-index-1** (Required) index: Open questions: The Open questions table holds rows that are not open: Q25 is decided and recorded in [ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md) yet stays, and the maintainer-settled rows (Q4, Q6, Q9, Q12, Q13, Q14, Q23) have no event that makes them due, against the table's own rule that a question leaves when it is settled. | L0, L1, L2b, L4, L5, L8, L9, L10, L11, L12, L13, L13b, L14 (high-signal) | M | Apply without a new 'decided, record pending' state: deleting Q25 and giving the maintainer rows one due point (the first release-candidate tag) merges the readers' two fixes into one rule. The plan task ids (T101, T005) leave the tables in the plan-task-id cluster; the duplicate interval row of L13-20 is in the interval cluster. |
| **R8-index-2** (Advisory) index: Open questions; index: Deferred decisions: Q9's 'Homebrew tap later' and Q23's 'a credential only if a private registry is needed' are deferrals written as open-question defaults, with no Deferred row and no event, while the index says a deferral without a row is an oversight. | L7, L13b (high-signal) | M | Apply for Q9 and Q23. The other half of L13b-22 (moving the token review interval and the ledger tuning rows to Open questions) is declined: the interval folds into the token row and the tuning row merges with rotation (see those clusters), which removes both rows instead of moving them. |
| **R8-index-3** (Required) 00 0.5; 07 7.5; index: Deferred decisions: The 'v1.1' bucket of 00 0.5 holds four items (the native egress queue in status, handoff --list filters, a Remote-SSH helper, a registry credential) with no trigger and no 'until then', beside a principle that says a deferral without a row is an oversight; Q23 lives in two tables. | L9, L11 (high-signal) | operator | Recommend the rows above. The text change is small, but it removes the 'v1.1' label from 00 0.5, the scope justification, so the operator decides whether those four items stay promised for v1.1 or become plain deferrals. Regression check: records the supersession of op-no-live-view's 'stays listed in 00 0.5' (round 6); the helper stays in the spec as a Deferred row. Decision SC7. |
| **R8-index-4** (Required) 06 6.6; 03 3.4; index: Deferred decisions: The kit publishing path is a deferral outside the Deferred table: six rows of design, including a host-settings key kits.source that 03 3.4 does not define, reopened by 'a maintainer decision', which [ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md) rule 3 does not accept as an event. | L3, L11 (high-signal) | M | Apply by deletion rather than an ADR in Proposed status: the design rows describe a registry and tags no code reads, and that decision is better taken when the trigger fires. The non-goal of 00 0.3 stays as it is; only its pointer moves. |
| **R8-index-5** (Blocking) index: Deferred decisions (narrowing the sandbox's GitHub token): The token-narrowing row, the acceptance of the widest decision in the project, has no producer for its triggers: 'every 2 autonomous work sessions' is counted by nothing (13 13.2 records no session start or end), and the security-log trigger depends on a log whose coverage the row itself calls unverified. | L2b, L5 (high-signal) | operator | Recommend the handoff-count form above: it keeps the operator's cadence and replaces memory with a command's output. The alternative, a calendar date, needs an exception to ADR 0005 rule 3 recorded in ADR 0005. Decision DR5. |
| **R8-index-6** (Advisory) index: Deferred decisions (the review interval of the token narrowing): The row 'the review interval of the token narrowing' repeats the interval of the row above it and is reopened by 'the operator saying so', which no reader or check can see fire. | L0, L5, L12 (high-signal) | M | Apply: what ADR 0008 decided (2 sessions, then 10 once review is cheap) stays; only the duplicate row and its unobservable trigger go. L0-12's other half (triggers no one observes) is answered by the issue-label cluster. |
| **R8-index-7** (Required) index: Deferred decisions: Many rows are reopened by events only a person notices ('The maintainer observes these', 'the first question about', 'written as an issue'), with no channel by which an adopter's event reaches anyone; no row names the check that detects its trigger, although ADR 0005 Consequences asks for it; and after round 8 no review round reads the table. | L3, L6, L9, L10, L11 (high-signal) | M | Apply one issue label and one reading event instead of a 'Detected by' column: one channel for every human-observed trigger merges the per-row observers into one concept, and a release-candidate tag is an event, not a date. |
| **R8-index-8** (Required) index: Deferred decisions (finer visibility per project in the cross-project view): The trigger 'the first private project on a machine that also runs a public one' cannot be observed: the spec has no notion of a private or public project (03 3.2), it misses projects of different clients that are all private, and the row names no observer. | L10, L16 (high-signal) | M | Apply the trigger rewrite only; a sync notice or a host-settings switch is the decision the trigger reopens. The accepted risk in 05 5.4 is unchanged. |
| **R8-index-9** (Required) index: Deferred decisions (a free-text note): The free-text note is reopened when an agent's need is 'written as an issue', but the agent, the only observer, is told nowhere that an issue is the channel, so the trigger cannot fire. | L5 | M | Apply without a CLI hint: the skill already carries the agent's instructions, and the label is the channel of the issue-label cluster. |
| **R8-index-10** (Required) index: Deferred decisions (docs versioning per release): The docs-versioning row's 'until then' is itself a defect once v1.0.0 ships: a user on release N reads main's generated error ids, exit codes and command pages, and the trigger fires the first week with nothing reporting it. | L8 | M | Apply the link at the tag; attaching the docs as a release asset is a second mechanism for the same need and waits for the row's trigger. |
| **R8-index-11** (Required) index: Deferred decisions: Several Reopened by cells name plan steps (the start of T005, item e) instead of the event the step produces, while plan.md says the spec is the plan's source of truth. | L13b | A | Apply as proposed. |
| **R8-index-12** (Required) index: Principles (Decide at the last responsible moment); ADR 0005 rule 1: The carve-out 'a security invariant, a merge gate and anything a later step depends on are never deferred' is open-ended (any text that adds a dependent step makes a need of the start) and is contradicted by five deferred rows that defer parts of the merge gate, only one of which records the collision. | L11, L13b (high-signal) | operator | Recommend the rewrite: it bounds the carve-out and names the one exception path ADR 0008 already used. Decision DR4. |
| **R8-index-13** (Required) [ADR 0007, adopt testify assert and require in tests](../adr/0007-adopt-testify-assert-and-require-in-tests.md) (Threat model); index: Deferred decisions: ADR 0007 defers two escapes (an untracked go.work, a caller's GOFLAGS -overlay) until the Q25 sandbox-token question is decided, through a row that exists in neither the index nor the plan; Q25 was decided on 2026-10-07, so the trigger fired and nothing reopened it. | L11 | operator | Recommend the row with the hook trigger, or deciding the guard now, since its first trigger has fired. Decision DR3. |
| **R8-index-14** (Required) index: Deferred decisions (a check that reads the rulesets again): The row says 'tools/ci all is offline', while 10 10.2 runs govulncheck over the network and pulls the reuse image, and 10 10.1's network rule lists neither. | L13 | A | Apply as proposed. |
| **R8-index-15** (Advisory) index: Deferred decisions (running the change's tests in a separate job): The row says an attack on the tools/ci process is 'out of its reach, and the review of the diff covers it', while 05 5.4 and 10 10.2 item a record that the ruleset requires no review. | L2b | A | Apply as proposed; adding release.yml to the trigger (L2b-8) is that finding's own cluster. |
| **R8-index-16** (Advisory) index: Deferred decisions (turning on the agent's own telemetry export): The telemetry row says romeu sets one variable in a sandbox and cites I1, which is about panes and the final argv, while 04 4.1 also sets SSH_AUTH_SOCK for the sbx process of a git-ssh-sign project. | L13b | A | Apply as proposed. |
| **R8-index-17** (Advisory) index: Deferred decisions (ledger rotation, tuning, finding-class): Two rows share one subject and one trigger (rotation of the runtime ledger and tuning its starting values, both reopened by doctor's size warning), and the finding-class row can fire only once the gate-run row's mechanism exists. | L4, L11 (high-signal) | M | Apply by merging; a field that depends on the gate-run record needs no trigger of its own. Regression check: the merged rows keep the union of their triggers and name the decisions folded (op-gate-run and op-finding-class, round 4; op-ledger-retention, round 5). |
| **R8-index-18** (Required) index: Deferred decisions (whether the maintainer's merge stays a manual checkpoint): The merge-checkpoint row fires on the first dora output with a non-null maintainerWaitSeconds, data the row itself says cannot tell the maintainer's merges from the sandbox's, and 12 12.10 names that number 'maintainer wait' although it covers CI, review and sandbox merges. | L9 | M | Apply; the rename is spec text (no code reads the field yet) and removes the cell's own disclaimer. Regression check: the measure is op-maintainer-wait's (round 4) under a clearer name; the measure itself is unchanged. |
| **R8-index-19** (Advisory) index: Deferred decisions: No row records the loss mechanisms v1 leaves out (snapshot history, salvage of a sandbox that does not start, a host-visible time of the last good snapshot, host-state migration), although a deferral without a row is an oversight. | L4 | M | Apply as one row instead of one per mechanism; they share a trigger. |
| **R8-index-20** (Advisory) index: Deferred decisions: No row covers the disk growth that only the user feels: salvage payloads of up to 1 GiB each, snapshots, .attic and review checkouts, which romeu never deletes (I16) and no command reports. | L1 | M | Apply the row without a size column in status now; a size report is the decision the row defers. |
| **R8-index-21** (Advisory) index: Deferred decisions: The table mixes deferrals in the product with deferrals in how this repository is run, so an adopter scanning for what v1 leaves out reads a dozen rows about one maintainer's GitHub account. | L10 | M | Apply; the merge of the interval row that L10-13 also asks for is in the interval cluster. |
| **R8-index-22** (Blocking) index: Success criteria: The objective's user promises have no success criterion: one command per project with nothing to think about (00 0.1, 00 0.2), a product that carries real daily work, and 'nothing sandbox-only is lost'; S1 to S11 measure the producer, and S7 passes a journey of any length. | L0, L1, L4 (high-signal) | operator | The page consolidator proposed M: apply with recorded counts and a review comparison instead of invented bounds. Routed to the operator by the gate review of this page: adding S12 is a scope change (a success criterion), and R8-00-3 and R8-09-2 decline S12, so the three are one decision. Recommendation: the counts as B3 result fields compared in review, no S12 now. Decision SC16. |
| **R8-index-23** (Required) index: Success criteria (S8): S8 gates a number the user does not feel (romeu's overhead with sbx time excluded, on an empty ledger) and asserts a wall-clock bound on shared hosted runners with no noise budget, a designed flake, while the end-to-end wait of a run and the origin of 2 s and 3 s are stated nowhere. | L0, L1, L6, L11 (high-signal) | M | Apply; dropping --timings (L11-13) is declined, because the ledger rotation trigger reads it and dropping it would change 00 0.5. Regression check: the romeu half follows the SC3 decision on --timings; the julieta half keeps its container e2e assertion (op-s8-limit, round 6) either way. |
| **R8-index-24** (Blocking) index: Success criteria (S1): S1's evidence says the attestation step signed the archives, while 05 5.4 says signing is an operator step with a credential outside GitHub, and its evidence is the same run's own verify, so nothing shows that a user who downloads the assets can verify them. | L7, L13 (high-signal) | M | Apply the S1 rewording and the consumer-side evidence; the missing signature mechanism belongs to 05 5.4, an accepted-risk section, and goes to the operator there. Follows DR2. |
| **R8-index-25** (Blocking) J1; J9: The user's release verification is underspecified: J1 step 2's gh attestation verify binds the repository and workflow path, not the tag or the commit; J9's upgrade has no verify steps; and J1 step 1 does not say that verify needs network access and an authenticated gh. | L7 | M | Apply; the residual risk (an agent holding the token can produce a release the contract accepts) is the existing 05 5.4 Release signing row, so no accepted risk is added. Publishing the attestation bundles for offline verify waits for a request. Follows DR2. |
| **R8-index-26** (Required) J1: No page decides code signing and notarization of the darwin binary, and J1 does not mention Gatekeeper, so the product's first step may teach users to remove the quarantine attribute. | L7 | M | Apply; the Apple behaviour stays marked inferred until B1 measures it. |
| **R8-index-27** (Required) J1; J2: J1 and J2 omit setup steps their commands require: J1 assumes a config repo already exists; J1 step 5 adds only secrets while 03 3.1 and 06 6.4 require gitHosts, workloadRepositories and signing.agentSocket; and J2 omits the name@project secret binding, so its sync exits 2. | L1, L13b (high-signal) | M | Apply without romeu init --scaffold and without a deferred row for a binding command: a documented copy and a printed entry meet the need with no new command. |
| **R8-index-28** (Advisory) J12: 09's opening says J12 runs J1 steps 3 to 6 while the J12 section says steps 1 to 5, and J12 does not say whether memory reaches the second machine. | L1, L13, L13b (high-signal) | A | Apply as proposed. |
| **R8-index-29** (Required) J6: J6 step 3 and 04 rm show the 'no final handoff' warning after julieta salvage --stop-agents has stopped the agent, so the one actor who could still write the final handoff is gone. | L13 | M | Apply as proposed. |
| **R8-index-30** (Required) J6; J10: Recovery of salvaged work has no documented, checked steps: J10 step 3 is a plain git command with '(docs show the hardened form)', outside [ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) rule 13's check, and no journey shows how to restore a dirty-tree commit, stash commits or ignored.tar.gz. | L4, L8 (high-signal) | M | Apply without a new romeu salvage --restore command; the checked command line closes the gate gap. Follows SC4. |
| **R8-index-31** (Required) J8: J8 step 3 says running sandboxes already have the tool installed from inside, contradicting 07 7.5: a new tool's domains reach the egress only after the change merges into ref and romeu sync applies it, so step 1's julieta install fails. | L5 | M | Apply as proposed. |
| **R8-index-32** (Advisory) J5: Every journey is told from the operator's side; none shows what the agent sees when its context is compacted, its request is blocked by egress, its push is refused or its spec change waits at the gate. | L5 | L | State as a limit; an agent-side journey is worth adding when the first regression in agent-facing output reaches a user. |
| **R8-index-33** (Advisory) index: Open questions (Q13): Q13's default (per-project fine-grained tokens) and ADR 0008 decision 3 (the token in every project is the operator's own) disagree on what holds today. | L2 | A | Apply as proposed. |
| **R8-index-34** (Required) index: Success criteria (S9): S9 neither counts a package with no test file (absent from the profile) nor states where its thresholds and its 90 percent package list came from, and it leaves out cmd/ without a reason. | L6, L11 (high-signal) | M | Apply; naming the 90 percent set by a property waits for the first coverage run, which shows which packages it fits. |
| **R8-index-35** (Required) index: Success criteria (S6): S6 is evidenced by tool names and no run; mutate is not a step of all and release.yml runs all, so the release commit never has a mutate result. | L6 | M | Apply as proposed. Regression check: in R8-10-28's form (the release refuses when the latest scheduled fuzz.yml run is not success), so mutate is not added to release.yml. |
| **R8-index-36** (Required) index: Success criteria (S7): J6, the nothing-lost journey, runs only in the host half, once, on the release candidate, though the hybrid level could reach it. | L6 | M | Apply as proposed. |
| **R8-index-37** (Advisory) index: Success criteria (S4): S4's evidence is a green config-repo run, which cannot show its second clause, exit 1 on an unknown. | L6 | A | Apply as proposed. |
| **R8-index-38** (Advisory) index: Success criteria (S10): S10 promises generated reference 'per command, file format, error id and exit code', while 04 4.1 and 12 12.3 generate one errors.md, one exit-codes.md and pages for two formats. | L8 | M | Apply the alignment only; more generated pages wait for a reader who needs them. |
| **R8-index-39** (Advisory) index: Boundaries: The Boundaries' local-gate sentences overclaim: 'run tools/ci all before opening a PR' cannot be met on one platform (license and the hybrid e2e are Linux only, the I27 cases macOS only), 'every merge gate CI runs also runs locally' ignores the ruleset, which has no local form, and S2 omits the administrator bypass. | L6, L14 (high-signal) | A | Apply as proposed. Regression check: the Boundaries rewording does not restate op-equivalence (round 3), which stays in 10 10.2. |
| **R8-index-40** (Advisory) 10 10.5; index: Success criteria: docs/acceptance.json is a one-shot v1.0.0 artifact, and nothing says how a later release states its criteria or which of S1 to S11 stay living. | L3 | L | State as a limit; a per-release acceptance format waits for the first release after v1.0.0. See SC15. |
| **R8-index-41** (Advisory) 12 12.8; index: Principles: ARCHITECTURE.md, the page 'Simple and explicit' promises a newcomer, has nothing mechanical keeping it true: headings.yaml covers README.md and SECURITY.md only, and only its diagrams are generated. | L3 | M | Apply; drift of this kind is mechanical and belongs to a static gate. |
| **R8-index-42** (Required) index (Status); 05 5.4; 10 10.2; 12 12.5: History has moved into the current-state spec: the index's Status paragraph grows a clause per round, deferred rows carry reasoning paragraphs, and 05 5.4 and 10 10.2 carry dated measurements, while 12 12.5 says history lives in reviews and ADRs. | L3 | M | Apply the index half without a cell-length bound in sequences; the sentence and review suffice until a row overflows again. |
| **R8-index-43** (Blocking) 10 10.2; index: Deferred decisions; ADR 0007: About 330 lines of 10 10.2 are check logic in prose with pinned versions and commit ids, while tools/ci is the truth and nothing holds the two equal; the index's deferred row covers generating the step table only. | L3 | M | Apply the index half; moving the prose is decided with page 10. |
| **R8-index-44** (Required) I7: I7's E test says each gate 2 change makes run, stop, salvage, rm, retire and pull exit 3, but no defined gate 2 change produces exit 3 for stop: a new spec commit changes no live file until promoted, and a tampered live file is drift, exit 4. | L13 | M | Apply as proposed. |
| **R8-index-45** (Required) I30: Boundary C and I30 say julieta runs only as delivered by romeu, while 01 1.1, 02 2.2 and 07 7.6 run it in the config repo's CI from a pinned release with no compatibility check. | L13 | A | Apply as proposed. |
| **R8-index-46** (Advisory) I24: The memory layout allowlist (I24, 08 8.1) reserves import/, and no command reads or writes it; julieta memory import reads a path given on the command line. | L13 | A | Apply by deletion. |
| **R8-index-47** (Advisory) I26: Sandbox identity is the name plus the workspace path plus an open generation, so a sandbox recreated by hand with sbx under the same name and path passes while romeu's open generation belongs to the removed one. | L2b | M | Apply, settled by A4's result. |
| **R8-index-48** (Required) index: Open questions; plan.md: The plan's Questions for the maintainer is a second list of open items about the spec (eleven unanswered), held outside the spec, and the index's Open questions neither holds them nor points at them. | L13, L13b (high-signal) | M | Apply; it makes the index the one list of open questions about the spec. |
| **R8-00-1** (Blocking) 00 0.5; 13; [ADR 0006, record runtime events in an add-only ledger ingested on the host](../adr/0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md); 12 12.2: The runtime ledger is in v1 with no row in 00 0.5, no success criterion and no journey, and it sits on the build path before romeu run. | L9, L11, L13 (high-signal) | operator | Recommend (b) unless the operator wants a smaller v1: one 00 0.5 row and ADR 0006's missing 'not in v1' alternative close the traceability gap at low cost; (a) is the right call if the ledger has no first-day reader the operator can name. The cut-line part of L9-20 is not taken into the spec. Regression check: one recommendation for the four ledger clusters, SC1; it supersedes op-ledger and the eleven op-ledger-* decisions of round 5 if taken. Decision SC1. |
| **R8-00-2** (Blocking) 00 0.5; 04 4.3; 08 8.1; 03 3.9; 10 10.5; 11 11.2; I24: The generic product carries one person's migration (julieta memory import and verify, the import/ entry, the memory-entry digest kind, the repo evidence kind, tools/ci acceptance --file, the C1 hugo check), justified in 00 0.5 by the maintainer's own pre-v1 memory. | L10, L11 (high-signal) | operator | Recommend (a): the import runs once, on one machine, and a config-repo script serves it without the product carrying six concepts for good. If the operator keeps them, apply (b), which is L10-28's change. Regression check: the recommendation reverses op-migration-set (round 6), a maintainer decision in the maintainer's own words, and says so. Decision SC2. |
| **R8-00-3** (Blocking) 00 0.2; index: Success criteria: The secondary user's need ('the product is generic') traces to no criterion, invariant or journey; every acceptance runs with the maintainer's own config repo. | L10 | operator | The page consolidator proposed L: state the limit in 00 0.2 and defer the stranger-run criterion with a trigger, instead of adding S12. Routed to the operator by the gate review of this page: whether v1 gains a user-facing criterion is one scope decision across R8-index-22, R8-00-3 and R8-09-2. Recommendation: the limit and the Deferred row as proposed. Decision SC16. |
| **R8-00-4** (Required) 00 0.1; 08 8.5: The objective's 'nothing sandbox-only is lost' has no scope, while 08 8.5 salvages a closed list (recorded repos, ignored files within the cap, the agent profile's state paths, transcripts on request). | L0, L4 (high-signal) | M | Apply the qualifier and the 'Not salvaged' list; drop the proposed 05 5.4 residual-risk row (it would change the accepted-risk list) and leave the 08 8.4 sentence to the page-08 consolidation. |
| **R8-00-5** (Required) 00 0.1; 00 0.2; 11 A11: The objective says nothing an agent wrote is ever executed on the host, while personal kits are agent-authored and sbx builds them at create, and no probe records where a kit's install step runs. | L0 | M | Weaken the universal to what the design guarantees and attach the measurement that could restore it; no new mechanism. |
| **R8-00-6** (Required) 00 0.5: 00 0.5 claims every v1 feature traces to a criterion, an invariant, a journey or a named need, but its table is a sample (romeu pull, retire, recreate, julieta lock, spec validate, pin workload and others have no row) and justifies julieta handoff list by 'kept in v1 by the maintainer' alone. | L11, L13b (high-signal) | M | Weaken the universal to what the table is and move the burden to the 04 command rows, instead of adding about twelve rows and a check; the 'marked v1.1' clause goes with R8-00-7. Regression check: the rewording only; the deletion branch for handoff list is the SC6 decision (op-handoff-list, round 6). |
| **R8-00-7** (Required) 00 0.5; 07 7.5; index: Deferred decisions: The 'v1.1' row of 00 0.5 holds four items (native egress queue in status, handoff --list filters, a Remote-SSH helper, a registry credential) with no trigger and no interim, and the registry credential also lives as Q23 under a different rule. | L11 | operator | The page consolidator proposed A: Apply as proposed: one table, one rule, every deferral with its interim and trigger. Routed to the operator by the regression check: the same change as R8-index-3 and R8-07-3; the v1.1 row is a scope label of 00 0.5 and op-no-live-view (round 6) kept the Remote-SSH helper listed there. Decision SC7. |
| **R8-00-8** (Required) index S8; 00 0.5; 04 4.2; 02 2.1; 03 3.13: S8 and run --timings justify each other: the flag exists only for the criterion and the criterion's evidence is the flag, so a schema, a CI median-of-5 test and a probe measure budgets (2 s, 3 s) with no stated origin. | L11 | operator | Recommend dropping --timings and timing in the harness; apply the starting-values sentence to S8 regardless. Regression check: removes the CI measurement te H4 (round 1) made of romeu's half and moots R3-testability-14 (round 6); the julieta half's container e2e assertion (op-s8-limit) stays either way. Decision SC3. |
| **R8-00-9** (Required) 00 0.6; 00 0.2: 00 0.6 is a 35-line analysis of an external report (with page numbers) for a user v1 does not serve, on the page that decides v1 scope, and its 'partial' rows read as commitments. | L3, L11 (high-signal) | M | Move the analysis to a docs note and keep a three-sentence 0.6, keeping the teams row in 0.2 with 'none'; the 12 12.10 trim is left to the page-12 consolidation. See SC12. |
| **R8-00-10** (Required) 00 0.3; 03 3.2; 12 12.9: 00 0.3 says a run-layout renderer interface exists as the hedge against herdr, a pre-1.0 project on the path of every romeu run, but the interface is specified nowhere and no row says what happens if herdr is abandoned. | L11, L15 (high-signal) | M | Delete the unsupported 'interface exists' claim and carry the hedge as a Deferred row with an abandonment trigger, instead of specifying a renderer contract now. |
| **R8-00-11** (Required) 00 0.2; 12 12.8; 09 J1: The prerequisites a stranger needs are not stated: how the agent logs in inside the sandbox, mise in each worked-on repo (07 7.2), GitHub for the config repo's CI, and Keychain items for secrets. | L10 | M | List the prerequisites once in 12 12.8 and point to it; drop the new 0.3 audience list, which would widen the non-goals (a scope change) and mostly repeats existing ones. |
| **R8-00-12** (Required) 00 0.4; index S1; 10 10.2: 00 0.4 and S11 cover the repository's files, not the release archives: nothing says an archive carries COPYING and the notices of statically linked modules, and nothing checks a new dependency's license against GPL-3.0-only. | L7 | A | Apply as proposed: a deterministic check is the right home for license compatibility. |
| **R8-00-13** (Required) 01 1.7; 00 0.2; 05 5.3; 05 5.4; 10 10.2; ADR 0008: The same person is 'operator', 'maintainer', 'developer' and 'user' across the pages, none of the four is in the vocabulary, and the sentence relating them is only in ADR 0008 Context. | L3 | M | Define the words once in 01 1.7 and link from 00 0.2; 'user' stays plain English, not a fourth term. Shared with the page-01 consolidation, which owns the rows. |
| **R8-00-14** (Advisory) 00 0.2; 00 0.6: No user row names the organization whose code passes through the tree, and 0.6 says memory answers 'loss of familiarity' without saying memory is machine-local and leaves with the developer. | L16 | M | State the limit in a sentence instead of adding a users row with a support promise. |
| **R8-00-15** (Advisory) 00 0.4: Product kits are GPL-3.0-only and users put personal kits in their config repo, but nothing says what a personal kit copied from a product kit is licensed under. | L10 | L | State the license outcome in one sentence; examples/ already carries kits (02 2.1), so no new example is needed. |
| **R8-01-1** (Blocking) 01 1.2: The 'Agent output that reaches the host' row omits byte streams romeu decodes from the sandbox (julieta --json stdout, snapshot/heads.json), so no grammar, size cap or validation is specified, fuzzed or probed for them, and the Truth row omits the 05 5.4 caveat. | L2, L2b (high-signal) | M | Apply the row extension and the single strict-decode rule in 04 4.1 instead of the full consumer table both lenses propose; the rule covers every stream, current and future, in one sentence. |
| **R8-01-2** (Blocking) 01 1.2: No single place lists every store the product writes or causes to exist with owner, readers and lifetime; facts are spread over seven sections and some stores (sbx per-name volumes, refs in host clones, the state attic) appear in none. | L16 | M | Add the table by hand in 01 and defer the generated reference page with its trigger, rather than adding a generator row now. |
| **R8-01-3** (Advisory) 01 1.2: The two workspace files are rewritten whole with no record of their hash, so a hand edit (for example a memory dir added as a folder, which boundary D forbids) stays live until the next promotion or retire and doctor has no row for it. | L0 | M | Use the derive-and-compare check the ledger view already has; no hash record. Regression check: recorded as a supersession of R3-doubt-17 (round 6), which chose not to drift-check the workspace files. |
| **R8-01-4** (Required) 01 1.2: romeu pull creates a review checkout, but review.code-workspace is written only at promotion and retire, so the checkout J4 step 4 opens is listed by no workspace file until the next promotion. | L13b | A | Apply; it depends on the operator's answer to R8-01-5 (if pull is deferred, this goes with it). Follows SC4. |
| **R8-01-5** (Required) 04 4.2 (pull); 01 1.2; 02 2.3; I13; I14; I19; 05 5.4: romeu pull, the hardened review checkout, both workspace files, I19 and two residual-risk rows exist for a host-side review path the start does not need, and dev.code-workspace is opened by no journey. | L11 | operator | Operator: it removes a v1 command (scope, 00 0.3) and two accepted-risk rows of 05 5.4. Recommendation: defer pull and the review checkout as proposed, and delete dev.code-workspace, which no journey opens. Decision SC4. |
| **R8-01-6** (Required) 01 1.2: The trust model covers only sandbox-to-host flows; which host facts the agent may read, and through which surface, has no class, so the agent that authored a spec change cannot learn whether it is unsynced, awaiting, declined or live. | L5 | M | Give the agent the one fact that answers the question (the live spec commit) and state the candidate state as host-only, instead of a view record of candidate state. |
| **R8-01-7** (Blocking) 03 (opening); 01 1.2; 12 12.3: The format tables of 03 are design input that the product later generates, so after v1 the spec holds a second, unmaintained copy of every format. | L3 | M | Apply the policy and the deletion pass; defer the header-equality check with its trigger. |
| **R8-01-8** (Blocking) 01 1.4: Nothing says what romeu does with an sbx version no recording covers: sbx has a floor and no ceiling, gate 1 compares for equality, and approve --toolchain shows nothing about sbx, so a never-recorded sbx is accepted on a blind acknowledgement and parsers may misread its output. | L14, L15 (high-signal) | M | Recommend warn, not refuse, outside the window, with fail-closed parsing as the safety net; no new state. |
| **R8-01-9** (Required) 01 1.4: Gate 1 identifies romeu and julieta only by their version strings, so a source build with the same version passes silently and a downgrade is shown as an ordinary change. | L7 | M | Apply the sha256 fields and the downgrade label; leave out the 'binary not built by release.yml' doctor check, which needs provenance verification, a separate decision. |
| **R8-01-10** (Advisory) 01 1.4: The spec does not say how a dev build of main is versioned, so dogfooding pre-release builds needs one acknowledgement per build with no stated rule. | L9 | L | State the cost as a limit (one y per build) instead of a weaker comparison for dev builds. |
| **R8-01-11** (Advisory) 01 1.4 (Gate 1); 03 3.8; 04 4.2 (approve --toolchain): Gate 1 is a per-machine acknowledgement with its own record, printer, exit code and flag, and the spec does not state what threat it stops that doctor would not, on a machine where the operator is the one who upgrades. | L11 | L | Keep gate 1 and state its threat; folding it into doctor would let an unattended upgrade widen egress silently. See SC8. |
| **R8-01-12** (Required) 01 1.4: sandboxOptions is stripped from renderDigest without a stated reason, and a spec field that carries no gate tag is in neither the digest nor the generated I7 flip table, so a forgotten tag passes every test. | L6, L13b (high-signal) | A | Apply. |
| **R8-01-13** (Advisory) 01 1.4 (Gate 2): The gate 2 summary shows every change in one form, so a content-only kit edit and a new capability look alike and train the user to approve without reading. | L1 | A | Apply. |
| **R8-01-14** (Required) 01 1.4; 06 6.1; I28; 12 12.2: The v3 descriptor grammar is parsed by two packages with no single owner, and nothing says what happens when sbx adds a capability type the strict grammar refuses. | L3 | M | Name internal/oci as the owner instead of a new package; state the fail-closed cost as a deferred row. |
| **R8-01-15** (Advisory) 01 1.4 (Gate 2, capabilities); 06 6.1; I28; Q20; Q23; 02 2.4: A registry client, a strict descriptor grammar, a descriptor cache and per-type conformance fixtures exist in v1 to decompose a workload already pinned by digest and allowlisted by repository, and no sentence says what the gate loses without it. | L11 | L | State what the decomposition buys and keep it in v1; if that sentence cannot be written truthfully, the deferral goes to the operator as a scope change. |
| **R8-01-16** (Blocking) 01 1.5: Preflight checks that cannot widen a sandbox block the commands whose job is to preserve data: gate 1, drift and the signing-socket check make stop, salvage, salvage --from-host, rm and pull exit before they can save work. | L0, L1, L4 (high-signal) | M | Apply as one rule for the protective commands, covering all three findings; keep the full preflight for run, the live changes of sync and any removal. Regression check: the exemption covers gate 1 (step 1), the signing-socket check (step 5) and the gate 2 digest that 01 1.5 already allows for destructive commands; drift (step 3) and the identity check (step 6) still stop every command, as sa H2 (round 1) ruled. T074 (plan) changes with it. |
| **R8-01-17** (Advisory) 01 1.5: Step 5 is justified as checks that 'can widen a sandbox behind romeu's back', yet it holds the VS Code trust check (I23), which widens no sandbox. | L0 | A | Apply. |
| **R8-01-18** (Required) 01 1.5: 01 1.5 says every command marked P runs the preflight, but sync is not marked P in 04 4.2 although it runs step 1 first and the rest before its first mutating call. | L13b | A | Apply. |
| **R8-01-19** (Required) 01 1.5; I22: Step 5's I22 check parses sbx listings and is not required to fail closed, so a changed or empty listing passes a security check on zero rows. | L15 | A | Apply; it uses the id introduced by R8-01-8. |
| **R8-01-20** (Advisory) 06 6.4 (git-ssh-sign); Q19; 05 5.3; 01 1.5 step 5: Commit signing from a dedicated host agent is a product kit with an ask-first surface, a settings field, a preflight check and an open question, and no sentence says why the start needs it. | L11 | operator | Operator: it adds a reason to the scope (00 0.5) that only the operator can state. Recommendation: accept the proposed sentence if it is the operator's reason. Decision SC11. |
| **R8-01-21** (Required) 01 1.6 (Candidate); 04 4.2 (approve); J11 step 5: The awaiting state, the superseded transition and `approve <name>` exist for a caller that runs sync without a terminal, and no component, journey or user is that caller. | L11 | operator | The page consolidator proposed M: Take the deletion branch: no caller exists, so the state is deferred with its trigger. Routed to the operator by the regression check: the same deletion as R8-04-19, which removes `approve <name>` from the v1 command set (00 0.5) and supersedes craft C2 (round 2). Decision SC5. |
| **R8-01-22** (Required) 01 1.6; 03 3.8: Which states persist is unstated: the candidate machine has six states and the record persists only awaiting, an approval recorded before promotion step 1 has no recovery, and the generation table has no row for stop, pull, salvage or retire finding the sandbox absent. | L3, L13 (high-signal) | M | Say approved is transient (a kill before step 1 means sync again) instead of adding an approved-window recovery; one row covers the four absent-sandbox commands. Regression check: R8-03-19 proposes the same in a second form; the 01 1.6 sentence (approved is transient) stands and 03 3.8 points at it. |
| **R8-01-23** (Blocking) 01 1.6: The generation machine states no persistence order and no recovery for a kill between the side effects of a destructive command (salvage refs per repo, the salvage record, sbx env rm, closed-removed). | L4 | M | One new state (removing) and one recovery sentence, instead of pending and complete record states. |
| **R8-01-24** (Blocking) 01 1.6: One sbx ls --json that does not list the sandbox is read as absent, and run then closes the generation as lost and creates a same-name sandbox; there is no unknown answer distinct from absent. | L4 | M | Drop the double read and the TTY prompt; a second command is the confirmation. |
| **R8-01-25** (Required) 01 1.6: The generation table has no row for a sandbox half that failed (julieta exit 1, a timeout, disk full), and 'open generation' in 1.3 F, 1.5 step 6, I26 and adopt is ambiguous beside the state names open and salvaging. | L4 | A | Apply. |
| **R8-01-26** (Required) 01 1.6: salvage --from-host can record result lost while the generation stays open and a later absent-sandbox row records a second lost, and nothing says what --from-host does when the sandbox is present or whether a bare salvage stops agents. | L13 | M | Keep the rule that only the absent-sandbox row closes; deduplicate the record instead of moving the close into --from-host. |
| **R8-01-27** (Required) 01 1.6: adopt records as an open generation a sandbox gate 2 never saw, on a prompt that shows only name and path; its mounts, kits and workload are never compared with the live sbxenv.yaml, so boundary C ('nothing unapproved is ever at a live path') does not hold for it. | L2, L2b (high-signal) | operator | Operator: it adds an accepted risk to 05 5.4 (an ungated live sandbox after adopt). Recommendation: accept it with the printed inventory and the status label; refusing adopt instead loses the J3 recovery. Decision AR2. |
| **R8-01-28** (Required) 01 1.6: When promotion recovery finds a candidate that no longer verifies after step 2 rewrote .romeu/bin and the workspace files, the text does not say which render.json the drift check then uses, so romeu reports its own half-finished promotion as drift. | L4 | A | Apply. |
| **R8-01-29** (Required) 01 1.6; index (principle); 11 11.4; 12 12.8: The index says every lifecycle is a transition table in internal/state, but the promotion commit is a numbered list, the probe lifecycle is a table in 11 11.4 implemented by tools/ci, and 12 12.8 promises four generated diagrams from tables that hold two. | L13b | M | Name which lifecycles are tables instead of adding two more tables. |
| **R8-01-30** (Advisory) 01 1.6: Tests generated from the transition tables cannot disagree with the tables, and the spec does not say the Effect column is asserted. | L6 | A | Apply. |
| **R8-01-31** (Advisory) 01 1.6; 04 4.2; 07 7.3; 07 7.5; 08 8.4: The spec quotes user-facing message text in prose with no error id ('candidate changed; run romeu sync' in 01 1.6 and four others), a second hand-written source of the text 04 4.1 keeps in one table. | L8 | A | Apply (moot for 01 1.6 if R8-01-21 deletes the awaiting rows). Follows SC5. |
| **R8-01-32** (Advisory) 07 7.5; 13 13.2; 04 4.2; 01 1.6: The escape hatches (egress.tools, egress.extra, tool: other, --accept-loss, --overwrite-drift, retire --force) are each bounded but none is counted, so nothing shows when one has become the main road. | L3 | L | State as deferred with its trigger; no counters now. |
| **R8-01-33** (Required) 01 1.7; 01 1.1; 05 5.3; 05 5.4; 09 (legend); 10 10.2; 00 0.2; 06 6.4: The vocabulary table lacks rows for concepts with several names or one name with several meanings (operator versus maintainer, developer and user; kit versus mixin; stale; session; drift; promotion; interrupted promotion; preflight; orphaned), spec collides with the spec's self-references, and eleven rows have an empty Not column, so the gate enforces about seven phrases. | L3, L8, L13, L13b (high-signal) | M | One pass adds the rows the five findings name; rows for 'tool', 'host tool' and 'compatibility check' wait until a rival word is found in use; the glossary generator is deferred. |
| **R8-01-34** (Blocking) 01 1.7; ADR 0008; 05 5.4; 06 6.4: The word sandbox names both the development sandbox this repository is built in (ADR 0008, the operator's keys) and a romeu-managed sandbox (06 6.4, one signing key), with one row in 01 1.7 and no page saying which is meant. | L13b | M | Apply in the spec pages and leave the decision record unchanged, so no ADR edit is needed; 05 5.4 changes wording only, no accepted risk. |
| **R8-01-35** (Required) 05 5.3; 01 1.7; 12 12.3; 10 10.2: The vocabulary table and the index's tables are check inputs, yet docs/spec/** is on no ask-first surface, while the generated denylist's path is not named and, if under tools/ci, every vocabulary edit crosses the checks surface. | L3, L12 (high-signal) | operator | Operator: which paths are ask-first is the operator's decision on a security-model surface. Recommendation: name the generated path outside tools/ci, and add the specification surface, since 05 5.4 and the gate inputs live there. Decision AR13. |
| **R8-01-36** (Required) 01 1.7; 04 4.2; J7: retire moves a project to .attic and never deletes, and nothing tells the owner how to delete what they own (.attic, the state attic, salvage payloads). | L16 | A | Apply. |
| **R8-02-1** (Required) 02 2.4; 02 2.5 (also 10 10.6, 04 4.1 via L3-21): Two universal claims on this page are refuted by its own tables: 2.5 'julieta never guesses paths' beside six $HOME-derived paths (julieta symlink, secondaries, state, hook dispatcher, mise, herdr) no manifest field carries, and 2.4 'ROMEU_SETTINGS is the only environment override' beside XDG_CONFIG_HOME and XDG_STATE_HOME locating settings and state. | L0, L3, L13b, L15 (high-signal) | A | Apply as proposed: reword the 2.5 and 2.4 claims to what is true, have the mixin set the mise and herdr paths explicitly, name the sandbox-home source in 03 3.6, date the 2.6 sbx behavior and tie it to A3. |
| **R8-02-2** (Required) 02 2.1 (tree): The tree has one `cmd/<binary>/main.go` and one internal/cli dispatcher for about 36 commands built by about 25 tasks, so every command PR edits the same shared files and tools/new command writes into a file the spec never names. | L12 | M | Apply in the smaller form: no new generator, the existing 12 12.3 command-definitions row emits the dispatcher table; two tree lines and one sentence in 12 12.3. |
| **R8-02-3** (Required) 02 2.1 (Embedding): Embedding says the julieta binaries are embedded 'at release time' but not what a plain source build (go build, go install @vX) embeds or whether it refuses to run. | L7 | M | Apply with the placeholder-plus-refusal form only; no second build path. |
| **R8-02-4** (Required) 02 2.2 (validate.yml): The config repo's validate.yml runs a downloaded julieta release that nothing verifies by bytes; julieta pin check --workflows checks freshness only. | L7 | M | Apply with a sha256 pin and the checksums.txt comparison; the attestation check follows whatever 10 10.2 decides for release attestation, not a second contract here. See SC9. |
| **R8-02-5** (Required) 02 2.3; 02 2.6: Salvage refs live only in host clones the spec calls plain operator clones, and the owner is told neither that deleting or re-cloning one loses them nor that git push --mirror publishes them (salvage commits can hold untracked files). | L4, L16 (high-signal) | M | Apply as one owner-facing statement in 2.3 plus the one doctor check; no new storage for salvage refs. |
| **R8-02-6** (Required) 02 2.4 (Each record is one file...); 01 1.6: 'every state transition of a project is a single atomic write' is false for promotion, salvage and retire, and retire makes two moves (tree under $ROMEU_ROOT, record under $XDG_STATE_HOME) with no recovery rule. | L4 | A | Apply as proposed. |
| **R8-02-7** (Required) 02 2.3 (host tree modes): No mode is fixed for the host tree that holds memory, salvage payloads, spool, view, .attic and review checkouts; only settings, state and the ledger have one and doctor checks only those. | L16 | M | Apply in the smaller form: fix the mode on the two parent directories (`<name>-env/`, .attic/) rather than on each subdirectory, so one doctor row covers the tree. |
| **R8-02-8** (Required) 02 2.1; 10 10.7; 12 12.2; S9: 02 2.1's package tree and its '(romeu only)', '(julieta only)' and '(shared)' comments are a hand copy of 10 10.7's boundary table and 12 12.2's module map, with different names for the same code, and 10 10.7 is fail-open for a package no row names. | L3 | M | Apply with a test holding the three lists equal, not a generator; the 2.1 annotations go, 10 10.7 becomes the only home and fails closed. |
| **R8-02-9** (Required) 02 2.1 (tree, schemas/); 03 (opening): The pin file read by four consumers is in no tree and no tools/kitpin subcommand writes its workload entry, and three formats (dora.v1, ledger-entry/v1, the view files) are missing from the schemas/ list though 03 says every format has a schema. | L3 | A | Apply as proposed for page 02; coordinate the schemas/ line with the other schema-list edits in one write. |
| **R8-02-10** (Required) 02 2.3; 01 1.3 (boundary D); 05 5.4: Memory dirs and the spool are agent-writable host directories under $HOME/dev that host indexers (Spotlight importers, Quick Look, Time Machine) parse without the operator opening anything, while 05 5.4 speaks only of an operator who opens the directory and boundary D's 'no host tool is pointed at an agent-writable tree' is false for them. | L2 | operator | Recommend applying: the marker and the doctor warning are cheap and inside the spec; the 05 5.4 row widening is the operator's call. Decision AR1. |
| **R8-02-11** (Required) 02 2.6 (Consequences); 07 7.4; 10 10.3: 'secondary repos need the project's git credential inside the sandbox' and 07 7.4's 'the github secret covers it' rest on a mechanism the spec never states (how a named secret becomes the credential git, gh and mise present to GitHub in the sandbox) and that no probe measures; 10 10.3 lists credential injection under Cannot model. | L0 | M | Apply with the check folded into A3 rather than a new probe. |
| **R8-02-12** (Required) 02 2.4; 03 3.8; 04 4.2: `projects/<name>.json` holds generations[] (each with salvage[]) appended forever in a file rewritten whole on every mutating command, with no bound and no compaction. | L3 | L | State the growth as a limit and defer compaction with a measured trigger; no move-on-close mechanism in v1. |
| **R8-02-13** (Required) 02 2.3 (dev/review workspaces); 01 1.2; 04 4.2 (pull); J4; I13; I14; I19; 05 5.4: The spec never says where the user edits and debugs day to day: dev.code-workspace is rewritten on every promotion, carries I13 and I14, is opened by no journey and only in Restricted Mode, while romeu pull, the review checkout and review.code-workspace serve a host review path the start does not need (ADR 0008 makes the PR the review barrier). | L1, L11 (high-signal) | operator | Recommend deferring pull and review.code-workspace and deleting dev.code-workspace, plus the one-sentence daily-setup statement in 2.3 (which can be applied on its own now). Decision SC4. |
| **R8-02-14** (Required) 02 2.1 (schemas/run-timings.v1); index S8; 00 0.5; 04 4.2; 03 3.13: S8 and run --timings justify each other: the flag, the run-timings.v1 schema, a CI median-of-5 test and probe B5 exist to measure a 2 s / 3 s budget no user asked for and no measurement produced. | L11 | operator | Recommend applying: the criterion stays, measured by the harness. Regression check: see SC3; op-s8-limit's julieta assertion stays either way. Decision SC3. |
| **R8-02-15** (Required) 02 2.3 (tree, reserved names); 03 3.1; 05 5.1; I3; 04 4.2; I24; 08 8.1; 04 4.3; 07 7.3; 07 7.4; 04 4.4; 13 13.4: Five lists two or more consumers must agree on are hand-written in two or three places each; on this page, the reserved names in the 2.3 tree repeat the 03 3.1 dir rule. | L3 | M | Apply in the smaller form: one home per list and links elsewhere, tests only where both consumers are code; no new generator rows. |
| **R8-02-16** (Required) 02 2.6; 12 12.5; 06 6.4; 08 8.1; 12 12.1: The product's architecture decisions (including the 2.6 secondary-repo clone, herdr, v3 kits, memory as plain files) have no ADR, and 12 12.5 sends the docs module to review-round files, which the index calls history, to write them. | L3 | M | Apply as the table only; writing the ADRs stays plan work and goes through docs/adr's ask-first rule. |
| **R8-02-17** (Advisory) 02 2.3 ($ROMEU_ROOT default); J1: The default root $HOME/dev must not be inside a git repository, so a home that is itself a git repository (dotfiles in $HOME) fails the default at J1 and init does not say what happens. | L0 | A | Apply as proposed, reusing doctor's check. |
| **R8-02-18** (Advisory) 02 2.1 (Embedding); 02 2.3 (.romeu/bin); 06 6.3; Q21: Every romeu embeds both julieta linux binaries and writes both into every .romeu/bin, while Q21 fixes sandbox arch equal to host arch, so one of the two is never invoked. | L11 | A | Apply as proposed. Regression check: stated as resting on Q21's default, which A4 may reopen (dd N2, round 2). |
| **R8-02-19** (Advisory) 02 2.4 (descriptors/); 01 1.4; 06 6.1; I28; Q20; Q23: A registry client, strict descriptor grammar, content-addressed descriptor cache and per-capability conformance fixture exist in v1 to decompose a workload already pinned by digest and allowlisted by repository. | L11 | M | Apply the smaller option: state the reason in 06 6.1; no deferral unless the reason is missing. |
| **R8-02-20** (Advisory) 02 2.4 (ROMEU_SETTINGS): With ROMEU_SETTINGS described as a test-only override, a machine has one root, one state and one ledger, so an owner cannot keep employer and personal projects in separate trees and ledgers. | L16 | L | State as a limit with a deferred row; no second-instance support in v1. |
| **R8-02-21** (Advisory) 02 2.2 (Rules); J1: The config repo holds every project's repo URLs, secret names, egress extras and names, and the spec never says whether it should be private or what a public one discloses. | L16 | A | Apply as proposed. |
| **R8-03-1** (Blocking) 03 (opening), 03 3.4, 03 3.7, 03 3.8, 03 3.6: No stored format except the manifest and the event says what a reader does with another version, so an upgrade or a rollback turns strict decoding into exit 2 on every command, while the manifest's N-1 window is the one rule written down. | L3, L4, L9, L10, L11, L14 (high-signal) | M | One rule in 03's opening replaces per-format windows: refuse newer with a named id, forward-only migration owned by the release that bumps, undecodable records never overwritten; the manifest keeps N-1 with its stated reason, which answers L11-31's second option. No generic N/N-1 machinery for host formats until a bump needs it. |
| **R8-03-2** (Blocking) 03 (opening): Outputs the product does not own (sbx ls --json, sbx policy and secret listings, mise.lock, herdr replies, registry manifests, GitHub API JSON) have no decoding rule, so a renamed field can decode as a zero value that drives a 01 1.6 state transition. | L15 | M | Rule in 03's opening plus a pointer from 01 1.6 and 01 1.5, and fixtures per upstream; no new 05 5.2 invariant. |
| **R8-03-3** (Blocking) 03 (opening), 03 3.2, 01 1.4, 12 12.3: 03's field and rule tables are design input that the product later generates into docs/reference/, so after v1 the spec keeps a second, unmaintained copy of every table, and 01 1.4 and 12 12.3 send the 3.2 table to two destinations. | L3, L8 (high-signal) | M | Tables are deleted in favour of links in the generator's PR, recorded as T021 acceptance; the proposed tools/ci docs header check is dropped, since the deletion leaves nothing for it to compare. |
| **R8-03-4** (Blocking) 00 0.5, 03 3.9, 03 3.12, 04 4.3, 08 8.1, 10 10.5, 11 11.2, I24: The generic product carries one person's migration and acceptance (memory import and verify, the import/ directory, the memory-entry digest kind, the repo evidence kind, tools/ci acceptance --file, the C1 hugo check). | L11 | operator | Operator decision needed: It reverses a recorded maintainer decision in 00 0.5 ('memory entries written before v1 are imported when v1 is adopted, once') and changes the scope. Recommendation: delete and move the import to the operator's config repo; this also settles R8-03-27 (L0-28) and R8-03-28 (L16-13). Regression check: reverses op-migration-set (round 6) and says so. Decision SC2. |
| **R8-03-5** (Required) 03 (opening), 03 3.13, 02 2.1: 03's opening says every format has a version field, strict decoding, a JSON Schema and CI-validated examples, while the ledger entry, the view files, dora.v1, heads.json, SHA256SUMS, e2e/scenarios/commands.yaml and the pin file have no schema, and run-timings.v1 is in 02 2.1's schema list but defined nowhere in 03. | L0, L3, L13, L13b (high-signal) | M | Weaken the universal to the measured set and list the exceptions with reasons; add no new schemas. |
| **R8-03-6** (Required) 03 3.1, 04 4.3: Some 03 3.1 rules (gitHosts, workloadRepositories, name@project secret bindings) read host settings, which julieta spec validate in the config repo's CI cannot see, and no sentence says which validator skips which rule. | L5, L13b (high-signal) | A | Apply as proposed. |
| **R8-03-7** (Required) 03 (opening), 03 3.2, 03 3.4, 03 3.7, 03 3.8, 13 13.2: The version is spelled three ways (apiVersion: romeu/v1, integer version: 1, string schema: runtime-event/v1), so a generic reader must know a file's family first and a new format can pick a fourth form. | L3 | A | Apply as proposed. |
| **R8-03-8** (Required) 03 3.8, 03 3.9, 03 3.10, 03 3.12, 13 13.3, I24: ULID case differs between stores (uppercase for memory, handoffs and state; lowercase Crockford in the ledger for an APFS case-collision reason), timestamp field names follow no rule, and manifestSha256 is neither a canon kind nor in 03 3.12's raw list. | L13 | M | One ULID case and one timestamp naming rule, applied now while no format is released; the case rule reaches the memory allowlist I24 by consequence, not by a new check. |
| **R8-03-9** (Required) 03 (opening), 01 1.4, 10 10.1: The strict YAML subset (no anchors, aliases, merge keys, includes) is stated for descriptors only, while agent-authored memory entry and handoff front matter, heads.json and the TOML locks are parsed on the host with no limit beyond the 1 MiB file cap and are not in the fuzz list. | L2b | A | Apply as proposed. |
| **R8-03-10** (Required) 03 3.1, 07 7.5: Kit args keys and values reach sbxenv.yaml and sbx's kit build with no validation rule, and the domain rule is stated for egress.extra only while lock-derived and kit-declared domains also become sbx policy argv. | L2b | A | Apply as proposed. |
| **R8-03-11** (Required) 03 3.4, I25: Nothing forbids a secret value placed as an argv element, which the gate diff prints and sbx's process list and remembered host commands keep. | L2b | M | A validation rule in rules.go replaces the proposed doctor row; the token shapes are a best-effort guard and are stated as such. |
| **R8-03-12** (Required) 03 3.3, 03 3.4: The environment a host helper runs in is unstated: a secret argv runs under env -i with HOME and PATH=/usr/bin:/bin, and gitCredentialHelper ('the only credential helper hardened git uses') does not say which values are accepted or where it runs, so a manager needing a session variable or a non-system PATH fails far from its cause. | L10 | M | State the environment and the accepted helper values as a limit, plus one doctor row that runs each argv once. Regression check: the doctor row reads each helper's output only to test for emptiness and never retains, prints or writes it, stated beside I25 (D12, round 1). |
| **R8-03-13** (Required) 03 3.1, 03 3.4, 01 1.1, 07 7.3: The forge scope is unstated: the repo url rule allows only `https://<host>/<owner>/<repo>` (no GitLab subgroups) while 03 3.4 invites any gitHosts entry with a caFile, a hedge with no trigger in a v1 whose host is github.com. | L10, L11 (high-signal) | L | State the limit and defer caFile with a trigger; no widening of the URL rule. Regression check: the caFile part is dropped; caFile has a present consumer, the E2E origins over httptest TLS (te C1, round 1), so R8-05-21's form stands. The forge-scope limit stays. |
| **R8-03-14** (Required) 03 3.5, 07 7.6, 12 12.8: The v1 egress catalog is built only from the author's private reference config repo's locks, so any other stack fails its first sync on an uncatalogued backend:tool, and no page tells a stranger to expect it. | L10 | L | State the coverage limit and the existing way through (egress.tools); defer a public seed with a trigger; no new journey. |
| **R8-03-15** (Required) 03 3.2: Every project spec repeats the product kits, workload digest, agent and personal kits, each widening and recreate-class, so a workload or personal-kit bump costs one PR edit, one approval and one recreate per project. | L1 | M | Deferred with a trigger, per ADR 0005; no defaults file in v1. |
| **R8-03-16** (Required) index S8, 00 0.5, 04 4.2, 02 2.1, 03 3.13: S8 and romeu run --timings justify each other: the flag exists only for the criterion and the criterion's evidence is the flag's output, with a schema (run-timings.v1), a CI median-of-5 test and probe B5 measuring budgets of unstated origin. | L11 | operator | Operator decision needed: It removes a row of the 00 0.5 scope table (--timings, justified by S8) and changes a success criterion. Recommendation: accept; it also settles the run-timings.v1 part of R8-03-5. Regression check: see SC3; op-s8-limit's julieta assertion stays either way. Decision SC3. |
| **R8-03-17** (Required) 03 3.2 (Run layout), index: Deferred decisions: The run layout is called multiplexer-neutral and carries renderer neutrality for tmux and cmux, which are non-goals, while nothing says what happens if herdr, the one renderer and a pre-1.0 project, stops being maintained. | L11 | M | Delete the neutrality claim and carry it as a Deferred row; the format itself is unchanged. |
| **R8-03-18** (Required) 03 3.1, 02 2.3, 05 5.1, I3, I24, 08 8.1, 04 4.2, 04 4.3, 04 4.4, 07 7.3, 07 7.4, 13 13.4: Five lists that two or more consumers must agree on (dangerous git keys, memory layout allowlist, mise environment, ingesting commands, reserved names) are hand-written in two or three places each with no single home. | L3 | M | One home per list with links; a test only for the git-key pair shared by two code consumers; no generators. |
| **R8-03-19** (Required) 01 1.6, 03 3.8: The candidate machine has six states but the project record persists only awaiting, without saying which states live in process memory, and the generation machine has no row for stop, pull, retire or salvage finding the sandbox absent. | L3 | M | One sentence each instead of four new table rows. Regression check: aligned with R8-01-22; 03 3.8 links the 01 1.6 sentence instead of stating it a second time. |
| **R8-03-20** (Required) 03 3.8, 02 2.4: generations[] grows by one element per recreate inside a file rewritten whole on every mutating command, with no bound and no compaction. | L3 | L | State the growth as a limit and defer compaction with a measurable trigger. |
| **R8-03-21** (Required) 06 6.6, 03 3.4, index: Deferred decisions: The kit publishing path is a deferral written as six rows of design in 06 6.6, including a host-settings key kits.source: published that 03 3.4 does not have, with a trigger (a maintainer decision) that ADR 0005 rule 3 would not accept. | L3 | M | Delete the design rows rather than moving them to a Proposed ADR, so no decision record is created. |
| **R8-03-22** (Required) 03 3.6, 02 2.3, 08 8.5: Snapshot and salvage bundles are thin against repos[].base, and on the host those prerequisites are held only by refs/romeu/origin, which a later fetch force-moves and gc can prune. | L4 | A | Apply as proposed. |
| **R8-03-23** (Required) 03 3.6, 10 10.7, 12 12.2: The manifest is the one type both binaries marshal and decode, and the spec names no Go package for it, so two agents in different phases can define two types. | L12 | A | Apply as proposed. |
| **R8-03-24** (Required) 03 3.3, 11 11.4: The sbxenv.yaml goldens are probe-confirmed by raw sha256 at block A, before render exists, so any byte the marshaller emits differently from the hand-written golden forces a two-host re-probe. | L12 | A | Apply as proposed. |
| **R8-03-25** (Advisory) 03 3.3, 11 11.4: The goldens are called probe-confirmed while 11 11.4 does not flag a golden with no recorded hash, so a golden added after block A is neither confirmed nor flagged. | L6 | A | Apply as proposed. Regression check: the flag is a non-failing notice; a failing flag would reopen the red-until-block-B defect R3-newcomer-01 closed (round 6). |
| **R8-03-26** (Advisory) 03 3.5, 03 3.6, 03 3.9: The manifest's 32 KiB cap has no stated check point or outcome, and 03 has two smaller explicitness gaps: a domain without a domains entry has two rules (treated as upload: true, and refused by CI), and the lesson tag switches behaviour in three homes with no list of tags that have behaviour. | L0, L3 (high-signal) | A | Apply as proposed. |
| **R8-03-27** (Advisory) 03 3.9: The memory entry digest is {title, body, created} and memory verify compares status only, so an entry whose tags (lesson among them) were stripped verifies equal. | L0 | A | Apply as proposed. |
| **R8-03-28** (Advisory) 03 3.9, 09 J12: Memory has an import format and a verify command but no export, so moving memory between machines has no defined round trip. | L16 | L | State the copy as the move; no export command. |
| **R8-03-29** (Advisory) 03 3.2: 03 3.2 says there is no field for an env var for the sandbox a few lines from panes[].env, and the index's telemetry Deferred row relies on a pane env, so the distinction between a pane process and the sandbox is unwritten. | L13, L13b (high-signal) | A | Apply as proposed. |
| **R8-03-30** (Advisory) 03 3.8, 08 8.5, 13 13.4: salvage[].reasons holds skipped items, which decide result, and ledger-incomplete, which changes no result, so one field has two meanings and three sections each explain the exception. | L13b | A | Apply as proposed. |
| **R8-03-31** (Advisory) 03 3.8, 08 8.1, 08 8.4, 13 13.5: Atomic writes are temp, fsync and rename without a directory fsync in 03 3.8, temp and rename with no fsync in 08 8.1, and only 'written atomically' in 08 8.4. | L4 | A | Apply as proposed. |
| **R8-03-32** (Advisory) 03 3.12: 03 3.12 lists eleven digest kinds on the ask-first digests surface without saying whether the table is closed and lands whole with internal/canon or grows per consumer. | L12 | A | Apply as proposed. |
| **R8-03-33** (Advisory) 03 3.2: The example spec's $schema points at a raw.githubusercontent.com URL addressed by a tag that can move. | L7 | L | State the hint's role as a limit; keep the readable tag URL. |
| **R8-03-34** (Advisory) 03 3.4, 03 3.7, 01 1.1, 02 2.2: The examples of the adopter's own files carry the author's instance (config project verona, Keychain service romeu/verona/github, root $HOME/dev), and no page lists the values an adopter must replace. | L10 | A | Apply as proposed. |
| **R8-03-35** (Advisory) 03 3.1, 03 3.2, 05 5.4: sandboxOptions (cpus, memory) reaches a host resource with no gate and no bound, so a merged spec can claim the host's whole memory without a prompt. | L2 | operator | Operator decision needed: The fix is either an accepted risk in 05 5.4 or a change to the gate classification of 01 1.4 and I7; both need the operator. Recommendation: the 05 5.4 row. Decision AR3. |
| **R8-03-36** (Advisory) 01 1.4 (Gate 1), 03 3.8, 04 4.2: Gate 1, a per-machine toolchain acknowledgement with its own record, diff, exit code and flag, guards against an sbx or romeu upgrade on a machine whose one operator is the one who upgrades, and the threat it stops beyond doctor is unstated. | L11 | L | State the threat; folding gate 1 into doctor would remove a part of I7 and is not proposed. See SC8. |
| **R8-03-37** (Advisory) 03 3.11, 08 8.5: The salvage manifest's agent-chosen paths (bundles[].file, files[].path, worktrees[].path, embedded-repo path) have no rule beyond os.Root, and the bytes romeu hashes for files[] are unbounded. | L2b | A | Apply as proposed. |
| **R8-04-1** (Blocking) 04 4.2 (How romeu run reaches the run layout, step 3): Run step 3 specifies only the absent sandbox; a present sandbox that is stopped, or whose live env file carries a promoted recreate-class change, has no stated behavior and no block A probe. | L0, L1, L13 (high-signal) | A | Apply as proposed: three cases in run step 3, the digest check moved before the start, refusal with exit 5 on a live recreate-class change, one block A probe covering both the stopped start and the changed env file, and J3a cites the step. This also gives run its exit 5 (R8-04-31). |
| **R8-04-2** (Blocking) 04 4.1 (Output, Error ids); 04 4.2 (doctor): Nothing produces evidence a user can paste into an issue: subprocess failure details, the upstream versions in use and the step that failed are unspecified, and no output is declared safe to share. | L14, L15 (high-signal) | M | Smaller form: no romeu report command and no generalized --timings (L14-15 conflicts with R8-04-17, which proposes dropping --timings). Subprocess error details gain argv, status, a bounded stderr excerpt, the step and the versions; romeu version --json gains the upstream versions; 05 declares both safe to share. |
| **R8-04-3** (Blocking) 00 0.5; 04 4.3 (memory import, memory verify); 08 8.1; 03 3.9; 03 3.12; 10 10.5; 11 11.2; I24: The generic product carries one maintainer's one-time migration and acceptance: julieta memory import and verify, the import/ directory, the memory-entry digest kind, the repo evidence kind, tools/ci acceptance --file and the C1 hugo check. | L11 | operator | Needs the operator: 00 0.5 records the maintainer's decision that pre-v1 memory entries are imported at adoption, so moving the import out of the product changes that scope record. Recommendation: move it to the operator's config repo as a script and delete the six items. Regression check: reverses op-migration-set (round 6) and says so. Decision SC2. |
| **R8-04-4** (Blocking) 04 4.1 (Error ids, fix hints): Fix hints are under no check and one id covers situations that need different fixes: rule 13 does not match the commands a hint names, gate 1 and gate 2 share RJ-301, exit 4 covers drift and a non-verifying promotion with one hint, and a hint may suggest a data-discarding flag without saying so. | L1, L8 (high-signal) | M | Apply the hint rule and the two splits; the rule 13 matcher gains the error table and help examples as inputs, which is an input of the existing check, so ADR 0001 needs no change. Regression check: the RJ-301 split moves a cited id (dd N3, round 2); the citations are updated in the same write pass. |
| **R8-04-5** (Required) 04 4.1 (Exit codes): Exit 1 means both 'operation failed' and 'the check found what it looks for' (doctor, spec validate --catalog, lock --check, memory verify, pin check, memory import), so program callers cannot tell an infrastructure failure from a finding. | L13, L13b (high-signal) | A | Apply as proposed: a distinct exit 6 for check findings, so exit 1 keeps one meaning. Two independent readers (L13, L13b) found it. |
| **R8-04-6** (Required) 04 4.1 (Output); 04 4.3: The Output rule promises --json, with a schema in schemas/, on every read command, and several read commands have neither (handoff show, memory check, memory verify, lock --check, pin check, spec validate; no schemas for status, doctor, ledger query, handoff, event list). | L3, L13, L13b (high-signal) | M | Narrow the rule to commands a program calls and make the rows and the schema list match it; add --json only where a program is the caller. |
| **R8-04-7** (Required) 04 4.2 (romeu approve); J11 step 5: The `approve <name>` row says it makes no sbx call while its Reads cell names live sbx version for gate 1, and its exit codes omit 3, which 01 1.4 requires of every command that invokes sbx. | L13, L13b (high-signal) | A | Apply as proposed. Two independent readers (L13, L13b) found it. Follows SC5. |
| **R8-04-8** (Required) 04 4.1 (Error ids): The error-id scheme has no integrity rules: ids go to whichever task writes first, nothing checks uniqueness or that each id is raised by a test, nn has two digits, and nothing says an id is never reused or that a move to another exit keeps it, though ids, command names and probe ids are ledger table members (13 13.7). | L3, L6, L8, L12 (high-signal) | A | Apply as proposed; the single sequence replaces a per-range allocation table, and the existing sequences check gives uniqueness without a new tool. Regression check: supersedes craft C7's `RJ-<exit><nn>` scheme (round 2); the cited ids (RJ-203, RJ-204, RJ-301) are updated in the same write pass. |
| **R8-04-9** (Required) 04 4.1 (Error ids); 04 4.2 (doctor): The failures field reports will be about, and the doctor checks and warnings a reader sees, have no id or slug: unparsed sbx output, subprocess timeout, registry unreachable, kit build failure, the three compatibility-check causes, disk full, each doctor check and each warning. | L8, L14 (high-signal) | M | Smaller form: the one existing table and its generated page take the doctor checks and warnings as rows; no sibling table, no new generator, no free-space doctor row. |
| **R8-04-10** (Required) 04 4.2 (How romeu run reaches the run layout, step 2; romeu status): When run closes a generation as lost it tells the user nothing, and neither run nor status reports the work at risk (snapshot age, the newest facts' dirty and stash counts), so a lost sandbox is discovered from missing work. | L1, L4 (high-signal) | M | Print the loss block and record the counts; no TTY prompt (run continues as today) and no snapshot-age doctor warning. |
| **R8-04-11** (Required) 04 4.2 (doctor, VS Code row); 01 1.3: The editor-trust check covers less than the threat model leans on: only VS Code is checked and the spec does not say so, and within VS Code a single folder under $ROMEU_ROOT trusted by itself passes. | L2, L10 (high-signal) | A | Apply as proposed, without the extra warn row for a machine with no VS Code settings. |
| **R8-04-12** (Required) 04 4.2 (rm, recreate, retire): The data-discarding flags accept too much without naming it: --accept-loss needs no TTY and accepts every reason at once, retire --force both allows an in-spec project and skips the salvage-only confirmation, and retire's incomplete inner salvage is unstated. | L4 | A | Apply as proposed; a named-reason flag replaces both the blanket flag and the --force overload without adding a TTY prompt. |
| **R8-04-13** (Required) 04 4.2; 03 3.8; 08 8.1: Retained state grows with no bound, measurement or deferred cleanup: generations[] with a salvage[] per generation in a record rewritten whole on every command, and salvage payloads, salvage refs and .attic on disk. | L3, L4 (high-signal) | L | State it as a limit with a measured trigger: a Deferred row plus one doctor warn row; no attic for generations now. |
| **R8-04-14** (Required) 04 4.3 (pin workload, pin check); J9; 02 2.2: julieta pin workload and pin check reach the container registry from the config project's sandbox, and the config project's derived egress names no registry host, so J9's pin step is blocked unless the spec adds the host by hand, which no page says. | L0 | M | State the egress.extra requirement in J9 and 02 2.2; no new derivation rule. |
| **R8-04-15** (Required) 04 4.2 (romeu run, step 8): There is no supported way into the sandbox when the run layout fails (for example a herdr protocol mismatch at the pre-1.0 pin), and the user does not know the sbx env exec argv. | L1 | M | Print the shell argv as the fix hint instead of adding romeu shell or run --shell. |
| **R8-04-16** (Required) 04 4.2 (run step 3); 11 A5; 03 3.3: The spec never states what sbx reads from the workspace tree on sbx env run --clone, so a repository-local sandbox configuration an agent pushed and the operator pulled could configure the sandbox outside gate 2; no probe plants one. | L2 | A | Apply as proposed. |
| **R8-04-17** (Required) index S8; 00 0.5; 04 4.2 (run --timings); 02 2.1; 03 3.13: S8 and run --timings justify each other: the flag, run-timings.v1, a median-of-5 CI test and probe B5 exist to measure a budget whose 2 s and 3 s figures have no stated origin. | L11 | operator | Needs the operator: S8 and 00 0.5 are scope. Recommendation: drop the flag and keep S8 as a harness measurement whose bound the first measurement sets. R8-04-2 assumes this (it does not generalize --timings). Regression check: see SC3; op-s8-limit's julieta assertion stays either way. Decision SC3. |
| **R8-04-18** (Required) 04 4.2 (pull); J4; 01 1.2; 02 2.3; I13; I14; I19; 05 5.4: romeu pull, the hardened review checkout, both VS Code workspace files, I19 and two residual-risk rows exist for a host-side review path the start does not need, and dev.code-workspace is opened by no journey. | L11 | operator | Needs the operator: it removes a v1 command from scope (00 0.5) and two accepted-risk rows of 05 5.4. Recommendation: defer pull with the stated trigger and delete dev.code-workspace. Decision SC4. |
| **R8-04-19** (Required) 01 1.6 (Candidate); 04 4.2 (approve); J11 step 5: The awaiting state, the superseded transition and `romeu approve <name>` exist for a caller that runs romeu sync without a terminal, and the spec names no such caller. | L11 | operator | Needs the operator: removing `approve <name>` changes the v1 command set (00 0.5). Recommendation: delete and defer with the stated trigger; R8-04-7, the approve wording fix, then shrinks to approve --toolchain. Regression check: supersedes craft C2 (round 2); R8-01-31 and R8-04-7 follow the outcome. Decision SC5. |
| **R8-04-20** (Required) 00 0.5; 04 4.2; 04 4.3: The scope table claims every v1 feature traces to a need, yet lists six of sixteen romeu commands and justifies julieta handoff list only by 'kept in v1 by the maintainer'. | L11 | operator | Needs the operator: the table is the scope record (00 0.5) and some rows are maintainer decisions. Recommendation: one row per command, checked; remove handoff list unless the operator names its need. Regression check: the deletion branch reverses op-handoff-list (round 6) and says so. Decision SC6. |
| **R8-04-21** (Required) 04 4.2 (doctor); 05 5.4; ADR 0008: doctor measures every credential path except the GitHub token the sandbox holds, whose reach 05 5.4 and ADR 0008 call inferred though its type and scopes are readable. | L2b | operator | Needs the operator: it changes an accepted-risk row of 05 5.4 and the measured-or-inferred statement of ADR 0008, and the doctor row sends the secret to the GitHub API from the host. Recommendation: add the probe now and make the doctor row a warn. Regression check: the doctor row as proposed has romeu read and transmit a secret value, against I25 and boundary B (D12, round 1); the recommendation is amended to the block A probe, run from the maintainer's own session, and no doctor row. Decision AR18. |
| **R8-04-22** (Required) 05 5.1; I3; 04 4.2 (doctor git row); I24; 08 8.1; 04 4.3 (mise env); 07 7.3; 07 7.4; 04 4.4; 13 13.4; 02 2.3; 03 3.1: Five lists that two or more consumers must agree on are hand-written in two or three places with different memberships and no test: the dangerous git keys, the memory layout allowlist, the mise environment, the commands that ingest, and the reserved names. | L3 | M | Smaller form: one home per list and links elsewhere; one test for the git-key list, whose three consumers are code; no generator rows. The ingest-caller list is also covered by R8-04-23. |
| **R8-04-23** (Required) 04 4.1; 04 4.2; 04 4.3; 13 13.3; 13 13.4; 08 8.4; 12 12.3: The cross-cutting side effects of a command (preflight, lock, ingest, drain, snapshot, appending) are hand lists in five sections that a new command must remember to join, and the meta-test reads the same hand list. | L3 | A | Apply as proposed: one source in the data replaces three hand lists. |
| **R8-04-24** (Required) 04 4.3 (hooks run); 08 8.2: The hook dispatcher stops at the first non-zero stage and prints nothing saying which stage failed, and a normal refusal stays listed as a hook failure at every SessionStart until the same hook next succeeds. | L5 | M | The page consolidator proposed A: Apply as proposed. Amended by the regression check: the dispatcher's stage line only; the SessionStart half is dropped, since 'since the last narrative handoff' would be a second rule for when a failure stops being shown, which R3-doubt-20 (round 6) removed. The surviving rule is 'until the same hook next succeeds'. |
| **R8-04-25** (Required) 04 4.3 (julieta status); 03 3.6; 07 7.5: No julieta command shows the effective egress allowlist, so a blocked request reaches the agent only as sbx's proxy 403 and the agent cannot tell an unknown host from a gated one. | L5 | M | Smaller form: the manifest carries what was applied, not the pending gated set, which lives on the host and would be stale in the sandbox. |
| **R8-04-26** (Required) 04 4.1 (Idempotency, Output): The appending julieta commands (memory add, handoff write, event add) print no defined success output and are not safe to retry, so an agent whose tool call timed out cannot tell whether the write landed. | L5 | M | Print the written id and path and dedupe memory add by content digest; no --key mechanism. |
| **R8-04-27** (Required) 04 4.2 (recreate); J6: recreate removes the old sandbox before anything checks the new one can be built, and the spec does not describe the state or recovery when an upstream artifact (workload digest, frontend digest, kit download host) is gone. | L15 | L | State the limit and the recovery; the salvage before removal already makes the gap lossless. |
| **R8-04-28** (Required) 04 4.3 (pin check); index Deferred decisions: The only freshness mechanism, pin check, covers spec workloads and the julieta release and nothing schedules it; the kit frontend, herdr, mise, gh, the action SHAs and the reuse image have no staleness signal and no Deferred row. | L15 | L | State it as a limit with an observable trigger, as ADR 0005 asks of an open hedge. See SC9. |
| **R8-04-29** (Advisory) 04 4.3 (julieta salvage); 04 4.2 (romeu salvage): julieta salvage takes an optional --stop-agents, J6 passes it for rm and recreate, and no row says whether a standalone romeu salvage passes it. | L0 | A | Apply as proposed. |
| **R8-04-30** (Advisory) 04 4.1 (Locking): romeu.lock is machine-wide and held through sbx env run, so creating one project makes romeu run of another exit 1 with RJ-101 although they share no state. | L0 | L | Name the serialization as a chosen cost; no per-project lock. |
| **R8-04-31** (Advisory) 04 4.2 (romeu run, exit codes): romeu run lists exit codes 0-5 and no step of run exits 5. | L0 | A | Apply together with R8-04-1. |
| **R8-04-32** (Advisory) 04 4.2 (romeu run, step 1): romeu run never compares the config repo's last fetched head with the synced spec commit, so a merged spec change is silently not applied until the next sync. | L1 | A | Apply as proposed. |
| **R8-04-33** (Advisory) 04 4.2; 12 12.8: No command or journey says how to remove the product from a machine (settings, state, the ledger, the attic, the sbx rules romeu applied). | L10 | M | A guide section instead of a new journey J14. |
| **R8-04-34** (Advisory) 01 1.4 (Gate 1); 03 3.8; 04 4.2 (approve --toolchain): Gate 1, a per-machine toolchain acknowledgement with its own record, diff printer, exit code and flag, guards against an upgrade behind the operator's back on a machine where the operator is the one who upgrades. | L11 | operator | Needs the operator: it removes an approval gate and a command from v1 (00 0.5). Recommendation: keep gate 1 and add one sentence to 01 1.4 naming the threat it stops (an sbx or romeu binary replaced by another process or person on the machine), since folding it into sync weakens a security gate. Regression check: the recommended branch keeps D3, sa M5, sa N4 and te N10 (rounds 1 and 2) intact; only the fold-into-sync branch would remove them. Decision SC8. |
| **R8-04-35** (Advisory) 04 4.3 (pin workload, pin check); 06 6.2: pin workload and pin check are two sandbox commands reading a registry and GitHub releases for a rare manual act the first user does once. | L11 | operator | Needs the operator: it removes a v1 command (00 0.5). Recommendation: keep pin workload with its stated reason, and keep pin check, since R8-04-28 leans on it as the existing freshness signal. Regression check: deferring pin check would remove D27 (round 1) and conflict with R8-02-4 and R8-04-28; the recommendation keeps it. Decision SC9. |
| **R8-04-36** (Advisory) 04 4.3 (lock); 07 7.3; 03 3.4 (caFile); 05 5.1: Several hedges carry no trigger: julieta lock locks macOS platforms the product never runs mise on, monorepo.lockfile serves a mode no v1 repo uses, and gitHosts caFile serves private git hosts in a v1 whose only host is github.com. | L11 | M | Apply the lock and caFile parts; drop the monorepo sentence only after checking in 07 7.3 that no v1 repo sets it. Regression check: the caFile part is dropped (te C1, round 1; R8-05-21's form stands); the two-platform lock default supersedes the macOS warning of R3-simplicity-06 (round 6), recorded as such. |
| **R8-04-37** (Advisory) 04 4.3 (memory); 08 8.1: Of the six memory subcommands, search duplicates list with a filter and rm is a delete path in a product that otherwise never deletes, while edit --status done already closes an entry. | L11 | operator | Needs the operator: it removes commands from the v1 set (00 0.5). Recommendation: drop rm and fold search into list. Decision SC10. |
| **R8-04-38** (Advisory) 04 4.4: 4.4 is a per-subsystem copy, by slug, of rows of the one error table, although it says the ledger has no catalog of its own, and it invites a 4.5. | L3 | A | Apply as proposed; keep a one-line 4.4 pointer if any page cites the anchor (check the citers when applying). |
| **R8-04-39** (Advisory) 07 7.5; 13 13.2; 04 4.2; 01 1.6: The escape hatches (egress.tools, egress.extra, tool: other, --accept-loss, --overwrite-drift, retire --force) are each bounded but none is counted, so nobody sees when one becomes the main road. | L3 | L | State the limit with a trigger; the ledger query already answers one of the counts. |
| **R8-04-40** (Advisory) 04 4.2 (doctor): doctor compares derived files and the ledger view but not the project record with the facts sbx owns (generations against sbx ls, egressApplied against sbx policy ls). | L4 | A | Apply as proposed; the salvage-refs check of L4-11 is consolidated with its own page. |
| **R8-04-41** (Advisory) 04 4.1 (Subprocesses); 08 8.5: The timeout of the salvage exec, which moves up to a GiB over virtiofs, and what a timeout leaves behind are unstated. | L4 | A | Apply as proposed. |
| **R8-04-42** (Advisory) 04 4.2 (romeu stop): romeu stop runs julieta snapshot --all then sbx stop with no rule for a failed snapshot. | L4 | A | Apply as proposed. |
| **R8-04-43** (Advisory) 04 4.3 (julieta setup): julieta setup fast-forwards a clean checked-out default branch, and a second romeu run on a running sandbox reruns setup while the agent works, moving its HEAD with the report only on the host terminal. | L5 | M | Record the move as a warning; no detection of running agent processes. |
| **R8-04-44** (Advisory) 04 4.2 (doctor); 10 10.1: doctor warn rows change no exit code and no test asserts them on output, so a warn that stops firing is invisible. | L6 | A | Apply as proposed; with R8-04-9 each doctor row has a slug to assert. |
| **R8-04-45** (Advisory) 04 4.3 (lock --check); 07 7.3: lock --check checks that lock entries cover the linux platforms, not that a lockable entry carries a checksum, so a checksumless entry in a worked-on repo is never refused or listed. | L7 | A | Apply as proposed. |
| **R8-04-46** (Advisory) 04 4.3 (mise environment); 11: The mise environment julieta sets is a set of upstream variable names that no test or probe shows still take effect at the pinned mise. | L15 | M | Test only the two variables that carry security weight. |
| **R8-04-47** (Advisory) 04 4.2 (doctor, root row); 12 12.8: doctor refuses a root inside a git repo or equal to $HOME, but not one inside a synced folder, where salvage payloads and transcripts would leave the machine. | L16 | M | Check only paths readable without guessing iCloud state; the ~/Documents case is a README sentence. |
| **R8-05-1** (Blocking) 05 5.4: The Release signing paragraph names an operator-held credential that signs release artifacts, but no page says what is signed, how, where it is published or how a user verifies it; S1, J1 step 2, B1 and 10 10.2 know only checksums.txt and the provenance attestation, and S1 calls the attestation the signature. | L0, L2, L2b, L3, L7, L9, L10, L13b (high-signal) | operator | Recommend (a): delete the Release signing paragraph, state that the attestation is the only release proof and what it does not prove, add the deferred row with its trigger, and reword ADR 0008 decision 6 in a short superseding record. (b) adds a key to keep and an operator step per release for a control no second user needs yet. Decision DR2. |
| **R8-05-2** (Blocking) 05 5.3; 05 5.4; 10 10.2; 12 12.4; 12 12.9: The default-branch ruleset is said to require a code-owner review in 05 5.3, 05 5.4, 10 10.2 item a, 12 12.4 and 12 12.9 while the same cells say that, as measured on 2026-10-08, it requires none; the spec describes the removed gate several times and the gate that exists nowhere. | L0, L2, L3, L13, L13b (high-signal) | M | Apply the spec-text change as written; leave ADR 0001 rule 8 and [ADR 0003, adopt six Extreme Programming practices and review as pairing](../adr/0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md) untouched in this pass and list "mark ADR 0001 rule 8 and ADR 0003 superseded in part by ADR 0008 for the review requirement" as an operator item, since it changes accepted decision records. Regression check: the superseding record names op-ruleset-approvals (round 6) and N1 (round 3) as superseded in part by the measurement of ADR 0008. Decision DR1. |
| **R8-05-3** (Required) 05 5.3: internal/sbxdrv (the only code that runs sbx, holding the I4 and I25 guards), e2e/fakesbx and skills/** sit on no ask-first surface, while gitsafe is a surface for the same reason on the git side, and tools/ci mutate never runs on a PR that touches only those guards. | L0, L2, L2b, L13 (high-signal) | M | Apply in the smaller form: one new sbxdrv surface and skills/** on the kits surface, plus the mutate trigger on guard tags instead of the readers and drivers surfaces of L2b-3, so more guards are tested without more approval lines. |
| **R8-05-4** (Required) 05 5.3; 12 12.4: The contracts and checks surfaces put an approval line in the maintainer's words on most PRs and no form approves a class of change or a phase, so the maintainer is the queue on the critical path. | L9, L12 (high-signal) | operator | Recommend the checkpoint grant for contracts, checks, dependencies, kits, catalog, ledger, digests and termsafe only; defer narrowing contracts to versions.go until the RJ uniqueness check exists. Regression check: names N2 (round 3), whose one approval-line form this adds a second to. Decision AR14. |
| **R8-05-5** (Blocking) 05 5.4: Text from strangers on GitHub (issues, PR bodies, comments, diffs) reaches agents acting with the operator's admin token, ADR 0008 names that class, and 05 has no residual-risk row, no threat-model sentence and no rule bounding what an agent does with it. | L5, L14 (high-signal) | operator | Recommend the row, the 12 12.9 working rule and the earlier trigger as written. Decision AR4. |
| **R8-05-6** (Advisory) 05 5.4: The token row accepts merges, v* tags and ruleset edits as process only, and the spec does not weigh the cheap deterministic guard against mistakes: a julieta-claude PreToolUse refusal of gh pr merge, a v* tag push and ruleset API calls. | L5 | operator | Recommend it: it moves a rule held in agent goodwill into a deterministic hook, and states its limit. Decision AR5. |
| **R8-05-7** (Required) 05 5.4; index: Deferred decisions; ADR 0008: The token row rests on two duties held in memory (reading the security log at each sitting, a review every 2 autonomous work sessions that nobody counts), and its release-candidate trigger makes the token decision due just before block B. | L9 | operator | Recommend the counted-PR trigger and the C7 checkpoint as written. Decision DR5. |
| **R8-05-8** (Required) 05 5.4: The residual-risk table every user reads mixes the risks of running romeu with the maintainer's own development sandbox: rows 2 and 3 say "the token is today the operator's own (ADR 0008)" while the product's token is whatever name@project credential the operator binds (03 3.4, Q13) and 06 6.4 forwards one key. | L0, L10 (high-signal) | operator | Recommend the split as written; it makes no acceptance wider or narrower, it only names whose risk each row is. Decision AR6. |
| **R8-05-9** (Required) 05 5.3; 05 5.4; 01 1.7: One person is "operator" in 01 1.1, 05 5.4 and 09, "maintainer" in 05 5.3, 10 10.2 and 12 12.9, and "developer" or "user" in 00 0.2; none of the words is in the vocabulary, and the one sentence that relates them is in ADR 0008 Context. | L3 | A | Apply as written. |
| **R8-05-10** (Required) 05 5.4; 10 10.2; index (Status): History sits in the current-state spec: 05 5.4 carries dated measurements and the paragraph "Round 4 recorded the acceptance of three rows", 10 10.2's maintainer block is a dated procedure, and the index status paragraph grows per round, against 12 12.5. | L3 | M | Apply the moves; leave the sequences cell-length bound out until a deferred row exceeds a measured length (deferral). Regression check: in R8-10-18's form; the maintainer block keeps the to-do items of round 7 (item e, the host checks) and moves only the done items and dated results. |
| **R8-05-11** (Required) 05 5.4 (last paragraph); 13 13.8; ADR 0006: Two ledger residual risks and six starting values are recorded as accepted "with the design the maintainer accepted as a whole", so a reader cannot tell a considered acceptance from an inherited one. | L11 | operator | Recommend marking them "inherited, undecided" now and asking the maintainer for the eight decisions in the next grouped block. Decision AR7. |
| **R8-05-12** (Required) 05 5.4: The salvage row's mitigation says "a push to origin is the only copy an agent cannot take back", while ADR 0008 records that the sandbox's token can delete branches, tags and releases. | L4 | operator | Recommend the rewritten cell and the deferred row; no new ref namespace now. Decision AR8. |
| **R8-05-13** (Required) 05 5.4: The signature row lists "confirm-on-use where the agent supports it" as a mitigation that no probe measures, doctor cannot read and no setting controls, and the forwarding of the signing key has no deferred row with a trigger. | L2 | operator | Recommend the deletion and the deferred row; the doctor line is optional. Regression check: reconciled with R8-06-13; ssh-add -c (confirm-on-use) stays as the guide step of 06 6.4 and the mitigation points at it instead of being deleted (sa H5, round 1). Decision AR9. |
| **R8-05-14** (Required) 05 5.4: The memory-planting row tells the owner to read memory only through romeu handoff, which shows only handoffs, so there is no safe way to read the lessons the agent recorded. | L16 | operator | Recommend the smaller form and the deferred row; the command waits for its trigger. Regression check: the pager clause loosens D8 (round 1: the host reads memory only through romeu handoff); the operator decides it by name. Decision AR10. |
| **R8-05-15** (Advisory) 05; 05 5.4: The threat model lists GitHub and web text as steering but not the product's own memory: a lesson entry and the latest handoff are printed into every later session by SessionStart (08 8.2), and romeu handoff prints agent prose to the operator, a path from agent text to a host action that escaping cannot stop. | L2 | operator | Recommend one merged row for both channels. Decision AR11. |
| **R8-05-16** (Required) 05 5.4; 10 10.2; 00 0.1: The maintainer runs agent-authored code on the host by design (the pre-push hook runs tools/ci fast; blocks O3 and O6 run tools/ci on a host clone after reading the diff), and 05 5.4 has no row for it. | L2b | operator | Recommend the row as written; leave a trusted hooksPath checkout as a deferred row reopened by the first tools/ci change the maintainer did not read before a host run. Decision AR12. |
| **R8-05-17** (Required) 05 5.3; 10 10.2: CI and the hook run the tools/ci of the branch under review, so a change to the gate is judged by itself, and no mechanism or deferred row names the gap. | L6 | M | Apply as written: state the limit and defer the base-branch run with its trigger. |
| **R8-05-18** (Blocking) 05 5.1; 04 4.3; 08 8.5: 05 5.1 defines the hardened git environment for subprocesses started by romeu only, while the index routes every git call through gitsafe and 10 10.7 links it into julieta; julieta's clone, fetch, fast-forward, bundle and commit-tree calls run under an environment no page states. | L13 | A | Apply as written; one runner with two named modes, not a second runner. |
| **R8-05-19** (Advisory) 05 5.1; I3: The insteadOf paragraph omits a local rewrite to git://127.0.0.1, the one git:// target gitsafe allows, core.gitProxy is not neutralized, and the I3 planted list omits both. | L2b | M | Apply the flag, the sentence and the plant rows; leave out the doctor row that reads each host clone's local config, since 05 5.1 trusts local config. |
| **R8-05-20** (Required) 05 5.1; I3; I24; 04 4.2: Lists that two consumers must agree on are hand-written in several places with different memberships: the dangerous git keys (05 5.1 -c list, I3 plant list, the doctor warn row), the memory layout allowlist, the mise environment, the ingesting commands and the reserved names. | L3 | M | Apply the one-home edits and the one test; no generator rows until a second divergence is measured. |
| **R8-05-21** (Advisory) 05 5.1; 03 3.4; 04 4.3; 07 7.3: Several hedges carry no trigger: julieta lock locks macOS platforms, monorepo.lockfile serves an unused mode, and gitHosts caFile with its unit test serves private git hosts in a v1 whose only host is github.com. | L11 | L | caFile has a present consumer (10 10.1); state it in 05 5.1 and change nothing else on this page. |
| **R8-05-22** (Advisory) 05 5.1; 08 8.5: The salvage cross-check and pull assume the sandbox daemon is git://127.0.0.1, and no page names a change of that transport by sbx as a break mode with a recovery. | L15 | A | Apply as written. |
| **R8-05-23** (Advisory) 05 5.2: Several invariant rows claim more or less than their tests measure: I3 and I7 enumerate nine and six of fifteen commands, I24 forbids archive extraction while salvage unbundles, I23 is universal over four tools, I30 says julieta runs only as delivered while 06 6.3 proves compatibility only, I2 cites an unlisted key set, I16's level disagrees with 10 10.1, and I18 mixes julieta's and romeu's halves. | L0, L2, L13b (high-signal) | M | Apply the cell edits as written. |
| **R8-05-24** (Blocking) 05 5.2; S6: S6's mutation check accepts any stub that makes a tagged test fail, one stub per invariant id, so a stub that panics meets S6 while the guarded decision is observed by no test. | L6 | M | Apply the stub definition and the assertion rule; defer per-site stubs with the trigger. |
| **R8-05-25** (Blocking) 05 5.2; 10 10.2: A guard deleted together with its test and both tags turns nothing red: tools/ci invariants pairs what remains and acceptance runs once. | L6 | M | Apply with the 05 5.2 table as the list instead of a second committed list. Regression check: the deleted-with-its-tags detection stays out of tools/ci all, as a diff-based rule in pr or in acceptance, so that all does not turn red while invariants land (R3-critic-04, round 6). |
| **R8-05-26** (Required) 05 5.2: The absence invariants I2, I3, I14 and I16 are tested by absence with no positive control, so an inert plant and a working guard look the same. | L6 | A | Apply as written. |
| **R8-05-27** (Blocking) 05 5.2 (I17); 10 10.1: I17 and 10 10.1 inject faults into the promotion commit only; no test kills romeu after each step of salvage, rm, recreate or retire, so "holds under a kill at any line" is unmeasured for the machine that protects work. | L4 | A | Apply as written. |
| **R8-05-28** (Required) 05 5.2 (I28); 06 6.1: I28 claims romeu parses descriptors the way sbx does, but the conformance fixtures are recorded once against one sbx and one frontend and nothing re-runs them when a pin moves. | L15 | A | Apply as written. |
| **R8-05-29** (Advisory) 05 5.2 (I16, I31, I32): The admitted-call-site table of the I16, I31 and I32 scans lives under tools/ci, so each package that legitimately removes a temp file edits the gate's code and needs a checks approval. | L12 | L | State the limit; a marker comment in the admitted function would let any PR admit itself without an approval. |
| **R8-05-30** (Advisory) 05 5.2 (I22); 04 doctor: Common pre-existing setups (a global sbx GitHub secret, VS Code workspace trust off, an existing ~/dev) are refused at preflight with no fix hint stated. | L1 | A | Apply as written in 04. |
| **R8-05-31** (Required) 05 5.4; 04 4.2; J4; 01 1.2; 02 2.3; I19: romeu pull, the hardened review checkout, both workspace files, I19 and two residual-risk rows exist for a host-side review path the start does not need, and dev.code-workspace is opened by no journey. | L11 | operator | Recommend the deferral with its trigger; it removes a command and two accepted-risk rows. Decision SC4. |
| **R8-05-32** (Required) 05 5.4 (herdr row); 03 3.2; Q16: The run layout is multiplexer-neutral for non-goal renderers while nothing says what happens if herdr, pre-1.0 with no signed releases, stops being maintained. | L11 | M | Apply the row; leave 03 3.2 as is. |
| **R8-05-33** (Advisory) 05 5.3 (signing); 06 6.4; 00 0.5: Commit signing from a dedicated host agent is a product kit with its own surface, setting, check and open question, and no sentence says why the start needs it. | L11 | M | Apply in 06 6.4 rather than 00 0.5, so the scope text does not change. See SC11. |
| **R8-05-34** (Advisory) 05 5.3: .github/workflows/release.yml matches the release and dependencies surfaces, so tools/ci pr asks for two approval lines and the overlap is named nowhere. | L13b | A | Apply as written. |
| **R8-05-35** (Advisory) 05 5.3 (dependencies); ADR 0007: The dependencies surface lists every path where mise reads its configuration as measured with mise 2026.10.3, and nothing re-measures mise's config discovery when the mise pin moves. | L15 | M | Apply the 05 5.3 sentence now; the ADR 0007 sentence goes to the operator with the next decisions batch. |
| **R8-05-36** (Advisory) 05 5.3; 12 12.8: Every ask-first surface has one owner and no contribution path or continuity statement is given for outside contributors. | L10 | M | Apply as written; no continuity promise beyond what one person can keep. |
| **R8-05-37** (Required) 05 5.4: The first mention of ADR 0008 in 05 is the bare "(ADR 0008)" in the second row of 5.4, before the linked mention, which rule 9 of ADR 0001 fails once its check lands in T086. | L8 | A | Apply as written. |
| **R8-05-38** (Required) 05 5.3; 01 1.7; index: docs/spec/** is on no ask-first surface while the spec is the plan's source of truth and tools/ci vocabulary and sequences read its tables, so gate inputs and the security model sit outside the gate. | L3 | operator | Recommend the three-file surface: it covers the check inputs and the security model without an approval on every spec edit. Decision AR13. |
| **R8-05-39** (Required) 05 (threat model): The threat paragraph trusts "the romeu release (built from reviewed, signed commits, with verified attestations)" while no review is required, commits are signed by the agent's forwarded key, and the attestation proves only that release.yml built the archive. | L7 | M | Apply the reword now; it holds under either outcome of the release-signing decision. |
| **R8-06-1** (Blocking) 06 6.1: Where a local kit is built and what its install steps and lifecycle hooks can reach (host side or microVM, network, filesystem) is stated nowhere and measured by no probe, although personal-kit install steps and hooks are agent-authored code admitted at gate 2. | L2, L2b, L10 (high-signal) | operator | The page consolidator proposed M: Apply the 06 6.1 Build sentence, the conditional refusal and the A14 observations; drop the proposed new 01 1.3 boundary row and the 05 5.4 residual row (the refusal leaves no residual until A14 says otherwise). The minimal personal-kit example under examples/kits/ waits for A11 and goes in as a pending item, not spec text. Routed to the operator by the regression check: the conditional refusal of install steps and lifecycle hooks in personal kits reverses the accepted cost of op-personal-kits (round 6: 'a personal kit can still run an install step; the gate diff is the control'). The 06 6.1 Build sentence and the A14 observations can be applied now; the refusal is the operator's call (AR23). Decision AR23. |
| **R8-06-2** (Required) 06 6.2: The workload's per-name volumes 'reattach on recreate', yet the spec never says what they hold, whether rm, recreate or retire removes them, whether salvage covers them, or how the owner deletes them, so 00 0.1's 'nothing sandbox-only is lost' and retire both have an unmarked exception. | L1, L4, L13, L16 (high-signal) | M | Apply the A12 observation and the 06 6.2 paragraph; write 'outside salvage, kept or removed as A12 measured' instead of promising a policy before the measurement. If A12 shows rm destroys agent state that matters, that loss goes to the operator as a 05 5.4 row then, not now. |
| **R8-06-3** (Required) 06 6.1: A11 is a go/no-go for every kit and for julieta delivery on a milestone-only frontend with 'no v2 fallback' (Q7), and no page says what the no-go state is or who acts on it. | L9, L12 (high-signal) | A | Apply the sentence to the Go/no-go row as written. |
| **R8-06-4** (Required) 06 6.2: A pin change (workload digest, kit frontend, host sbx) silently invalidates the probe facts and conformance fixtures recorded on the old pin (A11, A12, C3, C5, B2's S5 part), and no rule says which probes must run again. | L9, L15 (high-signal) | M | Apply the 11 11.4 table and the two 06 sentences; make the check one tools/ci probes rule (pin newer than recording fails) rather than a gate 2 printout, so the list lives in one place. |
| **R8-06-5** (Required) 06 6.6: The publishing path is a deferral designed in six rows outside the Deferred decisions table, with a trigger that is a maintainer decision rather than an observable event (ADR 0005 rule 3), a host-settings key kits.source absent from 03 3.4, and unsourced, undated Docker Hub limits. | L3, L8, L10, L11, L13b (high-signal) | M | Apply as stated; do not create a Proposed ADR for the six design rows (L3-23), since the design will be redone against the registries and sbx of the trigger's day. The renderer-interface half of L13b-27 is carried by R8-06-6. |
| **R8-06-6** (Required) 06 6.4: The run layout is built multiplexer-neutral with a renderer interface for non-goal multiplexers, while nothing says what happens if herdr, pre-1.0 and unsigned, stops being maintained. | L11 | M | Add the Deferred row and keep the neutral format because the row now justifies it; no change to 00 0.3 or 05 5.4. |
| **R8-06-7** (Required) 06 6.3: The compatibility check is under-defined: 'before the first call of a session' uses an undefined word and leaves out stop, salvage and pull, and the set of julieta protocols romeu accepts is defined on no page. | L7, L13b (high-signal) | M | Apply the 06 6.3, 03 3.6 and 04 4.2 edits and the warning; leave the separate 'session' vocabulary row and the 05 5.4 rewording of L13b-17 to pages 01 and 05, since 06 no longer uses the word. |
| **R8-06-8** (Required) 06 6.4: The skills and hook lines are julieta-claude kit content that changes only at recreate while the julieta binary updates at every release, so the agent can read a skill older than the julieta it describes, and no spec rule checks that a skill's commands exist. | L8 | A | Apply both sentences as written. |
| **R8-06-9** (Advisory) 06 6.4: Product kits carry one person's preferences with no stated intent and no escape: herdr's ctrl+b prefix (which collides with a host tmux) and julieta-claude replacing the agent's built-in memory. | L1, L10 (high-signal) | M | Document the double prefix and the override instead of adding a configurable prefix kit arg; state the memory replacement in 06 6.4 and 12 12.8. |
| **R8-06-10** (Required) 06 6.4: The agent's own memory dir is made non-writable, but nothing tells the agent, at the moment its habitual write fails, to use julieta memory instead. | L5 | A | Apply as written. |
| **R8-06-11** (Required) 06 6.4: Nothing checks at runtime that the SessionStart and SessionEnd hooks and the two skills are wired, and a personal kit installed after julieta-claude can replace them with no precedence rule. | L5 | M | Put the check in julieta status only (romeu status already reads it), not also in setup; the 06 6.5 sentence states the rule the check enforces. |
| **R8-06-12** (Required) 06 6.4: The signing rule (RJ-204, preflight step 5, doctor row) is an ask-first security rule with no invariant id, so it has no guard tag, test tag, mutate stub or row in S6's acceptance check. | L6 | A | Apply as written. |
| **R8-06-13** (Advisory) 06 6.4: The one-key signing socket bounds the key, but no rule says the key is registered on GitHub as a signing key only, and confirm-on-use is not a rule the guide states. | L2b | M | Apply the 06 6.4 sentence and the guide steps; leave the 05 5.4 Verified-signatures row unchanged, since editing an accepted risk needs the operator and the guidance closes the scenario without it. See AR9. |
| **R8-06-14** (Required) 06 6.4: The product rule 'the full host agent is never forwarded' is contradicted by the measured development sandbox of this repository (ADR 0008 Context: its forwarded agent holds the signing key and a GitHub authentication key), and the index deferred row describes that sandbox as if it were the product. | L2 | operator | Recommend approving the 06 6.4 scope sentence, the ADR 0008 Context rewording and the dogfooding Deferred row as written. Decision AR16. |
| **R8-06-15** (Advisory) 06 6.4: git-ssh-sign is a product kit with its own ask-first surface, settings field, preflight check and open question, and no sentence says why the start needs it. | L11 | M | Put the why-now clause in 06 6.4 instead of 00 0.5, so the scope page is untouched. See SC11. |
| **R8-06-16** (Required) 06 6.4: os-base installs an apt delta at kit build time with no version pin or snapshot, so a sandbox's OS packages change under an unchanged kit-tree digest and gate 2 shows nothing. | L7 | operator | Recommend Option 1, the 05 5.4 row with its trigger plus a Deferred row, decided after R8-06-17 is applied. Decision AR15. |
| **R8-06-17** (Advisory) 06 6.4: os-base is specified before probe A12 measures whether the workload lacks any package, so it may be an empty kit with a TZ arg. | L11 | A | Apply as written. See AR15. |
| **R8-06-18** (Advisory) 06 6.4: kitpin herdr reads the release API's asset digest, but the spec does not say what a missing or malformed digest field does. | L15 | A | Apply as written. |
| **R8-06-19** (Advisory) 06 6.1: A registry client, strict grammar, descriptor cache and conformance fixtures exist in v1 to decompose a workload already pinned by digest and allowlisted, and the spec does not say what the gate would lose without the workload half. | L11 | L | State the reason as one sentence and keep the mechanism; deferring the workload half would leave gate 2 showing only a digest for the one artifact built outside our review. |
| **R8-06-20** (Advisory) 06 6.2: julieta pin workload and pin check are two sandbox-binary commands reading a registry and GitHub releases for a rare manual act. | L11 | M | Apply as stated. Regression check: follows the SC9 decision; pin check stays (D27, round 1; R8-02-4 and R8-04-28 lean on it). |
| **R8-06-21** (Advisory) 06 6.3: Every romeu embeds and writes both julieta linux binaries into every .romeu/bin, while Q21 fixes sandbox arch equal to host arch, so one of the two is never invoked. | L11 | A | Apply as written; S3 stays measured per host. Regression check: stated as resting on Q21's default, which A4 may reopen (dd N2, round 2). |
| **R8-06-22** (Required) 06 6.4: Product kits are called both kit and mixin (06 6.4 Kind: mixin), and no vocabulary row names which word is the product's. | L3 | M | Apply the mixin row and header for page 06; the rest of L3-7 is page 01's. |
| **R8-06-23** (Required) 06 6.1: The v3 descriptor grammar is parsed by internal/oci and internal/render with no single owner, and nothing says what happens when sbx adds a capability type the strict grammar refuses. | L3 | A | Apply as written. |
| **R8-06-24** (Required) 06 6.4; 12 12.5: The product's architecture decisions (herdr, v3 kits and others) have no ADR, and the only written reasons for herdr are a dated paragraph in 06 6.4 and review transcripts. | L3 | A | Apply in 12 12.5; 06 needs no change beyond being cited. |
| **R8-06-25** (Required) 06 6.2: The one pin file read by four consumers is in no tree and no text names the writer of its workload entry, so the workload digest gets edited by hand. | L3 | M | Apply the 02 2.1 entry and the 06 6.2 clause; the rest of L3-34 is for the other pages' consolidations. |
| **R8-07-1** (Required) 07 7.5; 04 4.3; J9: julieta pin workload and pin check reach a registry and GitHub releases from the config sandbox, but 07 7.5 derives no egress for julieta's own commands and no catalog set holds a registry host. | L13, L13b (high-signal) | M | Smaller than a new derived set: the config project's own spec declares the hosts as gated egress.extra, stated once in 07 7.5 and shown in the 02 2.2 example; no new derivation step. |
| **R8-07-2** (Required) 07 7.4: The GitHub rate limit row says the project's github secret covers mise's api.github.com fallback without naming the variable mise reads or how sbx exposes the secret in the sandbox. | L13, L13b (high-signal) | L | Name GITHUB_TOKEN and state the sbx exposure as an open probe fact for B4 rather than asserting a mechanism the spec has not measured. |
| **R8-07-3** (Required) 07 7.5; 00 0.5; J8; index: Deferred decisions: A tool added on a feature branch hits a blocked host whose quick fix (sbx's native approval) is invisible from romeu, and listing that queue in status sits in a v1.1 bucket with no trigger and no interim. | L1, L11 (high-signal) | operator | The 07 7.5 and J8 sentences stay inside the spec and can be applied as A; the 00 0.5 part changes the scope, so the operator decides it. Recommend applying both. Regression check: see SC7; the 07 7.5 and J8 sentences apply now. Decision SC7. |
| **R8-07-4** (Required) 07 7.5 step 1; 07 7.3: romeu parses mise.lock in Go without naming the lock layout it reads or what an unknown layout does, so a relock by a newer mise can fail every sync with no named fix or silently narrow egress. | L15 | M | Tie the supported layout to the kit-pinned mise version instead of a table of measured layouts; an unknown layout is a named sync error with the relock fix. |
| **R8-07-5** (Required) 07 7.4; 04 4.3; 07 7.3: The environment julieta sets for mise is listed in 04 4.3 and partly repeated in 07 7.3 and 07 7.4, with no single home, so the copies can drift. | L3 | M | For page 07 only: link to the 04 4.3 list instead of repeating values; no generator row, since only julieta's mise driver consumes it. |
| **R8-07-6** (Required) 07 7.4; 10 10.2: mise's minisign verification at pin time names no trusted public key, and no CI step re-verifies a pin after it is written, so a hand edit of the sha256 passes tools/ci kits. | L7 | M | Name the committed key and re-run the existing kitpin verification offline in tools/ci kits; no new tool. The pin file is a pin surface, so the implementing diff stays ask-first. |
| **R8-07-7** (Advisory) 07 7.6; 07 7.5 step 3: julieta spec validate --catalog exits 1 on a lock host missing from the catalog while 07 7.5 step 3 gates such a host and continues, so the config repo's CI stays red for a case the product accepts. | L0 | A | Align validate's exit code with the derivation: an error only where sync errors, a warning where sync gates. Regression check: an unknown backend:tool keeps exit 1 (R3-testability-08, round 6), which S4's second clause needs; only the lock-host case becomes a warning. |
| **R8-07-8** (Advisory) 07 7.3; 07 7.6; J8: A relock in a worked-on repo, where backend renames and new tools arrive, runs no catalog lookup, so the user first meets the gap as a romeu sync exit 2. | L15 | M | A warning at commit time from the existing lookup, not a new hook row or a failing check. |
| **R8-07-9** (Advisory) 07 7.5 step 3; 03 3.1: Lock-derived artifact hosts reach an sbx policy allow network argv without passing the 03 3.1 domain rule, and no size cap bounds the repository blobs romeu reads on the host. | L2 | A | Reuse the existing domain rule for lock hosts and state one blob cap. |
| **R8-07-10** (Advisory) 07 7.3; 04 4.3: 07 7.3 carries hedges with no trigger: the monorepo.lockfile rule for a mode no v1 repo uses (with a known upstream flip and no check), and locking macOS platforms the product never runs mise on. | L11, L15 (high-signal) | M | Delete the monorepo hedge behind a Deferred row with a trigger, and narrow the lock default to the two platforms the sandbox runs. Regression check: the caFile part is dropped (te C1, round 1; R8-05-21's form stands); the two-platform lock default supersedes R3-simplicity-06's warning (round 6), recorded as such. |
| **R8-07-11** (Advisory) 07 7.4: The GitHub rate limit row hands the project's github secret to mise as a download token without saying it is today the operator's own account token (ADR 0008), so 07 reads it as a download credential where 05 5.4 sees administrator reach. | L2b | A | A pointer sentence to the accepted risk; the risk itself is unchanged. |
| **R8-07-12** (Advisory) 07 7.5; 13 13.2; 04 4.2; 01 1.6: The escape hatches (egress.tools, egress.extra, tool: other, --accept-loss, --overwrite-drift, retire --force) are bounded but never counted, so nothing shows when one has become the main road. | L3 | L | State where the count can be read today and defer the counters with a trigger instead of adding a doctor row and a record field. |
| **R8-08-1** (Blocking) 08 8.5: No page says how salvaged or lost-generation work gets back into the next sandbox; salvage proves preservation and stops there. | L0, L1 (high-signal) | M | State recovery as the operator's manual step (J10 step 3) in 08 8.5 and J6, print the salvage ref prefix after each preservation, and defer a restore command with a trigger instead of specifying one now. |
| **R8-08-2** (Required) 08 8.5: Salvage payloads (ignored.tar.gz, agent/, opt-in transcripts) carry secrets into a host memory dir with no stated modes, readable by every later generation and taken by backups, and no 05 5.4 row says so. | L0, L2, L16 (high-signal) | operator | Apply the modes and the paragraph in the spec; the operator decides between a new accepted-risk row in 05 5.4 and moving closed salvage out of the mount (recommended: accept the row, defer the move). Decision AR17. |
| **R8-08-3** (Blocking) 08 8.2: SessionStart output has no size bound, no order and no truncation rule, and it prints every open entry and every lesson entry, which also makes agent-written text an unbounded channel into the next session. | L0, L5 (high-signal) | M | Fix the order and an 8 KiB starting bound with a fixed truncation line, print open lesson entries only, and state the agent-written-text channel as a limit in 08 8.2 rather than adding a threat-model row. |
| **R8-08-4** (Blocking) 08 8.2: Only /clear re-runs SessionStart; compaction and resume, the common ways an agent loses context, re-assert no handoff, warnings or lessons. | L5 | A | Wire SessionStart for all sources of the pinned version and probe it in 11 11.2. |
| **R8-08-5** (Required) 08 8.2: A failing session hook has no stated behavior (exit status, what it prints, where it is recorded), and the hooks call julieta through a link setup may not have made. | L5 | A | State the absolute path, exit 0 with one failure line, and record the pinned version's hook behavior in the same probe row. |
| **R8-08-6** (Required) 08 8.3: The narrative handoff exists only when the operator remembers /handoff or /handoff --final, and SessionStart presents an old narrative as the latest without saying a session's narrative is missing. | L1, L5 (high-signal) | M | Make the gap visible at the next SessionStart and state the operator's step as a limit in J6; no confirmation prompt. |
| **R8-08-7** (Required) 08 (opening): The opening of 08 says nothing sandbox-only may be lost when a sandbox dies, but snapshots fire only on commits, rewrites, merges, stop and pull, so an unplanned death loses uncommitted work and the user is never told. | L1 | L | State the limit for unplanned death and defer a periodic dirty-tree snapshot with a trigger. |
| **R8-08-8** (Required) 08 8.4: The host cannot tell that snapshots of a repo stopped (core.hooksPath set by a tool, or the snapshot write failing), and 8.4 says nothing about a failed snapshot write. | L4 | M | Record failure and disablement where the host can read them (heads.json and the hook-failure record) and show snapshot age in status; no extra trigger. |
| **R8-08-9** (Advisory) 08 8.4: snapshot/heads.json has no schema, no 03 section and no version, and no rule says what romeu trusts when it disagrees with heads.bundle after a kill between the two writes. | L2b, L4 (high-signal) | A | Add the schema and section and state the write order and that the bundle is authoritative. |
| **R8-08-10** (Required) 08 8.4: Snapshot bundles stay only in the agent-writable memory dir until pull or preservation, so an accidental removal inside the sandbox leaves J10 nothing to import. | L4 | L | State where the snapshot lives and what second copy exists, and defer importing at every sync and run. |
| **R8-08-11** (Required) 08 8.4: Snapshot and salvage bundles omit reflog-only commits, so the snapshot follows a destructive git operation instead of protecting against it. | L4 | M | Bundle reflog-only tips as a refs class in both snapshot and salvage; keep one bundle rather than keeping the last N. |
| **R8-08-12** (Required) 08 8.5: No behavior is stated for a stopped sandbox or one that fails mid sandbox half (full VM disk, broken boot), so the operator is left with --from-host without being told what it drops. | L4 | M | State the start-and-stop rule and the failure message with what --from-host loses; no alternate object directory. |
| **R8-08-13** (Required) 08 8.5: Salvage step 1 does not quiesce every writer before the capture: pane processes, other shells and the agent's own SessionEnd hook (fired by the SIGTERM) keep touching worktrees and index.lock while step 2 runs, and an in-progress rebase is captured silently. | L4, L5 (high-signal) | M | Wait on the store lock, recheck and repeat once, and record two new skip reasons instead of stopping every process of the sandbox user. |
| **R8-08-14** (Advisory) 08 8.5: The stop-agents path (SIGTERM, 10 s wait, SIGKILL, recorded) is named by no test level. | L6 | A | Add the container case. |
| **R8-08-15** (Required) 08 8.5: The agent profile (process names, state paths) has no drift check: a renamed process makes --stop-agents stop nothing and a moved path makes step 5 copy nothing, and salvage still reports complete. | L15 | M | Make drift visible as two skip reasons and a setup warning; no separate drift-check tool. |
| **R8-08-16** (Advisory) 08 8.5: Salvage refs are created under refs/heads/salvage/, so git push --all or --mirror publishes them with their untracked files, and every later snapshot re-bundles all past salvage refs. | L4 | A | Move salvage refs out of refs/heads and bundle only the current generation's. |
| **R8-08-17** (Advisory) 08 8.5: Untracked non-ignored files have no cap while ignored files have 1 GiB, and a nested repository's dirty tree and stashes are not captured. | L4 | A | Apply as proposed. |
| **R8-08-18** (Advisory) 08 8.5: The host half unbundles salvage bundles into the host clone without the git fsck --strict that pull runs on an unbundled head, although 05 5.1 says transfer.fsckObjects does not cover unbundle. | L2 | A | Apply as proposed. |
| **R8-08-19** (Required) 08 8.5: Exit 5 lists skipped reasons whose only stated recovery is 'fix it or --accept-loss'; over-cap has no path to salvage.excludeIgnored shown and unreadable has none at all. | L8 | M | A hand table in one home linked from 03 3.11, without a generator row. |
| **R8-08-20** (Blocking) 08 8.1: Temp files of atomic writes and the store lock have no stated path or name, so a julieta killed mid-write leaves a file that violates the allowlist, and sync and run then refuse with exit 2, blocking the J10 recovery. | L4, L16 (high-signal) | A | Fix the temp-name grammar as a non-violation cleaned by setup and move the lock out of the memory dir. |
| **R8-08-21** (Blocking) 08 8.1; 00 0.5: julieta memory import and verify, the import/ allowlist entry and the evidence kinds that serve them carry one maintainer's one-time migration into the generic product, and import/ is described nowhere (no content, writer or cap). | L11, L13b (high-signal) | operator | The operator decides whether the one-time migration leaves the product (recommended); either way import/ is described or removed in 08 8.1 and I24. Regression check: reverses op-migration-set (round 6) and says so. Decision SC2. |
| **R8-08-22** (Required) 08 8.1: julieta memory rm is a delete path with no history in a product that otherwise never deletes, and search duplicates list with a filter. | L11, L16 (high-signal) | operator | The page consolidator proposed M: remove rm and fold search into list --query. Completeness check: routed to the operator, since it removes commands from the v1 set (00 0.5) and carries the finding L11-24 that R8-04-37 sends there; the change is written back to clusters-08.json. Recommendation: drop rm (edit --status done stays) and fold search into list --query rather than adding a trash or a residual-risk row. Decision SC10. |
| **R8-08-23** (Advisory) 08 8.4: The bundle excludes only objects reachable from repos[].base, which moves only on sync, so after pushes every snapshot re-bundles already-pushed objects and 'small' holds only right after a sync. | L0 | L | State the cost instead of refreshing base on run. |
| **R8-08-24** (Required) 08 8.1: Memory caps give no numbers for handoff/, while a facts file is written at every session end, so either a cap eventually makes sync and run refuse or the dir grows without bound. | L5 | M | Give starting values and make julieta prune only facts files; the cap warns rather than refuses. |
| **R8-08-25** (Advisory) 08 8.3: In a multi-repo project a handoff or entry goes to the repo of the agent's cwd, and which repo's memory --hook reads is unstated, so cross-repo work is filed by where the shell happens to be. | L5 | A | Apply as proposed. |
| **R8-08-26** (Required) 08 8.1: The memory layout allowlist is written out in 08 8.1 and in I24 (and in memstore) with no single home and no test that they agree. | L3 | M | One home in 08 8.1 and a link from I24; no generator row for this list. |
| **R8-08-27** (Required) 08 8.4: Which commands snapshot (and ingest, drain, take the lock) is a hand list in several sections that a new command must remember to join. | L3 | M | Link to the one side-effects table instead of listing the commands in 08 8.4. |
| **R8-08-28** (Required) 08 8.1: Architecture decisions the spec rests on (memory as plain files among them) have no ADR, and 12 12.5 sends the docs module to review-round history to write them. | L3 | M | One row in the 12 12.5 table; the ADR itself is plan work. |
| **R8-09-1** (Blocking) 09 J1: Onboarding assumes a config repo that already holds its own config-project spec, and no step, command or journey says how the first one is made. | L10 | M | Apply in the smaller form: the J1 step 0 by hand from examples/ and the 02 2.1 and 04 4.2 sentences; no new init flag. |
| **R8-09-2** (Blocking) 09: The clean-exit promise of 00 0.1 and 08 8.1 has no journey, guide or test: nothing says how to stop using romeu and julieta and what is left behind. | L16 | operator | The page consolidator proposed M: apply J14 as proposed, without a new S12, since S7 covers every journey. Routed to the operator by the gate review of this page: it declines the S12 that R8-index-22 proposes, and a success criterion is scope. Recommendation: J14 under S7, as proposed. Decision SC16. |
| **R8-09-3** (Required) 09 (opening), J12, J3, J10: 09 disagrees with itself on J12 (opening: J1 steps 3-6; J12: J1 steps 1-5), names cmux, a non-goal host multiplexer, in J3, and J10 step 3 points at a 'hardened form' no page defines. | L0, L3, L6 (high-signal) | A | Apply as proposed: one J12 range in both places, drop cmux, and J10 step 3 points at an existing path instead of an undefined form. |
| **R8-09-4** (Required) 09 J9, J1: No journey covers sbx (or its runtime) updated on the host, the most frequent upstream change: gate 1 exits 3 everywhere and no guide says what to check or when the recordings stop describing this sbx. | L0, L15 (high-signal) | M | Apply as a J9 bullet rather than a new journey, which keeps one journey for every upstream bump. |
| **R8-09-5** (Required) 09 (opening), S7: The spec does not say what a journey passing means in CI or which steps a fake-sbx run omits, so S7's 'every journey passes' has no per-journey meaning. | L6 | M | Apply as one rule in the 09 opening instead of a per-journey table; each scenario lists its own omitted steps in code. |
| **R8-09-6** (Required) 01 1.7, 09 (legend), 05 5.3, 05 5.4, 10 10.2, 00 0.2: One person is 'operator', 'maintainer', 'developer' and 'user' across pages, none is in the vocabulary, and the only sentence relating them is in ADR 0008 Context. | L3 | A | Apply as proposed; ADR 0008 is only cited, not changed. Owned by the page 01 consolidation; on 09 only the legend link. |
| **R8-09-7** (Advisory) 09 J1: J1 has no step and no recovery for the dedicated signing agent that git-ssh-sign requires, so sync exits 2 with RJ-204 and the guide has nothing for it. | L0 | M | Apply as a sentence in J1 step 5 pointing at 06 6.4, not a new journey; keeping the agent alive across reboots stays in the guide page. |
| **R8-09-8** (Advisory) 09 J1, 12 12.8: J1 shows nothing of the trust model before the host is changed, and the README trust paragraph it would rely on has no stated content. | L10 | M | Apply, with the paragraph linking 05 5.4 instead of restating three chosen risks, so there stays one list. |
| **R8-09-9** (Advisory) 09: The maintainer's release flow (rehearsal, rc tag, block B on both hosts, signing, v1.0.0, B1 again, acceptance) is not a journey, so it has no scenario, guide or per-error recovery. | L9 | L | State as a limit: the release flow stays a 12 12.2 checklist, not a journey; one sentence in 09 says so. |
| **R8-09-10** (Advisory) 09: 09 names two readers against ADR 0001 rule 1, and its steps are written for the scenario writer while the guides copy them for the operator. | L8 | A | Apply as proposed on 09; the other pages with two readers are handled when T006 moves readers to front matter. |
| **R8-09-11** (Advisory) 09 J2: On the `sync --from <sha>` path the gate 2 diff is the only review of an unmerged spec, and J2 does not say so. | L2b | A | Apply as proposed. |
| **R8-10-1** (Blocking) 10 10.2 (Release and bootstrap; workflow grammar): release.yml cannot pass the workflow grammar it is said to pass: the grammar admits only read or none permissions and no token, while attest, publish, verify and dora --attach need write scopes and a token, and no page says which job holds them, on which runner, or that the job holding them runs none of the change's code. | L0, L2, L2b, L7, L9, L13 (high-signal) | M | Apply as one paragraph: two job kinds, the publish job alone holding the three write scopes and GH_TOKEN for three named steps, refused elsewhere by tools/ci workflows. Building inside the publish job avoids an archive hand-off between jobs, so no artifact action joins the grammar's list (smaller than L7-10's form). |
| **R8-10-2** (Blocking) 10 10.2 (Release and bootstrap); 05 5.4 (Release signing); S1; J1: Release signing exists only as a prohibition: no page says what is signed, with which tool, where the signature is published or how a user verifies it, while S1 says the attestation step 'signed them'. | L3 | operator | Recommend the first option (no signature beyond the attestation in v1, with a deferred row reopened by the first adopter who asks for one). Decision DR2. |
| **R8-10-3** (Blocking) 10 10.2 (Release and bootstrap); 12 12.10; [ADR 0004, measure delivery with the five DORA metrics computed by a tool](../adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md): No page makes a published release immutable: dora.json is replaced on every release by design and 'published and never a draft' leaves assets replaceable, while 06 6.4 refuses a herdr release that is not immutable. | L7 | operator | Recommend apply. If R8-10-19 (L11-5, dora deferral) is accepted, the dora part falls away and the rest is spec text. Regression check: the wording reconciles with R3-critic-09 (round 6, 'never a draft'): the draft exists only inside one publish call. Decision DR6. |
| **R8-10-4** (Blocking) 10 10.5; 10 10.2 (Release and bootstrap); index: Success criteria: The four end-of-plan completeness checks (S6 tags, probe results, a guide per journey, the block B commit rule) run once, after v1.0.0 is public; a failure then has no path, and after v1.0.0 nothing re-checks them or says what a later release promises. | L3, L6, L8, L9 (high-signal) | operator | The page consolidator proposed M: Apply the smaller form: the checks move into all once the plan ends, run once before the tag, and a post-tag failure is a patch release; L9-4's pending-ids file is not added. Routed to the operator by the regression check: moving the four completeness checks into tools/ci all and running acceptance before the tag reopens op-acceptance-after-release and op-probe-completeness (round 6). Decision SC15. |
| **R8-10-5** (Blocking) 10 10.2 (lint row, hygiene row, Forbidden names); ADR 0007: About 330 lines of 10 10.2 restate tool configuration in prose, with pinned versions and commit ids (mise 2026.10.3, revive v1.17.0, golangci-lint 2.14.0, cab976e, fsfe/reuse:6.2.0) in table cells of several hundred words, which nothing holds equal to tools/ci and mise.lock. | L3, L8 (high-signal) | M | Apply the deletion of pinned facts and the one-rule-per-line split; the generated reference and the docs rule on version strings wait as the deferred row's trigger. |
| **R8-10-6** (Blocking) 10 10.2 (lint row); 12 12.1; ADR 0007 rule 8; ADR 0001 Consequences: What mise may install is stated four ways that disagree (the lint row's three tools, ADR 0007 rule 8's two, gh in mise.lock per 12 12.1 and the plan, ADR 0001's Markdown tools through mise.lock against 12 12.1's reuse image). | L13b | operator | Recommend gh in mise.lock, listed once in 12 12.1, with tools/release starting it by its resolved path rather than mise exec. Regression check: records that A-determinism-5 (a)'s mise exec form (round 3) is superseded for gh. Decision DR7. |
| **R8-10-7** (Blocking) 10 10.2 (maintainer block, pre-push hook); 12 12.4; 05 5.4; ADR 0008: The maintainer's host runs agent-written tools/ci at whatever main holds, through the tracked pre-push hook, hygiene add and the result pushes of blocks O3 and O6, while the only written control (read the diff in a plain clone) covers the bootstrap sitting alone. | L2 | operator | Recommend apply. Decision AR12. |
| **R8-10-8** (Blocking) 10 10.2 (maintainer block item a); 05 5.4; 12 12.4; 12 12.9; ADR 0001 rule 8; ADR 0003; ADR 0008: The merge-ruleset fact (no review is required) was patched into four places beside the rule it negates, and ADR 0001 rule 8 and ADR 0003 still describe a code-owner review that does not exist. | L3 | operator | Recommend apply; the spec side is mechanical. Regression check: the record names op-ruleset-approvals (round 6) and N1 (round 3) as superseded in part. Decision DR1. |
| **R8-10-9** (Blocking) 10 10.5 (--file, repo evidence); 00 0.5; 04 4.3; 08 8.1; 11 11.2; I24: The generic product carries one person's migration and acceptance: julieta memory import and verify, the import/ directory, the memory-entry digest kind, the repo evidence kind, acceptance --file and the C1 hugo check. | L11 | operator | Recommend deletion. Regression check: reverses op-migration-set (round 6) and says so. Decision SC2. |
| **R8-10-10** (Required) 10 10.2 (Release and bootstrap); index: Deferred decisions; S1; J1: 'The release workflow file must not come from the tagged commit' is fixed while its mechanism is deferred, and S1's evidence ('a ci-run of release.yml on the tagged commit') and J1's signer-workflow flags assume the opposite, so T088 must break one of them. | L0, L7 (high-signal) | operator | The page consolidator proposed M: decide workflow_dispatch from main with the tag as input; S1 and J1 follow; the deferred row closes. Routed to the operator by the gate review of this page: it answers an index deferred decision, which stage A shapes and never answers (lenses.md, "What each round reads"), and changes S1, a success criterion. Recommendation: the smaller form, dropping 'on the tagged commit' from S1 and the ref-specific flags from J1 until the row is decided. Decision SC17. |
| **R8-10-11** (Required) 10 10.5; 05 5.4 (tag row); 10 10.2 item d: Nothing records the commit a release tag named at publication, so a v* tag the token deletes and recreates on another commit is rebuilt and attested with no check noticing that the version moved. | L7 | operator | Recommend apply; with the acceptance checks moved into all (R8-10-4), the check runs on every pull request after v1.0.0. Decision AR21. |
| **R8-10-12** (Required) 10 10.5; 11 11.4: The end-of-plan check requires results only for probes named in the index's Settled by column, so A2, A3, A5, A6, A9 and A14, A3 among them, can be skipped unnoticed. | L0 | A | Apply as proposed. |
| **R8-10-13** (Required) 10 10.3; 04 4.1; 04 4.2; 11 11.2; 11 11.4; 12 12.1: Nothing says what romeu does on an sbx newer than every recording or when its output no longer parses, and nothing re-records or re-checks recordings and probe results after block A, so sbx drift stays green in CI and surfaces as a wrong error on a user's run. | L3, L4, L6, L10, L15 (high-signal) | M | Apply as one 'Tested window' row merging the five proposals; no 'stale' lifecycle state is added (the version field and the window check replace it), and the window widens only by re-recording. |
| **R8-10-14** (Required) 10 10.3 (Known argv); 11 11.1; 01 1.6: Neither 10 10.3 nor 11 11.1 lists the argv shapes and starting sessions block A must record, failure sessions included, so a command or a generation-machine failure row can find itself without a recording after the one two-host sitting. | L4, L12 (high-signal) | A | Apply as proposed. |
| **R8-10-15** (Required) 10 10.3 (Redaction): Redaction removes only home path, user name, host name and root path, while block A records the real global git config and sbx global rules into a public repository, where emails and internal hosts would be published for good. | L16 | A | Apply as proposed; the harm is irreversible, so it lands before T026 commits recordings. |
| **R8-10-16** (Required) 10 10.2 (tools/ci output; Forbidden names): tools/ci has no failure-output contract: refusals carry no id or fix line, are addressed to 'the operator' when the pusher may be an agent or a stranger, a forbidden-name hit gives no way forward, and the minutes-long pre-push prints no progress. | L5, L14 (high-signal) | M | Apply without --json and without a public error-id table (tools/ci is not a product interface); the fix lines and progress lines are the whole change. |
| **R8-10-17** (Required) 10 10.2 (vulnerabilities row); 10 10.8; 12 12.8: A merge gate whose result depends on the date (govulncheck in all) or on a flake leaves no owner for a red leg the change did not cause, and every open pull request turns red at once. | L12, L14 (high-signal) | M | Apply the ownership rule; re-gating on the merge result (a merge queue) is stated as a limit, since it is a repository setting on the ruleset surface. |
| **R8-10-18** (Required) 10 10.2 (maintainer block); 05 5.4; index (Status): History has moved into the current-state spec: the maintainer block mixes dated measurements and done items with steps still to do, and the removal of the required review sits inside item a. | L3, L9 (high-signal) | M | Apply the 10 10.2 part: the maintainer block keeps only current requirements and to-do items; the bound on deferred-row length is left to the index consolidation. Moving results into ADR 0008 itself would change a record, so a linked docs/ note is the default. |
| **R8-10-19** (Required) 10 10.2 (dora --attach); 12 12.10; ADR 0004; 00 0.2: tools/ci dora, a five-metric delivery tool, is a v1 deliverable for a repository with one contributor and no release, while its own ADR says nothing is lost by waiting. | L11 | operator | Recommend deferring; it also simplifies R8-10-1 and R8-10-3. Regression check: supersedes op-dora-late (round 6) and leaves op-practices, op-pairing-rule, op-backfill, op-maintainer-wait and op-pr-size (round 4) without their mechanism until the trigger; the record names them. Decision SC12. |
| **R8-10-20** (Required) 05 5.3; 10 10.2; 12 12.3: docs/spec/** is on no ask-first surface while two checks read its tables as input (tools/ci vocabulary from 01 1.7, sequences from the index), so gate inputs sit outside the gate. | L3 | operator | Recommend the data-file option, which puts the inputs on the existing checks surface. Decision AR13. |
| **R8-10-21** (Required) 02 2.1; 10 10.2: tools/ci has about 25 subcommands and 30 tasks in one tree with no internal layout, and every generator is wired through one generate.go, so the phase 0 and 1 tasks conflict pairwise. | L12 | A | Apply as proposed; ADR 0007 rule 8's single go:generate site stays. |
| **R8-10-22** (Required) 10 10.8: Done requires tools/ci all, but nothing says which steps can run inside an agent's sandbox (license pulls an image, container and hybrid e2e need Docker and network, vulnerabilities reads a database). | L5 | A | Apply as proposed. |
| **R8-10-23** (Required) 10 10.1 (Rules); 10 10.6 (Tests): 'Flaky tests are fixed, never skipped' has no mechanism: nothing forbids t.Skip, t.SkipNow, t.Skipf or testing.Short. | L6 | A | Apply as proposed. |
| **R8-10-24** (Required) 10 10.1 (Rules): An absent precondition of the hybrid and container levels (Docker daemon, image digest, herdr binary, mise download) has no stated outcome, so a step that cannot start can exit 0. | L6 | A | Apply as proposed. |
| **R8-10-25** (Required) 10 10.2 (step table): Steps that run on some runners only are selected inside tools/ci and nothing asserts what ran, so a selection that runs nothing, or the wrong subset, is green. | L6 | M | Apply the smaller form: the selection is data with a test, and a zero-test step fails; a per-test expected manifest waits for a measured miss. |
| **R8-10-26** (Required) 10 10.2 (step table): No step of all compiles the host suite, so a host suite that does not build is found at block B and forces a new candidate. | L6 | A | Apply as proposed. |
| **R8-10-27** (Required) 10 10.2 (race and e2e rows): The race run omits -tags e2e and the e2e step has neither -race nor -count=1, so subprocess-heavy paths miss the race detector and e2e can be served from the test cache. | L6 | A | Apply as proposed. |
| **R8-10-28** (Required) 10 10.2 (subcommands outside all): Nothing says what a red scheduled fuzz.yml (fuzz, mutate) does: no required check, no issue, no release block. | L6 | A | Apply as proposed. |
| **R8-10-29** (Required) 01 1.7; 10 10.2; 05 5.3; 05 5.4; 00 0.2: The same person is 'operator', 'maintainer', 'developer' and 'user' across pages, none of them in the vocabulary. | L3 | A | Apply as proposed. |
| **R8-10-30** (Required) 02 2.5; 02 2.4; 10 10.6; 04 4.1: Four universal claims are refuted by the spec's own tables, 10 10.6's 'every lifecycle is a transition table in internal/state' beside the probe lifecycle in tools/ci among them. | L3 | A | Apply as proposed. |
| **R8-10-31** (Required) 10 10.7; 02 2.1: The module-boundary table says nothing about a package in no row, so tools/ci imports is fail-open for every new package. | L3 | A | Apply as proposed. |
| **R8-10-32** (Required) 10 10.3 (Cannot model); 12 12.9: Field bug classes sit under 'Cannot model' with no seam, so test-first cannot apply to their fixes and a stranger cannot contribute the recording that would make the bug testable. | L14 | A | Apply as proposed, after the redaction fix of L16-12. |
| **R8-10-33** (Advisory) 10 10.2 (vulnerabilities row); ADR 0001 rule 5: govulncheck reads the network inside all while ADR 0001 rule 5 says all stays offline, so a local all without network is not the equivalent run 10 10.2 requires. | L6 | operator | Recommend apply; the 10 10.2 sentence alone can land now. Decision DR8. |
| **R8-10-34** (Advisory) 10 10.2 (hygiene row, pushed range row, Forbidden names); 12 12.4: Two product CI rules encode one person's working agreement without explaining it: no tracked PROLOGUE.md because 'the user's agreement file' lives elsewhere, and a hashed denylist whose class of names and false-positive contact are not stated. | L10, L13 (high-signal) | M | Apply: the name becomes data in the existing denylist mechanism, and the class and contact are one sentence. |
| **R8-10-35** (Advisory) 10 10.2 (step table): Neither fast nor all has a time budget, so the pre-push loop and the macOS legs can grow unnoticed until contributors push with --no-verify or the Actions quota runs out onto the maintainer's two machines. | L12, L14 (high-signal) | M | Apply one number for fast and a deferred row for the runner budget; no Linux-only split now. |
| **R8-10-36** (Advisory) 10 10.2 (Runners); S2: Runner labels are pinned, but GitHub retires labels and changes images under a fixed label, and the spec neither names the event nor bounds what the release takes from the image. | L7, L15 (high-signal) | A | Apply as proposed. |
| **R8-10-37** (Advisory) 10 10.1 (container e2e cache); 10 10.2 (grammar): The container e2e's mise cache names no action, no pin and no scope, so a cache written by a branch run under the sandbox's token could be restored by a tag run. | L7 | A | Apply as proposed. |
| **R8-10-38** (Advisory) 10 10.2 (setup): go run ./tools/ci setup is the one step that fetches modules, and its environment (GOPROXY, GOSUMDB, GONOSUMDB, GOFLAGS, GOENV) is not stated, so a caller's settings can fill the module cache unverified. | L7 | A | Apply as proposed. |
| **R8-10-39** (Advisory) 10 10.2 (maintainer block item d); 05 5.4; ADR 0008: Item d did not try the ruleset body change, so 'an agent can disable the ruleset' stays inferred in 05 5.4 and ADR 0008 where a reversible try was available. | L2b | operator | Recommend running the try at the next sitting. Decision AR22. |
| **R8-10-40** (Advisory) 10 10.2 (Release and bootstrap); 12 12.6: Release notes are generated from commit subjects for v1.0.0, whose range is the whole history, so the generator gates a first release for a second that does not exist. | L11 | L | The page consolidator proposed A: Apply as proposed. Amended by the regression check: resolved through R8-12-3 and R8-12-4; the generator stays (craft C19, round 2) and the whole-history notes of v1.0.0 are stated as a limit. Hand-written v1.0.0 notes are not adopted. |
| **R8-10-41** (Advisory) 10 10.8: For host-facing behavior a task is done when its scenario function exists, which proves it compiles, not that it ran. | L12 | A | Apply as proposed. |
| **R8-10-42** (Advisory) 10 10.8: The definition of done names nothing about the hand-written docs a change makes stale (a guide step, the fix hint of a new error id). | L8 | A | Apply as proposed. |
| **R8-10-43** (Advisory) 10 10.2 (Forbidden names): The real denylist entries are proven to match only at hygiene add time, so a matcher refactor can stop them matching unseen. | L6 | A | Apply as proposed. |
| **R8-10-44** (Advisory) 10 10.2 (Release and bootstrap): tools/ci acceptance --pre-tag is a hand step before an unrepeatable tag, and release.yml does not run it. | L6 | operator | The page consolidator proposed A: Apply as proposed; R8-10-4 relies on this run. Routed to the operator by the regression check: tools/ci acceptance --pre-tag inside release.yml reopens op-acceptance-after-release (round 6), which took acceptance out of the release workflow. Decision SC15. |
| **R8-10-45** (Advisory) 10 10.1; 05 5.2: 10 10.1 lists I16 as unit-level while 05 5.2 gives I16 an E-level fs snapshot diff. | L6 | A | Apply, keeping both levels. |
| **R8-10-46** (Advisory) 10 10.1 (container e2e); I17: The container e2e names 'salvage completeness cases' without a list, while I17 and T061 hold two different lists of one set. | L4 | A | Apply as proposed. |
| **R8-10-47** (Advisory) 10 10.7; 08 8.1; 02 2.1: One problem (a binary must not link the other's write path) has two mechanisms: memstore is split so romeu links no delete, while internal/ledger stays shared and julieta links the I31 create function. | L13b | A | Apply the split, so one rule covers both packages. |
| **R8-10-48** (Advisory) 10 10.2 (id table); tools/ci sequences: Sections are cited mostly as plain text ('10 10.2'), which no check resolves, so a renumbering breaks citations silently. | L8 | A | Apply as proposed. |
| **R8-10-49** (Advisory) 10 10.6 (code example); 03 3.2: The 10 10.6 code example carries only SPDX-License-Identifier while 00 0.4 requires SPDX-FileCopyrightText too, and 03 3.2's example hard-codes a v1.0.0 $schema URL. | L8 | A | Apply as proposed. |
| **R8-10-50** (Advisory) 10 10.2 (step table); ADR 0001: The gate is specified in final form, about 27 subcommands and a 20-rule docs standard, with no mark of which steps the first day needs. | L11 | M | Apply the 10 10.2 marking only; the ADR 0001 rule-adoption split is left out, since it would change a decision record for wording. Regression check: the marking matches the day-one list round 6 wrote into 12 12.2 (R3-simplicity-02), or both change together. |
| **R8-10-51** (Advisory) 11 11.3; 11 11.4; 10 10.2 (probes): A four-state probe lifecycle with a computed decision, affects paths and a generated diagram is machinery for about twenty measurements run once per host. | L11 | M | Apply; it agrees with R8-10-13, which adds a version field instead of a 'stale' state. |
| **R8-10-52** (Advisory) 03 3.6; 13 13.7; 10 10.1: The manifest protocol window (N and N-1) and the event version window are specified and tested before a second version of either exists. | L11 | M | The page consolidator proposed A: Apply as proposed. Amended by the regression check: resolved through R8-13-10's one rule in 03; the exact-match replacement of the manifest and event windows is not taken, since it would reverse dd N8 (round 2) and op-ledger-versioned (round 5) and contradict R8-06-7. |
| **R8-11-1** (Blocking) 11 11.2 (C1); 00 0.5; 04 4.3; 08 8.1; 03 3.9; 03 3.12; 10 10.5; I24: The generic product carries one operator's migration and acceptance: memory import and verify, the import/ allowlist, the memory-entry and repo evidence kinds, tools/ci acceptance --file, and the C1 hugo check. | L11 | operator | Changes the scope (00 0.5 records the maintainer's decision that pre-v1 memory entries are imported once at adoption), so the operator decides. Recommendation: delete from the product and move to the operator's config repo; page 11's share is removing C1, with its id left unused per 11 11.1's rule. Regression check: reverses op-migration-set (round 6) and says so. Decision SC2. |
| **R8-11-2** (Required) 11 11.1 (opening, A3, A5): Block A 'needs no product code', yet A3's Expect requires romeu/julieta checks and ledger ingest, and A5 records hashes of render goldens before render exists. | L6, L9, L12, L13, L13b (high-signal) | A | Apply as proposed: A3 keeps only the mount facts, the product half becomes C6 in block B, and 11 11.1's opening states that the A5 goldens are hand-written first and render must reproduce them. |
| **R8-11-3** (Required) 11 11.4: An overturned result fails tools/ci probes, a step of all, so the pull request that commits block A results cannot merge until the fix, its ADR and resolvedBy ride with it, unlike 10 10.5's end-only check of S6 rows. | L9, L12 (high-signal) | M | Apply L12-15's form (move the failure to tools/ci acceptance), which reuses the existing end-only check instead of L9-12's coupling of the fix to the results PR; the resolved-row checks stay in tools/ci probes. |
| **R8-11-4** (Required) 11 11.3; 11 11.4: Probe results never go stale: they record the sbx version but nothing compares it with the floor, the agent version behind C3 and C5 is not recorded, and no rule says which pin bump re-runs which probe. | L3, L8, L15 (high-signal) | M | Apply without a new 'stale' state or per-version file names: the version comparison joins tools/ci acceptance and git history keeps the superseded results; the guide clause covers L8-19's affects half. |
| **R8-11-5** (Required) 11 11.1: Block A measures none of the sandbox events salvage and a working month rely on: exec on a stopped sandbox, a Docker or daemon restart with a sandbox running, the signing socket afterwards, sbx env rm and the workload's volumes, an sbx upgrade under existing sandboxes. | L1, L4 (high-signal) | M | One probe A15 for the stop, restart and rm facts; the upgrade case is stated as a limit, measured at the next floor bump (R8-11-4), and the stopped-disk-from-host read is dropped. |
| **R8-11-6** (Required) 11 11.2 (the rule compares sources); 10 10.2: The v1.0.0 binaries are a second build declared the same as the candidate's only by argument from pins, with no build contract and no check, and the post-tag B1 run starts no sandbox. | L6, L7 (high-signal) | M | Fix the build contract and extend the post-tag run to B2 on one host; a byte-for-byte rebuild check is not added and the spec says so. |
| **R8-11-7** (Required) 11 11.2 (the release candidate): The docs-only difference between the candidate's commit and the v1.0.0 tag is checked only by tools/ci acceptance after the tag, which cannot be redone, and before it only by the maintainer remembering. | L13 | operator | The page consolidator proposed M: Apply the pre-tag check for the candidate comparison. The finding's second half (the security log at each sitting, 05 5.4) is not on this page and touches an accepted risk; it belongs to the 05 consolidation. Routed to the operator by the regression check: a pre-tag candidate comparison inside release.yml reopens op-acceptance-after-release (round 6); decided with R8-10-4 as SC15. The security-log half belongs to DR5. Decision SC15. |
| **R8-11-8** (Required) 11 11.1 (A12); 11 11.2 (B2): A12 measures ssh-add -L only with SSH_AUTH_SOCK set to a one-key agent; the default case, the variable unset in romeu's scrubbed env with an ssh-agent capability declared, is unmeasured in A12 and B2. | L2b | A | Apply as proposed in A12 and B2. |
| **R8-11-9** (Required) 11 11.2: The spec defines what v1.0.0 needs and is silent on a patch release: whether a fix after v1.0.0 needs a candidate and block B again, and who tags it. | L14 | L | State the gap as a deferred decision with its trigger instead of a table now: the split of paths is best decided with the first real post-v1 change in hand. |
| **R8-11-10** (Advisory) 11 11.1 (A2): A2 measures sbx re-prompting for a changed host command, and nothing measures the macOS keychain's own prompt when a secrets command such as security find-generic-password (03 3.4) runs from a non-interactive sbx env run. | L0 | A | Add the keychain observation to A2; a doctor row or a J1 note waits for the result. |
| **R8-11-11** (Advisory) 11 11.4; 11 11.3: A four-state probe lifecycle with its own generated diagram is machinery for about twenty measurements one person runs once per host. | L11 | M | Keep the result format and every check, drop the states and the diagram; this also carries R8-11-3's move of the overturned failure to acceptance and removes the probe half of R8-11-12. |
| **R8-11-12** (Advisory) 11 11.4; 12 12.8; 01 1.6: The probe lifecycle is a table owned by tools/ci probes while the index puts every lifecycle in internal/state and 12 12.8 promises its diagram generated from those tables; the promotion commit, a numbered procedure, is promised a diagram too. | L13 | M | Resolve through R8-11-11 for the probe and edit 12 12.8's list for the promotion commit; no new decision in 01 1.6. |
| **R8-11-13** (Advisory) 11 11.4 (kept row); index Open questions; 12 12.5; ADR 0005: When a settled question becomes an ADR is stated three ways: always (index, 12 12.5), only if expensive to reverse (ADR 0005 decision 4), and not at all for a kept probe result (11 11.4). | L13b | M | Align the spec with the existing decision record (ADR 0005) rather than change the record; the edits stay inside the spec text. |
| **R8-11-14** (Advisory) 11 11.3: A block B result holds typed observations and a computed verdict but not the values its predicate evaluated, so a later reader cannot re-judge it. | L6 | A | Apply as proposed. |
| **R8-11-15** (Advisory) 11 11.3; 12 12.5: Committed probe results pin spec heading anchors in affects, and nothing catches a heading rename that orphans them. | L3 | M | The check alone; the proposed 12 12.5 sentence asking authors to remember is dropped as a remembered mechanical step. |
| **R8-11-16** (Advisory) 11 (opening, arch-sensitive probes); 05 5.4: The arch-sensitive probes and S3 need both an Intel and an Apple silicon Mac, and nothing says what holds if one host is unavailable or sbx drops Intel during v1. | L9 | L | Record as a deferred decision with its trigger rather than a 05 5.4 risk row or a fallback now; narrowing S3 then would be the operator's call. |
| **R8-12-1** (Blocking) 12 12.4, 12 12.9: The merge row of 12 12.4 and three Enforced-by cells of 12 12.9 (releasable, merge commit, the reviewer loop) name ruleset checks and a required review that the measured ruleset does not give against the sandbox's token, and ADR 0001 rule 8, [ADR 0002, develop on the trunk with short-lived branches](../adr/0002-develop-on-the-trunk-with-short-lived-branches.md) and ADR 0003 still assert them as Accepted text. | L0, L3, L9 (high-signal) | operator | The spec-text part applies as written; superseding ADR 0001 rule 8, ADR 0002 and ADR 0003 in part, and adding a partial-supersede status form to ADR 0001 rule 7, change decision records. Recommendation: accept both, in one ADR. Regression check: the record names op-ruleset-approvals (round 6) and N1 (round 3) as superseded in part. Decision DR1. |
| **R8-12-2** (Advisory) 12 12.8: The README outline promises 'first run (J1 in six commands)' while J1 has seven steps, so rule 13 forces the README to match a count that is wrong. | L0, L1 (high-signal) | A | Apply as proposed. |
| **R8-12-3** (Required) 12 12.6: Release notes are commit subjects and Why sections with an undefined 'format-version or exit-code change' detection, and no upgrade section tells the upgrading user what the release costs (gate 1, gate 2, recreate, changed error ids). | L1, L8 (high-signal) | M | Apply the detection rule and the Upgrading section from the three diffs only; the toolchain-acknowledgement summary of L1-24 is not added as its own item. |
| **R8-12-4** (Advisory) 12 12.6: The release-notes generator gates the first release while the notes of v1.0.0, 'since the last tag that is not a prerelease', are the whole history of the repository. | L11 | L | State as a limit. Deferring the generator is not proposed: R8-12-3 gives it an Upgrading section the second release needs, and keeping 'nobody writes release notes by hand' keeps one concept. |
| **R8-12-5** (Advisory) 12 12.8, 12 12.2: ARCHITECTURE.md, the newcomer's entry promised by the 'Simple and explicit' principle, is a headings-only skeleton for the whole build, gives no path per critical flow, and no check keeps it in step with new invariants or modules. | L3, L10, L14 (high-signal) | M | Links instead of copied tables in the skeleton; one id-presence rule instead of a headings.yaml entry, since the outline's headings are not fixed by rule 12. |
| **R8-12-6** (Required) 12 12.3: 01 1.2, 02 2.1 and 12 12.3 hold three different inventories of generated artifacts, and 12 12.3 has go generate write diagrams into ARCHITECTURE.md, a hand-written page, which tools/ci generated and the hand-edit rule cannot classify. | L13 | A | Apply as proposed; it matches the plan's choice of linked files (plan question 27). |
| **R8-12-7** (Blocking) 12 12.8: The guide outline asks for 'recovery from each expected error id' while every journey in 09 is a success path, so most error ids and every non-journey failure have no recovery page, and nothing checks that each id leads to one. | L8 | M | The error-table column and the recovery page; no new journeys, so 09 and its scenario functions stay as they are. Also closes the second half of L14-16. |
| **R8-12-8** (Blocking) 13; ADR 0006; 12 12.2; 00 0.5: The runtime ledger is a v1 feature with no success criterion, no journey and no 'not now' alternative in ADR 0006, yet the build order of 12 12.2 puts it before romeu run. | L11 | operator | Recommendation: defer the ledger (one Deferred row, 13 kept as a design page); it removes the largest unexercised surface from the path to romeu run. Regression check: one recommendation for the four ledger clusters, SC1. Decision SC1. |
| **R8-12-9** (Required) 12 12.10, 12 12.9, 12 12.2; ADR 0004: tools/ci dora, a five-metric tool with its own schema, trailer and release asset, is a v1 deliverable whose metrics are empty for the whole build (one deployment, at the end), has no reading point, and takes 150 lines of the current-state spec, while ADR 0004 itself says nothing is lost by waiting. | L3, L9, L11 (high-signal) | operator | Recommendation: defer the tool and keep the trailer check. Regression check: supersedes op-dora-late (round 6); the round 4 decisions that depend on the tool are named in SC12. Decision SC12. |
| **R8-12-10** (Blocking) 12 12.9; index: Deferred decisions: The no-stack rule plus a maintainer merge for every PR make the calendar critical path equal to the task graph's depth in maintainer sittings, and the deferred row that could change it reopens only on dora output that exists after about 89 of 102 tasks. | L12 | operator | Recommendation: apply the clause and the trigger, and let the maintainer decide on the cadence and on N. Decision SC14. |
| **R8-12-11** (Advisory) 12 12.9; ADR 0002; ADR 0004: The merge-commit rule has two incompatible reasons: 12 12.9 and ADR 0002 say the fix marker and release notes read a PR's commits on main as written, while 12 12.10 and ADR 0004 decision 3 read commits per PR from GitHub so the merge method does not matter. | L13, L13b (high-signal) | M | Fix the spec sentence now; align ADR 0002 only through the superseding record of R8-12-1, so no extra ADR is written for this. |
| **R8-12-12** (Required) 12 12.4, 12 12.7: tools/ci pr requires a non-empty Lessons section on every pull request, which forces a lesson for each change before any change has taught one. | L11 | M | Accept 'none'; the extra rule tying a required lesson to a Middleware 'none' line is not added, since it is a second concept for the same judgment. |
| **R8-12-13** (Required) 12 12.7: docs/lessons.md is one append-heavy file that every learning PR touches, so concurrent pull requests that each record a lesson conflict on it. | L12 | M | Reuse the ADR numbering and index generator (one concept, not a new ULID scheme). |
| **R8-12-14** (Required) 12 12.7; 08 8.1: Two stores are called 'lesson' - docs/lessons.md (this repository, checked by tools/ci lessons) and memory entries tagged lesson (08 8.1, shown at SessionStart) - and the spec never says whether they are one concept. | L5 | M | Name the two concepts apart; no SessionStart change. |
| **R8-12-15** (Required) 12 12.3: Generated files with several writers (the error table's consumers, the ADR index, the ledger tables) have no merge rule: a hand edit is forbidden and generated fails a diff, but nothing says how a conflict in one is resolved or that generators emit stable, line-per-record output. | L12 | M | The two sentences; no column and no merge driver. |
| **R8-12-16** (Required) 12 12.2: The capability map's module-level 'Depends on' and the chained build order are wrong where the critical path runs (kitpin, needed by block A, sits under kits after julieta-core; the error table sits under romeu-cli after everything; egress, render and gate are serial stages). | L12 | M | Correct the edges named; the full package-level graph is not drawn. Regression check: 'Depends on' keeps its plan-order meaning (R3-craftsmanship-01, round 6); only the edges named are corrected. |
| **R8-12-17** (Required) 12 12.1; 10 10.2: gh is 'pinned in mise.lock' and started 'through mise exec', while the lint row of 10 10.2 admits only go, golangci-lint and govulncheck and 10 10.2 measured mise exec running a program of the search path when the tool is missing. | L7, L13 (high-signal) | A | Apply as proposed. Regression check: records that A-determinism-5 (a)'s mise exec form (round 3) is superseded for gh. Follows DR7. |
| **R8-12-18** (Required) 12 12.4; 12 12.8: A stranger has no defined way to obtain the approval line that nearly every likely first PR needs (catalog, error id, dependency bump), since the line quotes the maintainer's words. | L14 | M | The stranger path only; what a catalog PR's Evidence shows (07 7.6) belongs to page 07. |
| **R8-12-19** (Required) 12 12.7; 12 12.8; ADR 0001: Issues have no shape: ADR 0001 and 12 12.7 give them the PR template, no check reads them, 02 2.1 has no issue forms, and only SECURITY.md has a response expectation, while three deferred triggers wait on needs 'written as an issue'. | L14 | operator | Recommendation: accept; the spec lines apply now and the ADR 0001 row follows in the superseding record of R8-12-1. Decision DR1. |
| **R8-12-20** (Required) 12 12.8; ADR 0001: Neither the README outline nor ADR 0001 rule 12's heading order has a License heading, so the GPL-3.0-only product with CC0 examples/ and schemas/ is stated only in the spec and REUSE.toml. | L7 | operator | Recommendation: accept before T084. Decision DR1. |
| **R8-12-21** (Advisory) 12 12.6; 12 12.10: Release notes credit no contributor and link no issue, so a reporter is never told their failure is fixed. | L14 | M | Use GitHub's closing keywords; no new convention beside Fixes-release. |
| **R8-12-22** (Advisory) 12 12.4: The PR body sections, the Middleware line, the goldens and the approval lines are checked only by tools/ci pr on a saved payload of an existing PR, so the first feedback comes after the PR is public. | L5 | A | Apply as proposed. |
| **R8-12-23** (Advisory) 12 12.5: 12 12.5 says the spec describes the current state only, but never defines current state against the plan, so 02 2.1 lists files that do not exist yet and 13 13.1 carries a migration note for a build that never shipped. | L13b | A | Apply as proposed. |
| **R8-12-24** (Required) 12 12.5; 12 12.4; 12 12.9; 05 5.4; 10 10.2; index: History has moved into the current-state spec: dated measurements and 'replaces three rows round 4 recorded' in 05 5.4, a dated Try/Result procedure in 10 10.2, a status paragraph that grows each round, reasoning paragraphs in deferred rows, and the 'as measured on 2026-10-08' clauses of 12 12.4 and 12 12.9. | L3 | M | The rule and the moves; no sequences length bound. The 05 5.4 rows are shortened in wording only, so no accepted risk changes. Regression check: in R8-10-18's form; the to-do items of round 7 stay in 10 10.2. |
| **R8-12-25** (Required) 12 12.5: The decisions the spec cites as authority (rounds 2 to 7, and product choices such as herdr, v3 kits, memory as files, stdlib flag, GPL-3.0-only, the two gates) get ADRs only from the docs module after block B, the 12 12.5 list stops at round 6, and a probe overturn has no record to supersede. | L3, L8, L9 (high-signal) | M | The timing rule and the round-7 line; no table of decisions in the spec, since the review files already list them. |
| **R8-12-26** (Required) ADR 0007; ADR 0001; 12 12.5: ADR 0007 and ADR 0001 hold mechanism (pinned versions, function names such as lintEnv and judgeCommit) in records that are never rewritten, and ADR 0007 cites a deferred row that ADR 0008 removed. | L3 | operator | Recommendation: apply the 12 12.5 rule now and the ADR 0007 status line in the superseding record of R8-12-1. Decision DR1. |
| **R8-12-27** (Advisory) 12 12.5; 11 11.3: Committed probe results pin spec heading anchors in affects, so a heading rename orphans committed data and no rule or check says so. | L3 | A | Apply as proposed. |
| **R8-12-28** (Blocking) 03 (opening); 01 1.2; 12 12.3: The format tables of 03 are design input the product later generates, so after v1 the spec holds a second, unmaintained copy of every format. | L3 | M | Apply; the deletion pass is plan work after T021. |
| **R8-12-29** (Required) 04 4.1 to 4.3; 13 13.3, 13 13.4; 08 8.4; 12 12.3: The cross-cutting side effects of each command (preflight, lock, ingest, drain, snapshot, idempotency) are hand lists in five sections that a new command must remember to join. | L3 | M | Reuse the existing command-definitions source; no new generator. |
| **R8-12-30** (Required) 05 5.3; 01 1.7; 12 12.3: docs/spec/** is on no ask-first surface while two checks read its Markdown tables as gate input (vocabulary from 01 1.7, sequences from the index), so a PR can edit a gate's input and a residual-risk row with no approval line. | L3 | operator | Recommendation: the glob; it reuses the approval line and adds no concept. Decision AR13. |
| **R8-12-31** (Required) 11 11.4; 10 10.3; 12 12.1: A floor or pin bump needs probes re-run, but results carry no sbx version, the lifecycle of 11 11.4 has no stale state, and no section says which bump re-runs which probe. | L3 | M | A column in the existing pin table instead of a new 11.5 table. |
| **R8-12-32** (Required) 01 1.4; 06 6.1; I28; 12 12.2: The v3 descriptor grammar is parsed by internal/oci and internal/render with no single owner, and nothing says what happens when sbx adds a capability type the strict grammar refuses. | L3 | M | oci as the one owner; no new package. |
| **R8-12-33** (Advisory) 02 2.1; 12 12.2; S9: The package tree, the module map and S9's coverage list are three hand lists with different names for the same code. | L3 | M | A consistency test, not a generator. |
| **R8-12-34** (Required) 02 2.1; 06 6.1; 12 12.3; 03; 13 13.5; 05 5.3: The pin file read by four consumers is in no tree and has no writer for the workload entry; three formats have no schema although 03 says each has one; the I4, I25 and I8 guards live in packages outside every ask-first surface, so mutate does not run on a PR that changes only them. | L3 | M | Exempt by name rather than add three schemas while R8-12-8 and R8-12-9 may defer two of the formats; mutate keyed on guard tags, not a new surface. |
| **R8-12-35** (Required) 12 12.2: The build order brings the product to real sbx only at the release candidate, while 10 10.3 lists what the fake cannot model, so each real-host failure costs a new candidate and a new block B. | L9 | operator | Recommendation: accept; two sittings are cheaper than one extra block B. Decision SC13. |
| **R8-12-36** (Required) 12 12.2: The spec mandates a plan and fixes its order but has no re-plan rule, and the plan's counts and dependencies are kept by hand. | L9 | A | Apply as proposed. |
| **R8-12-37** (Required) 12 12.2; 12 12.8; 11 11.2: Guides and the README are written in the docs module after block B, so every guide's expected output is typed after the only real-host run, in text blocks no check compares. | L8 | M | The reorder only. Regression check: recorded as a supersession of M1's placement (round 7); the candidate-to-v1.0.0 docs-only rule is unchanged. |
| **R8-12-38** (Advisory) 12 12.10: dora runs tokenless as the last step of release.yml and backfills the whole history, and the spec does not say what a 403 or 429 does to --attach or the release. | L15 | M | Applies only if R8-12-9 keeps the tool in v1; otherwise it moves with the tool. Follows SC12. |
| **R8-12-39** (Advisory) 12 12.10: Change fail rate and rework count rely on Fixes-release trailers that authors must remember, which is mechanical memory. | L9 | A | Apply; it holds whether or not R8-12-9 defers the tool, since the trailer data cannot be backfilled. |
| **R8-12-40** (Advisory) 12 12.10: maintainerWaitSeconds mixes agent and outside PRs in one median, so the stranger's wait is hidden. | L14 | L | State as a limit; the split is deferred to its trigger. |
| **R8-13-1** (Blocking) 13; ADR 0006; 00 0.5; 12 12.2: The runtime ledger is a v1 feature that traces to no success criterion, journey or 00 0.5 row, and ADR 0006 records no 'not now' alternative under ADR 0005 rule 1. | L0, L11 (high-signal) | operator | Changes the scope (00 0.5) and the decision record ADR 0006 either way. Recommendation: option 1 only if the maintainer can name the consumer that needs events from the first run; otherwise option 2, since ADR 0005 prefers deferring and the cost of waiting (unrecorded early events, one recreate per project) is stated and small. Regression check: one recommendation for the four ledger clusters, SC1. Decision SC1. |
| **R8-13-2** (Required) 13 13.5; I33; 05 5.4: A project created later under a retired project's name reads the retired project's entries, ids included, as its own, and no 05 5.4 row or deferred row carries it. | L0, L13b, L16 (high-signal) | operator | Recording the crossing as accepted adds a 05 5.4 row; the alternative adds a refusal. Recommendation: accept with the retire message and the deferred row, since the event fields are closed and the only agent-chosen value that crosses is `id`. Decision AR19. |
| **R8-13-3** (Required) 13 13.5; 05 5.4; ADR 0006: The no-delete rule binds the owner too, and the only remedy is rotation, which cannot apply to a project name the user chose and that every entry keeps. | L16 | operator | The absolute rule is the maintainer's (13 opening, ADR 0006) and the change edits a 05 5.4 row. Recommendation: accept owner removal as a stated limit of the rule; the 13 13.5 enforcement list already calls the host side a promise. Regression check: supersedes op-ledger-absolute (round 5: 'no exception, no tombstone, no redaction event') by name if taken. Decision AR20. |
| **R8-13-4** (Required) 13 13.8; 05 5.4; ADR 0006: The spec records that six starting values and two residual risks were accepted with the design as a whole, not decided one by one. | L11 | operator | Only the operator can decide an accepted risk. Recommendation: decide the two risks by name now (the merge decision needs them) and leave the six values marked proposed, since the index already has a deferred row that reopens them. Decision AR7. |
| **R8-13-5** (Required) 13 (opening); 13 13.9: The page never says the ledger stays on the machine, so 'the maintainer reads them to improve the product' and 'which error ids users hit' claim a cross-user reader that does not exist. | L10, L14, L16 (high-signal) | M | Apply the wording; no opt-in export row and no romeu-side event now. The cross-machine gap is already the 'cross-machine sync of the ledger' and 'more event types and fields' deferred rows. |
| **R8-13-6** (Required) 13 13.9; 13 13.6; 04 4.3: A session never sees its own events through julieta event list, because the view changes only at a host ingest, and no page says so; 'no failures' looks the same as 'not ingested yet'. | L5, L13 (high-signal) | M | Apply the latency sentence and the pending count; the smaller form drops the extra ingest in stop and the SessionStart output, which add a step to two more commands for a signal the count already gives. |
| **R8-13-7** (Required) 13 13.4; 04 4.2; 04 4.4; 12 12.3: The list of commands that ingest is hand-written in 13 13.4, 04 4.2 and 04 4.4, beside the other cross-cutting side effects, so a new command must remember to join it. | L3 | M | On page 13, delete the member list and link to the generated 04 table; the generator and the other four lists of L3-6 (git keys, memory allowlist, mise environment, reserved names) belong to the 04, 05, 07, 08 and 12 consolidations. |
| **R8-13-8** (Required) 13 13.2; 13 13.9: Every non-zero julieta exit emits command-failed, usage errors included, so an agent's typos become permanent entries that read like real failures. | L5 | M | Smaller than excluding ids: the error id already names the usage error, so making ref required lets every consumer filter typos without a second rule. |
| **R8-13-9** (Required) 13 13.2; 13 13.10: The claim that every non-zero julieta exit emits command-failed is tested by one positive and three negatives, not over the command definitions. | L6 | A | Apply as proposed, with the ref clause from R8-13-8. |
| **R8-13-10** (Blocking) 13 13.7; 03 (opening); 03 3.6; 10 10.1: Read compatibility is uneven: the manifest and the event specify an N and N-1 window tested only through an injected seam, while the other stored formats say nothing about another version. | L3, L11 (high-signal) | M | Apply as one rule plus a stated limit; it keeps ADR 0006's N and N-1 window unchanged and does not add the product-wide ADR L3-3 proposed. Primary home is the 03 consolidation. |
| **R8-13-11** (Required) 13 13.2; 03 3.2; 03 3.4; 03 3.7; 03 3.8: One idea, the format version, has three spellings: apiVersion: romeu/v1, an integer version, and schema: runtime-event/v1. | L3 | M | Apply in 03 with the ledger's form; page 13 keeps its text. Primary home is the 03 consolidation. |
| **R8-13-12** (Required) 13 13.7; 04 4.1: 04 4.1 does not say that error ids are never reused and that removing or renaming an error id, a command or a probe id breaks the event format, which only 13 13.7 states. | L3 | M | Apply the cross-reference in 04 4.1; the id-width question belongs to the 04 consolidation. |
| **R8-13-13** (Required) 13 13.7; 03 (opening); 02 2.1: ledger-entry/v1 and the view files have no JSON Schema although 03 says every format has one. | L3 | M | State the exemption by name rather than add two schemas with no external reader. |
| **R8-13-14** (Advisory) 13 13.2; 03 3.8; 03 3.9; 03 3.10; 03 3.12: Ledger ids are lowercase ULIDs for an APFS reason while memory, handoff, candidate, generation and salvage ids are uppercase, and the stores use three timestamp naming pairs. | L13b | M | Apply the lowercase rule in 03 3.12; for timestamps, state the reason per store rather than rename. Primary home is the 03 consolidation. |
| **R8-13-15** (Advisory) 13 13.2: The ref domain admits invariant ids and probe ids, which no v1 producer writes except a hand-typed note. | L0 | A | Apply as proposed; it also removes two of the fan-in sources of R8-13-16. |
| **R8-13-16** (Advisory) 13 13.2; 12 12.3: The type, tool and ref tables are generated from several sources, so every PR that adds a command, catalog key or error id regenerates the same schema and reference page. | L12 | L | State the fan-in as a limit; with R8-13-15 the sources drop to four and a regenerate is one command on short trunk-based branches (ADR 0002). |
| **R8-13-17** (Advisory) 13 13.10; 11 11.2: 13 13.10 assigns the I31 hard-link race on APFS to the host suite, and no host row or block B step owns it. | L6 | L | State the limit and delete the unowned claim instead of adding a host check. |
| **R8-13-18** (Advisory) 13 13.1: 13 13.1 reasons about a project created by a build older than this section, a pre-release history no v1 reader has. | L11 | A | Apply as proposed. |
| **R8-13-19** (Advisory) 13 13.9; 13 13.2; 07 7.5; 04 4.2: No escape hatch is counted, so tool: other and the override flags can become the main road unnoticed. | L3 | L | State that the count is a query the ledger already answers; no new mechanism on page 13. |
| **R8-13-20** (Advisory) 13 13.2; 13 13.9; 03 3.9: Nothing joins a memory lesson entry to the ledger event it explains. | L5 | L | State the citation convention as the limit; no new front-matter field. |
| **R8-adr-1** (Blocking) ADR 0008 (Consequences); ADR 0001 rule 8; ADR 0002 rule 3; ADR 0003; 05 5.4; 10 10.2 item a; 12 12.4; 12 12.9: ADR 0008 chose to leave the review and merge-gate claims of ADR 0001 rule 8, ADR 0002 rule 3, ADR 0003 and four spec sections standing beside their negation ('as measured on 2026-10-08'), with no Supersedes link, against ADR 0001 rule 3 (stale content is deleted, not caveated). | L2b, L3, L8 (high-signal) | operator | Needs the operator: it reverses ADR 0008's recorded choice 'We do not rewrite them' and adds a decision record on the decisions surface. Recommendation: accept; the spec-side deletions (10 10.2 item a, 12 12.4, 12 12.9) can be applied in the same write pass once the record is approved. R8-adr-12 (ADR 0001's deferral count) rides in the same record. Regression check: the record names op-ruleset-approvals (round 6) and N1 (round 3) as superseded in part. Decision DR1. |
| **R8-adr-2** (Blocking) 13; ADR 0006; 12 12.2; 00 0.5: The runtime ledger is a v1 feature with no success criterion, no journey, no 00 0.5 row and no 'not in v1' alternative in ADR 0006, yet it sits on the build path before romeu run. | L11 | operator | Needs the operator: it changes the scope (00 0.5) and a decision record (ADR 0006). Recommendation: defer the ledger out of v1 as stated, per the last-responsible-moment rule; the stated cost of waiting (events before it exists are not recorded, one recreate per project for the mounts) is small next to its v1 footprint. Decide together with R8-adr-15. Regression check: one recommendation for the four ledger clusters, SC1. Decision SC1. |
| **R8-adr-3** (Blocking) 10 10.2; index: Deferred decisions; ADR 0007; ADR 0001; 12 12.5: Check mechanism is written as prose with pinned versions, commit ids and function names (10 10.2, about 330 lines; ADR 0007, about 300 lines; ADR 0001's rules) while tools/ci and mise.lock are the truth and nothing holds them equal, and accepted records are never rewritten. | L3 | M | Apply in the smaller form stated in change: the 12 12.5 rule, version strings in 10 10.2 replaced by pointers to their pin files, and the precedence sentence; the generated docs/reference/ci.md and the version-string docs rule are not added (one mechanism less; reopen if a stale 10 10.2 row misleads an implementation). See DR1. |
| **R8-adr-4** (Blocking) 05 5.4; S1; J1; 10 10.2 (Release and bootstrap); ADR 0008 decision 6: Release signing exists only as a prohibition (ADR 0008 decision 6: the credential is outside GitHub, signing is an operator step) while S1's evidence says the attestation 'signed them' and no page says what is signed, with which tool, where it is published or how a user verifies it. | L3 | operator | Needs the operator: it decides the meaning of ADR 0008 decision 6 and the release ask-first surface. Recommendation: the no-signature-in-v1 branch with the deferred row, which matches the last-responsible-moment rule and removes the false 'signed' from S1. Decision DR2. |
| **R8-adr-5** (Required) ADR 0007 (Threat model); index: Deferred decisions; plan.md: ADR 0007 defers the two pre-tools/ci escapes (an untracked go.work in the tree or a parent directory, a caller GOFLAGS such as -overlay) to Q25 and to a plan row 'whether agents push from a second account' that exists in neither plan.md nor the index; Q25 was decided by ADR 0008 on 2026-10-07, so the trigger fired, nothing reopened it, and ADR 0008's inventory of texts it leaves stale omits ADR 0007. | L0, L2, L2b, L5, L8, L11, L13, L13b, L14 (high-signal) | operator | Needs the operator: the pointer fix touches accepted records (ADR 0007, ADR 0008) and the close-now option touches the hook, an ask-first checks surface. The index row itself stays inside the spec and can be applied now. Recommendation: close it now with GOWORK=off and GOFLAGS unset in the hook (two variables, cheaper than keeping the row alive), recorded with the ADR 0007 pointer note; otherwise apply the row as written. Raised independently by both noise-probe pairs (L2 and L2b, L13 and L13b) and by nine lenses in all. Decision DR3. |
| **R8-adr-6** (Required) ADR 0008 (Context); 05 5.4; index: Deferred decisions; J1; J2; 06 6.4: ADR 0008's Context, and the 05 5.4 and deferred rows written from it, describe the sandbox the product is developed in (operator's full token, signing key plus a GitHub authentication key) without saying so, which contradicts 06 6.4's one-key socket for a romeu sandbox and leaves the product user with no guidance to narrow the token per project. | L1, L2b, L13 (high-signal) | operator | Needs the operator: the sentence changes how an accepted risk in 05 5.4 is scoped. Recommendation: accept as written; no ADR change is needed because ADR 0008 decision 2 makes 05 5.4 the single statement. Found by both L13 and L2b (each a member of a noise-probe pair whose partner did not raise it). Decision AR16. |
| **R8-adr-7** (Required) ADR 0008 decision 5 and Deferred 1; 05 5.4: The only control over the operator's token is a habit (read the GitHub security log at each sitting) whose coverage the record calls unverified, which cannot attribute an action to the agent, reviewed 'every 2 autonomous work sessions' that nothing counts, and whose collision with the first principle (no mechanism resting on memory) is not named as the ADR 0005 collision is. | L0, L2 (high-signal) | operator | Needs the operator: it changes ADR 0008 decision 5 and deferred item 1 and the accepted risk in 05 5.4. Recommendation: do the documentation read now (it is a measurement, cheap), name the collision, and make the review trigger the handoff checkpoint; skip the six-line checklist until the log read shows a gap. Decision DR5. |
| **R8-adr-8** (Required) index: Deferred decisions; ADR 0005 (Consequences); ADR 0008: Deferred triggers have no reader outside review rounds, which end with v1: many fire only when a person notices ('the maintainer observes it', 'written as an issue', 'the operator saying so'), several are machine-detectable but no row names its detector, although ADR 0005 Consequences asks for exactly that. | L3, L5, L9, L11, L14 (high-signal) | M | Apply in the smaller form stated in change: the reader-after-v1 sentence and '(seen by ...)' on the rows that already have a detector; the fourth column, the sequences check and the release-notes section are not added. ADR 0005 is not changed, since its Consequences already ask for the detector. The token row's session trigger is left to R8-adr-7, which needs the operator. |
| **R8-adr-9** (Required) index: Decide at the last responsible moment; ADR 0005 rule 1: The carve-out 'a security invariant, a merge gate and anything a later step depends on are needs of the start' is open-ended: anything becomes a need of the start once a later section is written to depend on it. | L11 | operator | Needs the operator: the index sentence restates ADR 0005 rule 1, so narrowing it changes a decision record. Recommendation: accept; it is the rule that makes R8-adr-2 and R8-adr-14 decidable. Decision DR4. |
| **R8-adr-10** (Required) ADR 0001 rule 7; ADR 0001 rule 9; 12 12.5: ADR 0001 rule 7 requires contiguous ADR numbers and tools/new adr takes the next free one, so the twenty or so records due in v1 collide when written concurrently, and rule 9's cite-by-number multiplies the cost of each renumber. | L12 | operator | Needs the operator: it amends an accepted record (ADR 0001 rule 7) and a check. Recommendation: the drop-contiguity form, which deletes a constraint instead of adding a reservation table; it can share the record of R8-adr-1. Regression check: relaxes the sequence check of det-seq (round 1). Decision DR1. |
| **R8-adr-11** (Required) ADR 0001 rule 13; 10 10.2 (docs step); skills/*/SKILL.md: The skills, the only documents written for the agent, fall under no rule of the documentation standard: rule 1 excludes SKILL.md, rule 13 checks commands in README and guides only, rule 16 is review, and only plan T078 adds a command-name test the spec does not require. | L5 | M | Apply in the smaller form stated in change: the requirement goes in 10 10.2, not into ADR 0001 by a superseding record. |
| **R8-adr-12** (Advisory) ADR 0001 (Consequences); index: Deferred decisions: ADR 0001 Consequences says 'Three things are deferred' while the index holds four rows citing ADR 0001 rule 5 (the linter and spell-checker choice too), and no front-matter field ties a hand-written page to the release or probe it was last verified against. | L8 | operator | Needs the operator: the count lives in an accepted record. Recommendation: fold the corrected list into the R8-adr-1 record; drop the 'verified:' key until a stale guide is found (the index is the authoritative list, so the miscount misleads only a reader of the ADR). Decision DR1. |
| **R8-adr-13** (Advisory) ADR 0001 rule 13; 02 2.1: Rule 13 fixes one committed file, e2e/scenarios/commands.yaml, that every host-facing command task appends to, so phase 4 tasks conflict on it. | L12 | L | State as a limit: grouped-by-journey hunks keep conflicts textual; the per-journey split waits for a conflict that costs more than a rebase. ADR 0001 rule 13 is unchanged. |
| **R8-adr-14** (Required) 12 12.10; ADR 0004; 10 10.2; 12 12.9: tools/ci dora, a five-metric delivery tool with its own schema, trailer and release asset, is a v1 deliverable for a one-contributor repository with no release, while ADR 0004 itself says nothing is lost by waiting. | L11 | operator | Needs the operator: it changes the v1 scope (00 0.5) of a deliverable a decision record adopted. Recommendation: defer as stated; ADR 0004's own argument supports it. Regression check: supersedes op-dora-late (round 6); see SC12. Decision SC12. |
| **R8-adr-15** (Required) 13 13.8; ADR 0006 (Context); 05 5.4 (last paragraph): Parts of the ledger were accepted without being decided: six starting values 'the maintainer has not read one by one', a design 'accepted as a whole', and two residual risks that 'came with the design the maintainer accepted'. | L11 | operator | Needs the operator: it changes accepted risks (05 5.4). Recommendation: decide with R8-adr-2; if the ledger leaves v1, the two rows and six values leave with it and nothing needs marking. Decision AR7. |
| **R8-adr-16** (Required) index: Open questions; ADR 0005 (Alternatives considered): The Open questions table holds items in the wrong state: Q25 is decided and still listed; Q4, Q6, Q12, Q13 and Q14 have a default taken and only 'maintainer' to settle them; Q9 (Homebrew tap later) and Q23 are deferrals without a trigger in the Deferred table. | L11 | M | Apply in the smaller form stated in change: table edits only; the five ADRs stay with T101 rather than moving to the docs module's first task. |
| **R8-adr-17** (Required) 01 1.7; 01 1.1; 05 5.3; 05 5.4; 09 (legend); 10 10.2; 00 0.2; ADR 0008 (Context): One person is 'operator', 'maintainer', 'developer' and 'user' depending on the page, none of the four is in the vocabulary, and the only sentence relating them is in ADR 0008 Context. | L3 | A | Apply as proposed; ADR 0008 is not edited (its Context already uses both words consistently with these rows). |
| **R8-adr-18** (Advisory) ADR 0001 rule 13; 10 10.2 (tools/release verify): Rule 13 exempts the checksum and attestation commands of the install and verify guides because release.yml 'runs that verification', but it runs tools/release verify, not the guide's command lines. | L7 | M | Apply in 10 10.2 only, which makes ADR 0001 rule 13's claim true without changing the record. |
| **R8-adr-19** (Advisory) 10 10.2 (workflows grammar; lint row); ADR 0001: The gate tool is specified in final form (about 27 subcommands, a closed workflow grammar, a meta-lint, a 20-rule documentation standard adopted before a page exists) with no mark of which steps the first-day path needs. | L11 | M | Apply the 10 10.2 marking only; the ADR 0001 rule split is dropped, since rules already land with the first page that needs them. Regression check: the marking matches the day-one list of 12 12.2 (R3-simplicity-02, round 6), or both change together. |
| **R8-adr-20** (Advisory) ADR 0001 rule 12; 12 12.8: Rule 12 fixes the README's level-2 headings with a check that fails an extra heading, and the list has no place for what romeu stores on the machine or how to stop using it. | L16 | operator | Needs the operator: it amends ADR 0001 rule 12 on the decisions and checks surfaces. Recommendation: defer to T095, the first input that needs it, and fold it into the R8-adr-1 record if that record is written first. Decision DR1. |
| **R8-adr-21** (Advisory) ADR 0006 (Alternatives considered): ADR 0006's alternatives treat every deletion as history rewriting and do not consider the owner discarding a project's entries or the whole ledger on the host, so there is no recorded reason for forbidding it. | L16 | operator | Needs the operator: it adds an alternative to an accepted record. Recommendation: moot if R8-adr-2 takes the ledger out of v1; otherwise decide with L16-4. Regression check: touches op-ledger-absolute (round 5). Decision AR20. |
| **R8-adr-22** (Advisory) ADR 0008 (what the token can reach); 05 5.4: The classes of what the operator's token can reach omit repositories of other organizations the account is a member of, such as an employer's, and 05 5.4 reuses the sentence for every project. | L16 | operator | Needs the operator: it widens an accepted risk in 05 5.4 by naming a reach the acceptance did not list. Recommendation: accept the clause; it is a fact of the account, and R8-adr-6's per-project token advice is its mitigation. Decision AR16. |
| **R8-plan-3** (Required) ADR 0007 (Threat model); index: Deferred decisions; plan.md: ADR 0007 defers the untracked go.work and caller GOFLAGS -overlay escapes to 'the plan's deferred-decisions row whether agents push from a second account', reopened by block O1 item d; that row exists in neither the plan nor the index, and its Q25 trigger fired on 2026-10-07 (ADR 0008). | L0, L2b, L11 (high-signal) | operator | Apply as written once the operator agrees: the index row is plain spec text, but the note in ADR 0008's Consequences changes a decision record. Not parked: L11-2 is a stage A finding on the index and ADR 0007, though L0-P7 and L2b-29 were parked. Decision DR3. |

## Parked for stage B

These clusters are findings on `plan.md`, marked `parked for B` by
their reviewers as the brief asked. They get no resolution here; stage
B reads them with the three artifacts named in lenses.md ("From stage
A to stage B"). R8-plan-3 is not in this table: its finding is on the
index and ADR 0007, and it stands in the table above.

| Cluster | Lenses |
|---|---|
| **R8-plan-1** (Advisory) plan.md (opening; What v1 leaves out; T096 to T100): The plan's hand-kept counts have drifted: 'the seven decision records under adr/' (there are eight), 'The table has 28 rows' (the index has 36), 'the six accepted records', '101 of 102 tasks'. | L0, L1, L2b, L3, L4, L5, L8, L9, L10, L11, L13, L13b, L14, L16 (high-signal) |
| **R8-plan-2** (Advisory) plan.md (Decisions this plan defers; T002): The plan's Decisions this plan defers table holds the apply tag of secrets, which is the index's Q18 under a second name, and T002's acceptance carries a history note that 12 12.5 sends to docs/reviews/. | L13b |
| **R8-plan-4** (Required) plan.md (T088): T088 accepts 'release.yml passes tools/ci workflows' under a 10 10.2 grammar that admits permissions of read or none only, while the same task's publish, attest, verify and attach steps need write and a token; its acceptance also has no line for the 05 5.4 operator signature, release immutability, the job's permissions or a build contract. | L0, L2b, L7, L9 (high-signal) |
| **R8-plan-5** (Required) plan.md (T019; T066; T075): T066 and T075's J3a acceptance ('starts or reattaches with sbx env run -d') inherits the 04 4.2 contradiction and needs a recording of sbx env run -d on a stopped sandbox that no block A definition makes (A4 records a stop and a listing). | L0, L1 (high-signal) |
| **R8-plan-6** (Required) plan.md (O5; O6; T094; T095): No task or block runs J1 on a clean account without the maintainer's reference config repo, and none measures cold-create or resume time; T094 and T095 are scored by review only. | L1, L10 (high-signal) |
| **R8-plan-7** (Required) plan.md (T022): No task produces an adopter starter; T022 builds examples/ for tests and docs only. | L10 |
| **R8-plan-8** (Advisory) plan.md (ci-bootstrap; T084; T095): The README lands in T084/T095, after the candidate tag, so the public first page stays two lines for the whole build. | L10 |
| **R8-plan-9** (Advisory) plan.md (Questions for the maintainer 19, 26, 28, 30, 31): Questions 19, 26, 28, 30 and 31 record spec defects (formats without a schema, the J12 step mismatch, guard packages on no ask-first surface, the untagged-field list, goldens without hash) that the plan patches in tasks while the spec text stands unpatched. | L0, L2, L6 (high-signal) |
| **R8-plan-10** (Advisory) plan.md (block O3): Block O3's --require-pass list is the only place that demands results for A2, A3, A5, A6, A9 and A14; 10 10.5 does not. | L0 |
| **R8-plan-11** (Advisory) plan.md (12 12.2 build order; T063 to T066): romeu run (T066) depends on the ledger view (T065), so the first run against real sbx waits on three ledger tasks no journey needs. | L11 |
| **R8-plan-12** (Advisory) plan.md (T012; The first slice): The first slice lands the whole pin tool (release-API fixtures, the immutable-release check) when it needs only the pin file. | L11 |
| **R8-plan-13** (Required) plan.md (dependency graph; T096 to T101): The longest dependency chain is about 36 merges deep with four operator blocks on it, and the T096 to T101 documentation chain depends on nothing but ADR numbering. | L12 |
| **R8-plan-14** (Required) plan.md (task fields; Decisions this plan defers: splitting this page): No task carries a file set, so concurrency is decided from module ids about ten times coarser than a task, and the page (2 920 lines) is about to fire its own 3 000-line split trigger. | L12 |
| **R8-plan-15** (Advisory) plan.md (Questions for the maintainer; operator blocks): Nine of thirteen maintainer questions are due at or before C0 or T019 with two answered, and T019, T052, T083, T084 and T088 each wait on an unanswered one. | L12 |
| **R8-plan-16** (Advisory) plan.md (Verify lines of T002, T009, T026, T092, T094, T095): Some Verify lines are not commands another agent can run. | L12 |
| **R8-plan-17** (Advisory) plan.md (Ask-first fields): About two tasks in three forecast an ask-first surface, and the forecast field has no standing in tools/ci pr. | L12 |
| **R8-plan-18** (Advisory) plan.md (T008; T029): T008 and T029 carry no acceptance for subprocess error details or timeout values and messages. | L14 |
| **R8-plan-19** (Advisory) plan.md (T012; T029): The re-probe rule after O3 is a comment in the pin file, not a check, and no task records a second sbx version. | L15 |
| **R8-plan-20** (Advisory) plan.md (T088; candidate tag): T088 tests publish, verify and --attach only against a gh helper and a recorded fixture, so the first real GitHub run of the release path is the candidate tag. | L15 |
| **R8-plan-21** (Advisory) plan.md (Phase 6; Phase 9; T069): No task builds a data inventory, export, owner removal or leaving journey. | L16 |
| **R8-plan-22** (Required) plan.md (O1; O3; O6; T026; T092): The maintainer runs the hook (go run ./tools/ci fast at main) in a plain host clone, and the plan asks to read a resolution commit's diff, not main's diff since the host last ran it; T001 to T003 are merged, so the window is open now. | L2 |
| **R8-plan-23** (Advisory) plan.md (T073; T074): No task injects a kill after each step of salvage, rm, recreate or retire, and T074 asserts salvage exits 3 on a gate 1 change, cementing L4-4. | L4 |
| **R8-plan-24** (Advisory) plan.md (T034; T061; T068): No task covers host-state migration or an undecodable project record, and T061 and T068 carry different completeness case lists. | L4 |
| **R8-plan-25** (Advisory) plan.md (T059; T078; T079): T059 has no acceptance for a SessionStart bound or hook failure, T078's test checks command names but not flags or headings, T079 has no test that a personal kit cannot unwire the hooks. | L5 |
| **R8-plan-26** (Advisory) plan.md (Verify lines; -run patterns): The plan guards against a -run pattern matching no test by having the reviewer read -v output. | L6 |
| **R8-plan-27** (Advisory) plan.md (T002; T007): T002's coverage fixtures miss a package absent from the profile; T007's mutate acceptance does not say what a stub may be. | L6 |
| **R8-plan-28** (Advisory) plan.md (T075; T002): T075 compiles the host suite in a Verify line (go vet -tags host) and no task adds it as a step of all. | L6 |
| **R8-plan-29** (Required) plan.md (Toolchain pins: gh): The Toolchain pins row for gh ('mise.lock, started through mise exec') contradicts the 10 10.2 lint row and its measured mise exec fallback. | L7 |
| **R8-plan-30** (Required) plan.md (T088; T090): T088 says the candidate tag is one 'the tag ruleset lets nobody retry under the same name', while 10 10.2 item d measured that the token creates and deletes v* tags through the bypass. | L7 |
| **R8-plan-31** (Required) plan.md (block O7; T052): Block O7 pins validate.yml in the reference config repo to the release, but no task makes that workflow verify what it downloads, and mise's public key exists only in T052, not in 07 7.4. | L7 |
| **R8-plan-32** (Advisory) plan.md (T094): T094 accepts the guides' error-id recovery as review only and depends on T092, after block B, so the guides are never run as written. | L8 |
| **R8-plan-33** (Advisory) plan.md (T002; T006; T066): T002, T006 and T066 are each more than one logical change. | L9 |
| **R8-plan-34** (Advisory) plan.md (O1; O2; O5 to O7): O1 and O2 say 'No task can proceed meanwhile' with no duration, the plan names no agent work during those blocks, and the rehearsal deferral fires only after a candidate fails block B. | L9 |

## Measurements

The lens page asks for three measurements per round, which decide
which lenses are rewritten before stage B: the `listed` to `own` ratio
per lens, the clusters only L0 raised, and the overlap of each
noise-probe pair. This section records them, with the coverage matrix,
the missing-viewpoint answers, the lens critiques and the regression
check beside them. They are measurements, not verdicts on the lenses;
the lens page decides what to do with them.

### Per lens

`listed` is a finding a bullet of the lens's examples prompted; `own`
is one the reviewer's own reading did. "Clusters the lens sits on"
counts cluster memberships, so a lens whose findings cross pages (L3,
L11) sits on more clusters than it has findings.

| Lens | Model | Findings (B / R / A) | listed : own | listed share | Parked | Clusters the lens sits on |
|---|---|---|---|---|---|---|
| L0 | fable | 52 (2 / 17 / 33) | 0 : 52 | 0% | 7 | 50 |
| L1 | opus | 30 (3 / 13 / 14) | 5 : 25 | 17% | 3 | 29 |
| L2 | fable | 24 (2 / 11 / 11) | 2 : 22 | 8% | 2 | 24 |
| L2b | fable | 29 (4 / 11 / 14) | 6 : 23 | 21% | 3 | 29 |
| L3 | fable | 34 (5 / 20 / 9) | 1 : 33 | 3% | 1 | 92 |
| L4 | opus | 43 (5 / 19 / 19) | 5 : 38 | 12% | 3 | 41 |
| L5 | opus | 37 (2 / 22 / 13) | 5 : 32 | 14% | 2 | 35 |
| L6 | fable | 41 (2 / 19 / 20) | 3 : 38 | 7% | 4 | 41 |
| L7 | opus | 34 (5 / 20 / 9) | 5 : 29 | 15% | 4 | 30 |
| L8 | opus | 31 (3 / 12 / 16) | 9 : 22 | 29% | 2 | 30 |
| L9 | opus | 30 (2 / 15 / 13) | 8 : 22 | 27% | 4 | 30 |
| L10 | opus | 32 (2 / 12 / 18) | 5 : 27 | 16% | 4 | 31 |
| L11 | fable | 35 (2 / 15 / 18) | 2 : 33 | 6% | 3 | 84 |
| L12 | fable | 30 (1 / 15 / 14) | 13 : 17 | 43% | 5 | 30 |
| L13 | fable | 37 (4 / 19 / 14) | 6 : 31 | 16% | 1 | 38 |
| L13b | fable | 43 (4 / 17 / 22) | 9 : 34 | 21% | 2 | 43 |
| L14 | opus | 24 (2 / 12 / 10) | 4 : 20 | 17% | 2 | 22 |
| L15 | opus | 25 (2 / 10 / 13) | 2 : 23 | 8% | 2 | 25 |
| L16 | opus | 25 (2 / 10 / 13) | 6 : 19 | 24% | 2 | 25 |
| all | | 636 (54 / 289 / 293) | 96 : 540 | 15% | 56 | 503 clusters, 729 lens memberships |

The share of `listed` findings is 96 of 636 across
the round. L0 has no examples by design and is all `own`; L12 is the
only lens whose examples prompted more than a third of its findings.

### Clusters only L0 raised

Seventeen clusters (16 in stage A, one parked) have L0
as their only lens. The count the lens page asks for, the count of
findings only L0 raised, is 17 as well: the L0 findings whose every
cluster has L0 as its only lens. The clusters:

- R8-00-5 (Required, 00 0.1; 00 0.2; 11 A11): The objective says nothing
  an agent wrote is ever executed on the host, while personal kits are
  agent-authored and sbx builds them at create, and no probe records
  where a kit's install step runs.
- R8-01-3 (Advisory, 01 1.2): The two workspace files are rewritten
  whole with no record of their hash, so a hand edit (for example a
  memory dir added as a folder, which boundary D forbids) stays live
  until the next promotion or retire and doctor has no row for it.
- R8-01-17 (Advisory, 01 1.5): Step 5 is justified as checks that 'can
  widen a sandbox behind romeu's back', yet it holds the VS Code trust
  check (I23), which widens no sandbox.
- R8-02-11 (Required, 02 2.6 (Consequences); 07 7.4; 10 10.3):
  'secondary repos need the project's git credential inside the sandbox'
  and 07 7.4's 'the github secret covers it' rest on a mechanism the
  spec never states (how a named secret becomes the credential git, gh
  and mise present to GitHub in the sandbox) and that no probe measures;
  10 10.3 lists credential injection under Cannot model.
- R8-02-17 (Advisory, 02 2.3 ($ROMEU_ROOT default); J1): The default
  root $HOME/dev must not be inside a git repository, so a home that is
  itself a git repository (dotfiles in $HOME) fails the default at J1
  and init does not say what happens.
- R8-03-27 (Advisory, 03 3.9): The memory entry digest is {title, body,
  created} and memory verify compares status only, so an entry whose
  tags (lesson among them) were stripped verifies equal.
- R8-04-14 (Required, 04 4.3 (pin workload, pin check); J9; 02 2.2):
  julieta pin workload and pin check reach the container registry from
  the config project's sandbox, and the config project's derived egress
  names no registry host, so J9's pin step is blocked unless the spec
  adds the host by hand, which no page says.
- R8-04-29 (Advisory, 04 4.3 (julieta salvage); 04 4.2 (romeu salvage)):
  julieta salvage takes an optional --stop-agents, J6 passes it for rm
  and recreate, and no row says whether a standalone romeu salvage
  passes it.
- R8-04-30 (Advisory, 04 4.1 (Locking)): romeu.lock is machine-wide and
  held through sbx env run, so creating one project makes romeu run of
  another exit 1 with RJ-101 although they share no state.
- R8-04-31 (Advisory, 04 4.2 (romeu run, exit codes)): romeu run lists
  exit codes 0-5 and no step of run exits 5.
- R8-07-7 (Advisory, 07 7.6; 07 7.5 step 3): julieta spec validate
  --catalog exits 1 on a lock host missing from the catalog while 07 7.5
  step 3 gates such a host and continues, so the config repo's CI stays
  red for a case the product accepts.
- R8-08-23 (Advisory, 08 8.4): The bundle excludes only objects
  reachable from repos[].base, which moves only on sync, so after pushes
  every snapshot re-bundles already-pushed objects and 'small' holds
  only right after a sync.
- R8-09-7 (Advisory, 09 J1): J1 has no step and no recovery for the
  dedicated signing agent that git-ssh-sign requires, so sync exits 2
  with RJ-204 and the guide has nothing for it.
- R8-10-12 (Required, 10 10.5; 11 11.4): The end-of-plan check requires
  results only for probes named in the index's Settled by column, so A2,
  A3, A5, A6, A9 and A14, A3 among them, can be skipped unnoticed.
- R8-11-10 (Advisory, 11 11.1 (A2)): A2 measures sbx re-prompting for a
  changed host command, and nothing measures the macOS keychain's own
  prompt when a secrets command such as security find-generic-password
  (03 3.4) runs from a non-interactive sbx env run.
- R8-13-15 (Advisory, 13 13.2): The ref domain admits invariant ids and
  probe ids, which no v1 producer writes except a hand-typed note.
- R8-plan-10 (Advisory, plan.md (block O3)): Block O3's --require-pass
  list is the only place that demands results for A2, A3, A5, A6, A9 and
  A14; 10 10.5 does not.

One of them, R8-04-14, is the defect R8-07-1 records on page 07 from
L13 and L13b, so the cluster-level count overstates L0's uniqueness by
at least one. What the rest share is the shape of the gap: a command's
edge behaviour or a check's scope (which probes, which exit code, which
host prompt) that no lens's reading list points at. The lens page's
bound says a cluster of findings only L0 raised may justify a lens;
these point at a reading line (each command row read against its exit
codes, flags and the commands that call it) more than at a new
viewpoint. That is an observation for the lens page, not a decision.

### The noise probe

The overlap of a pair is the clusters both readers raised over the
clusters either raised.

- L2 and L2b: 7 clusters with both over 46 with either, 0.15; 17 only
  L2, 22 only L2b. Both raised: R8-01-1, R8-01-27, R8-05-1, R8-05-3,
  R8-06-1, R8-10-1, R8-adr-5.
- L13 and L13b: 16 clusters with both over 65 with either, 0.25; 22 only
  L13, 27 only L13b. Both raised: R8-index-1, R8-index-28, R8-index-48,
  R8-01-33, R8-03-5, R8-03-29, R8-04-5, R8-04-6, R8-04-7, R8-05-2,
  R8-07-1, R8-07-2, R8-11-2, R8-12-11, R8-adr-5, R8-plan-1.

Two readers of the same lens found largely different defects. The
clusters both raised are high-signal by construction and include the
release-signing paragraph (R8-05-1), the workflow grammar against
release.yml (R8-10-1), the kit build location (R8-06-1), the exit code
of check findings (R8-04-5) and ADR 0007's fired trigger (R8-adr-5,
raised by both pairs). Read with the `listed` share (L2 2 of 24, L2b 6
of 29, L13 6 of 37, L13b 9 of 43), the measurement says the two lenses'
examples under-determine what a reader finds; most of each report is
the reader's own, and a second reader is worth the cost on these two
lenses.

### Coverage

Per page, from the coverage table of each report. No report misses a
coverage key, and no report cites a finding id in its coverage table
that is not among its findings. Every page has a finding from at least
fourteen of the nineteen reports, and an explicit "nothing for this
lens" from the rest, so the "at least two lenses" criterion holds for
every page.

| Page | Reports with a finding | Reports with "nothing for this lens" | Lenses that wrote "nothing" |
|---|---|---|---|
| index | 19 | 0 | none |
| 00 | 17 | 2 | L6, L12 |
| 01 | 19 | 0 | none |
| 02 | 18 | 1 | L8 |
| 03 | 19 | 0 | none |
| 04 | 18 | 1 | L9 |
| 05 | 19 | 0 | none |
| 06 | 19 | 0 | none |
| 07 | 15 | 4 | L4, L9, L12, L16 |
| 08 | 16 | 3 | L7, L9, L12 |
| 09 | 19 | 0 | none |
| 10 | 19 | 0 | none |
| 11 | 19 | 0 | none |
| 12 | 19 | 0 | none |
| 13 | 14 | 5 | L2, L2b, L4, L7, L15 |
| adr | 19 | 0 | none |
| plan | 19 | 0 | none |

Per criterion and journey, measured over the finding text (section,
defect, scenario) of every report, because the report format has a
coverage cell per page and none per criterion or journey, so a lens
that read a journey and found nothing leaves no mark here:

| Criterion or journey | Lenses with a finding naming it | Lenses |
|---|---|---|
| S1 | 10 | L0, L1, L2, L2b, L3, L7, L9, L10, L13, L13b |
| S2 | 2 | L6, L15 |
| S3 | 3 | L6, L9, L10 |
| S4 | 4 | L0, L6, L10, L13 |
| S5 | 5 | L5, L6, L9, L10, L15 |
| S6 | 5 | L0, L4, L6, L9, L12 |
| S7 | 6 | L1, L4, L6, L10, L11, L13 |
| S8 | 7 | L0, L1, L3, L5, L6, L10, L11 |
| S9 | 3 | L3, L6, L11 |
| S10 | 3 | L6, L8, L9 |
| S11 | 6 | L0, L1, L3, L7, L9, L10 |
| J1 | 15 | L0, L1, L2, L2b, L3, L6, L7, L8, L10, L11, L12, L13, L13b, L14, L15 |
| J2 | 5 | L1, L2b, L9, L12, L13b |
| J3 | 5 | L1, L2, L3, L10, L11 |
| J4 | 4 | L2, L8, L11, L13b |
| J5 | 2 | L5, L8 |
| J6 | 7 | L0, L4, L6, L8, L13, L14, L15 |
| J7 | 1 | L12 |
| J8 | 4 | L1, L5, L8, L15 |
| J9 | 9 | L0, L7, L8, L10, L11, L12, L13, L13b, L15 |
| J10 | 6 | L0, L1, L3, L4, L8, L12 |
| J11 | 7 | L1, L5, L8, L11, L12, L13, L13b |
| J12 | 9 | L0, L1, L3, L6, L8, L12, L13, L13b, L16 |
| J13 | 5 | L4, L6, L8, L12, L16 |

Thin spots by this measurement: J7 (retire a project) has a finding
from one lens, L12; S2 (CI green on the release commit) and J5
(handoff before `/clear`) have two each. Whether they are gaps in the
lenses or journeys that are simply right cannot be told from finding
text alone; the next report format should carry a per-criterion and
per-journey cell so that "nothing for this lens" can be counted there
too. That is a pending item for lenses.md, not a finding on the spec.

### Missing viewpoints, grouped

Each reviewer named the viewpoint missing from the set, given only the
lens titles. Grouped by theme, with the lenses that gave each:

- The human approver and reviewer at the gate and the merge, whose
  attention every security claim spends (approval fatigue, review load):
  L0, L5, L11
- The first outside contributor who is neither the maintainer nor an
  agent (onboarding, trust model for a third party, first PR): L3, L12,
  L13
- The cold newcomer or evaluator who meets only the README, the release
  page and the first error and decides whether to adopt: L1, L8
- The platform and upstream vendors (sbx, Docker, Anthropic, mise): what
  they actually do now and what they will change, and the channel back
  to them: L2b, L14, L15, L0
- Cost and sustainability as one budget (agent tokens, Actions minutes,
  CI time per merge, maintainer hours, Docker plan): L9, L10, L6
- Legal, licensing and organizational compliance (GPL-3.0-only, CC0,
  REUSE, third-party terms, MDM and org policy on a managed Mac): L10,
  L13, L16, L7
- Incident response and recovery on the remote (bad merge, deleted
  branch, bad release recall, token and key revocation, the GitHub
  account owner who holds the blast radius): L7, L4, L2b
- The reader after the loss: forensic reconstruction from probes,
  ledger, salvage and CI logs, and the person who does the restore: L6,
  L0
- The hostile agent inside the sandbox, tracing an attack from the
  inside out across pages: L2
- The implementing agent that reads one section cold, without the index:
  L13b
- Terminal environment and accessibility (screen readers, color, nested
  multiplexers, non-English locales): L1

Two themes were named by three or more lenses each and by no lens that
is itself one of them: the human approver and reviewer whose attention
every security claim spends, and the first outside contributor. The
lens page's measurement section decides whether a theme becomes a lens
or a line saying why none is needed.

### Lens critiques, one line each

Each reviewer said what its lens gets wrong about success for this
product. In one line each:

- L0: measures the spec against targets it wrote for itself, so it
  proves contradictions well and cannot say whether the product is worth
  using.
- L1: counts friction for one person and could trade away the gates that
  are the product's reason to exist; its findings read best as "make the
  safe step the short one".
- L2: treats "agent bytes are data on the host" as the whole of success,
  looks from the host inward, and would refuse the human checkpoint ADR
  0008 chose rather than make its radius explicit.
- L2b: its definition is satisfiable while the agent still holds the
  operator's whole GitHub identity; it counts a ten-thousand-line diff
  nobody reads as a control and leans toward adding checks over taking
  one decision.
- L3: prices stability above learning speed for a v1 written before any
  code; agents, not humans, inherit this spec, and "cheap to rebuild
  from the generated truth" may be the truer bar.
- L4: treats every byte in a sandbox as work, while much of it is better
  lost than kept and pushing early is the real defence.
- L5: serves the agent, who is also the adversary of 05; several of its
  findings widen what the agent learns, and it assumes one agent's hook
  semantics.
- L6: scores by whether a gate turns red, where author and reviewer are
  one account and a well-recorded host run is the strongest evidence
  available.
- L7: defines success as a stranger verifying bytes that never move; for
  one developer on their own machines the release risk is governance,
  not build engineering.
- L8: equates success with current pages, while the binary's own fix
  hints, exit codes and status are the stronger recovery path.
- L9: optimises the operator's idle time, while the operator's sittings
  are the security control, and the real failure is a correct v1
  abandoned for its running cost.
- L10: asks for no opinion of the author in a tool that is explicitly
  for one developer; the better test is whether a stranger can see,
  replace or refuse each opinion.
- L11: "nothing the start does not need" is the wrong unit twice: safety
  has no first-day user and still ships, and the maintainer is a present
  user; it discounts the cost of adding later.
- L12: scores a short critical path and disjoint files, where an
  ask-first surface and a maintainer merge are controls; shorten the
  path only where a person adds no judgment.
- L13: a spec can be consistent and wrong; it scores deliberate
  redundancy as two concepts and cannot tell a measured sbx fact from an
  invented one.
- L13b: rewards agreement over truth (the review requirement was stated
  consistently and was false everywhere); its findings are many and its
  Blocking ones few.
- L14: judges by strangers who report and return, in a v1 with no
  support promise; it pulls toward free-text logs where closed, redacted
  evidence is the better answer.
- L15: catching every upstream change before a user does is not
  achievable for one maintainer; the truer bar is that every upstream
  change fails closed with a named tested range.
- L16: imports a multi-party governance frame into a v1 for one person
  on one Mac; most of its value here is in words (an inventory, a
  leaving journey), not mechanisms.

### Regressions

The regression check read all 503 clusters against the round pages of
rounds 1 to 7. Of the clusters, 238 have an earlier round's resolution beside
them; the check found no case where an earlier fix had failed to hold:
each of them is a gap the earlier fix left open ("was never stated",
"not considered then"), or one of the reversals below. There are 74 entries on 72
clusters say the proposed change would reopen something an earlier round
closed or supersede a decision; the table carries the outcome of each:
63 rows are marked "Regression check", and the 9 whose status changed
are marked "Routed to the operator by the regression check" (6) or
"Amended by the regression check" (3). Two more rows, R8-01-22 and
R8-03-19, carry the "Regression check" mark for the conflict settled
below, which the check recorded among the earlier resolutions rather
than the regressions, so 65 rows carry that mark in all. In groups:

1. **Routed to the operator** (6): R8-06-1 reverses the
   accepted cost of op-personal-kits (round 6); R8-10-4, R8-10-44 and
   R8-11-7 reopen op-acceptance-after-release and op-probe-completeness
   (round 6); R8-00-7 is the v1.1 row that op-no-live-view (round 6)
   kept in 00 0.5; R8-01-21 is the deletion R8-04-19 already sends to
   the operator (craft C2, round 2).
2. **Recommendations that reverse a maintainer decision, already routed
   to the operator, now saying so by name**: the migration set
   (op-migration-set, round 6, in the maintainer's own words; SC2); the
   runtime ledger (op-ledger and the eleven op-ledger-* decisions,
   round 5; SC1); tools/ci dora (op-dora-late, round 6, and the round 4
   decisions op-practices, op-pairing-rule, op-backfill,
   op-maintainer-wait and op-pr-size that depend on it; SC12); S8 and
   `--timings` (te H4 round 1, R3-testability-14 round 6, with
   op-s8-limit's julieta assertion kept either way; SC3); `handoff
   list` (op-handoff-list, round 6; SC6); the v1.1 row (op-no-live-view;
   SC7); owner removal from the ledger (op-ledger-absolute and
   op-ledger-secret, round 5; AR20); the merge-gate text (N1 round 3,
   op-residual-risks round 4, op-ruleset-approvals round 6, superseded
   by the measurement ADR 0008 records; DR1); the checkpoint grant (N2,
   round 3; AR14); ADR numbering (det-seq round 1, craft C9 round 2;
   DR1); the pager clause (D8, round 1; AR10); the awaiting state (craft
   C2, round 2; SC5); the spec as an ask-first surface (the round 3
   decision on which documents are surfaces; AR13).
3. **Forms amended so that a fix does not undo a fix**: R8-10-52
   (resolved through R8-13-10, keeping dd N8 and op-ledger-versioned);
   R8-10-40 (the notes generator stays, craft C19); R8-05-25 (the
   detection stays out of `all`, R3-critic-04); R8-03-25 (a non-failing
   notice, R3-newcomer-01); R8-03-13, R8-04-36 and R8-07-10 (the caFile
   part dropped, te C1, and the macOS warning of R3-simplicity-06
   superseded by name); R8-04-21 (the probe, not the doctor row, D12);
   R8-03-12 (the limit beside I25, D12); R8-01-16 (drift and identity
   still stop every command, sa H2); R8-05-13 (ssh-add -c kept as the
   guide step, sa H5); R8-04-24 (the dispatcher's stage line only,
   R3-doubt-20); R8-00-6 (rewording only; the deletion branch is SC6);
   R8-06-20 (follows SC9, D27); R8-12-16 ("Depends on" keeps its
   plan-order meaning, R3-craftsmanship-01); R8-index-23 (follows SC3);
   R8-05-10 and R8-12-24 (R8-10-18's form keeps round 7's to-do items);
   R8-index-35 (R8-10-28's form); R8-07-7 (an unknown key keeps exit 1,
   R3-testability-08); R8-10-50 and R8-adr-19 (the day-one marking
   matches 12 12.2, R3-simplicity-02).
4. **Supersessions to record, no change of form**: R8-01-3
   (R3-doubt-17); R8-04-8 and R8-04-4 (craft C7's id scheme and the
   cited ids); R8-02-18 and R8-06-21 (Q21's default, dd N2);
   R8-index-17 (op-gate-run, op-finding-class, op-ledger-retention);
   R8-index-18 (op-maintainer-wait); R8-12-37 (M1's placement, round
   7); R8-10-6 and R8-12-17 (A-determinism-5 (a) for gh); R8-10-3
   (R3-critic-09's "never a draft"); R8-index-39 (op-equivalence stays
   in 10 10.2); R8-04-34 and R8-04-35 (the recommended branch keeps
   D3, sa M5, sa N4, te N10 and D27 intact).

Two conflicts between clusters that the check noted are settled here,
the first marked in both rows and the second through SC3: R8-01-22 and
R8-03-19 propose two forms for which candidate states persist (the 01
form, one sentence that approved is transient, stands, and 03 3.8 points
at it); R8-index-23 against the four S8 clusters (SC3 decides).

## Decisions for the operator

Everything the consolidation cannot decide, in one block, grouped by
what it changes: the scope (SC), an accepted risk or the security model
of 05 (AR), a decision record (DR). The 48 items (17 SC, 23 AR, 8 DR) cover
the 96 operator rows of the table, because the same decision was raised
under several cluster ids and is merged here. Each item names its
clusters, what it changes, the earlier decision it would supersede, and
the consolidation's recommendation. The recommendation is not a
decision; the operator's words, when given, are recorded beside each
item and in the spec text they change. Advisory findings outside this
block are applied by default, as the protocol says.

Three items decide others: SC1 (the ledger) decides AR7, AR19 and
AR20; SC12 (dora) decides the dora part of DR6 and R8-12-38; DR1 (the
superseding record) carries the status lines and notes that DR2, DR3,
DR5, DR7, DR8, AR4, AR6 and AR12 need.

### Scope (00 0.3, 00 0.5, success criteria, the v1 command set)

- **SC1 The runtime ledger in v1** (R8-00-1, R8-12-8, R8-13-1,
  R8-adr-2; Blocking). The ledger (13, ADR 0006) traces to no success
  criterion, journey or 00 0.5 row, ADR 0006 has no "not in v1"
  alternative, and 12 12.2 builds it before `romeu run`. Supersedes, if
  deferred: op-ledger and the eleven op-ledger-* decisions of round 5,
  which accepted the design as a whole. Options: (a) defer, as one
  Deferred row ("the runtime ledger; until then julieta state (08 8.2)
  and command stderr are the record; reopened by the first lesson
  naming a sandbox failure julieta state could not reconstruct, or a
  second machine"), 13 kept as a designed-not-built page like 06 6.6,
  the mounts (03 3.3), ingest steps (04 4.2, 08 8.5), I31 to I33, the
  ledger doctor rows and the plan tasks T063 to T065, T071 and T072
  leaving v1; (b) keep, with a 00 0.5 row naming the need and the
  rejected alternative written into ADR 0006. Cost of waiting, stated
  by ADR 0005's own rule: events before the ledger exists are not
  recorded, and the two mounts cost one recreate per project later (13
  13.1). Recommendation: (a), unless the operator can name the reader
  who needs events from the first run; three of the four clusters
  recommend deferral and the fourth recommends keeping only on that
  condition.
- **SC2 The one-time migration set** (R8-00-2, R8-03-4, R8-04-3,
  R8-08-21, R8-10-9, R8-11-1; Blocking). `julieta memory import` and
  `verify`, the `import/` allowlist entry, the memory-entry digest
  kind, the repo evidence kind, `tools/ci acceptance --file` and the C1
  check carry one person's migration into the generic product.
  Supersedes: op-migration-set (round 6), the maintainer's quoted
  answer that they stay in v1. Options: (a) delete the six and run the
  one-time import as a script in the operator's config repo (02 2.2
  already allows an operator `acceptance.json` there); the plan loses
  T058 and parts of T082 and T090; (b) keep them, rewrite the 00 0.5
  cell as a general need and mark the commands operator-only in 04 4.3.
  Recommendation: (a), knowing it reverses the maintainer's words; six
  concepts for one run on one machine is the cost the scope skeptic and
  the second adopter both priced.
- **SC3 S8 and `romeu run --timings`** (R8-00-8, R8-02-14, R8-03-16,
  R8-04-17; Required; R8-index-23 follows). The flag exists for the
  criterion and the criterion's evidence is the flag; the 2 s and 3 s
  bounds have no stated origin. Supersedes: te H4's CI measurement of
  romeu's half (round 1) and R3-testability-14 (round 6); op-s8-limit's
  julieta assertion in the container e2e stays either way. Options:
  (a) drop `--timings`, `run-timings.v1` and the CI assertion, keep S8
  as a harness measurement (B5) whose bound the first measurement sets;
  (b) keep the flag. In both, S8 gains "the 2 s and 3 s bounds are
  starting values; the first B5 measurement confirms or resets them".
  Recommendation: (a).
- **SC4 `romeu pull`, the review checkout and the workspace files**
  (R8-01-5, R8-02-13, R8-04-18, R8-05-31; Required; R8-index-30 and
  R8-01-4 follow). A host-side review path the start does not need,
  since ADR 0008 makes the pull request the review; `dev.code-workspace`
  is opened by no journey. Supersedes nothing decided, but removes the
  hardening rounds 1 and 6 built on pull (sa L1, D6, te M's I19 test,
  R3-security-08), which leaves with it, and two 05 5.4 rows.
  Recommendation: defer pull, the review checkout and
  `review.code-workspace` as one Deferred row ("until then the PR on
  GitHub is the review; reopened by the first time the maintainer needs
  a sandbox branch on the host before it is pushed, written as an
  issue"); delete `dev.code-workspace`; the one-sentence daily-setup
  statement in 02 2.3 (editing and debugging happen inside the sandbox;
  host clones are for reading in Restricted Mode) applies now.
- **SC5 The awaiting state and `approve <name>`** (R8-04-19, R8-01-21;
  Required; R8-01-31 and R8-04-7 follow). The non-TTY sync path has no
  named caller. Supersedes: craft C2's state machine (round 2) in part.
  Recommendation: a non-TTY sync exits 3 with the digest and no awaiting
  record; the awaiting and superseded rows and `approve <name>` leave;
  one Deferred row "approval of a candidate rendered without a terminal;
  reopened by the first scripted sync".
- **SC6 The 00 0.5 scope table and `handoff list`** (R8-04-20;
  Required; R8-00-6's deletion branch). The table lists six of sixteen
  romeu commands and justifies `julieta handoff list` by "kept in v1 by
  the maintainer". Supersedes, if the command leaves: op-handoff-list
  (round 6). Recommendation: one 00 0.5 row per command, with the need
  it serves on the first day, checked by `tools/ci sequences`; for
  `handoff list` the operator writes the need in one line, and the
  command leaves only if the operator withdraws op-handoff-list.
- **SC7 The v1.1 bucket of 00 0.5** (R8-index-3, R8-07-3, R8-00-7;
  Required). Four items (the native egress queue in `status`, `handoff
  --list` filters, a Remote-SSH helper, a registry credential) are
  promised to a release with no trigger and no interim; Q23 lives in
  two tables. Supersedes: op-no-live-view's "the v1.1 helper stays
  listed in 00 0.5" (round 6), in wording only, since the helper stays
  in the spec as a Deferred row. Recommendation: one Deferred row per
  item with its interim and an observable trigger, Q23 merged into the
  registry-credential row, the 00 0.5 sentence changed to "anything
  that does not is a row of Deferred decisions"; the 07 7.5 error hint
  and the J8 sentence apply now.
- **SC8 Gate 1** (R8-04-34; Advisory; R8-01-11 and R8-03-36, both L,
  share its finding L11-7). Gate 1 guards against an
  upgrade behind the operator's back on a machine where the operator
  upgrades. Recommendation: keep gate 1 and add one sentence to 01 1.4
  naming the threat it stops (an sbx or romeu binary replaced by
  another process or person on the machine); the fold-into-sync branch
  would remove D3, sa M5, sa N4 and te N10 and weaken a security gate.
- **SC9 `pin workload` and `pin check`** (R8-04-35; Advisory;
  R8-06-20 follows). Two sandbox commands for a rare manual act.
  Recommendation: keep both; `pin check` is the one freshness signal
  R8-04-28 and R8-02-4 lean on (D27, round 1), and `pin workload`
  states the both-arch manifest-list check as its reason.
- **SC10 `memory rm` and `memory search`** (R8-04-37, R8-08-22;
  Required; R8-08-22 is its page-08 form). `rm` is a delete path in a
  product that otherwise never deletes; `search` duplicates `list` with
  a filter. Recommendation: drop `rm` (`edit --status done` stays) and
  fold `search` into `list --query`.
- **SC11 Why the start needs commit signing** (R8-01-20; Advisory;
  R8-05-33 and R8-06-15 put the clause in 06 6.4 regardless). A product
  kit, a surface, a setting, a check and an open question with no
  stated reason. Recommendation: the sentence "the maintainer signs
  every commit and will not run an agent that cannot" in 00 0.5, if it
  is the operator's reason; otherwise the 06 6.4 clause alone.
- **SC12 `tools/ci dora` in v1** (R8-10-19, R8-12-9, R8-adr-14;
  Required; the dora parts of R8-10-3 and R8-12-38 follow; R8-00-9, M,
  shares the finding L3-24 of R8-12-9). A
  five-metric tool for a one-contributor repository with no release,
  whose own record says nothing is lost by waiting. Supersedes:
  op-dora-late (round 6) and leaves the round 4 decisions
  op-practices, op-pairing-rule, op-backfill, op-maintainer-wait and
  op-pr-size without their mechanism until the trigger. Recommendation:
  defer the tool (Deferred row: "until then `gh pr list --json` by hand
  and the review judge the three rows of 12 12.9 that cite it; reopened
  by the third deployment or the first lesson that wants a delivery
  number"), keep ADR 0004 Accepted as the design, keep the
  Fixes-release trailer check in `pr` because its data cannot be
  backfilled, shorten 12 12.10 to one paragraph; the plan loses T089
  and one release.yml step.
- **SC13 Two real-host smoke steps in the build order** (R8-12-35;
  Required). The product meets real sbx only at the release candidate.
  Recommendation: accept the two one-host smoke steps (after the sync
  slice; after run), pasted under Evidence and not committed as probe
  results; two short sittings are cheaper than one extra block B.
- **SC14 Merge cadence and the one-PR clause** (R8-12-10; Blocking).
  The no-stack rule plus a maintainer merge per PR make the calendar
  path equal to the graph's depth in sittings. Recommendation: apply
  the 12 12.9 clause (consecutive dependent tasks of one chain by one
  agent may ship as one pull request) and the changed trigger (the
  first checkpoint with more than N pull requests waiting); the cadence
  (a green PR with its approval lines merged in the next sitting; at
  least one sitting per working day) and N are the operator's, since
  they commit the operator's time.
- **SC15 Where the end-of-plan completeness checks and the pre-tag
  checks run** (R8-10-4, R8-10-44, R8-11-7; Blocking; routed by the
  regression check; R8-index-40, L, shares the finding L3-30 of
  R8-10-4). The four completeness checks run once after
  v1.0.0 is public, with no path for a failure then; the candidate
  comparison is checked only after the tag. Supersedes, as proposed:
  op-acceptance-after-release and op-probe-completeness (round 6).
  Recommendation: the completeness checks join `all` only once the
  plan's last task has landed (so `all` does not turn red before block
  B, the cost op-probe-completeness avoided); the pre-tag run of
  `acceptance` and the candidate comparison are maintainer steps of the
  release checklist in 12 12.2, outside release.yml (so
  op-acceptance-after-release's "leaves the release workflow" holds);
  a post-tag failure is a patch release; R8-10-44's release.yml step is
  not taken.
- **SC16 A user-facing success criterion (S12)** (R8-index-22, R8-00-3,
  R8-09-2; Blocking; routed by the gate review of this page). Three
  clusters take two positions on one scope question. R8-index-22 (L0,
  L1, L4) proposes S12: on both block B hosts, J1 from a clean host, J2
  and the daily resume of a stopped sandbox record the count of operator
  commands and prompts and the wall clock, compared with 00 0.2 in the
  acceptance PR, and the release candidate carries the maintainer's own
  work for two autonomous work sessions across two projects, each ending
  in a final handoff; the page consolidator took it in a smaller form,
  recorded counts and a review comparison with no invented bound.
  R8-00-3 (L10) and R8-09-2 (L16) decline S12: the first states in 00
  0.2 that v1 acceptance runs only with the maintainer's config repo and
  defers the stranger-run criterion with a trigger (the first issue from
  an adopter blocked in J1 to J3b); the second adds J14, stopping the
  use of romeu and julieta, under S7, which covers every journey. A
  success criterion is scope, which only the operator adds or declines.
  Recommendation: no S12 in v1; the counts R8-index-22 asks for become
  fields of the B3 result, compared with 00 0.2 in the acceptance PR's
  review; the 00 0.2 limit and the Deferred row of R8-00-3 land, the
  Deferred row naming a user-facing success criterion as the decision
  the first B3 measurement or the first adopter issue reopens; J14 lands
  under S7. That is what the three texts support: one asks for the
  measurement before the bound, two decline the criterion, and a bound
  set from a measurement is the principle the proposals were judged by.
- **SC17 workflow_dispatch and the S1 evidence** (R8-10-10; Required;
  routed by the gate review of this page). 10 10.2 fixes "the release
  workflow file must not come from the tagged commit" while the index
  defers its mechanism, and S1's evidence ("a ci-run of release.yml on
  the tagged commit") and J1's signer-workflow flags assume the
  opposite, so T088 must break one of them. The page consolidator
  proposed deciding the mechanism now: release.yml runs on
  `workflow_dispatch` from main with the tag as its input and refuses a
  tag whose commit is not on main; S1 and J1 follow; the deferred row
  closes. That answers a deferred decision, which stage A shapes and
  never answers (lenses.md, "What each round reads"), and changes S1, a
  success criterion. Options: (a) decide the mechanism now, as proposed;
  (b) keep the deferral and apply the smaller form, dropping "on the
  tagged commit" from S1's evidence and the ref-specific flags from J1
  until the row is decided, so that neither sentence contradicts the
  rule. Recommendation: (b); it removes the contradiction T088 would
  have to break and leaves the mechanism to the row's own trigger, at
  the cost of a less specific S1 evidence line until then.

### Accepted risks and the security model (05 5.3, 05 5.4)

- **AR1 Host indexers on agent-writable directories** (R8-02-10;
  Required). Spotlight, Quick Look and Time Machine parse memory dirs
  and the spool without the operator opening anything; boundary D's
  sentence is false for them. Recommendation: apply the
  `.metadata_never_index` marker written by sync, the doctor warning
  and the boundary D wording now; widen the 05 5.4 row to host
  indexers.
- **AR2 `adopt` records an ungated generation** (R8-01-27; Required;
  both noise-probe readers of L2). Recommendation: accept with the
  printed inventory before the prompt, the "adopted, not gated" status
  label and an A4 observation; refusing adopt would lose the J3
  recovery.
- **AR3 `sandboxOptions` ungated** (R8-03-35; Advisory). A merged spec
  can size the sandbox up to what sbx allows. Recommendation: the 05
  5.4 row ("accepted because every spec change is a reviewed PR the
  operator merges; reopened by the first spec merged without the
  operator's review"), not a gate-class change.
- **AR4 Text from strangers steering agents that hold the token**
  (R8-05-5; Blocking). Recommendation: the threat-paragraph clause, the
  05 5.4 row with the 12 12.9 working rule (agents read such text as
  data and take no merge, close, label, workflow or settings action on
  it without the maintainer's word), and the earlier ADR 0008 trigger
  ("the first issue or PR from an account other than the operator's"),
  recorded in DR1.
- **AR5 A PreToolUse refusal as a guard against mistakes** (R8-05-6;
  Advisory). Recommendation: accept; it moves a rule held in agent
  goodwill into a deterministic hook of the julieta-claude kit and
  states its limit (not against an agent that edits its own hook).
- **AR6 Split 05 5.4 by whose risk it is** (R8-05-8; Required).
  Recommendation: accept the split into "running romeu" and "how this
  repository is developed", the two cell rewrites and the scope line in
  ADR 0008 (DR1); no acceptance becomes wider or narrower.
- **AR7 The ledger's inherited risks and starting values** (R8-05-11,
  R8-13-4, R8-adr-15; Required). Two residual risks and six starting
  values were accepted "with the design as a whole". Recommendation:
  decide with SC1; if the ledger leaves v1 they leave with it; if it
  stays, decide the two risks by name now and leave the six values
  marked proposed, since the index already has a row that reopens them.
- **AR8 The salvage row's premise** (R8-05-12; Required). "A push to
  origin is the only copy an agent cannot take back" while the token
  can delete branches. Recommendation: the rewritten cell and the
  Deferred row for a create-only host ref; no new ref namespace now.
- **AR9 The signature row's mitigation** (R8-05-13; Required).
  Recommendation: point the mitigation at the `ssh-add -c` guide step
  of 06 6.4 (R8-06-13) instead of deleting confirm-on-use (sa H5, round
  1), and add the Deferred row for a signing path that is not
  forwarded.
- **AR10 How the owner reads lessons** (R8-05-14; Required). `romeu
  handoff` shows only handoffs. Recommendation: the smaller form (a
  pager that shows bytes and runs nothing; never a workspace folder)
  and a Deferred row for `romeu memory list|show`; the pager clause
  loosens D8 (round 1) and the operator decides it by name.
- **AR11 Memory as a steering channel** (R8-05-15; Advisory).
  Recommendation: one merged 05 5.4 row for the SessionStart print and
  `romeu handoff`, both labelled as agent-written text.
- **AR12 The maintainer runs agent-authored `tools/ci` on the host**
  (R8-05-16, R8-10-7; Blocking). The pre-push hook, `hygiene add` and
  blocks O3 and O6 run it at whatever main holds. Recommendation: one
  05 5.4 row with the standing control "the host runs tools/ci only
  from a commit whose diff since the last host run the maintainer has
  read, or from the last released tag", the index Never entry, the
  scoping of 00 0.1's host sentence to romeu's runtime, and the ADR
  0008 Consequences note (DR1); a trusted hooksPath checkout is a
  Deferred row.
- **AR13 The spec as an ask-first surface** (R8-01-35, R8-05-38,
  R8-10-20, R8-12-30; Required). `docs/spec/**` is on no surface while
  `tools/ci vocabulary` and `sequences` read its tables and 05 holds
  the security model. Supersedes: the round 3 decision on which
  documents are surfaces, by addition. The four clusters propose three
  forms. Recommendation: the three-file surface (`docs/spec.md`,
  `docs/spec/01-system-model.md`, `docs/spec/05-security.md`), with
  the generated denylist's path named outside `tools/ci`; the
  data-file alternative (the two inputs under `tools/ci`, rendered into
  the spec) is the fallback if the operator wants no specification
  surface.
- **AR14 A checkpoint grant for approval lines** (R8-05-4; Required).
  The approval line in the maintainer's words sits on most PRs.
  Supersedes: N2's one form (round 3), by adding a second.
  Recommendation: the grant for contracts, checks, dependencies, kits,
  catalog, ledger, digests and termsafe only; gates, gitsafe, signing,
  release, decisions and ask-first keep per-PR words.
- **AR15 os-base's floating apt delta** (R8-06-16; Required).
  Recommendation: option 1, the 05 5.4 row with its trigger and a
  Deferred row for pinning, decided after R8-06-17 (os-base may be an
  empty kit once A12 reports).
- **AR16 Whose sandbox the token rows describe** (R8-adr-6, R8-adr-22,
  R8-06-14; Required). 05 5.4 and ADR 0008's Context describe the
  sandbox this repository is developed in, not a romeu sandbox; the
  token's reach omits other organizations' repositories.
  Recommendation: accept the three together: the heading sentence over
  the token rows and the per-project token advice in J1 and J2
  (R8-adr-6), the organizations clause (R8-adr-22), and the 06 6.4
  scope sentence with the ADR 0008 Context rewording and the
  dogfooding Deferred row (R8-06-14; the Context rewording is a
  decision-record edit on the signing surface).
- **AR17 Salvage payloads are secret-bearing** (R8-08-2; Required).
  Recommendation: the modes (dirs 0700, files 0600), the 08 8.5
  paragraph and the printed line apply now; accept the 05 5.4 row for
  closed-generation salvage in the mounted memory dir and defer moving
  it out of the mount ("a second adopter or a shared host").
- **AR18 Measuring the sandbox token's reach** (R8-04-21; Required).
  05 5.4 and ADR 0008 call the reach inferred though it is readable.
  Recommendation, amended by the regression check: a block A probe run
  from the maintainer's own session that reads the token type and
  scopes, so "inferred" becomes measured; no doctor row, since it
  would have romeu read and transmit a secret (D12, I25).
- **AR19 A retired name reused in the ledger** (R8-13-2; Required).
  Recommendation: accept with the retire message, the I33 clause and a
  Deferred row for a project identity other than its name; moot if SC1
  defers the ledger.
- **AR20 Owner removal from the ledger** (R8-13-3, R8-adr-21;
  Required). The no-delete rule binds the owner too. Supersedes:
  op-ledger-absolute and op-ledger-secret (round 5) by name.
  Recommendation: accept owner removal as a stated limit of the rule
  (the host side of 13 13.5 is already called a promise), with the
  alternative recorded in ADR 0006 when it is next superseded; moot if
  SC1 defers the ledger.
- **AR21 A moved release tag** (R8-10-11; Required). Recommendation:
  the acceptance check "the tag's commit equals the source digest in
  its attested provenance" and the sentence in the token row; it runs
  where SC15 puts the checks.
- **AR22 The ruleset body try** (R8-10-39; Advisory). "An agent can
  disable the ruleset" stays inferred. Recommendation: a fifth
  reversible try at the next sitting, recorded in 05 5.4 and the next
  record.
- **AR23 Personal-kit install steps and lifecycle hooks** (R8-06-1;
  Blocking; routed by the regression check; both noise-probe readers of
  L2). Where a local kit builds and what its steps reach is measured by
  nothing. Supersedes, if the refusal is taken: op-personal-kits'
  accepted cost (round 6, "a personal kit can still run an install
  step; the gate diff is the control"). Recommendation: apply the 06
  6.1 Build sentence and the A14 observations now; offer the refusal
  of install steps and lifecycle hooks in personal kits as the option
  the operator may take once A14 reports where they run, so the
  accepted cost is superseded knowingly or kept.

### Decision records (`docs/adr`)

- **DR1 One superseding record for the merge gate and the ADR 0001
  amendments** (R8-adr-1, R8-05-2's ADR part, R8-10-8, R8-12-1;
  Blocking; with R8-adr-10, R8-adr-12, R8-12-19, R8-12-20, R8-12-26 and
  R8-adr-20 riding in it; R8-adr-3, M, shares the finding L3-26 of
  R8-12-26). The review requirement is stated in six
  places and measured false in one; ADR 0008 chose "we do not rewrite
  them". Supersedes: that choice, and in part N1 (round 3),
  op-residual-risks (round 4) and op-ruleset-approvals (round 6), by
  the measurement ADR 0008 records. Recommendation: accept one short
  record, dated, that supersedes in part ADR 0001 rule 8, ADR 0002
  decision 3 and ADR 0003 on the review requirement; the three gain a
  "Superseded in part by" status line and ADR 0001 rule 7 admits that
  form (checked by `tools/ci sequences`). The same record carries: ADR
  0001 rule 7 keeping uniqueness and dropping contiguity (R8-adr-10;
  relaxes det-seq); the corrected list of ADR 0001's four deferred rows
  (R8-adr-12); the issue row of ADR 0001's lead-practice table
  (R8-12-19; the issue forms and outline lines apply now); License in
  ADR 0001 rule 12's README headings (R8-12-20, before T084); the ADR
  0007 status line on its superseded cross-reference and the 12 12.5
  rule that mechanism belongs to the spec (R8-12-26; the rule applies
  now); the "data and uninstall" README heading (R8-adr-20) if the
  operator wants it before T095, otherwise deferred to T095; the ADR
  0008 Consequences notes of AR4, AR12 and DR3; the scope line of AR6.
  The spec-side deletions (10 10.2 item a, 12 12.4, 12 12.9) land in
  the same write pass once the record is approved.
- **DR2 Release signing** (R8-05-1, R8-10-2, R8-adr-4; Blocking;
  eight lenses on R8-05-1, both noise-probe readers of L2; R8-index-24
  and R8-index-25 follow). The 05 5.4 paragraph names an operator-held
  credential that signs releases, and no page says what is signed, how,
  where it is published or how a user verifies it; S1 calls the
  attestation the signature. Changes ADR 0008 decision 6. Options: (a)
  v1 publishes no signature beyond `checksums.txt` and the keyless
  provenance attestation; the paragraph says what the attestation
  proves and does not prove; one Deferred row ("an operator signature
  over checksums.txt made outside GitHub; reopened by the first release
  whose tag the maintainer did not push, a second contributor, or the
  token narrowing of ADR 0008"); "signed" leaves S1; ADR 0008 decision
  6 is reworded in DR1's record; (b) name the signed object, tool, key,
  published fingerprint, the operator step in 10 10.2, the J1 verify
  command, an S1 evidence item and a B1 predicate. Recommendation: (a);
  (b) adds a key to keep and an operator step per release for a control
  no second user needs yet.
- **DR3 ADR 0007's fired trigger** (R8-adr-5, R8-index-13, R8-plan-3;
  Required; nine lenses, both noise-probe pairs). ADR 0007 defers the
  untracked `go.work` and caller `GOFLAGS` escapes to Q25 and to a row
  that exists nowhere; Q25 was decided on 2026-10-07 and nothing
  reopened them. Options: (a) close now: the hook starts `tools/ci`
  with `GOWORK=off` and `GOFLAGS` unset (two variables on the checks
  surface, ask-first), with the ADR 0007 pointer note in DR1's record;
  (b) one index Deferred row with the trigger "the next change to
  .githooks/pre-push" and the same pointer note. Recommendation: (a);
  it is cheaper than keeping the row alive. The index row itself is
  spec text and can be written now in either case.
- **DR4 The carve-out of "decide at the last responsible moment"**
  (R8-index-12, R8-adr-9; Required). "A security invariant, a merge
  gate and anything a later step depends on" is open-ended and
  contradicted by five deferred rows. Changes ADR 0005 rule 1.
  Recommendation: narrow it to "anything a step of the first-day path
  (J1 to J3, J5, J6) depends on; a dependency that exists only because
  a later section was written does not count", a part of one deferred
  only as a risk accepted by name in 05 5.4, mirrored in ADR 0005 by a
  superseding note; it is the rule that makes SC1 and SC12 decidable.
- **DR5 The token's review trigger and ADR 0008 decision 5**
  (R8-index-5, R8-05-7, R8-adr-7; Blocking; the security-log half of
  R8-11-7 follows). "Every 2 autonomous work sessions" is counted by
  nothing; the security-log reading is a habit whose coverage is
  unverified and whose collision with the first principle is not named.
  The three clusters propose three producers. Recommendation, one form:
  the trigger counts final handoffs of the product project ("the second
  final handoff since the date of the last review, written in this row;
  `romeu handoff --list` shows them"), which needs no tool and holds
  whether SC12 defers dora; the release-candidate trigger moves to the
  checkpoint before the last build phase (C7); the security log is
  worded as a judgment, not a detector, its documented coverage read now
  and recorded with its date; the collision with the first principle
  named in DR1's record.
- **DR6 Release immutability and ADR 0004's dora.json** (R8-10-3;
  Blocking). Nothing makes a published release immutable; dora.json is
  replaced on every release by design. Recommendation: apply ("a
  published release never changes; a fix is a new version"; GitHub's
  immutable-releases setting as a maintainer-block item; `tools/release
  publish` creates a draft inside one call, uploads every asset and
  publishes, reconciled with R3-critic-09's "never a draft"; "replace
  an asset of a published release" in the index Never list); the dora
  part falls away if SC12 defers the tool.
- **DR7 What mise may install** (R8-10-6; Blocking; R8-12-17 follows).
  Four texts disagree (the lint row, ADR 0007 rule 8, 12 12.1, ADR
  0001 Consequences). Changes ADR 0007 rule 8 and ADR 0001
  Consequences. Recommendation: `gh` in `mise.lock`, the allowed
  `[tools]` set written once in 12 12.1 with the lint test reading it,
  the two records pointing at it in DR1's record, and `tools/release`
  starting `gh` by its resolved path rather than `mise exec` (recorded
  as superseding A-determinism-5 (a) for gh).
- **DR8 `all` is offline except vulnerabilities** (R8-10-33;
  Advisory). govulncheck reads the network inside `all` while ADR 0001
  rule 5 says `all` stays offline. Recommendation: apply; the 10 10.2
  sentence lands now, the ADR 0001 rule 5 sentence in DR1's record.

## Exit criteria of stage A

Of the five criteria in lenses.md, one holds in full at this commit (the
three measurements are recorded above), one holds for the pages and is
unproven for the criteria and journeys (every page has a finding or an
explicit "nothing for this lens" from at least two lenses, nineteen
reports, no coverage key missing; the per-criterion and per-journey half
is measured only over finding text and shows J7 at one lens), and three
wait: "no Blocking open, every Required fixed or declined with the
operator's words" waits for the Decisions block and the write pass (60
Blocking and 252 Required clusters; 96 operator rows, 82 of them
Blocking or Required); the shape of every open question and deferred row
waits for the write pass (R8-index-1 to R8-index-21 and R8-adr-16 found
rows with no trigger, fired triggers and duplicates); and the targeted
re-audit has not run.

## From stage A to stage B

Three artifacts leave this round once the write pass lands: this page
with the operator's answers recorded; the list of sections the write
pass changed, by number as the spec cites itself; and the plan delta,
the tasks whose "Implements" field cites a changed section. The
33 parked clusters above are stage B's first input beside
those three.
