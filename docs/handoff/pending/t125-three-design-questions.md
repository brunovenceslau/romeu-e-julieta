# T125: three design questions

T125 (`tools/schemagen`, `tools/ci schema` and the license allowlist)
waits for three answers, measured on 2026-10-18.

1. The `$schema` hint of each YAML file under `examples/` names "the
   release that ships it" (REQ-0836; 03 3.2,
   `.../romeu-e-julieta/v1.0.0/schemas/project.v1.json`). No file says
   what the next release is, and no tag exists yet. Recommended: one
   committed version file that `tools/release` will also read.
2. "`go generate` writes a schema for the generator's fixture type",
   but every generated output must match a glob of `plan.Generated`
   (`TestGeneratedOutputsAreOwnedByNoTask`), and `schemas/` is for
   product schemas (the first is T135's). Recommended: the fixture
   schema is written in a test over a fixture tree, not committed.
3. The new `schema` check and the second step of `license` join the
   pinned lists in `tools/ci/checks_test.go` and `tools/ci/all_test.go`,
   which T009 (#138, open) owns. Recommended: start after T009 merges,
   with a re-plan that names those files.

The license allowlist itself is clear: read the license of every
module `go.sum` pins by its source (those `setup` downloads); a module
pinned by its `go.mod` alone gives no package to any build.

Reopen: the operator's answers.
