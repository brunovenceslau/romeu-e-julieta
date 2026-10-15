# Finish T023: gitsafe

Branch feat/t023-gitsafe, based on docs/spec-fixes-2026-10-13, holds
the builder's work as one WIP commit; no gate has run. The first
builder finished the runner (all gates exit 0, 98.7% coverage, 21 of 21
mutants killed). The second had begun, test-first, the network
pass-through list and the Clone verb (clone.go, clone_test.go); the
tests may be red.

To do: finish both (a test pins the list exactly; GIT_* and other
ambient variables still never reach git), re-run every gate, rewrite the
WIP commit, ship gate (code-reviewer opus, security-auditor opus,
test-engineer sonnet), PR with an unticked `gitsafe` line, stacked on
the spec fixes PR.

Notes for the PR body: 04 4.1 has no timeout class for host git beyond
clone and fetch, so version, ls-remote --get-url, update-ref and bundle
use HostGitFetch; the sandbox fetch namespaces (refs/remotes/<remote>/,
refs/salvage/<generation>/) are a design reading of 04 4.3 and 08 8.5;
stricter than the spec: user info in URLs, http://, upper-case schemes
and shell-form credential helpers are refused; an unreadable git version
is reported as git-floor; FastForward is sandbox-only; safe.directory is
not in the spec, so nothing was added; macOS is untested.

Reopen: the next session.
