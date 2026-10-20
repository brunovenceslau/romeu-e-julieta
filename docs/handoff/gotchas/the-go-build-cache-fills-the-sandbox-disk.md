# The Go build cache fills the sandbox disk

The sandbox disk is 20 GB. On 2026-10-20, with several subagents
building and testing in worktrees of their own, the Go build cache
(`~/.cache/go-build`) reached 15 GB and `go run ./tools/ci all` failed
race, e2e and coverage with "no space left on device", on a docs-only
change.

When a check fails for no reason the diff explains, run `df -h /`
first; `go clean -cache` frees the space, and the next build refills
the cache.
