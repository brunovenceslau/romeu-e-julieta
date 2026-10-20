# T013 leftovers

What is at stake: small; each is a correctness or hardening gap in the
container e2e or the workflows grammar, caught by review if it bites.

- 10 10.2 (REQ-0800) says `ci.yml` calls `go run ./tools/ci all`; since
  #186 it builds `tools/ci` and runs the binary, so exit 3 reaches the
  job. A spec pull request on the `spec` surface.
- A corrupt cache entry fails the e2e until the pins change; on a
  cached-file mismatch, delete it and fetch once (the file must still
  match the pin).
- The workflows grammar admits `github.event.*` in step `env` values,
  which reach `tools/ci` as arguments (argument injection, not shell);
  deny it outside `concurrency.group`. It predates #186.
- Infrastructure errors are matched by test name: a test that fails an
  assertion and then records an infrastructure error reads as infra.
  Only the Prepare paths record one today.
- The 60-minute job timeout was not re-measured with the image pull.

Reopen: after #186 merges, as one follow-up pull request, or the first
of them that bites.
