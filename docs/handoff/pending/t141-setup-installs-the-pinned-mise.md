# T141: setup installs the pinned mise

The spec is REQ-1236 to REQ-1239, with the two measurements recorded by
#61. The code goes in tools/ci/internal/mise and tools/ci/internal/setup,
the paths T105 PR-C creates, so this pull request is stacked on PR-C.

A sandbox whose image ships another mise needs the pinned mise installed
by hand until this task merges. On 2026-10-16 the mise of the sandbox
image was 2026.9.15, in /usr/local/bin.

Reopen: T105 PR-C opens.
