# Session of 2026-10-20, the skills of T078 and the CI plan

Merged on 2026-10-20, with the gate it passed:

- #217 (b627388): T078, the julieta and handoff skills and
  `TestSkills*`, rewritten with skill-creator's progressive disclosure
  (each skill a `SKILL.md` and `references/`); ADR 0040 and REQ-1255
  (12 12.3); REQ-0595 (06 6.4) installs each skill's whole directory.
  Two code reviews, each fixed in one pass; a dry-run validation of five
  scenarios ("The skill validation scenarios of T078",
  01m5drzqxhskbxg2fx04rm3dzx); `go run ./tools/ci all` after the last
  commit; CI green on both hosts.

#217 added ADR 0040 as Accepted under `docs/adr/`, the ask-first
`decisions` surface, and its body carried only the `kits` approval line;
nothing checks the lines until `tools/ci pr` (T005) lands, and the review
missed it. Open for the operator in now.md.

This record ticks T078.

Closed by this record:

- "T078 awaits its review and pull request"
  (01m5daf06ddqsdsh6m5cn71wx6), by #217.

Decided on 2026-10-20, answering items raised in this session:

- Every skill is authored with `anthropic-skills:skill-creator` and its
  progressive disclosure: ADR 0040, for the skills under `skills/`
  (every adopter) and for every repository and sandbox of the
  maintainer (the payload today, T114 later, through "Rules to move when
  the interim payload is retired", 00000000001v2n0qea13kr8yvy).
- No prompt goes to the config repository now: T114 ("The personal kit
  in the config repository") builds its instructions file, and a rule
  meant for it waits in the note of rules to move.
- The CI task of "One race run, test tags and parallel CI jobs"
  (01m5drzqsp6zg34xzmywckwb7b) is approved for planning: one test run,
  always with `-race`; filters as build tags; parallel jobs, with macOS
  running only the race; the Go idiom findings fixed as defects.
- Sharding the macOS race and a GOCACHE cache are deferred: "Shard the
  macOS race run and cache GOCACHE, deferred"
  (01m5drzqtpe5w9q37kv3g04xnp).
- romeu-e-julieta and the config repository use the harness's defaults
  as far as they go, keep their own customizations few, and publish one
  once validated: "Prefer the harness's defaults and publish what is
  left" (01m5drzqvjrf3e7p7b2v68temw), with its open questions.

In flight at the end of the session: #218, the findings of the skill
validation (dates with a time zone, when `--repo` applies, `memory list
--query` before an add, the final handoff outliving the sandbox); code
review fixed, `go run ./tools/ci all` exit 0 after its last commit.

Shared with the maintainer, outside the repository: two secret gists
describing the mechanics of the handoff and julieta skills, their
Mermaid diagrams checked with mermaid's own parser before publishing.

Lessons:

- A dry run of a skill finds what its structural test cannot: the test
  proved every command existed, and an agent still dated a decision
  "today" and said a host file would be lost with the sandbox.
- A fixture that asserts a substring of a message quoting its input
  passes for the wrong reason; assert the part that names the defect.
- A `;` inside a Mermaid sequence message ends the statement; parse a
  diagram before publishing it.
