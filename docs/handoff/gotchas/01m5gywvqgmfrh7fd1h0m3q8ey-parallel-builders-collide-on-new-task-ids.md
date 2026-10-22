# Parallel builders collide on new task ids

The task generator gives the next free id of its own tree, so two
builders working in parallel both get the same id. The plan check
requires contiguous ids, so a task with the next id cannot pass `all`
until the one before it exists.

Give each builder its id in the brief, and stack the later task on the
earlier (2026-10-22, T158 and T159).
