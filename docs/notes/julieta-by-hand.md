# julieta by hand, before romeu renders

Reader: whoever runs julieta in a sandbox that romeu does not manage
yet, in milestones M1 and M2 of the [plan](../plan.md). Type:
explanation.

This note records one reading of the specification and the shape of
the period it covers. It is not part of the specification, and nothing
in it is a product command or a rule of a spec section: the bridge it
describes is outside the product (plan rule 9, proposed on 2026-10-12).

## The reading

The manifest is the product's type. [03 3.6](../spec/03-formats.md)
names the format: the fields, the schema `manifest.v1`, the protocol
window of N and N-1 that a reader accepts. It names no writer. `romeu
sync` is the writer the specification expects, and a manifest written
by hand that carries the same fields validates the same way, through
the same decoder and the same schema. Nothing in julieta reads where a
manifest came from, so a hand-written one is not a mode, a flag or a
second path: it is the same input.

The same holds for the two mounts julieta reads. The memory directory
of [08 8.1](../spec/08-memory-handoff-salvage.md) is a layout on the
host, read through `os.Root` with the allowlist and the caps; who
mounted it is invisible to the reader. The read-only bin directory of
`.romeu/bin` ([06 6.3](../spec/06-kits.md)) holds a julieta binary and
its `SHA256SUMS`; `julieta setup` links the binary and checks the sum,
whoever built it.

So julieta runs, whole, in a sandbox built and started by hand, with
three things supplied by hand: the manifest in its environment
variable, the memory directory mounted in its final host layout, and
the bin directory mounted read-only. That is the bridge.

## The period

The bridge exists between sitting M1, when julieta's daily commands
reach every sandbox the maintainer uses, and sitting M3, when `romeu
sync` renders every project definition and writes every manifest. In
that period:

- each project definition gains the three edits above and a fourth
  that removes the store's writer kit and turns the mount of the store
  read-only until the recreate of T115, and
  one change of the sandbox agent payload removes the inbox skill from
  every sandbox at once, so `julieta memory` is the route for open
  items, and a runbook in the maintainer's config repository records
  each edit and that change with its undo (plan task T111);
- the `julieta-claude` kit (T079), pinned at a SHA of this repository
  with its `julietaBin` arg set to the path of the julieta binary in the bin mount, installs the
  SessionStart and SessionEnd hook lines of
  [08 8.2](../spec/08-memory-handoff-salvage.md), each as a wrapper
  that exits 0 when the bin mount is absent, and the two skills of
  `skills/`, which are the official ones (decided on 2026-10-23; there
  is no interim kit);
- the personal kit (T114) joins at sitting M2, with a fifth edit per
  definition: the other kit entries with their args, `signingKey` set
  to the public key, by hand until `romeu sync` writes them (plan task
  T115);
- `romeu sync` writes every manifest from sitting M3 on, one project
  per slot, and the runbook sections close.

The hand-written manifest and mounts are the "remember" pattern, work
that a deterministic tool should hold; the plan lists them as an
accepted risk for this window, with the runbook as the record and
`julieta status`, which prints the manifest's repositories, as the read
that shows a drift at sitting M1.

## What this note does not do

It names no repository other than this one and the maintainer's config
repository, and it describes no tool by name.
