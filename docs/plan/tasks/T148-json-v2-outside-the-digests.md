---
id: T148
title: "encoding/json/v2 outside the digests"
milestone: M1
module: none
fate: new
owns:
  - tools/schemagen/schemagen.go
  - tools/schemagen/schemagen_test.go
  - tools/planviewer/internal/view/view.go
  - tools/planviewer/internal/view/view_test.go
  - tools/new/command_test.go
  - tools/new/kind_test.go
  - e2e/container_test.go
  - e2e/julieta_setup_test.go
  - e2e/internal/container/container.go
  - tools/kitpin/internal/pin/frontend.go
  - tools/kitpin/internal/pin/herdr.go
  - tools/kitpin/internal/pin/herdr_test.go
  - tools/ci/checks_test.go
  - tools/ci/internal/container/container.go
  - tools/ci/internal/imports/imports.go
  - tools/ci/internal/kits/kits.go
  - tools/ci/internal/license/modules.go
  - tools/ci/internal/license/modules_test.go
  - tools/ci/internal/lint/config_report_test.go
needs: []
after: []
ask_first:
  - kits
  - checks
sitting: none
---

# T148 - encoding/json/v2 outside the digests

Goal: every package outside `internal/canon` and `internal/manifest`
reads and writes JSON with `encoding/json/v2` and `encoding/json/jsontext`,
as REQ-0160 and REQ-0960 already require, and none imports the v1
package `encoding/json`. The standard library replaces nothing
hand-written here; the change is the decoder's defaults: a duplicate
member name is an error, member names match exactly, and text that is
not UTF-8 is refused instead of read as U+FFFD. The digests and the
manifest are T149's.

- Implements: REQ-0160 (03 (opening)), REQ-0960 (10 10.6).
- Operator: no.
- Acceptance:
  - No file outside `internal/canon` and `internal/manifest` imports
    `encoding/json`.
  - `go generate ./...` leaves `schemas/` unchanged: the generator writes
    the committed bytes (two-space indent, sorted keys, `<`, `>` and `&`
    as they are, U+2028 and U+2029 escaped, a final newline).
  - The plan viewer's data still escapes `<`, `>`, `&`, U+2028 and
    U+2029, as v1 did, and writes text that is not UTF-8 as U+FFFD; a
    test holds the line separators and the invalid text.
  - `tools/kitpin herdr` refuses a release whose `immutable` member is
    duplicated or spelled in another case; a test holds both.
  - The `go list -json`, `go mod download -json` and `go test -json`
    streams, the Dockerfile exec form and the upstream replies decode
    with v2; the `mise doctor --json` test names its members (`dirs`,
    `data`), which v1 matched by case folding. That test file,
    `tools/ci/internal/mise/which_test.go`, is T053's, which stays open
    (kept), so it is left out of owns; the change is its import and
    two tags.
- Verify: `go test ./tools/... && go vet -tags e2e,host ./e2e/... &&
  go generate ./... && git diff --exit-code schemas/`.
