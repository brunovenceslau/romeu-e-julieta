# The spec catches up with the hardening of T127, T143 and T144

What is at stake: small; the code is stricter than the specification
says, so nothing is admitted that should not be. A reader of the
specification sees less than the checks enforce.

- REQ-0808 (10 10.2) states the minimum for the release publish job.
  Since #188 the grammar also requires `on: push` with `tags` only, admits
  only checkout, mise and `actions/attest-build-provenance` in that job,
  and admits only `"$GITHUB_REF_NAME"` as a quoted variable in its four
  token-carrying steps.
- REQ-0798 (10 10.1) says a digest or checksum mismatch "is always a test
  failure". Once T144 lands, a cached download that fails its pin is removed
  and fetched once; the fetched file is still held to the pin. The
  sentence needs that exception, and the healing writes no log line, so
  a corrupted or tampered cache entry no longer shows; decide whether it
  should, without adding output the infrastructure classifier counts.
- 05 5.1 (REQ-0484, the I3 plant list) and the decision note
  "gitsafe-neutralizes-every-config-key-that-runs-a-program" do not say
  how `filter.<name>.clean` and `.process` are neutralized: their names
  are dynamic, so T143's `Status` lists the drivers and overrides each
  per call. The plant list gains a driver defined only in an `includeIf`
  file and one in the global config. The decision note is not edited; a
  new note names it (05 5.1 is on the `spec` surface).
- REQ-0901 (the order build, attest, verify, notes, publish) and the
  required attest and verify steps are not checked by the grammar; they
  belong with T088's `release.yml`.

Reopen: after T143 and T144 merge, as one docs pull request; or when
T088 starts.
