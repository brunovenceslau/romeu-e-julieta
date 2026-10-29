# A recreated sandbox needs mise install and tools/ci setup

After a recreate, `go run ./tools/ci all` stops with "run mise trust and
mise install", and then fails every Go step with "the module cache lacks
a module of go.sum ... (GOPROXY=off)".

In each new worktree run `mise trust` and `mise install` (the latter
once), then `go run ./tools/ci setup` (2026-10-20).

Before those, run `julieta setup` once, from the binary on the read-only
bin mount (`<bin mount>/julieta-linux-<arch> setup`, step 7 of the
sandbox definitions repository's julieta runbook). Without it `julieta`
is not on `PATH` and git's global `core.hooksPath` is unset, so commits
pass only the system hooks, not the julieta dispatcher; the session hooks
still work, since they call julieta by its absolute path, so nothing
looks wrong (measured on 2026-10-28). A dotfile in the memory dir, such
as a `.DS_Store` the Finder writes on the host, fails it: remove the
file and run it again (issue #303).

Reopen: when T179 merges, `julieta setup` runs at every boot and the
first paragraph's commands become T178's repo task; this note then
shrinks to what is left by hand.
