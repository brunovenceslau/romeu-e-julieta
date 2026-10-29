# A recreated sandbox needs julieta setup, mise install and tools/ci setup

After a recreate, `go run ./tools/ci all` stops with "run mise trust and
mise install", and then fails every Go step with "the module cache lacks
a module of go.sum ... (GOPROXY=off)".

Until T178 and T179 merge, first run `julieta setup` once, from the
binary on the read-only bin mount
(`<bin mount>/julieta-linux-<arch> setup`, step 7 of the sandbox
definitions repository's julieta runbook). Without it `julieta` is not
on `PATH` and git's global `core.hooksPath` is unset, so commits pass
only the system hooks, not the julieta dispatcher. Nothing looks wrong,
because the session hooks call julieta by its absolute path (measured on
2026-10-28). A dotfile in the memory dir fails it, such as a `.DS_Store`
the Finder writes on the host, which can come back while the folder is
open there (REQ-0645 admits no dotfile but julieta's temporary names):
remove it and run `setup` again (issue #303).

Then, in each new worktree, run `mise trust` and `mise install` (the
latter once), then `go run ./tools/ci setup` (2026-10-20).

When T178 merges, `julieta setup` itself trusts the mise configuration,
installs the locked tools and runs this repository's `setup` task, which
fills the module cache (REQ-1306, REQ-0400, REQ-1307); when T179 merges,
it runs at every boot. This note then shrinks to what is left by hand.
