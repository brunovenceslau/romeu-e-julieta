# fast does not run coverage or race

`go run ./tools/ci fast` runs format, vet, generated, lint and unit;
coverage (with its 80% floor, S9), the race run and the license step run
only in `all`. On 2026-10-19 a pull request passed `fast` and failed CI
on coverage, and on macOS only, where TMPDIR is under a symbolic link
(/var to /private/var).

Before opening a pull request, run `go run ./tools/ci all` on a clean
tree; to rehearse the macOS case on Linux, run the tests once with
TMPDIR set to a symbolic link.
