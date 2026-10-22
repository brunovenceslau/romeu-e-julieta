# The container e2e annotates a cached download it heals (T158)

Done on 2026-10-22, in the pull request that carries this record:

- When the container e2e finds a cached download that fails its pin, it
  removes it, fetches it once more and appends a notice, naming the
  file and the pin, to a private file the e2e step makes
  (`ROMEU_E2E_NOTICE_FILE`). After the run `tools/ci` prints each notice
  before the verdict: a GitHub Actions annotation on a runner, a
  `warn` line elsewhere. The channel is a file the harness writes, not
  the output of a test, so no text a test prints reaches it or turns a
  failure into an infrastructure error. REQ-0798 in 10 10.1 says so.
- Decided on 2026-10-20: the removal writes a job annotation, a channel
  the classifier does not read.
- Not changed: the output of a failing e2e step is printed raw, so a
  workflow command a failing test prints is still read by the runner
  (the output of a step that runs code stays raw). Reopened by a
  decision to print it through `git.SafeLines`.

Closed by this record:

- "The spec catches up with the hardening of T127, T143 and T144"
  (0000000000pz8zh0x006xkqa4w), whose last open question this was, by
  T158.
