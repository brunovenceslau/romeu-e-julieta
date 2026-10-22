# A failing e2e step's raw output reaches the runner's workflow commands

What is at stake: medium (T158's security audit). A failing e2e step's
output is printed raw, and the GitHub runner reads workflow commands in
it (`::add-mask::`, `::stop-commands::`, `::error::`), so a test can
forge annotations or hide log lines. Pre-existing, not worsened by T158.

Scope: this repository.

Fix: print the output through `git.SafeLines`, or neutralise lines that
start with `::` or hold `##[`. Recorded in T158's task file.

Reopen: the next change to the check printer or to a step's output, or
the first forged annotation seen in a log.
