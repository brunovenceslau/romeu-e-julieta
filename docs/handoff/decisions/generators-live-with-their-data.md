# Generators live with their data

Decided on 2026-10-17.

- A generator lives in the package that owns its data. The generator of
  `docs/reference/errors.md` lives in `internal/cli`, next to the error
  table.
- The generated check of `tools/ci` only calls a generator and checks
  the drift; it holds no generator of its own.
- `go generate ./...` keeps its one directive, in `tools/ci/generate.go`,
  which already writes every output.
- A generated Go file is named `*_gen.go`.
- The generator of the per-command reference pages imports
  `github.com/spf13/cobra/doc`. It lives in a subpackage next to
  `internal/cli` that no binary imports (for example
  `internal/cli/clidoc/`), because ADR 0015 decision 8 keeps `cobra/doc`
  out of every product binary.
