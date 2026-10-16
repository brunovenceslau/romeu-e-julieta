# Ship the spec fixes PR

Branch docs/spec-fixes-2026-10-13 holds one commit: the 05 5.1 network
pass-through list, the canon.Digest call sites in 01, 03 and 07 written
in the typed form, T023's goal naming Clone and the pass-through list,
and T007's owns widened to internal/cli/writer*.go. Its gate state is
unknown: review it, run the ship gate (security-auditor opus, since 05
is the security model) and open the PR with an unticked `spec` approval
line. It is the bottom of a stack with feat/t023-gitsafe on top.

Reopen: the next session.
