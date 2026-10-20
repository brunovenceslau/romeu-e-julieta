# T137 signal-spec PR: items left at the round cap

Gate GO at 50ec159 and c76e04e.

- The call-site scan reads only files in the tree, so a go.mod
  replace/require route or a dependency that wraps os/exec is missed:
  base T039's callsites check on `go list -deps`.
- A non-admitted tool under tools/ (tools/new/main.go) still reaches git
  through tools/ci/git; it closes with T105's switch. Add a T105
  acceptance line saying so.
- T105 should also name tools/ci/lintconfig_test.go's go/build import
  beside the 13 os/exec test files.
- tools/ci/check_callsites.go should join the `procgroup` surface at T039.
- Nit: the T039 task file has a 79-character line; rewrap.
- Nit: toolImports also lets tools/mutants import go/build; the list goes
  away at T105.

Reopen: T039 starts, or the next spec pass on signals. T105 merged on
2026-10-16 (#63) without taking these items on.
