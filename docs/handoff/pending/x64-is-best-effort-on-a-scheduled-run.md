# x64 is best effort, on a scheduled run

Decided on 2026-10-17: the two x64 legs, `macos-26-intel` and
`ubuntu-26.04`, leave the `ci.yml` matrix that every pull request waits
for, and run `tools/ci all` on a scheduled workflow instead. darwin/amd64
and linux/amd64 support becomes best effort: a red scheduled run is
fixed when it is seen, and blocks no pull request.

Why: the cut is for speed and for the share of users, not for
failures. The Intel leg is the slowest one every pull request waits
for, and few users run an Intel Mac. linux/amd64 goes with it because
it exists only as the sandbox of an Intel Mac host
(S3 of docs/spec.md; 00 0.2, a macOS host, Intel or Apple silicon, and
no Linux host). With the Intel host best effort, its sandbox arch is
too.

What stays on every pull request: `fast` lints `linux/amd64` and
`darwin/amd64` on each leg, so an amd64 build break still shows; the
Linux-only steps (`license`, the container and hybrid e2e) still run on
`ubuntu-26.04-arm`. What moves to the scheduled run: tests and race on
real x86_64.

The search below was shallow, done while deciding: the task that makes
the change repeats it in full (`git grep -n -i -E
"amd64|x64|x86_64|intel|macos-26-intel|ubuntu-26.04\b"` over the spec,
the plan, `tools/ci`, `.github/` and `e2e/`) before trusting it.

- `.github/workflows/ci.yml:50-58`, the matrix; the scheduled workflow
  joins `.github/workflows/` (`approvals`, `dependencies`).
- 10 10.2 REQ-0868 (`docs/spec/10-testing-style.md`, near line 493):
  the four runners of `ci.yml` and the labels verified on its first run.
- The test-level table of 10 (`10-testing-style.md:19`): the e2e level
  names "linux amd64, ... macOS Intel" in CI.
- S1 of docs/spec.md (`spec`): the release still ships the four
  archives, the amd64 ones marked best effort. S3 still needs an Intel
  Mac host.
- T002's acceptance names `macos-26-intel` among the runners.
- `tools/ci` keeps both labels (`workflows.go:94`, `citest.go:334`) and
  the mise pins (`pin.go:41`), since the scheduled run uses them.

Reopen: the operator's order, given on 2026-10-17; the change follows
T107.
