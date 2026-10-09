# The DORA report on teams adopting AI-assisted development

Reader: anyone weighing which team problems the product could take on
after v1. Type: explanation.

This note holds the analysis behind the premise of
[00 0.6](../spec/00-scope.md#06-teams-adopting-ai-assisted-development).
It is not part of the specification: nothing in v1 is built from it.

v1 is built for one developer. The maintainer's premise is that the
people it should serve next are teams that adopt AI-assisted
development, and one report describes them at length: DORA and Google
Cloud, "The ROI of AI-assisted Software Development", v. 2026.1
(2026), linked from [dora.dev](https://dora.dev/ai/roi/report/). DORA
is a research program run by Google Cloud, and Google sells AI and
consulting. We label that and do not discount the report for it.

The report addresses technology leaders who must justify AI spending
to boards and finance (p.8), with teams of product, engineering, UX
and site reliability roles (p.55). It presents no new data of its own.
So each row below says how the report knows what it says: **measured**
is a survey or study the report cites, second-hand here and not read
by us; **argued** is the authors' own reasoning. Then, for each: what
in this spec addresses it, and what does not.

| Problem the report attributes to these teams | Page; how the report knows | What this spec has | Gap |
|---|---|---|---|
| A gain in productivity is perceived, and the return is unclear | p.18, measured: more than 80% of survey respondents perceived a gain; p.11-12, argued from three cited studies that disagree | nothing | the product measures no team's delivery. The delivery metrics of [12 12.10](../spec/12-engineering.md#1210-delivery-metrics) are about this repository only |
| A verification tax: time spent reviewing generated code, which the authors call "the most immediate barrier to ROI" | p.9, 20, 33, 40; argued | partial. Checks that need no reviewer: julieta's dispatcher runs each repo's own hooks inside the sandbox (I21), and the host gate shows a reviewer the widening changes as one complete diff (I7, I29) | the product does not review code and does not account for the time or cost of verifying it |
| Software delivery instability rises with adoption | p.19, 25; the association is measured, +0.097 with an 89% credible interval of +0.067 to +0.129 (p.20, Figure 4); that the rise is temporary is argued | nothing | the product records no failure or rework of a team's releases |
| Friction and burnout do not fall | p.20, measured: both effects have intervals that cross zero | nothing | both are perceptions of people; the product sees none, and a survey is outside its scope |
| Individual gains pile up at manual review gates and brittle pipelines | p.13, 19; argued | partial. The host approval gate asks only for changes that widen what a sandbox can do, and shows them in one diff ([01](../spec/01-system-model.md)) | the queue of pull requests in front of a team's reviewers is untouched |
| Loss of familiarity with the codebase | p.33; argued | partial. Per-repo memory and handoff files keep what a session learned and survive the sandbox ([08](../spec/08-memory-handoff-salvage.md)) | nothing measures familiarity, and the documentation standard of this repository applies to this repository, not to a team's. Memory is machine-local (Q14) and outside the repository, so it leaves with the developer |
| A dip after adoption, of unknown depth and duration, that leadership reads as failure | p.8-10, 30; argued | nothing | no dated marker of an adoption or a change exists, so there is no before and after to compare |
| No baseline, so no attribution | p.4; argued | nothing | as the first row |
| "Shadow AI": people turn to unauthorized tools when sanctioned ones do not deliver | p.12; measured, from a study the report cites | partial. A sanctioned sandbox whose egress is derived from a catalog, with anything beyond it behind the host gate ([07](../spec/07-mise-egress.md)) | the report's cause is sanctioned tooling that fails to help. Nothing here shows that this product helps, so we do not claim it solves this |
| Weak context makes the agent produce bloat | p.41, 44; argued | partial. Memory, handoff and the skills that tell an agent to use them ([08](../spec/08-memory-handoff-salvage.md)) | the quality of a team's own documentation is the team's work |
| Low trust deepens the dip; teams need "a clear and communicated AI stance" | p.40; argued | nothing. The security model ([05](../spec/05-security.md)) is a reason to trust the tool, which is a different thing from an organization's stance | a stance is a policy of an organization toward its engineers, not a feature |
| The ongoing cost moves to governance | p.36; argued | nothing | as the verification tax: the cost is not accounted |

Five rows have a partial answer and seven have none. That is the
honest count for a product whose v1 serves one developer.

One thing we do not claim. The report's return-on-investment model
needs a team's own numbers: time saved per change, failures and
recoveries of its releases, the cost of its tools, dated markers of
what changed and when. The product does not let a team fill that model
from its own data. It would need data gathered across more than one
machine, and this spec has none. Recording those inputs is a
[deferred decision](../spec.md#deferred-decisions) with its trigger;
computing a return on investment is not planned at all.
