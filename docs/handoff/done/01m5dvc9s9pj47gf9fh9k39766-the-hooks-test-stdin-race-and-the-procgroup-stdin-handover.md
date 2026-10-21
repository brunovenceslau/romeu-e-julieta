# The hooks test stdin race and the procgroup Stdin handover

Done on 2026-10-20, in the pull request that carries this record:

- `internal/cmd/julieta/hooks`: the stdin seam is a function read once
  per run, as `environ` is, and the test's `Process` gives each run a
  fresh reader, as each process has its own stdin.
  `TestRunGivesEachRunItsStdin` failed on the shared reader (the second
  run read an empty stdin) and passes now; `go test -race -count=50` of
  the package passed.
- `internal/procgroup`: the doc of `Command.Stdin` says the reader is
  handed over, so the caller must not read, reuse or share it after
  Run returns. Decided on 2026-10-20: a handover in the doc, not a wait,
  since neither Run nor os/exec can interrupt a `Read` blocked in a
  reader that is not a file.

Gates: a code review (no second racing path found; production stdin is
an `*os.File`, which gets no copy); `go run ./tools/ci all` after the
last commit; CI on both hosts.

Closed by this record:

- "A data race in the hooks run test under load"
  (01m5ds9qp1hxjt062r5y3fjaf6).

Lesson: a reader handed to a goroutine nobody waits for belongs to that
goroutine; a test seam that takes a value, not a shared reader, makes
the reuse impossible.
