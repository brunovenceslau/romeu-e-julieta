# fast does not run the license step

`go run ./tools/ci fast` runs format, vet, generated, lint and unit
(tools/ci/fast.go); the `license` step, `reuse lint` among it, runs only
in `all` (tools/ci/all.go). In the planviewer work a new file without an SPDX
header (tools/planviewer/template.html) passed `fast` and the pre-push
hook and failed `license` in CI.

Before opening a PR that adds a file, run the license step: `go run
./tools/ci all`, or `reuse lint` on its own.
