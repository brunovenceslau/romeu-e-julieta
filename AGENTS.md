# romeu-e-julieta

Pointers only; each linked file holds the rule.

- Current state and next steps: `docs/handoff/now.md`. What the handoff
  tree is: `docs/handoff/about.md`.
- Specification: `docs/spec.md` and `docs/spec/`. Build plan:
  `docs/plan.md` and `docs/plan/tasks/`.
- Ask-first surfaces: the list is `.github/ask-first.yaml`; how a change
  to one is approved is in `docs/spec/12-engineering.md` (12 12.4), and
  the standing grants are in `docs/grants.yaml`.
- Decisions: `docs/adr/`, with these three read first.
  - `docs/adr/0038-search-for-a-maintained-library-before-writing-one.md`:
    search for a maintained library before writing code that parses,
    encodes or implements a standard.
  - `docs/adr/0039-state-the-scope-of-every-decision-record.md`: every
    decision record states its scope and where the rule is written.
  - `docs/adr/0040-author-every-skill-with-skill-creator-and-progressive-disclosure.md`:
    skills are authored with skill-creator and its progressive
    disclosure.
- Local CI: `go run ./tools/ci all`; its parts are `fast`, `static`,
  `race` and `e2e` (`docs/reference/checks.md`).
- A recreated sandbox needs setup first: `docs/handoff/gotchas/`.
