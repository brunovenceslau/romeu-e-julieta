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
on one commit of the default branch. The brief carries:

- the lens section below, whole, and nothing from another lens;
- the baseline commit, so every reviewer reads the same text;
- the instruction to refute: find where the spec fails this lens's
  definition of success, not where it meets it;
- the output shape of the next section;
- a scratch directory outside the repository, and the rule that the
  reviewer creates no file inside the tree, even transiently.

Nothing writes to that tree until every reviewer has reported. The
reports are merged into one list, duplicates kept once with every lens
that raised them (a finding raised by two lenses independently is
high-signal), and applied in one write pass, recorded as one review
round under this directory.

Model per lens: the lens's "Model" line names the tier the round
chose. A lens whose findings decide security, architecture or scope
runs on the strongest tier available; no lens runs on the smallest
tier, because a lens that misses a finding costs a review round later.

## What a reviewer returns

The first line of the report is `<lens id> · <role> · <model>`. Then:

| Field | Content |
|---|---|
| Verdict | one line: does the spec meet this lens's definition of success, and what is the largest gap |
| Findings | one row each: id (`<lens id>-<n>`), severity on the lens's own scale, the spec section cited as the spec cites itself ("04 4.3"), the defect in one sentence, the concrete failure scenario, the proposed change and what it costs |
| Not tested | what the lens could not judge from the text alone, and what evidence would settle it |
| Confidence | one line |

Severity scale for every lens, so the merge is mechanical:
**Blocking** (the spec would ship a defect this lens exists to catch),
**Required** (fix before the next plan task that implements the cited
section), **Advisory** (improve when the section is next touched).
A lens does not report what the spec already lists as a deferred
decision, a non-goal or an accepted risk, unless the trigger or the
acceptance is itself the defect.

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
  user does not.
- **J1 in one sitting** because a tool that needs the spec read before
  the first run is not adopted; the comparison in this person's head is
  one `brew install` and one `init`.
- **A gate that asks too often teaches approval without reading**
  because the recreate class (01 1.8) decides how many `run:` edits
  become a prompt. Incident: a deploy tool that asked "are you sure?"
  on every change trained a team to type `y` before the diff rendered.
- **What happens on the second machine** (J12) because this person has
  a laptop and a desktop, and the machine-local state does not travel.

Read first: 00, 09 (J1, J3, J11, J12), 04 4.2, 01 1.3 and 1.8, S8.
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

Read first: 05 whole, 01 1.2 and 1.6, 13, 08 8.1, ADR 0008.
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
  hour of "which of the two do you mean".
- **Generators that outlive their single source** (12 12.3) because a
  generator is a bet that the table it reads stays the only one.
  Incident: three generators abandoned after their output format
  changed; their outputs were edited by hand "just this once" until the
  drift check was deleted.
- **Two names for one person** because the spec says "operator" where
  it means the machine and "maintainer" where it means the process, and
  neither is in the vocabulary. A new reader will see two roles and
  invent a third.
- **Import boundaries enforced by a check** (10 10.7) because a module
  boundary in a document lasts until the first agent needs a shortcut;
  one in `tools/ci imports` lasts.
- **A plan of 102 tasks against a draft spec** because a plan that size
  freezes decisions early, against the principle of deciding at the
  last responsible moment. The lens asks which tasks disappear if half
  of 13 is deferred.
- **Every "ask first" in a document and not in a file** because an
  agent reads `.github/ask-first.yaml` and skips prose.

Read first: index (principles), 01 1.8, 12 whole, 10 10.7, plan.md.
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
  fails quietly teaches people not to trust snapshots.
- **`retire` moves to `.attic/` and never deletes** (01 1.8) because it
  is the right call and it fills the disk; the lens asks who cleans it
  and when the spec says so.
- **sbx changing its JSON between versions** (I26, `RJ-203`) because the
  facts romeu reads from `sbx ls --json` are owned by another project.
  Incident: a Docker Desktop upgrade renamed a field and a tool refused
  every sandbox as unknown.
- **Two machines, one config repo** (J12) because `$XDG_STATE_HOME` does
  not travel, and two hosts will disagree on applied egress and on the
  toolchain acknowledgement.
- **`doctor` as the only way to learn the truth** because a tool that
  can be wrong about state needs one command that compares every
  derived file against its source, byte for byte, and says so.

Read first: 01 1.5 to 1.7, 09 (J6, J10, J12), 04 4.2 (`doctor`,
`salvage`, `retire`), 08 8.4, 13 13.6.
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
  agent will try to write, fail, and improvise a path; the spec has to
  say what the failure looks like and what to do instead.
