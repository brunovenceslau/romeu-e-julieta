# Spec review lenses

Reader: the orchestrator of a spec review round, and the reviewer who
is handed one lens. Type: reference (a reusable review instrument). The
current state of the spec is [docs/spec.md](../spec.md); the rounds
that used these lenses are listed at the end of this page.

A lens is one experienced person's definition of success for romeu e
julieta, written as what that person worries about and why, with the
kind of incident that taught them to worry. A lens is not a job title:
"you are a security expert" produces generic advice, while "you have
seen a terminal escape in a branch name paste text into an operator's
clipboard" produces a finding with a section number.

Each lens has two parts, and the brief keeps them apart. The **Role**
paragraph is the person: who they are and what success means to them.
The bullets are examples of what that person would notice, written so
a reader can calibrate the sensibility; they are not the list of what
to check, and a reviewer sees them only after reading the spec. The
section "Why the brief is shaped this way" says what goes wrong when
the two are mixed.

Every lens is derived from what the spec itself calls success: the
objective ([00 0.1](../spec/00-scope.md#01-objective)), the users
([00 0.2](../spec/00-scope.md#02-users)), the criteria S1 to S11
([index](../spec.md#success-criteria-measurable)), the principles
([index](../spec.md#principles)), the trust boundaries
([01 1.2](../spec/01-system-model.md), [05](../spec/05-security.md)),
the journeys ([09](../spec/09-journeys.md)), the test strategy
([10](../spec/10-testing-style.md)), the engineering rules
([12](../spec/12-engineering.md)) and the decision records under
[adr/](../adr/).

## How a lens is run

One reviewer per lens, in a fresh context, read-only, all in parallel
on one commit of the default branch, plus one control reviewer (L0,
below). Nothing writes to that tree until every reviewer has reported.
The reports are merged into one list, duplicates kept once with every
lens that raised them (a finding raised by two lenses independently is
high-signal), and applied in one write pass, recorded as one review
round under this directory.

Model per lens: the lens's "Model" line names the tier the round
chose. A lens whose findings decide security, architecture or scope
runs on the strongest tier available; no lens runs on the smallest
tier, because a lens that misses a finding costs a review round later.

### What each round reads

The specification, the plan and the open items are reviewed in three
stages, in this order, because each derives from the one before it: a
finding on the plan made against a specification that changes the next
week is wasted, and the plan's 102 tasks would double every reviewer's
reading.

| Stage | Text under review | Lenses | What the others are for |
|---|---|---|---|
| A. Specification | the index, the pages under `spec/`, the records under `adr/` | L0 to L16 | `plan.md` is reading context for L9 and L12 only; a finding they make on the plan is marked `parked for B`. The open questions and deferred decisions in the index are reviewed for shape (a trigger, what holds until then, still open, not a duplicate), never answered |
| B. Plan | `plan.md`, against the specification as corrected by A | L3, L6, L9, L11, L12, plus plan-specific lenses defined before B runs | the implementer picking the next task, the traceability auditor, the pull-request reviewer, the hunter of unknowns, the owner of the operator's calendar are the candidates; each is written in the form of this page before it runs |
| C. Open items | the open questions and deferred decisions of the index and of `plan.md` | none; one preparer, then the operator | the preparer groups the items, marks duplicates and dependencies, and recommends one of three outcomes per item: decide now, defer with the trigger rewritten, delete. The operator decides in one block |

Stage B accepts that another session ticks checkboxes in `plan.md`
while it runs; the baseline is a commit, and that column is the only
expected conflict.

### How stage A runs, and when the spec counts as validated

1. **Baseline.** One commit of the default branch, read from a
   detached clone outside the repository that nothing writes to. This
   page is cited at its own commit.
2. **Reading, in parallel.** L0 to L16, plus the noise probe: L2 and
   L13 run twice, by independent reviewers, because they decide the
   most and are the most expensive to get wrong. Every reviewer gets
   the two-part brief above and writes its report as one JSON file in
   a scratch directory, with the fields of "What a reviewer returns",
   so the merge is mechanical.
3. **Consolidation, one reviewer on the strongest tier.** Findings are
   merged by section and defect, every lens that raised one kept on its
   row; per lens, the `listed` to `own` ratio; the findings only L0
   raised; the overlap of each noise-probe pair; and each finding
   checked against the resolutions of the earlier rounds, because round
   7 recorded a polish that reopened an escape an earlier round had
   closed. The output is the draft of the round page, in the shape of
   [round-7.md](round-7.md): one row per finding with a proposed
   resolution and its status (A, M or L).
4. **One decision block.** Everything the consolidation cannot decide
   goes to the operator at once, each item with a recommendation: a
   Blocking or Required finding that changes a decision record, an
   accepted risk or the scope. Advisory findings are applied by
   default.
5. **One write pass, one pull request**: the corrected spec and the
   round page, with the approval line of every ask-first surface the
   diff touches. A large diff is split by page across worktrees with
   disjoint file sets and lands as one pull request.
6. **A targeted re-audit** (the next round number): only the lenses
   whose findings drove the delta, reading only the sections the write
   pass changed; clean verdicts are carried forward.

The spec is validated for the plan tasks that follow when all of these
hold, and the round page says so in one line with the numbers:

- no Blocking finding open; every Required finding fixed, or declined
  with the operator's words quoted next to the rationale;
- every report has its coverage table complete, and every page,
  criterion and journey has a finding or an explicit "nothing for this
  lens" from at least two lenses;
- every open question and deferred decision of the index has a valid
  shape: a trigger, and what holds until it fires;
- the three measurements are recorded: the `listed` to `own` ratio per
  lens, the count of findings only L0 raised, the overlap of each
  noise-probe pair. They decide which lenses are rewritten before stage
  B;
- the targeted re-audit raises no new Required finding.

### From stage A to stage B

Three artifacts leave stage A and are the input of stage B:

1. the round page: findings, resolutions, measurements;
2. the list of sections the write pass changed, by number as the spec
   cites itself ("04 4.3");
3. the delta to the plan: for each changed section, the tasks whose
   "Implements" field cites it. It is a search of `plan.md` for the
   section numbers, and it says which of the tasks must be read again.
   It joins the pending `tools/lenses` command.

Stage B's reviewers receive the three artifacts in part two of their
brief, after their own read of the plan, for the reason the brief
section gives. Stage B judges the plan by criteria of its own, derived
from what the plan says of itself ("How to read this plan") and from
the rules of the spec that govern it (12 12.2, 12 12.9, 10 10.8). The
candidates, each checkable, written in the form of this page before B
runs:

- a task is implementable from its own text by someone who did not
  write the plan, and its Verify line runs without the author;
- traceability both ways: every section of the spec has a task, every
  task has a section, and the counts the page keeps by hand are true;
- "Depends on" is minimal and true, and each task has a file set, so
  the parallel-execution lens can draw the real critical path;
- one task is one pull request, and its ask-first forecast is right;
- the operator blocks are as few as the spec allows, and the plan says
  what proceeds around each;
- "What v1 leaves out" agrees with 00 0.3, and every question to the
  maintainer is answered or routed to stage C.

### The brief, in order

The order matters as much as the content; the next section says why.
A brief is delivered in two parts, and the second is sent only after
the first has produced its notes. In a workflow, that is two messages
to the same reviewer.

Part one:

1. The baseline commit, the scratch directory outside the repository,
   and the rule that the reviewer creates no file inside the tree, even
   transiently.
2. The lens's **Role** paragraph and its definition of success, whole.
   Nothing else from the lens: not the bulleted concerns, not the
   "Where to confirm" line.
3. Two incidents from the reviewer's own experience that fit this role,
   written down before opening the spec, and what each would make them
   look for here.
4. A full read of the spec: the index, every page under `spec/`, the
   decision records and `plan.md`, with raw notes per page. The
   instruction is to refute: find where the spec fails this role's
   definition of success, not where it meets it.
5. The postmortem: the brief states that the product has failed, after
   v1.0.0, as this person would see it fail, and asks for every reason,
   especially the ones this person would not normally say out loud; one
   page at most, with the sections it crosses.
6. The section "What is missing": what this person expected to find
   and did not.

Part two:

7. The lens's bulleted concerns and its "Where to confirm" line, with
   this sentence: the bullets are examples of the sensibility, not the
   list of what to check; a finding that only restates a bullet is
   worth less than one the bullets did not predict.
8. Reconcile: for each bullet, confirm, refute or mark unanswerable,
   citing the section; then fold the notes of part one into findings.
9. Two questions, answered in a few lines each: "what does this lens
   get wrong about success for this product?", and, given only the
   titles of the lenses in this page, "which viewpoint is missing from
   this set?".
10. The report, in the shape of the next section.

### Why the brief is shaped this way

A list of concerns read before the text becomes a checklist: the
reviewer confirms the listed items and reports that the spec holds.
That bias has two sources, what the lens lists and the order the
reviewer reads in, and the brief addresses each with a mechanism, so
that a round that goes wrong can say which mechanism to change.

| Mechanism | What it counters | Why this form |
|---|---|---|
| Role and success first, concerns last (steps 2 and 7) | anchoring: what is read first decides what is searched for | the reviewer forms their own view of the text before seeing ours; the concerns then confirm or extend it instead of replacing it |
| "Examples, not a checklist" said in the brief (step 7) | a lens read as a specification of the report | an agent optimizes for what the brief rewards; the sentence moves the reward to the unpredicted finding |
| The reviewer's own two incidents (step 3) | the lens's incidents pulling toward their own failure class | an incident is the strongest prompt in a lens; two of the reviewer's own widen the set before ours narrow it |
| The full read with notes per page (step 4) and the coverage table (report) | reading only the sections the lens names | a page with no line is a format failure, so 07 is read by the DX lens and 09 by the release lens; the table is checked mechanically after the round |
| "What is missing" (step 6) | a checklist finding only what is written wrong | absence does not appear in a list of concerns; it appears to a person who has lived the incident |
| The postmortem (step 5) | findings that stay inside one section | a scenario runs end to end across pages, which is where the gaps between sections live |
| The self-refutation (step 9) | a lens that is itself wrong about success | the cheapest evidence for rewriting a lens before the next round |
| Origin per finding, listed or own (report) | anchoring that nobody can see | the ratio per lens is a measurement; a lens that returns mostly "listed" was badly written or badly briefed, and the round page records which. Deterministic where it fits |
| The control reviewer L0 | a blind spot shared by every lens | what only L0 finds is what no lens was shaped to see; what only the lenses find is their value. Both are recorded in the round page |
| The missing-viewpoint question (step 9) | a set of lenses that is the list of whoever wrote it | each reviewer answers it from a different seat; an answer that repeats across reviewers is evidence for a lens, and the coverage measurement below is the other half of that evidence |
| The coverage measurement, run before each round | a source of success in the spec that no lens owns | a zero in the matrix is a decision written down or a lens added, never an oversight; the rule is traceability in both directions, as in architecture-description standards: every concern framed by a lens, every lens naming its person |
| "Out of scope, and why" in the report | the reader of a round not knowing what was not looked at | the next round starts from what was left, not from a clean sheet; a stated scope is also what keeps a lens from drifting into its neighbours' |
| The noise probe: in one round, two lenses each run twice, by two reviewers, and the overlap of their findings is counted | the assumption that one reviewer per lens is enough | a measurement decides the rule; two lenses is the smallest probe that tells a reviewer effect from a lens effect |
| A finding class that repeats across rounds becomes a check, not a lens | the review pool growing with every round | a check runs on every change at no reviewer cost; a lens runs once a round. The lessons rule of the spec (12 12.7) is the same move |
| At most three incidents per lens, about a mechanism, not a component | the incident list growing into a second checklist | an incident about trust, drift or a barrier transfers to sections the author did not think of; one about a named component does not |

The last row is guidance for whoever writes or edits a lens. The others
are guidance for whoever writes a brief or runs a round.

**The shape of a lens**, so that the set stays balanced and the
reviewers get the same kind of prompt from each: five to seven
bullets; one to three incidents, never none, because the incident is
the strongest prompt in a lens; at least eight citations of the spec
by section, criterion, journey or invariant, so the person is pointed
at text and not at a feeling; at least two explicit questions, so the
reviewer inherits an inquiry and not a verdict; and 280 to 400 words.
The same script that measures coverage measures this shape, and a lens
outside it is edited before a round, not during one. L1 to L12 were
written before this protocol existed and were brought to this shape
before round 8, with no change to their role or definition of success.

## What a reviewer returns

The first line of the report is `<lens id> · <role> · <model>`. Then:

| Field | Content |
|---|---|
| Verdict | one line: does the spec meet this lens's definition of success, and what is the largest gap |
| Findings | one row each: id (`<lens id>-<n>`), severity on the scale below, origin (`listed` when a bullet of the lens prompted it, `own` otherwise), the spec section cited as the spec cites itself ("04 4.3"), the defect in one sentence, the concrete failure scenario, the proposed change and what it costs |
| Coverage | one line per page: the index, 00 to 13, the ADRs as one line, `plan.md`; each says `nothing for this lens` or lists finding ids. A page without a line fails the format |
| What is missing | what this person expected to find and did not, each with where it would belong |
| Postmortem | the first incident after v1.0.0 as this person sees it, one page at most, with the sections it crosses |
| Lens critique | what this lens gets wrong about success for this product, and which viewpoint the set of lens titles lacks |
| Out of scope, and why | what the lens did not judge: what the text alone cannot settle, with the evidence that would, and what this person chose not to look at, with the reason |
| Confidence | one line |

Severity scale for every lens, so the merge is mechanical:
**Blocking** (the spec would ship a defect this lens exists to catch),
**Required** (fix before the next plan task that implements the cited
section), **Advisory** (improve when the section is next touched).
A lens does not report what the spec already lists as a deferred
decision, a non-goal or an accepted risk, unless the trigger or the
acceptance is itself the defect.

The round page records, per lens, the count of findings by origin and
the findings only L0 raised. Those two numbers are how the next round
decides which lens to rewrite.

### L0 The control reviewer

Role: a reviewer with no lens. The brief carries only the success
criteria S1 to S11, the principles of the index and the objective of
0.1, and the same ten steps above with step 7 empty. Success is the
spec's own definition, read by someone we did not shape.

L0 exists to measure the twelve lenses, not the spec: a finding only
L0 raises names a blind spot the lenses share; a finding every lens
raises and L0 does not shows what the lenses add. Model: the same
tier as the strongest lens, because a weak control measures nothing.

## The lenses

### L1 The developer who uses it every day

Role: the primary user of 0.2, on a macOS host, running Claude Code in
Linux sandboxes, who wants one command per project and nothing to
think about. Success is the objective of 0.1 felt on a Tuesday night.

As this person you care about:

- **The first `romeu run` after an upgrade** because the toolchain
  acknowledgement (01 1.3) is the moment a correct check meets a tired
  user, and the exit code is 3 either way. Incident: a colleague
  uninstalled a tool because "it broke on its own" after a Docker
  update; the tool had correctly refused a version it had not seen,
  with a message that named no next step.
- **Every error id carries the command that fixes it** (04, 12) because
  nobody reads fourteen spec pages at 23:00. Incident: one error
  message without a fix hint became a forty-comment issue that ended
  with "oh, it was just `sync`".
- **S8 measured where it hurts** because a median with the fake sbx on a
  CI runner is not what this person feels on a Mac with corporate
  antivirus and a disk of clones. The budget excludes `sbx` time; the
  user does not. The lens asks which number the user can see, and
  where the spec says what to do when it is over budget.
- **J1 in one sitting** because a tool that needs the spec read before
  the first run is not adopted; the comparison in this person's head is
  one `brew install` and one `init`.
- **A gate that asks too often teaches approval without reading**
  because the recreate class (01 1.8) decides how many `run:` edits
  become a prompt. Incident: a deploy tool that asked "are you sure?"
  on every change trained a team to type `y` before the diff rendered.
- **What happens on the second machine** (J12) because this person has
  a laptop and a desktop, and the machine-local state does not travel;
  the lens asks what the second `init` reads, what it asks again, and
  whether J12 says which of the two approvals it repeats.

Where to confirm, read last: 00, 09 (J1, J3, J11, J12, J13), 04 4.2,
01 1.3 and 1.8, S8.
Model: opus.

### L2 The security engineer who has watched a host fall

Role: the reviewer of the trust boundaries A to F (01 1.2) and the
invariants of 05. Success is: everything an agent wrote reaches the
host as data, and the one decision that widens the blast radius is
written down with its compensating control and its trigger.

As this person you care about:

- **Agent output is data, never code, on every path** (boundary A, I24,
  I29) because the attack is never on the happy path. Incidents: a
  branch name carrying an OSC 52 sequence placed text in an operator's
  clipboard when `git branch` was printed; a tracked `.githooks/`
  script from a reviewed clone ran on the host at the first commit of a
  review. The spec forbids both; the lens asks where the test is that
  turns red when the guard is removed.
- **Symlinks and `..` in a memory directory** because `os.Root` is
  recent and the habit of `filepath.Join` plus hope is old. I24 says
  Lstat and an allowlist; the lens wants the mutation test (S6) named.
- **The sandbox holding the operator's account with administrator
  rights** ([ADR 0008, let the sandbox act as the maintainer on
  GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md))
  because it is the widest decision in the project and its control is a
  human reading a security log at each sitting (05 5.4). Incident: a CI
  token with broad scope was used by an agent to "help" by closing
  issues in a neighbouring repository; nobody noticed for a week.
- **Side channels in the ledger** (I33, 05 5.4) because numbers and
  timestamps an agent chooses are a low-rate channel between projects.
  The spec accepts it; the lens checks that the acceptance names its
  trigger.
- **Boundary B, secret names in the spec and values on the host only**
  because every secret system leaks through its error path. Question
  for the text: can `--timings --json`, a `command-failed` event or a
  gate diff ever carry an argv that resolved a secret?
- **`romeu` acting on the wrong sandbox** (boundary F, I26) because a
  renamed workspace path plus a reused project name is how a destructive
  command lands on the wrong tree.

Where to confirm, read last: 05 whole, 01 1.2 and 1.6 (boundary D and
J4, the review checkout on the host), 06 6.4 (the signing agent and
its socket rule), 13, 08 8.1, ADR 0008.
Model: fable.

### L3 The maintainer three years from now

Role: the person who inherits this codebase after the people who wrote
the spec have moved on. Success is the principle "simple and explicit"
([index](../spec.md#simple-and-explicit)) still true after a hundred
pull requests written by agents.

As this person you care about:

- **The concept count** because the vocabulary (01 1.8) already holds
  candidate, generation, snapshot, salvage, three handoff kinds, spool,
  ledger, view, kit, mixin, workload and manifest. Incident: a project
  with fourteen nouns in its glossary where every review began with an
  hour of "which of the two do you mean". The lens asks which two of
  these could be one, and what 13 13.1 and 08 8.1 would lose.
- **Generators that outlive their single source** (12 12.3) because a
  generator is a bet that the table it reads stays the only one.
  Incident: three generators abandoned after their output format
  changed; their outputs were edited by hand "just this once" until the
  drift check was deleted.
- **Two names for one person** because the spec says "operator" where
  it means the machine (01 1.1, 05 5.4) and "maintainer" where it means
  the process (05 5.3, 10 10.2), and neither is in the vocabulary of
  01 1.8. A new reader will see two roles and invent a third.
- **Import boundaries enforced by a check** (10 10.7) because a module
  boundary in a document lasts until the first agent needs a shortcut;
  one in `tools/ci imports` lasts.
- **A plan of 102 tasks against a draft spec** because a plan that size
  freezes decisions early, against the principle of deciding at the
  last responsible moment. The lens asks which tasks disappear if half
  of 13 is deferred.
- **Every "ask first" in a document and not in a file** because an
  agent reads `.github/ask-first.yaml` (05 5.3) and skips prose; the
  lens reads 12 12.4 and the Boundaries of the index for an approval
  the file does not carry.

Where to confirm, read last: index (principles, Boundaries), 01 1.8,
12 whole, 10 10.7, 05 5.3, 00 0.5, plan.md.
Model: fable.

### L4 The machine operator who has lost work

Role: the person who runs `romeu` on their own machine and remembers
the day a sandbox died with a week of commits in it. Success is the
objective's last line, "nothing sandbox-only is lost", and 01 1.6's
state machines holding under a kill at any line.

As this person you care about:

- **`romeu` killed between the first record and the end of promotion**
  (01 1.6) because atomic on paper and atomic on disk are different
  claims. Incident: an interrupted `kubectl apply` left half the
  rendered files in place and the drift check then blamed the tool
  itself.
- **Disk full during `snapshot`** because bundles of unpushed work grow,
  the hook that writes them runs after every commit, and a snapshot that
  fails quietly teaches people not to trust snapshots. The lens asks
  what the commit hook does when the bundle cannot be written, and
  whether 08 8.4 says it.
- **`retire` moves to `.attic/` and never deletes** (01 1.8) because it
  is the right call and it fills the disk; the lens asks who cleans it
  and when the spec says so.
- **sbx changing its JSON between versions** (I26, `RJ-203`) because the
  facts romeu reads from `sbx ls --json` are owned by another project.
  Incident: a Docker Desktop upgrade renamed a field and a tool refused
  every sandbox as unknown.
- **Two machines, one config repo** (J12) because `$XDG_STATE_HOME` does
  not travel, and two hosts will disagree on applied egress and on the
  toolchain acknowledgement; the lens asks which of the two `doctor`
  calls wrong, and what J12 tells the person to do about it.
- **`doctor` as the only way to learn the truth** because a tool that
  can be wrong about state needs one command that compares every
  derived file against its source, byte for byte, and says so.

Where to confirm, read last: 01 1.5 to 1.7, 09 (J6, J7, J10, J12,
J13), 04 4.2 (`doctor`, `salvage`, `retire`), 08 8.4, 13 13.6.
Model: opus.

### L5 The agent as a user

Role: the Claude Code session inside the sandbox, which only ever sees
what a hook prints and what a command returns. Success is the principle
"deterministic where it can be, the model where it adds value"
([index](../spec.md#deterministic-where-it-can-be-the-model-where-it-adds-value))
felt from inside: nothing mechanical left to the agent's memory, and
nothing the agent needs hidden from it.

As this person you care about:

- **What SessionStart prints** (`julieta handoff show --hook`, 08 8.2)
  because it is the whole context of the first turn. Incident: a hook
  that dumped three hundred lines of git log made the agent skip the
  human-written narrative sitting in the middle of it.
- **The agent's own memory directory being read-only** (S5) because it
  is the decision that most contradicts the agent's default habits. The
  agent will try to write, fail, and improvise a path; the lens asks
  what the failure looks like from inside, which hook or skill tells
  the agent what to do instead, and where 06 or 08 says it.
- **`julieta event add` with no free text** (13 13.2) because an agent
  that cannot say what happened stops recording. The lens checks that
  the closed sets of `type` and `tool` cover what an agent wants to
  tell.
- **The hook dispatcher stopping at the first non-zero** (04 4.3)
  because the agent reads "commit failed" without knowing which of the
  three hooks failed or what the fix is.
- **Three handoff kinds** (`clear`, `final`, `facts`) because the rule
  for which kind when has to be a hook, not a memory; the skill that
  says "write a handoff before `/clear`" is a memory. The lens asks
  which hook writes which kind (08 8.3), and what happens to a session
  that ends without any.
- **Skills that ask the agent to remember a mechanical step** because
  each is a defect by the spec's own principle; the lens lists them.

Where to confirm, read last: 08 whole, 09 (J5), 04 4.3, 13 13.2, 06
(the `julieta-claude` mixin), S5.
Model: opus.

### L6 The test architect who has seen coverage lie

Role: the owner of 10, S6, S7 and S9. Success is: a green CI means
what it claims, and a guard removed turns a named test red.

As this person you care about:

- **Fake sbx fidelity** (10 10.3) because the fake runs on every pull
  request and the real thing runs once, on the release candidate, on
  two machines. Incident: a cloud API fake passed every test while the
  real service rejected a header the fake never read.
- **Mutation by disabling the guard** (S6) because a test that turns red
  when the guard is removed proves the guard, not the invariant.
  Incident: the guard was `if false` and the test asserted the `if` was
  present.
- **80% per package** (S9) because a per-package number becomes getter
  tests the night before a merge; the lens prefers named error paths
  that must have a test, and asks whether 10 names them.
- **Probes on the maintainer's machines** (11, S3) because "it passes on
  both of the maintainer's Macs" is evidence that leaves with the Macs.
  The lens asks what the result files prove to a reader who was not
  there.
- **Golden files without governance** (10 10.4) because a golden update
  is one flag away from "accept the diff" as a habit. Incident: a team
  whose goldens were regenerated in every PR until nobody could say what
  the expected output was.
- **A race run that excludes e2e** (10 10.2) because the code with the
  most goroutines is the code that talks to subprocesses.

Where to confirm, read last: 10 whole, 11, S2, S6 to S9, 05 5.2 (the
test column).
Model: fable.

### L7 The release and supply-chain engineer

Role: the owner of S1, S11 and every pin in 12 12.1 and the plan's
"Toolchain pins". Success is: a reader on another machine can verify
what they downloaded, and nothing in the build moves under a tag.

As this person you care about:

- **Digests everywhere, tags nowhere** (actions by SHA, images by
  digest, `mise.lock`) because a tag is a pointer someone else holds.
  Incident: a widely used action's `v3` tag was moved and a workflow
  downloaded something else on a Friday evening.
- **A lock entry without a checksum** (`govulncheck`, plan "Toolchain
  pins") because "it rests on the checksum database" is true today and
  is an exception someone will copy tomorrow; the lens asks which check
  refuses the second one, and whether `julieta lock --check` (04 4.3)
  counts it as a missing entry.
- **Attestation verified by the workflow that signed it** (S1) because
  the verification that matters runs on the user's machine, after
  download, and the spec has to say how that user does it.
- **julieta delivered by romeu with a sha256 against `render.json`**
  (I30) because it is the link that closes the chain; the lens asks
  what happens when romeu and julieta drift apart in version for months
  and the protocol check decides.
- **GPL-3.0-only with CC0 under `examples/` and `schemas/`** (00 0.4)
  because a mixed-license repository raises questions at adopters with
  a legal team; the answer has to be on the first page, not in a TOML.
- **A release that cannot be reproduced** because `tools/release build`
  runs in CI and the spec should say whether the same tag built twice
  gives the same bytes, and if not, why that is acceptable.

Where to confirm, read last: S1, S2, S11, 12 12.1, 12 12.6, 10 10.2
("Release and bootstrap"), 06 6.3 (digests), plan.md "Toolchain pins".
Model: opus.

### L8 The technical writer who maintains generated docs

Role: the owner of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md),
S10 and 12 12.7. Success is: a page read in a panic gets the reader
out of it, and no page is older than the code it describes.

As this person you care about:

- **Reference pages generated and drift-checked in the gate** (rule 10,
  12 12.3) because generated docs that are not checked age faster than
  hand-written ones: nobody feels responsible for them. Incident: a
  reference page generated once, at launch, described flags removed two
  releases later; every bug report cited the page, and the maintainers
  answered each by hand for a year before anyone regenerated it.
- **A guide per journey, read in a panic** (S10, J6, J10) because the
  guide for "salvage before recreate" is read by someone who has just
  lost a sandbox. The lens asks what the spec requires of that page
  beyond existing.
- **Checks that arrive late** because the text standard's gate
  (`tools/ci docs`) lands in plan task T006, and the pages written
  before it are reviewed by eye. The lens lists what those pages can
  break meanwhile.
- **Citing sections by number** ("10 10.2") because renumbering breaks
  every citation silently; the spec cites tests by title for exactly
  that reason, and sections by number. The lens asks what check of
  rule 5 catches a citation whose section moved, and whether the spec's
  own pages pass it.
- **The voice rule** (rule 19) because personality in choices is hard to
  review and easy to fake with adjectives; the lens checks that the
  spec pages themselves pass the rubric of rule 18.
- **The reader named in the front matter is the reader the page serves**
  because a page for "anyone" is a page for nobody.

Where to confirm, read last: ADR 0001, S10, 12 12.7 and 12.8, 09 (as the source of the
guides), the spec pages as pages.
Model: opus.

### L9 The delivery manager who has seen hundred-task plans

Role: the owner of
[ADR 0002, develop on the trunk with short-lived branches](../adr/0002-develop-on-the-trunk-with-short-lived-branches.md),
[ADR 0003, adopt six extreme programming practices and review as pairing](../adr/0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md),
[ADR 0004, measure delivery with the five DORA metrics computed by a tool](../adr/0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md),
[ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md),
12 12.9, 12 12.10 and plan.md. Success is: the plan lands, the operator is never the idle
bottleneck, and every deferral has a trigger that fires.

As this person you care about:

- **Operator blocks on the critical path** (plan, O1 to O7) because
  "101 of 102 tasks depend on O1" makes one person's calendar the
  schedule; the lens asks what the agents do while they wait, and
  whether the plan says it.
- **Dark merges of unfinished work** (12 12.9) because it is the right
  practice and it needs a way to hide the work; the lens asks whether
  the spec names one.
- **Delivery metrics measured on this repository only** (12 12.10, 00
  0.6) because it is honest and because a metric nobody reads in a
  meeting dies in two months.
- **Deciding at the last responsible moment with twenty-five open
  questions** because deferring without a trigger is forgetting, and
  each row in Deferred decisions needs the event that reopens it.
- **A re-plan rule** because a spec reviewed seven times will change
  again, and the plan's counts and dependencies are kept by hand.
  Incident: a plan whose "depends on" column was stale by task twenty
  and nobody trusted it by task forty.
- **The size of a task** because one logical change per pull request is
  the rule, and the lens checks a sample of tasks for the ones that are
  three.
- **Feasibility as a concern of its own** because architecture
  description standards list it next to purpose and suitability, and
  this set of lenses otherwise reaches it only through the plan's
  realism; the lens names the three claims of the spec that would be
  hardest to build and asks what evidence the spec offers for each.

Where to confirm, read last: plan.md whole, ADR 0002 to ADR 0005, 12 12.9 and 12.10,
index (Deferred decisions, Open questions).
Model: opus.

### L10 The second adopter

Role: the secondary user of 0.2, who found the public repository and
wants to run it with their own config repo. Success is: nothing in the
product is about one person, and the first page tells them whether it
is for them.

As this person you care about:

- **Personal values with nowhere to go** because the reference config
  repo has a name (01 1.1) and the host tree has a default path (00
  0.1); the lens asks what this person replaces, where (03 3.4), and
  how they find out. Incident: a tool whose default configuration
  pointed at its author's home directory; every adopter's first issue
  was the same path error, and the fix was one line nobody had been
  told about.
- **Linux hosts as a non-goal** (00 0.3) because half the potential
  adopters run Linux and the decision has to be visible on the first
  screen, not in a bullet of 0.3.
- **`ARCHITECTURE.md` before the code** (12 12.8) because this person
  contributes only if the trust boundary (01 1.2) is clear in twenty
  minutes.
- **Secrets as `name@project -> argv` in host settings** (01 1.2
  boundary B, 03 3.4) because this person's secret manager is not the
  maintainer's; the lens asks what the spec assumes about it.
- **Their own kits next to the product's** (06 6.2, J9) because a
  personal kit is where this person's machine differs, and the lens asks
  what a kit can do, what a wrong one costs, and how the gate of 01 1.4
  shows them the difference.
- **The one onboarding doc that is also the trust pitch** because for
  this person the README's "trust model" section (rule 12) decides
  whether they run a binary that drives their sandboxes; the lens asks
  what J1 shows them before the first `sync` that would make them stop.

Where to confirm, read last: 00 0.2 and 0.3, 03 3.4 (host settings),
06 6.2 (personal kits), 09 (J1, J2, J9), 01 1.1 and 1.2, the README
rules of ADR 0001.
Model: opus.

### L11 The scope skeptic

Role: the person who asks, of every v1 feature, whether the start needs
it. Success is the principle "decide at the last responsible moment"
([index](../spec.md#decide-at-the-last-responsible-moment)) applied to
the spec itself: everything the start needs, nothing it does not, and
every deferral with a trigger.

As this person you care about:

- **The runtime ledger in v1** (13) because it is the feature with the
  least direct line to the objective of 0.1 and the most new security
  surface (I33, the side channel of 05 5.4). 0.5 justifies it; the lens
  asks for the trigger that would remove it. Incident: an audit log
  added to a small tool "for later analysis" was never read by anyone,
  and was the one component that shipped the tool's only security
  advisory.
- **Snapshot and salvage, complete and "not smaller"** (00 0.5) because
  the security review required it and the cost in julieta is high; the
  lens asks whether one bundle per commit is the simplest form of
  "nothing is lost".
- **One renderer and it is a small project's** (02 2.5, J3) because
  herdr is on the path of `romeu run`, and the lens asks what the spec
  says if it stops being maintained.
- **Six memory commands plus import and verify** (04 4.3, 08 8.1)
  because a store that an agent writes through a CLI competes with the
  agent's own memory, and the lens asks what the agent would lose with
  half of them.
- **A feature justified by a success criterion that it alone
  justifies** because 0.5 traces features to criteria, and a criterion
  written for one feature is a circle.
- **Everything the spec says is partial for teams** (00 0.6) because an
  honest "nothing" is cheaper than a partial that implies a roadmap.

Where to confirm, read last: 00 0.3, 0.5 and 0.6, index (Deferred decisions), 13, 08,
04 4.3.
Model: fable.

### L12 The parallel-execution engineer

Role: the person who will turn this spec and its plan into work for
many agents at once, and who knows that the speed of implementation is
set by the shape of the dependency graph, not by the number of agents.
Success is: the plan's critical path is short, the file sets of
concurrent tasks do not overlap, and the merge is mechanical.

As this person you care about:

- **The real critical path** because "depends on" in the plan is plan
  order (12 12.2) and the import graph of 10 10.7 is another graph; the
  lens draws both and names the longest chain. Incident: a plan with
  forty parallel tasks whose every task touched one shared table, so
  they landed one at a time anyway.
- **Shared append-heavy files** because `docs/plan.md`, `docs/lessons.md`,
  the generated tables and `tools/ci` are touched by many tasks, and
  each is a serialization point; the lens lists them and asks which can
  be split or generated.
- **Operator blocks as barriers** (plan O1 to O7) because a barrier that
  one person opens in one sitting is a global stop; the lens asks which
  tasks can start before the barrier and finish after it.
- **Interface-first tasks** because two agents can build a producer and
  a consumer at once only if the type and the error table land first,
  in a task of their own; the lens checks that `spec`, `canon` and the
  error ids (04 4.4, 12) are such tasks.
- **Tests as the merge contract** because a parallel plan merges on
  green, and the lens asks whether every task's Verify line is a
  command another agent can run without the author.
- **The ask-first surfaces as a lock** because a task touching `checks`
  or `gates` (05 5.3) waits for the operator and blocks every task that
  overlaps it; the lens counts how many tasks name those surfaces and
  whether the count can drop.

Where to confirm, read last: plan.md (dependencies, phases, Ask-first fields), 12 12.2,
10 10.7, 05 5.3, 04 4.4.
Model: fable.

### L13 The reader who holds the spec against itself

Role: someone who reads the specification as one system, not as pages.
Success is: one problem, one mechanism, one name, on every page; and
every feature passing the principles and the Boundaries of the index
with no exception that is not written down.

As this person you care about:

- **One problem solved two ways** because that is where contributors
  diverge. Candidates in the text: two romeu approvals plus sbx's own
  for "widening"; hash verification on some derived files and not on
  the workspace files (01 1.5 states the exception, and it is an
  exception); three stores (memory in plain files, ledger in JSON,
  state in JSON) with their own conventions for ids, timestamps and
  digests. Incident: a system with three id formats, noticed only when
  one log had to join them.
- **Conventions of 04 4.1 that the command tables do not honour**
  (04 4.2, 04 4.3, 04 4.4) because an exit code is a contract. Does 2
  always mean a failed precondition? Does `--json` exist where the
  caller is a program, and only there? Incident: a CLI where exit 1 meant both "failed" and
  "found differences", and a CI script that ignored the differences for
  years.
- **Names that change between pages** because the vocabulary of 01 1.8
  is the one source and not every page uses it; operator and maintainer
  is one case, and the lens lists the others.
- **A feature against a principle** because "deterministic where it can
  be", "simple and explicit" and "decide at the last responsible moment"
  are claims testable against each feature of 00 0.5; a feature that
  fails one with no written exception is the finding.
- **Promises that cross** because one section states what another
  contradicts, and rounds 6 and 7 found several; the lens looks for what
  is left, above all between 04, 09 and 13, which describe the same
  commands from three angles.
- **Exceptions with no owner** because every "except" and "unless" is a
  partial rule; the lens counts them and asks of each whether it is a
  decision or an accident.

Where to confirm, read last: 04 4.1 and the command tables, 01 1.8,
index (Principles, Boundaries), 00 0.5, 09 against 04 and 13.
Model: fable, because it is a cross-reading of the whole text.

### L14 The maintainer on duty

Role: the person who receives a bug report from a stranger, and the
first pull request from a stranger. Success is: a failure reported by
a stranger becomes a fix in an afternoon, by another stranger, and both
want to come back.

As this person you care about:

- **What a user can paste into an issue** because diagnosis starts with
  what arrives. The spec has error ids with fix hints, `--timings
  --json`, `doctor` and the ledger view, and no log level, no verbose
  mode and no command that packs redacted evidence (I29, 11). Incident:
  two hundred issues saying "it does not work", until a command existed
  that packed the evidence.
- **Tracing one critical flow end to end** because `sync` has nine
  steps, `run` ends in an exec and `salvage` crosses two machines; a
  maintainer needs to see in which step the time and the failure went.
  `--timings` exists for S8; profiling does not.
- **The contributor's local loop** because `tools/ci fast`, the fake
  sbx and the scaffolds decide whether the first fix takes an afternoon
  or a week. Incident: a project whose CI
  took forty minutes received one pull request per contributor and
  never the second.
- **The critical flows as teaching material** because `ARCHITECTURE.md`
  and the error table are what a newcomer learns from; the lens asks for
  a "read this first" path per flow, and whether each error id leads to
  a runbook (S10).
- **Open source as a relationship, not a license** because
  `CONTRIBUTING.md`, issue templates, a response expectation
  (`SECURITY.md` has one; issues do not) and release notes that name
  contributors decide whether anyone returns. Incident: a
  well-built tool with no contributors because every pull request
  waited three weeks.
- **Reliability engineering with no server** because the failure modes
  in the field are disk, denied egress, sbx version skew, mount
  permissions, and the chore that fails quietly after every commit for
  months (13 records `hook-failed`; the lens asks who reads it); for
  each, the lens asks for an error id, a runbook and a safe way to
  collect evidence without leaking a secret.

Where to confirm, read last: 04 (the error table, `--timings`,
`doctor`), 12 12.2 to 12.4 and 12.8, 10 10.1, 13, S8, S10, rules 16
and 20 of ADR 0001. L3 keeps decay, L8 the page, L10 adoption; L14
keeps the day something breaks. Model: opus.

### L15 The ground that moves

Role: the person who maintains integrations with other people's tools
and has woken up to a renamed `--json` field. Success is: a change in
sbx, mise, herdr, Docker Desktop or the GitHub API is caught by a probe,
a pin check or a recording before a user meets it, and the spec says
what happens when it is not.

As this person you care about:

- **Facts owned by someone else** because `sbx ls --json`, the
  `remote.sandbox-<name>.*` keys, the kit descriptor grammar and the
  herdr `layout.apply` request are all read or written by code we do
  not control (01 1.1, 06, 02); the lens lists each and asks which probe
  of 11 covers it, and what `doctor` says when one changes shape.
- **A version floor without a ceiling** because `sbxdrv` has a version
  floor and the toolchain acknowledgement (01 1.3) records a version;
  neither says what a version above the one tested means. Incident: a
  tool that accepted every newer version of its dependency until the
  dependency changed an exit code.
- **The egress catalog as a mirror of the world** (07, S4) because
  backends add hosts, mise adds backends, and the catalog is embedded in
  a release; the lens asks how a user whose tool is missing finds out
  and gets unblocked without a release.
- **Pins that someone must bump** (J8, J9, `pin check`) because a
  digest pin is only as fresh as the person who bumps it, and a stale
  pin is a security finding waiting to be filed; the lens asks what
  notices staleness.
- **A renderer owned by a small project** because herdr is on the path
  of `romeu run`, and the renderer interface of 02 is the only hedge;
  the lens asks whether the interface is real enough to carry a second
  renderer, or a shape on paper.
- **Recordings as the contract with the upstream** (10 10.3, 11)
  because a recorded `sbx` conversation is a fact at one version; the
  lens asks how a recording is dated, and how many of them a version
  bump invalidates.

Where to confirm, read last: 07, 06, 02, 11, 01 1.1 (sbx-owned facts),
04 4.2 (`doctor`), 09 (J8, J9), S3, S4. Model: opus.

### L16 The owner of the data

Role: the developer, and the employer whose code it is, who notice that
their repositories, memory entries, handoffs, saved transcripts and
runtime events now live in directories a tool manages. Success is: they
know what is where, who can read it, how to export it, how to delete
it, and how to stop using romeu while keeping every clone and every
note as plain files.

As this person you care about:

- **Transcripts saved on request** (`salvage --include-transcripts`, 08)
  because an agent transcript holds the code, the secrets a developer
  pasted and the employer's prompts; the lens asks who reads the
  salvage directory, whether it enters a machine backup, and whether
  the spec says so to the user. Incident: a support-bundle feature that
  archived the home directory sent a developer's private keys to a
  vendor's ticket system, and the vendor found them first.
- **A ledger that is never redacted and never deleted** (13, 05 5.4)
  because the spec's rule is absolute and the remedy is to rotate a
  credential; the lens asks what a user does when the value in the
  ledger is not a credential but a name.
- **An attic that never empties** (`retire`, 01 1.8) because "never
  deletes" is a promise to the data and a liability to its owner; the
  lens asks for the one command, or the one sentence, that tells the
  owner how to delete what they own.
- **Cross-project visibility by design** (I33) because a sandbox reads
  other projects' names, timings and allowlisted fields; one developer
  with a work project and a personal project is the case the lens
  checks, and the employer is the reader who would object.
- **Leaving** because plain clones, no submodules and an agent-neutral
  memory format are the spec's promise of a clean exit (00 0.1, 08
  8.1); the lens asks for the journey that proves it, and finds that 09
  has none.
- **Where the memory format is specified** because "format owned by
  julieta, agent-neutral" (01 1.1) is a claim about portability; the
  lens asks what a second tool would need to read it.

Where to confirm, read last: 08, 13, 05 5.4, 01 1.1 and 1.8, 00 0.1,
09 (J6, J7). L2 keeps the attacker; L16 keeps the owner. Model: opus.

## Measuring the set of lenses

A set of viewpoints is always the list of whoever wrote it. The
measurement below replaces the feeling that something is missing with
a matrix, run before each round, whose zeros are decisions or lenses.

**What is counted.** For the lens sections (L1 onward), the mentions of
each source of success the spec defines: the index and each page (00 to
13, `plan.md`); each criterion S1 to S11; each journey J1 to J13; each
party in the component table of 01 1.1 (sbx, mise, herdr, GitHub, the
signing agent, the agent runtime); and two lists from outside the spec,
kept because two independent frames pointing at the same gap is the
signal that justifies a lens: the stakeholder classes of Rozanski and
Woods (acquirers, assessors, communicators, developers, maintainers,
production engineers, suppliers, support staff, system administrators,
testers, users) and the minimum concerns of ISO/IEC/IEEE 42010
(purpose, suitability, feasibility, life-cycle risks, maintainability).

**What a zero means.** Every row with no lens gets one of two
outcomes, written in this page: a lens, or one line saying why none is
needed. A row with one lens is reviewed for whether that lens's
definition of success reaches it, or only its reading list.

**The measurement before round 8** (on the twelve lenses L1 to L12 as
first written): pages 02 and 07 had no lens; criteria S2 and S4 had
none; journeys J4, J5, J7, J8, J9 and J13 had none; of the external
parties, mise and herdr had one mention each, sbx five against forty in
01, the signing agent none; the themes privacy, transcripts, backups,
export and leaving had none. Against the 42010 concerns, feasibility
had no lens. Against the stakeholder classes, suppliers had none. The
zeros clustered into two viewpoints, L15 and L16, and into reading
lines added to L1, L2, L4, L5, L6 and L7 (the signing agent went to
L2), and a bullet in L9; the measurement is why they exist. The script
counts the lens sections only, so this page's own measurement text does
not count as coverage.

**Rows with no lens, by decision.** Accessibility and colour of the
terminal output: the spec defines no colour semantics and no
interactive interface beyond two TTY prompts, and I29 fixes what the
output may contain; reopen when 04 gains colour, a progress display or
an interactive command. Deprecation and migration of the product's own
formats: the spec versions its file formats (03) and the plan starts at
v1.0.0 with nothing to migrate from; reopen at the first format bump.

**How it is run.** By a script that reads this page and the spec and
prints the matrix. Until that script lives in the repository, the
round's orchestrator runs it from a scratch directory and pastes its
output in the round page. Pending item: a `tools/lenses coverage`
command in Go, with the lists above as committed data, so the matrix
is a check and not a habit; it is written with the first round page
that needs it, by the rule of the spec that a scaffold lands with the
module that owns its first input (12 12.2).

**Bound.** This page stops adding lenses at the point where a new one
cannot name a source of success that the matrix shows uncovered, or a
cluster of findings that only L0 raised. Above about fifteen lenses the
move is to merge two, not to add a third, by the same rule the spec
applies to its concepts.

## What we learned from others

Each practice below was read from its primary source and recorded in
the form the project uses for anything borrowed: their decision, the
constraint that probably produced it, what it cost them, whether the
same constraint holds here, and what we decided. "Tool X does it" is
not a reason; a divergence is recorded with its reason.

| Practice and source | Their decision | Probable why | What it cost them | Holds here? | Our decision |
|---|---|---|---|---|---|
| IETF: RFC 3552 (BCP 72), RFC 6973, the Security Directorate and Gen-ART pages at wiki.ietf.org | one section is mandatory in every RFC, Security Considerations, with a floor: name the attacks out of scope "and why", consider a fixed list of attack classes. Privacy came later as an Informational questionnaire, needed case by case. Separately, area directorates review every document the IESG sees, and Gen-ART reviews what is "no area's special interest"; reviews are advisory and do not block | "Historically, such sections have been relatively weak" (RFC 3552, 1); directorates exist so area directors can focus on troublesome documents | about 50 reviewers for about 20 documents a month; a document can come back two or more times; RFC 6973 admits designers cannot foresee "all of the privacy implications" | partly. We have one document and one maintainer, not thousands of drafts and an IESG, so a mandatory section per document and an advisory review do not transfer. The catch-all reviewer and "out of scope, and why" transfer whole | L0 is our Gen-ART; the report carries "Out of scope, and why"; our must-fix severities stay binding, because nothing sits above the round to decide what an advisory review leaves open |
| Rozanski and Woods, viewpoints-and-perspectives.info (viewpoints, perspectives, stakeholders) | a fixed catalogue in two axes: viewpoints that partition the description, and perspectives for qualities that cut across every view; a fixed list of eleven stakeholder classes to catch what is missing. They retired a Security viewpoint when experience showed security cuts across every view | one all-in-one model "is often incomplete, incorrect, or out-of-date" | the set is aimed at "large-scale information systems"; the pages read give no rule for adding or dropping a perspective beyond the stakeholder check | the two-axis split holds: the spec's pages are the partition, and every lens here is a perspective applied across them. The stakeholder-class list holds as a completeness check | the stakeholder classes join the coverage measurement; we do not adopt the viewpoint catalogue, because the spec already partitions itself by page |
| ISO/IEC/IEEE 42010, the overview at iso-architecture.org (Clause 5 of the 2011 edition; the 2022 conceptual model) | stakeholders hold concerns, concerns are framed by viewpoints; completeness is traceability: every identified concern "must be framed by at least one viewpoint"; a minimum list of stakeholders and of concerns to consider | the standard "does not specify one set of views", so completeness has to be defined against the concern list | not stated in the pages read | yes, directly: our sources of success are the concerns, our lenses the viewpoints | the coverage measurement is a two-way traceability rule; the standard's minimum concerns join the lists it reads, which is how feasibility reached L9 |
| SEI ATAM, Kazman, Klein and Clements (CMU/SEI-2000-TR-004) | not read: both SEI hosts redirect to www.sei.cmu.edu, which was not reachable from the sandbox | - | - | - | pending; the row is filled when the report is read. It matters because ATAM elicits scenarios from stakeholders rather than from evaluators, which is the one move this page has not borrowed |
| Nielsen Norman Group, "How to Conduct a Heuristic Evaluation" (2023) and "Why You Only Need to Test with 5 Users" (2000) | three to five evaluators who evaluate independently and do not see each other's notes until done, then merge by affinity; the measured curve: one participant finds about 31% of problems, five about 85%, fifteen nearly all; three rounds of five beat one of fifteen; the curve holds only for comparable users, and each distinct group needs its own three to four | "each individual (no matter how experienced or expert) is likely to miss some" problems | each extra evaluator adds less; the method is "not a replacement for user research" | partly. Each lens here is a distinct group, so one reviewer per lens is below their floor; but a reviewer here costs minutes, not a day, and the overlap between two reviewers of one lens has never been measured | independence before merging stays; the noise probe (two lenses run twice in one round) measures whether one reviewer per lens is enough, and the rule is decided from that number |
| Gary Klein, "Performing a Project Premortem", Harvard Business Review, September 2007 (read from the article body the publisher serves; the rendered page is paywalled) | announce that the project "has failed spectacularly"; each member writes every reason independently, "especially the kinds of things they ordinarily wouldn't mention"; then round-robin until every reason is recorded | people are "reluctant to speak up about their reservations" while planning; prospective hindsight makes dissent safe | minutes per session; the article reports no criticism | yes | step 5 of the brief states failure as a fact and asks for the reasons this person would not normally say; the source is cited so the wording is not rediscovered |
| Google SRE, "Evolving the SRE Engagement Model", Site Reliability Engineering, chapter 32 | one to three SREs run a Production Readiness Review against a checklist "specific to the service", drawn from domain expertise, similar systems and past postmortems; recurring concerns were later built into frameworks | design changes found late "come at a high cost" | two to three quarters per onboarding, serialization of takeovers, lead times of months, "cognitive burden" on reviewers; staleness of the checklist is not discussed | partly. A review per launch with months of lead time is the cost we avoid; "read the past postmortems" and "recurring concern becomes a framework" transfer | the consolidator, not the reviewers, checks new findings against previous rounds, so the reviewers stay unanchored; a finding class that repeats across rounds becomes a `tools/ci` check, not a lens |
| Kahneman, Rosenfield, Gandhi and Blaser, "Noise", Harvard Business Review, October 2016 (read as the Klein article was; the 2021 book was not read) | a noise audit: members of a unit judge a common set of cases independently and the spread between them is measured; in a case roundtable each participant forms a defensible opinion alone and sends it to the leader before the meeting | discussion produces "spurious agreement" as people "converge on the opinions stated first or most confidently"; executives are "completely unaware" of the noise | executives expected 5 to 10% disagreement and the audits measured 48% and 60%; the authors call the discipline "not at all easy" and say professionals "drift" | yes | reviewers never see each other before the merge; the consolidator records a disagreement between two reviewers of one lens as data for the round page, never resolves it by confidence; the noise probe is our noise audit |

Across the seven sources read, three things recur and are now in this
page: independence before pooling (Nielsen, Klein, Kahneman and
colleagues), a stated out-of-scope with its reason (RFC 3552, 42010),
and completeness as traceability against a list rather than as a
feeling (42010, Rozanski and Woods). One thing recurs that we did not
take: viewpoints after the first are added as guidance more often than
as mandates (RFC 6973, the retired Security viewpoint). Here every
lens runs every round, because a round costs minutes and a lens that
sits out costs the round that would have needed it.

## Rounds that used these lenses

| Round | Baseline | Lenses | Page |
|---|---|---|---|
| 8 | the default branch at the commit this page was added | L0 to L16, with the noise probe on two lenses the round names | round-8.md, when written |
