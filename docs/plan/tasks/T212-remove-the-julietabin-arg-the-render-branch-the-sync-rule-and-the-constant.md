---
id: T212
title: "Remove the julietaBin arg: the render branch, the sync rule and the constant"
milestone: M4
module: render
fate: new
owns:
  - internal/render/kits.go
  - internal/render/kits_test.go
  - internal/render/kits_personal_test.go
  - internal/render/types.go
  - internal/render/env_test.go
  - internal/render/render_test.go
  - internal/render/testdata/env/**
  - internal/cmd/romeu/sync/rule_julieta_bin*.go
  - internal/cmd/romeu/sync/step6_render_test.go
needs:
  - T200
  - T201
after:
  - T204
ask_first: []
sitting: none
---

# T212 - Remove the julietaBin arg: the render branch, the sync rule and the constant

Goal: the dead code of the reserved kit arg `julietaBin` goes: the
render branch that passes it to a descriptor that declares it, the
constant `render.JulietaBinArg`, the sync rule that refuses a spec
setting it, and their tests and golden lines.

Added on 2026-11-07 (scope this repository), as decided on 2026-11-07
(C1-12): T200's item 4 moves here. Once T201 removes the arg's
declaration from the `julieta-claude` kit, the render never passes the
arg, since it passes it only to a descriptor that declares it, so the
branch is dead; no sandbox of romeu exists before T066. Removing it
here, after T200 and T201, keeps the critical path as it was, and no
task needs this one. It comes after T204 for the render's env tests and
golden, which T204 also edits.

1. `internal/render/kits.go`: the branch that renders `julietaBin`, and
   the constant, go; the render passes no arg to the `julieta-claude`
   kit (06 6.4 item 2).
2. `internal/cmd/romeu/sync/rule_julieta_bin.go`: the rule and its
   registration go, with the sync test that asserted its error (T043's
   acceptance line is history).
3. The tests and the golden that carry `julietaBin` change with them.

- Implements: REQ-0596 (06 6.4).
- Operator: no.
- Acceptance:
  - `git grep -n julietaBin -- internal e2e` finds nothing, and no
    rendered kit carries args for `julieta-claude`.
  - The render tests of a project with the product kits pass with no
    `Rendered` args, and the sync test of the removed rule is gone.
- Verify: `go test ./internal/render/... ./internal/cmd/romeu/sync/... -run
  '^(TestProductKits|TestRenderCandidates)' -v`. The pattern names the
  tests this task changes, and the reviewer reads them in the `-v`
  output (10 10.8).
