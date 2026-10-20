# A recreated sandbox needs mise install and tools/ci setup

After a recreate, `go run ./tools/ci all` stops with "run mise trust and
mise install", and then fails every Go step with "the module cache lacks
a module of go.sum ... (GOPROXY=off)".

In each new worktree run `mise trust`, once `mise install`, then `go run
./tools/ci setup` (2026-10-20).
