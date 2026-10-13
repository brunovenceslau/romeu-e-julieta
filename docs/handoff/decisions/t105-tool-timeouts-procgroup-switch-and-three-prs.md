# T105: tool timeouts, procgroup switch and three PRs

Decided on 2026-10-13:

- procgroup.LookPath is exported. Tool test fixtures go through
  procgroup.Run with an explicit Env; signal tests signal the pid with
  syscall.Kill and read ExitError; `tools/ci/lintconfig_test.go` moves
  from go/build to go/build/constraint.
- The tools' timeout rows: step 15m, race 40m, git 2m, and a probe class
  of 30s (uname, sysctl, mise which and --version, go list). `go env` in
  tools/mutants runs in the Mise class (30m); the mutants' go builds and
  tests run Unbounded.
- SIGHUP is dropped from tools/mutants, recorded as a behaviour change
  the spec forces.
- Three sequential PRs. A: the 04 4.1 Starting values rows alone
  (REQ-1220). B: the procgroup switch. C: the register-by-file move.
  T105 and T137 are marked merged on the last.
