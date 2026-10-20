---
id: T146
title: "The atomic writes from renameio"
milestone: M1
module: none
fate: new
owns:
  - internal/cmd/julieta/setup/state.go
  - internal/cmd/julieta/setup/link.go
  - tools/kitpin/internal/pin/pin.go
  - tools/ci/internal/hygiene/add.go
  - tools/ci/internal/setup/install.go
  - tools/planviewer/internal/view/view.go
  - e2e/internal/container/container.go
  - go.mod
  - go.sum
  - docs/spec/12-engineering.md
needs:
  - T145
after: []
ask_first:
  - dependencies
  - checks
  - kits
sitting: none
---

# T146 - The atomic writes from renameio

Goal: the second swap of
[ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md),
decision 5: the hand-written replacements of a file by a temporary file
and a rename go through `github.com/google/renameio/v2` (v2.0.2,
Apache-2.0), approved as a new dependency on 2026-10-20.

Each site keeps its behavior: the temporary file is created beside the
target (`WithTempDir`; the library's default may pick `$TMPDIR`), the
permission each site sets today is set again (`WithStaticPermissions`,
since `WriteFile` would copy an existing file's mode), and the data is
synced before the rename. `tools/release/internal/build/build.go` renames
a directory, which the library does not cover, so it keeps its code.

- Implements: none: a swap under ADR 0038; the requirements stay with
  the tasks that own each site.
- Operator: no.
- Acceptance:
  - The seven sites of `owns` call the library and keep their tests;
    `link.go` replaces its link with `renameio.Symlink`.
  - A test per package that writes a file holds that the temporary file
    is created in the target's directory, not in `$TMPDIR`, and that the
    mode is the one the site sets, over an existing file of another
    mode.
  - REQ-1049 (12 12.1) lists the module.
- Verify: `go test ./internal/cmd/julieta/setup/... ./tools/kitpin/... ./tools/ci/internal/hygiene/... ./tools/ci/internal/setup/... ./tools/planviewer/... ./e2e/internal/container/...`.
