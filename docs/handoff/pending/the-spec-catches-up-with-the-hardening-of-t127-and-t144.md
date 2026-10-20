# The spec catches up with the hardening of T127 and T144

What is at stake: small; the code is stricter than the specification
says, so nothing is admitted that should not be. A reader of the
specification sees less than the checks enforce.

- REQ-0808 (10 10.2) states the minimum for the release publish job.
  Since #188 the grammar also requires `on: push` with `tags` only, admits
  only checkout, mise and `actions/attest-build-provenance` in that job,
  and admits only `"$GITHUB_REF_NAME"` as a quoted variable in its four
  token-carrying steps.
- REQ-0798 (10 10.1) says a digest or checksum mismatch "is always a test
  failure". With T144 a cached download that fails its pin is removed
  and fetched once; the fetched file is still held to the pin. The
  sentence needs that exception, and the healing writes no log line, so
  a corrupted or tampered cache entry no longer shows; decide whether it
  should, without adding output the infrastructure classifier counts.
- REQ-0901 (the order build, attest, verify, notes, publish) and the
  required attest and verify steps are not checked by the grammar; they
  belong with T088's `release.yml`.

Reopen: after T144 merges, as one docs pull request; or when T088
starts.
