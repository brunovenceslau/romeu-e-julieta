# The go that builds tools/ci is not held to the pin

Since #69 the go shim installs nothing. With the pinned go missing, the
shim falls through to the next go on PATH, and the hook's probe only
holds that go to the floor of `go.mod`: a newer go builds and runs
`tools/ci` itself. The checks still run on the pinned go, which `mise
which` resolves and checks against `mise.lock`, so only the checker's
own build differs. That go is the pusher's own, so this is no trust
boundary. A fix would have `tools/ci` compare `runtime.Version()` with
the go of `mise.lock` and refuse, naming `go run ./tools/ci setup`; it
is a spec change (10 10.2) and touches the `checks` surface.

Reopen: a check result differs between a push and CI, or the next
change to the pre-push hook or to `tools/ci`'s start.
