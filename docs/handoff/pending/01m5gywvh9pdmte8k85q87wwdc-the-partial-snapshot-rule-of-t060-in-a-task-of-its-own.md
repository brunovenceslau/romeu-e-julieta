# The partial-snapshot rule of T060 in a task of its own

What is at stake: low. The code holds a rule nobody has confirmed, so a
snapshot may be written partial where a person would want a refusal.

Scope: this repository.

T060's pull request (#228) wrote the rule: a refused ref, an unreadable
reflog or worktree, a reflog commit git cannot walk, or the caps are
skipped and recorded and the snapshot is written; the loss of a whole
namespace keeps the earlier pair. Its merge did not confirm the rule
(decided 2026-10-21).

Reopen: plan the task that confirms or changes it; until then the code
holds the rule as built.
