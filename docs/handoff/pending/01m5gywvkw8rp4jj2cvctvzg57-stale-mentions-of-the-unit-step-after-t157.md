# Stale mentions of the unit step after T157

What is at stake: low. The stale mentions mislead a reader; they match
nothing and weaken no check.

Scope: this repository.

T157 retired the `unit` step. Mentions outside its owns remain in:

- comments of the tools/ci packages (license, drift, check step and
  printer, lint mise config test, container test, crossbuild "on every
  runner");
- the `tools/ci/internal/unit/**` glob and the reason text of the
  `checks` surface in .github/ask-first.yaml and CODEOWNERS;
- docs/plan.md.

T157's task file lists them with paths. Ask-first: the `checks` and
`approvals` surfaces for the yaml.

Reopen: the next change to any of those files, or a task that sweeps
them.
