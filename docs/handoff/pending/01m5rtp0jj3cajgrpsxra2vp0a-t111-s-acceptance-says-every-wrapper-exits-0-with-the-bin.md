# T111's Acceptance says every wrapper exits 0 with the bin mount absent

What is at stake: low. A plan line describes behavior the kit does not
have, so a builder or reviewer of T111 may check the wrong thing.

Scope: this repository, T111.

The Acceptance of
`docs/plan/tasks/T111-the-bridge-julieta-by-hand-in-every-daily.md`
(line 127) says "With the bin mount absent, each wrapper prints its
warning line and exits 0". The PreToolUse guard of the `julieta-claude`
kit exits 2 and refuses every Bash call with the mount absent
(`kits/julieta-claude/julieta-claude.yaml` at 26d49d75, REQ-0673); only
the session hooks print a line and exit 0. Fix the Acceptance line to
say so.

Reopen: the next edit of T111's file.
