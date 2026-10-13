# Stage-B docs pass leftovers

- The index row on the sandbox's GitHub access counts final handoffs
  with `grep -h '^## Session checkpoint (' docs/reviews/round-*.md`,
  which misses stage-b.md; widen it to docs/reviews/*.md (moot once the
  checkpoints are migrated, see "migrate-the-docs-reviews-checkpoints-into-docs-handoff").
- Mark the answered items of the "Open for the operator" list of
  docs/reviews/stage-b.md (items 1 to 3 and 7) as answered.
- The L17 lens body in docs/reviews/lenses.md is 416 words by a plain
  count (410 with markdown stripped, 395 without the Model line and
  "Where to confirm"); T122 requires 280 to 400 per lens and the counting
  script is not in the tree. Trim about 16 words, or record the counting
  rule near lenses.md line 253. L20 and L21 are 397 by the plain count.

Reopen: the next edit of lenses.md, or the docs pass after #45.
