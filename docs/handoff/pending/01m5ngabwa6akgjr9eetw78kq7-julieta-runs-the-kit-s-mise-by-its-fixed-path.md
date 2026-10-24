# julieta runs the kit's mise by its fixed path

What is at stake: low. Until this lands, julieta runs the `mise` that
`PATH` resolves (T053), so a `mise` earlier on `PATH` than the kit's
would run instead; the kit puts its own first, so nothing breaks today.

Scope: this repository.

The re-plan of 2026-10-22 in T077 deferred resolving `mise` by a fixed
path until the kit defined where it installs mise. T077 now installs it
at `/usr/local/bin/mise` (decided on 2026-10-23). What is left is
julieta's code, in T053's package and outside T077's `owns`: the
manifest carries the path of the kit's mise, and julieta runs mise by
that path instead of finding `mise` on `PATH`.

Reopen: when T077 is merged; a task that names the manifest field and
T053's package carries it.
