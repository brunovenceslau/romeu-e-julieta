# The markdown renderer of planviewer is tested by hand

- The task reader of `tools/planviewer` (#176) renders a task's body
  with a small markdown renderer in tools/planviewer/template.html. It
  escapes the text first, admits only allow-listed links and uses
  patterns that run in linear time. CI has no JavaScript runtime, so no
  automated test covers it: it was exercised by hand under node before
  the merge.
- Decided on 2026-10-19: keep the manual test, rather than add node to
  CI for one page.

Reopen: CI gains a JavaScript runtime, or the renderer changes
materially (a new block kind, a new link rule, a new pattern).
