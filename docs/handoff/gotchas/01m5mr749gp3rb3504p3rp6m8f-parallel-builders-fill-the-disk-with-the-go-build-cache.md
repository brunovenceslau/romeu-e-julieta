# Parallel builders fill the disk with the Go build cache

With four builders at a time, `~/.cache/go-build` reached about 14 GB on
the 20 GB sandbox disk, twice (2026-10-23). Runs failed with "no space
left on device".

Run `df -h /` before judging a failure; `go clean -cache` frees the
space, and the next build refills the cache. See also "The Go build
cache fills the sandbox disk" (0000000000r27w7xpq62fxry14).
