# A cancelled CI run reads as failing checks

After the body of a pull request is edited, its CI run on the same
commit can be cancelled and a new one started; `pr_check.py after` and
`gh pr checks` then count the cancelled run's jobs as failing.

Measure the runs with `gh run list --branch <branch>` before reading a
failure (2026-10-20, #202).
