# Step-table rows without a REQ id

The hygiene and workflows rows of the step table in 10 10.2 carry no
REQ id. The generated docs/reference/checks.md of T105 PR-C therefore
shows "-" for them. Giving each row an id is a spec change of its own.

Reopen: T105 PR-C merges.
