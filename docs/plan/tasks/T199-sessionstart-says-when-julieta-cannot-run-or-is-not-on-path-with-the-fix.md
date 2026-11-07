---
id: T199
title: "julieta resolves by name through a launcher, and SessionStart says when it cannot run or is not on PATH, with the fix"
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
  - e2e/kit_julieta_claude_test.go
needs: []
after: []
ask_first:
  - kits
sitting: none
---

# T199 - julieta resolves by name through a launcher, and SessionStart says when it cannot run or is not on PATH, with the fix

Goal: `julieta` resolves by name in every sandbox with the kit, and a
session that starts without a usable julieta says so in a block that
stands apart from the other SessionStart lines, with the command that
repairs it.

Added on 2026-11-03 (scope this repository): with the `.romeu/bin`
mount absent the kit's wrapper printed `... could not run, the
.romeu/bin mount may be absent; run julieta status`, and `julieta
status` cannot run either (reproduced: `julieta: not found`, exit 127);
with the mount present and no `PATH` link, SessionStart said nothing,
and since the skills call `julieta` by name, a sandbox where `setup`
had not run wrote a handoff to a file instead.

1. The launcher (REQ-1353): a kit step writes `/usr/local/bin/julieta`,
   root-owned, 0755, `exec <julietaBin> "$@"`; the hooks and the guard
   keep the absolute path.
2. The alert block (REQ-1352): an empty line, `!! <fault>`, indented
   `<label>: <value>` lines ending with `fix:`, an empty line.
3. The kit's wrappers (REQ-0673): on status 126 or 127 only, the session
   hooks print the block with `fix: on the host, romeu recreate
   <name>`, `<name>` from `SANDBOX_NAME` with every character outside
   `A-Za-z0-9._-` removed, and that `romeu recreate` is not built yet
   (T068) so the sandbox is recreated by hand; the PreToolUse guard
   prints the same block on stderr and still exits 2.
4. `julieta handoff show --hook` (REQ-1351): when `julieta` is not
   found on `PATH`, the block `julieta-link-missing` with `fix:
   <running julieta> setup`; when the one found is neither the launcher
   nor the running julieta, the same slug with `fix: rm <its path>`.

Deferred: when T068 merges, the fix line of item 3 drops the "not built
yet" line; that merge is the trigger.

Amended on 2026-11-06 (the sbx 0.51 and kit schema m.12 amendments): the
launcher of item 1 and its acceptance line are history: the
specification now names a root-owned symbolic link
`/usr/local/bin/julieta` to `/opt/julieta/bin/julieta` (REQ-1353), which
T201 installs in place of the `sh` launcher once the mount has its fixed
target (T200); the alert block, the wrappers and `handoff show --hook`
are unchanged, and the launcher the block names is now the link.

- Implements: REQ-1353 (06 6.4), REQ-1351 (08 8.2), REQ-1352 (08 8.2), REQ-0673 (08 8.2).
- Operator: yes, the approval line for `kits/**` (ask-first `kits`),
  then the sandbox definitions repository's pin of the kit and a
  recreate, so the new wrappers reach a sandbox.
- Acceptance:
  - The kit e2e runs each session wrapper with julieta absent and
    asserts the whole block on stdout and exit 0, with and without
    `SANDBOX_NAME`; the guard case asserts the block on stderr and exit 2.
  - The kit e2e installs the launcher root-owned 0755, the agent cannot
    rewrite it, and `julieta version` runs by name through it; a
    hostile `julietaBin` writes no launcher.
  - A unit test of `handoff show --hook` prints each block after the
    warnings and before the built-in memory line: `julieta` off `PATH`,
    and another program on `PATH`; with `julieta` on `PATH` no block.
- Verify: `go run ./tools/ci all`.
