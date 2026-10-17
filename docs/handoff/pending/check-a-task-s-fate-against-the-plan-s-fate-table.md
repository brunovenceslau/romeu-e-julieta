# Check a task's fate against the plan's fate table

`tools/ci plan` checks that a task's `fate` is one of the known fates and
that `merged:` and `pr:` come as a pair (`plan.go:279` and `plan.go:296`).
It does not read the table "The fate of each earlier task" of
docs/plan.md, so a task file and its row there can disagree. The rows
are prose ("deferred to M6", "kept, narrowed"), so a check first needs
the table in a form it can read.

Reopen: T108 rewrites the fate table, or a review finds a task whose
file and row disagree.
