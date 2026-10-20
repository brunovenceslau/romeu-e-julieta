# T105, T023 and T017 follow-ups decided on 2026-10-13

Decided on 2026-10-13, answering the open questions the T105-B, T023 and
T017 builders reported:

- The data-race fix in internal/procgroup/pipes.go (abort() drains, so
  the copy goroutines are done with the writers when Run returns on a
  start failure) stays in T105 PR-B; pipes.go and procgroup_test.go join
  T105's owns.
- The `git pack-objects` and `git unpack-objects` pair of tools/mutants
  keeps two tool-git runs started together, each under the 2 min limit;
  the 04 4.1 text was aligned to the code (merged in #52).
- The setup of `tools/ci all`: `mise install` runs in the mise class
  (30 min); `mise trust` and `go mod download` run as command steps
  (15 min). The text merged in #52; the code is part of T105 PR-B.
- REQ-0300's exception list names the go runs of tools/mutants (merged
  in #52).
- gitsafe's sandbox mode (05 5.1, REQ-0470) passes a closed network list
  on top of the fresh environment: HTTPS_PROXY, HTTP_PROXY, NO_PROXY,
  their lowercase forms and SSL_CERT_FILE, and still no GIT_* variable.
  Why: in a sandbox every egress goes through a proxy whose CA is in
  SSL_CERT_FILE, so `julieta setup`'s clone and fetch (REQ-0398,
  REQ-0399) fail without them. Host mode is unchanged.
- The spec text of canon.Digest follows the typed API,
  `Digest[K Kind, T Input[K]](kind K, v T)`, with typed kind constants;
  the code and its compile-fail test stay.
- The gitsafe Clone verb lands in T023, in both modes (host for
  `romeu init`, T041; sandbox for `julieta setup`, REQ-0398).
