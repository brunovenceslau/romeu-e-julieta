---
id: T167
title: "gitsafe: `http.followRedirects=false` and the error of a moved repository"
milestone: M3
module: gitsafe
fate: merged
owns:
  - internal/gitsafe/**
  - docs/spec/05-security.md
needs:
  - T151
after:
  - T061
  - T039
  - T164
  - T183
ask_first:
  - gitsafe
  - spec
  - contracts
sitting: none
merged: 2026-10-29
pr: 327
---

# T167 - gitsafe: `http.followRedirects=false` and the error of a moved repository

Goal: `http.followRedirects=false` joins gitsafe's fixed git config, so
a repository that has moved is not followed to a host the operator
never named. A moved repository then fails with a clear error that says
the repository has moved and asks for its new URL. Decided on
2026-10-23, scope this repository.

`CheckURL` admits only the `https://` URLs of the settings and the spec
(05 5.1), and a redirect lets the server pick another host for the rest
of the transfer. With the key set to `false`, git stops at the redirect;
gitsafe turns that failure into an error that names the URL it was given
and asks for the new one (the fix is the operator's: the spec's or the
settings' URL).

The task is `after` T039 because both own `docs/spec/05-security.md`
(the plan check refuses an overlap that nothing orders), and after T164
because T164 may add a gitsafe file this task also touches.

The task reads the fixed config of 05 5.1 and REQ-0468 first: the key
belongs to host mode; whether the sandbox mode (which leaves the `http.*`
keys to the sandbox) and the local config of a review checkout (REQ-0468
lists its keys as exact) take it is decided there and written in 05 5.1.

- Implements: REQ-1312 (05 5.1), REQ-1313 (05 5.1).
- Operator: no.
- Acceptance:
  - Every host-mode git invocation of gitsafe carries
    `-c http.followRedirects=false`; a table test over the verbs holds
    it, and 05 5.1 lists the key.
  - Against a test origin that answers a clone or a fetch with a 301 to
    another host, the call fails with an error that names the URL asked
    for and says to use the new URL; no request reaches the second
    host.
  - Whether the error has a slug in the package's `errors.go` or is a
    plain finding is decided in the task and stated in its pull request.
- Verify: `go test ./internal/gitsafe/...`.
