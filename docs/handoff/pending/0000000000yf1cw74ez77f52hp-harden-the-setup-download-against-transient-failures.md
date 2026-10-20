# Harden the setup download against transient failures

The macos-intel leg of #27 failed once on a runner DNS error
(`proxy.golang.org` lookup during setup) and passed on a re-run
(2026-10-09). Decided on 2026-10-09: harden the setup's module download
against transient failures, never by skipping a check. `tools/ci` has
no retry today, and REQ-1223 only says where a red leg that the change
did not cause is fixed.

Reopen: a second runner failure on the download, or the next change to
`tools/ci setup`.
