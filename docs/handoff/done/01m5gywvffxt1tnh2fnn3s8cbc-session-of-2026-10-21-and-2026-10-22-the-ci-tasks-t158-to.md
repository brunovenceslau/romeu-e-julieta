# Session of 2026-10-21 and 2026-10-22, the CI tasks, T158 to T160 and T077

Done on 2026-10-22, in the pull request that carries this record. Each
item names its gate and its merge commit on main.

- #228 T060 `julieta snapshot`, merge 54e2a13b, and #229 the plan of
  T155 to T157, merge 49ea53f5 (both 2026-10-21). #230 the checkpoint,
  merge 87e1442f: its `now.md` conflict was resolved by a merge of main.
- #231 T150, the imports check refuses `encoding/json` v1; merge
  c0e83859; code review, local CI `all`, hosted CI green.
- #232 T155, the race run's slow tests and `t.Context`; merge
  efe4f10e; code review, security audit, `all`, hosted CI green. Drift
  went from 92.8 s to 4.2 s, the release build from 110.8 s to about
  28 s, the whole race run 111 s.
- #233 T156, one source of the suite tags, one race argv, one module
  root, and the x/tools cover package v0.51.0 for the cover profile;
  merge 7e495d36; code review, security audit, `all`, hosted CI green.
- #234 T157, one race run and the static, race and e2e jobs behind one
  aggregator per host; merge 19ffca36; code review, security audit,
  `all`, hosted CI green. Three CI proofs ran on the pull request: green,
  a red Linux job, and a job cancelled by its own timeout. Job times of
  the green run: static 3m37, race Linux 3m48, e2e 1m47, race macOS
  9m43.
- #235 T158, the container e2e annotates a cached download it heals;
  merge a31b3e7f; code review, security audit, `all`, hosted CI green.
- #236 T159, an AGENTS.md of pointers and a one-line CLAUDE.md; merge
  e2226b4a; code review, `all`, hosted CI green.
- T160, setup, status and SessionStart report snapshots disabled: built
  and reviewed, in pull request #237, not merged at this checkpoint.
- T077, the julieta kit, embedded: built, reviewed and audited on the
  pushed branch `feat/t077-julieta-kit`, with no pull request. It waits
  for the A12 measurement ("T077 waits for the A12 measurement",
  01m5gywvj5vj90yfedah9y670b).

Closed by this record: "One race run, test tags and parallel CI jobs"
(01m5drzqsp6zg34xzmywckwb7b), which said it closed when T157 merges;
its pending file is removed in this change.

An issue was opened in the sandbox kit's repository (issue 283) about
the brief guard, whose subagents named skills and never loaded them,
with the library-flag preference below; its follow-up belongs to that
repository's session.

Decisions of the session, each answering item 7 of "Open for the
operator" of `now.md` of 2026-10-21 where it does:

- The merge of #228 confirms the `gitsafe` approval line only; the
  partial-snapshot rule written in T060's pull request is discussed
  again in a task of its own (decided 2026-10-21).
- T156 keeps the e2e step on `./e2e/...`; T157 adds the race to `fast`
  only if a warm race run stays under the 3-minute budget, measured
  after T155 (decided 2026-10-21). It was, 116 to 118 s, so `fast` runs
  the race.
- The x/tools cover package may be imported for the cover profile
  (decided 2026-10-21).
- T156 includes the module root (part 4), its owns widened to 02 2.1,
  12 12.2 and the imports rows file (decided 2026-10-22).
- T077 is built without os-base, and its pull request waits for the A12
  measurement at the sitting; the branch is pushed with no pull request
  (decided 2026-10-22).
- When a maintained library would pay off and a task's plan rules out a
  new dependency, the maintainer is told with a recommendation before
  building by hand; the import stays ask-first (decided 2026-10-21; its
  home for every repository is the sandbox payload, carried in issue
  283).
