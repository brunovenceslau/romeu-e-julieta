# T111's Acceptance names the fail-closed guard

Done on 2026-10-25. The Acceptance of
`docs/plan/tasks/T111-the-bridge-julieta-by-hand-in-every-daily.md` said
that with the bin mount absent each wrapper prints its warning line and
exits 0. It now says that the SessionStart and SessionEnd wrappers do,
and that the PreToolUse guard refuses the call, as REQ-0673 (08 8.2)
excludes the guard from that rule so it fails closed.

This closes the pending item 01m5rtp0jj3cajgrpsxra2vp0a "T111's
Acceptance says every wrapper exits 0 with the bin mount absent", whose
file is removed in the same pull request.
