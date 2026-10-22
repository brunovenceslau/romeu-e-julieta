# The Done record of T159 has a name the handoff check refuses

What is at stake: small. The handoff check exits 1 on main, so every
checkpoint's step "the handoff check exits 0" fails until it is fixed.

Scope: this repository's notes, and the handoff check of the sandbox
payload, which judges them.

Measured on 2026-10-22: the Done record of T159 (#236) is named
`01m5gspa2ne40avkdkktapva65-an-agents-md-of-pointers-and-a-claude-md-that-imports-it-t159.md`;
the check wants the slug cut at its length, without `-t159`. A rename
is refused: a Done record is only ever added, and the one rename the
check admits is the migration to a legacy id (`exact_rename`). The
local CI does not run the handoff check, so the PR passed.

Proposed: the payload's check admits a rename of a Done record to its
canonical name when the blob is identical, as it does for the
migration; then this file is renamed.

Reopen: that change of the payload's check, or a decision to accept
the finding.
