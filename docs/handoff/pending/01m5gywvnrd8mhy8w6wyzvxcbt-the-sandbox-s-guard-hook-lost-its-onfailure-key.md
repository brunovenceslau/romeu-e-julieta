# The sandbox's guard hook lost its onFailure key

What is at stake: medium. The brief guard fails open in this sandbox, so
a brief that breaks its rule is not blocked.

Scope: the sandbox kit's repository (issue 283's session).

Measured on 2026-10-22: the payload's seed writes `"onFailure": "block"`
on the brief guard, but ~/.claude/settings.json of this sandbox lacks it
(written 21:06 on 2026-10-20, five minutes after the seed). Cause not
measured.

Reopen: a recreate of the sandbox, or that issue's fix.
