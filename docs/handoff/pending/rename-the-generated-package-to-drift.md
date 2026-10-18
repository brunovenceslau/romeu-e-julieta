# Rename the generated package to drift

Decided on 2026-10-17 (decision
"no-generated-code-header-and-no-package-named-generated"): the
package `tools/ci/internal/generated`, which holds the drift check and
today's generators (CODEOWNERS, the ADR index, `checks.md`), becomes
`tools/ci/internal/drift`, and its generators move next to their data.
The step keeps the name `generated`, which the step table of 10 10.2
names (REQ-0822).

The same pull request gives the drift check a way to take a generator
from outside its package, since T008 owns one file of `tools/ci`,
`tools/ci/generator_cli.go`, that hands it the two generators of
`internal/cli` and `internal/cli/clidoc`, and owns nothing under the
drift package.

Surface: `checks` (`.github/ask-first.yaml` names
`tools/ci/internal/generated/**`; the glob changes with the rename).

Reopen: the start of T008's rework on cobra (#109), which needs this
rename merged first.
