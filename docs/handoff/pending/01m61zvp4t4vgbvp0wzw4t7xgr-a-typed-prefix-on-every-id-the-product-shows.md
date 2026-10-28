# A typed prefix on every id the product shows

What is at stake: medium. An id the product shows is a bare lowercase
ULID (`01m61bhkm0rwttamfkyfntbktd`), and the same shape names a memory
entry, a handoff item, a generation, a salvage, a stamp. Read outside
its context, in a chat line, a pull request body or a note, nothing
says which it is, and a reader or a command takes one kind for another.
Changing the shape after entries, stamps and records exist costs a
migration of every file name and every reference; deciding it before
T174 writes the handoff type and the stamps costs one format change.

Scope: every adopter of the product. When decided, the rule is written
in 03 3.12 next to REQ-0258 (the ULID rule) and in the format of each
kind that carries an id (03 3.9 memory entries, 03 3.10 the stamp, 01
1.6 the generation, 3.8 the salvage).

Decided on 2026-10-28: the need is current, not deferred. Memory entry
ids already travel outside their store (this session named one in a
chat and in a private note), and the handoff type of T174 adds a kind
that shares the store.

The candidate: `<kind>_<ulid>`, the kind a short lowercase word from
the kinds table of `internal/canon` (REQ-0257), an underscore so the
whole id stays one shell word and one click, the ULID unchanged, so a
decoder that strips the prefix keeps REQ-0258. Open: whether the file
name carries the prefix too (`entries/mem_<ulid>.md`) or only what the
commands print and accept; whether a command accepts a bare ULID as
before; and which kinds get one (every id a person may see, or every
ULID the product writes).

Next: a task at the next re-plan, ordered before T174, that writes the
rule and the list of kinds, and changes `julieta memory` (list, show,
edit, add) and the index line of 03 3.9; the decision record goes to
decisions/ when the task lands.
