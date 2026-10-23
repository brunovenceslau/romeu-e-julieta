# On macOS the race of internal/gate on main can hang until the test timeout

On 2026-10-23 the `race on macos-26` job of #264, #265 and #266 failed
with `panic: test timed out after 10m0s` in `internal/gate`, on pull
requests that do not touch it. The cause is the prompt order of `Ask`
on main: it writes the question and only then drops the typed-ahead
input, so an answer typed at the question can be dropped and the test
waits forever. #263 (T042) fixes it for `Ask` and `Confirm` and its own
macOS race passed.

Reopen: when #263 merges, re-run the failed macOS jobs of the open pull
requests (`gh run rerun <run> --failed`, a cause outside their diff)
and remove this note in the pull request that records it.
