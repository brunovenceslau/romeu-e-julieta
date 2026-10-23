# The changed-golden list in CI's shallow checkout stays a notice

What is at stake: low. The list of changed goldens is a reading aid for
the reviewer, and the golden check does not depend on it; a short list
hides a changed golden from the log, not from the check.

Decided on 2026-10-23. Scope: this repository, `tools/ci golden`.

In a shallow checkout, where the merge base cannot be found, the list of
changed goldens (`Changed` in `tools/ci/internal/golden/golden.go`) is
empty and the check prints a note that says why. That note stays a
notice and does not become a failure.

Reopen: a changed golden passed without being listed.