- **`julieta event add` with no free text** (13 13.2) because an agent
  that cannot say what happened stops recording. The lens checks that
  the closed sets of `type` and `tool` cover what an agent wants to
  tell.
- **The hook dispatcher stopping at the first non-zero** (04 4.3)
  because the agent reads "commit failed" without knowing which of the
  three hooks failed or what the fix is.
- **Three handoff kinds** (`clear`, `final`, `facts`) because the rule
  for which kind when has to be a hook, not a memory; the skill that
  says "write a handoff before `/clear`" is a memory.
- **Skills that ask the agent to remember a mechanical step** because
  each is a defect by the spec's own principle; the lens lists them.

Read first: 08 whole, 04 4.3, 13 13.2, 06 (the `julieta-claude`
mixin), S5.
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

Read first: 10 whole, 11, S6 to S9, 05 5.2 (the test column).
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
  is an exception someone will copy tomorrow.
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

Read first: S1, S11, 12 12.1, 12 12.6, 10 10.2 ("Release and
bootstrap"), 06 6.3 (digests), plan.md "Toolchain pins".
Model: opus.

### L8 The technical writer who maintains generated docs

Role: the owner of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md),
S10 and 12 12.7. Success is: a page read in a panic gets the reader
out of it, and no page is older than the code it describes.

As this person you care about:

- **Reference pages generated and drift-checked in the gate** (rule 10)
  because generated docs that are not checked age faster than
  hand-written ones: nobody feels responsible for them.
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
  that reason, and sections by number.
- **The voice rule** (rule 19) because personality in choices is hard to
  review and easy to fake with adjectives; the lens checks that the
  spec pages themselves pass the rubric of rule 18.
- **The reader named in the front matter is the reader the page serves**
  because a page for "anyone" is a page for nobody.

Read first: ADR 0001, S10, 12 12.7 and 12.8, 09 (as the source of the
guides), the spec pages as pages.
Model: opus.

### L9 The delivery manager who has seen hundred-task plans

Role: the owner of ADR 0002 to ADR 0005, 12 12.9, 12 12.10 and
plan.md. Success is: the plan lands, the operator is never the idle
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

Read first: plan.md whole, ADR 0002 to ADR 0005, 12 12.9 and 12.10,
index (Deferred decisions, Open questions).
Model: opus.

### L10 The second adopter

Role: the secondary user of 0.2, who found the public repository and
wants to run it with their own config repo. Success is: nothing in the
product is about one person, and the first page tells them whether it
is for them.

As this person you care about:

- **Personal values with nowhere to go** because the reference config
  repo has a name and the host tree has a default path; the lens asks
  what this person replaces, where, and how they find out.
- **Linux hosts as a non-goal** (00 0.3) because half the potential
  adopters run Linux and the decision has to be visible on the first
  screen, not in a bullet of 0.3.
- **`ARCHITECTURE.md` before the code** because this person contributes
  only if the trust boundary is clear in twenty minutes.
- **Secrets as `name@project -> argv` in host settings** (01, boundary
  B) because this person's secret manager is not the maintainer's; the
  lens asks what the spec assumes about it.
- **The one onboarding doc that is also the trust pitch** because for
  this person the README's "trust model" section (rule 12) decides
  whether they run a binary that drives their sandboxes.

Read first: 00, 03 (host settings), 06 (personal kits), 09 (J1, J2),
the README rules of ADR 0001.
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
  asks for the trigger that would remove it.
- **Snapshot and salvage, complete and "not smaller"** (00 0.5) because
  the security review required it and the cost in julieta is high; the
  lens asks whether one bundle per commit is the simplest form of
  "nothing is lost".
- **One renderer and it is a small project's** because herdr is on the
  path of `romeu run`, and the lens asks what the spec says if it stops
  being maintained.
- **Six memory commands plus import and verify** (04 4.3) because a
  store that an agent writes through a CLI competes with the agent's
  own memory, and the lens asks what the agent would lose with half of
  them.
- **A feature justified by a success criterion that it alone
  justifies** because 0.5 traces features to criteria, and a criterion
  written for one feature is a circle.
- **Everything the spec says is partial for teams** (00 0.6) because an
  honest "nothing" is cheaper than a partial that implies a roadmap.

Read first: 00 0.3, 0.5 and 0.6, index (Deferred decisions), 13, 08,
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

Read first: plan.md (dependencies, phases, Ask-first fields), 12 12.2,
10 10.7, 05 5.3, 04 4.4.
Model: fable.

## Rounds that used these lenses

| Round | Baseline | Lenses | Page |
|---|---|---|---|
| 8 | the default branch at the commit this page was added | L1 to L12 | round-8.md, when written |
