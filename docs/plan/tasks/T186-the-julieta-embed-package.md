---
id: T186
title: "The julieta embed package"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/embedded/**
  - internal/cmd/romeu/approve/toolchain.go
  - internal/cmd/romeu/approve/toolchain_test.go
  - tools/ci/internal/lint/config_directives_test.go
  - tools/ci/internal/imports/rows.go
  - tools/release/main.go
  - tools/release/main_test.go
  - tools/release/internal/build/build.go
  - tools/release/internal/build/build_test.go
  - tools/release/internal/build/fixture_test.go
  - tools/ci/internal/imports/imports_test.go
  - internal/cmd/romeu/sync/step1_lock.go
  - REUSE.toml
  - docs/spec/02-layouts.md
  - docs/spec/10-testing-style.md
  - docs/spec/12-engineering.md
needs:
  - T042
  - T103
after:
  - T181
ask_first:
  - checks
  - release
sitting: none
---

# T186 - The julieta embed package

Goal: the package of romeu that reads the julieta binary
`tools/release build` maps into `internal/embedded/julieta` (T103). T103
deferred it to "T043 and T066, whose embed package consumes the path",
but neither task owned the path, so no task made it. Decided on
2026-10-29, scope this repository: a task of its own, which starts now,
after T181 (which also edits 12) and in parallel
with T183 and T184, and which `sync` (T043) and `run`
(T066) read.

1. `internal/embedded/` holds a checked-in empty placeholder file at
   `internal/embedded/julieta` and a `//go:embed julieta` byte var: a
   plain `go build` embeds zero bytes (REQ-0130), and the release build
   replaces the file through its overlay. Without the placeholder a
   plain build does not compile.
2. Gate 1 (`approve`) reads its julieta digest from the package in
   place of its placeholder seam, and `TestPlaceholderJulietaUntilEmbedded`
   is rewritten for an embedder that exists: today it asserts that no
   file embeds the path.
3. The `go:embed` scope of the lint step admits
   `internal/embedded/embedded.go` (ADR 0007), and the import rows of 10 10.7 put `embedded` on the
   romeu-only row (REQ-0968), so julieta never links it.
4. 02 2.1 lists `internal/embedded/` in the tree and states that the
   build maps the file with an overlay and writes nothing into the
   tree; 12's module table lists the directory under `romeu-cli`.
5. The note `tools/release build` prints when no package consumes the
   path no longer names T043, T066 and T077; `REUSE.toml` covers the
   placeholder. Since `go list` names the committed placeholder with or
   without the overlay, the build also checks that each romeu binary
   holds the bytes of its julieta, and a release refuses one that does
   not.

- Implements: none: the consumer of REQ-0129 and REQ-0130, which T103
  and T043 implement.
- Operator: no.
- Acceptance:
  - A plain `go build ./cmd/romeu` compiles, and its embedded byte var
    is empty; `sync` and gate 1 see a source build.
  - `go list -deps -json` of romeu with the release overlay reports
    `julieta` among the package's embed files, the match
    `tools/release build` uses to say a package consumes the path.
  - `tools/ci imports` refuses a julieta package that imports
    `internal/embedded`.
  - `go run ./tools/ci all` passes, `license` and `lint` included.
- Verify: `go run ./tools/release build --version v1.0.0 --dry-run`
  prints no line saying the archives carry no julieta.
