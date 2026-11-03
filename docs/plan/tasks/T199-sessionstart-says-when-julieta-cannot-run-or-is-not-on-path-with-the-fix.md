---
id: T199
title: "SessionStart says when julieta cannot run or is not on PATH, with the fix"
milestone: M2
module: kits
fate: new
owns:
  - kits/julieta-claude/managed-settings.json
  - kits/julieta-claude/julieta-claude.yaml
  - internal/agent/claude/hooks.go
  - internal/agent/claude/digest.go
  - internal/handoff/print.go
  - internal/cmd/julieta/handoff/show.go
  - internal/cmd/julieta/handoff/show_test.go
  - internal/cmd/julieta/handoff/helper_test.go
  - internal/cmd/julieta/handoff/export_test.go
  - internal/cmd/julieta/setup/link.go
  - internal/cmd/julieta/setup/hooks_path.go
  - e2e/kit_julieta_claude_test.go
needs: []
after: []
ask_first:
  - kits
sitting: none
---

# T199 - SessionStart says when julieta cannot run or is not on PATH, with the fix

Goal: a session that starts without a usable julieta says so in a block
that stands apart from the other SessionStart lines, with the command
that repairs it.

Added on 2026-11-03 (scope this repository): with the `.romeu/bin`
mount absent the kit's wrapper printed `... could not run, the
.romeu/bin mount may be absent; run julieta status`, and `julieta
status` cannot run either (reproduced: `julieta: not found`, exit 127);
with the mount present and no `PATH` link, SessionStart said nothing.

1. The alert block (REQ-1352): an empty line, `!! <fault>`, indented
   `<label>: <value>` lines ending with `fix:`, an empty line.
2. The kit's wrappers (REQ-0673): the session hooks print the block with
   `fix: on the host, romeu recreate <name>`, `<name>` from
   `SANDBOX_NAME`, and a line that `romeu recreate` is not built yet
   (T068) so the sandbox is recreated by hand; the PreToolUse guard
   prints the same block on stderr and still exits 2.
3. `julieta handoff show --hook` (REQ-1351): when
   `$HOME/.local/bin/julieta` does not lead to a file, the block
   `julieta-link-missing` with `fix: <running julieta> setup`, after
   the one-line warnings.

Deferred: when T068 merges, the fix line of item 2 drops the "not built
yet" line; that merge is the trigger.

- Implements: REQ-1351 (08 8.2), REQ-1352 (08 8.2), REQ-0673 (08 8.2).
- Operator: yes, the approval line for `kits/**` (ask-first `kits`),
  then the sandbox definitions repository's pin of the kit and a
  recreate, so the new wrappers reach a sandbox.
- Acceptance:
  - The kit e2e runs each session wrapper with julieta absent and
    asserts the whole block on stdout and exit 0, with and without
    `SANDBOX_NAME`; the guard case asserts the block on stderr and exit 2.
  - A unit test of `handoff show --hook` with no `PATH` link prints the
    block after the warnings and before the built-in memory line, and
    with the link prints no block.
- Verify: `go run ./tools/ci all`.
