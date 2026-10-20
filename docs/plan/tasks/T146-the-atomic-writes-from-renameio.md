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
  - tools/ci/internal/check/write.go
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
synced before the rename. Under an `os.Root`, the library creates its
temporary file at the top of the root, so a site opens the root of the
target's directory first. Two sites keep their code, with the reason of
ADR 0038, decision 3: `tools/release/internal/build/build.go` renames a
directory, which the library does not cover, and
`tools/ci/internal/setup/install.go` removes the downloads an
interrupted run left by their name (`.mise-*.download`), while the
library's temporary names are its own and not a contract.

Two effects of the swap: `tools/ci/internal/check/write.go`, `view.go`
and `container.go` now sync the file before the rename, as the other
sites already did; and the library leaves its empty temporary file
behind when the `Chmod` right after creating it fails (`NewPendingFile`
returns before the caller can clean up), which the code it replaces did
not. The second is accepted, since fixing it is a change upstream.

- Implements: none: a swap under ADR 0038; the requirements stay with
  the tasks that own each site.
- Operator: no.
- Acceptance:
  - The seven code files of `owns` call the library and keep their tests
    (the two that looked for leftovers by the old names read the
    library's);
    `link.go` replaces its link with `renameio.Symlink`.
  - A test per package that writes a file holds that the temporary file
    is created in the target's directory, not in `$TMPDIR`, and that the
    mode is the one the site sets, over an existing file of another
    mode.
  - REQ-1049 (12 12.1) lists the module.
- Verify: `go test ./internal/cmd/julieta/setup/... ./tools/kitpin/... ./tools/ci/internal/hygiene/... ./tools/ci/internal/check/... ./tools/planviewer/... ./e2e/internal/container/...`.
