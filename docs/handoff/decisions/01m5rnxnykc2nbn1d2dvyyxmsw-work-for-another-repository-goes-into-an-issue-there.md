# Work for another repository goes into an issue there

What is at stake: low. A finding sent to a subagent by message dies
with the session; an issue survives it and reaches that repository's
own sessions. Undoing it is a habit, not a change.

Decided on 2026-10-25. Scope: every repository and sandbox of the
maintainer; written here until the personal kit (T114) carries the
house rules, then there.

When a review or a session of this repository finds work for another
repository of the maintainer, it opens one issue there with the
findings and the decisions taken, linked to the pull requests they
concern, instead of steering a subagent by message. When the token
cannot reach that repository, the issue text waits in a local file and
the maintainer is asked.

Reopen: the maintainer asks, or T114 merges.
