# 5. Decide at the last responsible moment and record the trigger

Date: 2026-10-01

## Status

Accepted

## Context

Friar Laurence decided everything on the first night: the potion, the
tomb, the letter, the escape. Each step depended on facts he did not
have yet, and the plan had no way to notice when one of them turned
out false. We would rather decide less on the first night.

The maintainer's principle, stated for this project: what the start
needs, we build as soon as it is needed; a decision that can wait
until we have more context and more use is very often better made
then; and what waits is written down. Review round 3 already judged
the spec by it
([round 3](../reviews/round-3.md)), and the spec already has the
place where deferrals live, the
[Deferred decisions](../spec.md#deferred-decisions) table. What was
missing is the principle itself, written where a contributor reads
it, with its sources.

The principle is the maintainer's. The phrase "last responsible
moment" is ours, a heading for it, and we attribute it to no source.

| Source | Their decision | Their why | What it cost them |
|---|---|---|---|
| Extreme Programming, as described by Martin Fowler, [Yagni](https://martinfowler.com/bliki/Yagni.html), 26 May 2015 | "Yagni originally is an acronym that stands for 'You Aren't Gonna Need It'. It is a mantra from ExtremeProgramming that's often used generally in agile software teams." | the page ties it to design: "Yagni is a way to refer to the XP practice of Simple Design (from the first edition of The White Book, the second edition refers to the related notion of 'incremental design')." | - |
| Kent Beck with Cynthia Andres, *Extreme Programming Explained: Embrace Change*, 2nd ed., Addison-Wesley, 2004 | the practice Fowler's page points to | bibliography only: we have not checked a copy, and quote nothing from it | - |
| Mary Poppendieck and Tom Poppendieck, *Lean Software Development: An Agile Toolkit*, Addison-Wesley, 2003 | named as background for this principle | bibliography only: we have not checked a copy, so no wording, page or passage here is attributed to it | - |

What it cost them is the column we cannot fill from what we read, and
we leave it empty instead of guessing. The cost we expect is our own
and is under Consequences.

Does their constraint hold for us? Theirs is that a team builds for
needs it predicts and then carries code nobody uses. Ours is sharper:
an agent will write a mechanism in minutes if asked, so the cost of
building early is close to nothing and the cost of owning it (tests,
docs, a surface to review, a concept to count) is the same as ever.
The pressure to add is higher here, and so is the need for a rule
against it. That is our reading.

### Alternatives considered

- **Decide everything in the spec before the plan.** Several of the
  spec's facts are not known until a probe runs on a real host
  (11). Deciding them early would mean deciding them twice. Rejected.
- **Defer silently: leave it out and say nothing.** Then nobody knows
  whether a gap is a decision or an oversight, and nothing brings it
  back. This is the letter that never arrived. Rejected.
- **A `TODO` in the text where the gap is.** It records the gap and
  not the event that should reopen it, and a search is the only way
  to find them all. Rejected; one table, with a trigger per row.
- **Open questions only.** An open question has a default the spec
  follows and a known way to settle it (a probe or the maintainer). A
  deferral has no default mechanism at all: the thing is absent until
  an event happens. They are two tables because they are two states.

## Decision

We decide at the last responsible moment:

1. **Build what v1 needs in order to start.** A security invariant, a
   merge gate and anything a later step depends on are needs of the
   start. They are never deferred.
2. **Leave out a mechanism the start does not need**, and write one
   row for it in the spec's Deferred decisions table: what is
   deferred, what holds until then, and the observable event that
   reopens it.
3. **The trigger is an event, not a date and not a feeling.** "The
   first PR that changes which steps `fast` runs" is a trigger. "When
   we have time" is not.
4. **When the event happens, the decision is made** and the row
   leaves the table. If the decision is expensive to reverse, it
   becomes an ADR
   ([ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md),
   rule 9).

The principle is in the spec's
[Principles](../spec.md#decide-at-the-last-responsible-moment). It
agrees with "Simple and explicit": that principle counts the concepts
we add, and this one says when a concept may be added at all.

Why this fits us: the spec already defers things with triggers, and
the reviewers already judge by this. Writing it down turns a habit of
one review round into a rule the next contributor can read.

## Consequences

- `tools/ci sequences` checks that each row of the Deferred decisions
  table has all three cells filled. Whether a trigger is observable,
  and whether something was a need of the start, stay review
  judgments.
- Deferring has a cost, and each row should say it where it is known.
  Data that a deferred mechanism would have recorded is not recorded
  until it exists, so its baseline starts late. A deferred mechanism
  may also be harder to add once code has grown around its absence.
- A trigger nobody watches does not fire. The table is read in each
  review round, and nothing else reminds anyone; where a trigger can
  be detected by a check, the row should say which.
- This record does not defer anything by itself. Each deferral is its
  own row, and the rows added with this record are listed in
  [round 4](../reviews/round-4.md).
- If someone checks a copy of either book and finds that it says
  something else, the correction is a new record that supersedes this
  one.
