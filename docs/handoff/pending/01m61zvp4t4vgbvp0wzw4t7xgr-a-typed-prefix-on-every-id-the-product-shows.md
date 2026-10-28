# A typed prefix on every id the product shows

What is at stake: medium. An id the product shows is a bare lowercase
ULID of 26 characters (`01m61bhkm0rwttamfkyfntbktd`), and the same
shape names a memory entry, a handoff item, a generation, a salvage
entry, a stamp. Read outside its context, in a chat line, a pull
request body or a note, nothing says which it is, and a reader or a
command takes one kind for another. Changing the shape after entries,
stamps and records exist costs a migration of every file name and every
reference; deciding it before T174 (needs T035 and T171, after T166 and
T043) writes the handoff type and the stamps costs one format change.

Scope: every adopter of the product. When decided, the rule is written
in 03 3.12 next to REQ-0258 (the ULID rule), and in the format of each
kind it covers; the rule names that set. The places that write or take
a ULID today, a provisional list: 03 3.9 the memory entry file
(REQ-0241), 03 3.10 the stamp, 01 1.6 and 03 3.8 the generation id and
the salvage entry id (REQ-0235), 03 3.7 and 3.8 `candidateId`, 13 13.3
the spool id, the index of 08 8.2 and the commands of 04 4.1 that print
or take an id (`julieta memory show|edit`, `romeu memory list|show`,
REQ-0333, REQ-0684).

Decided on 2026-10-28, while measuring the ssh-agent bound:

- The need is current, not deferred: a memory entry id leaves its
  store whenever a session or a note names it (04 4.1 `show <id>` takes
  it back), and the handoff type of T174 adds a kind that shares the
  store and the index.
- The candidate: `<kind>_<ulid>`, the kind a short lowercase word of a
  new closed table of id kinds (the digest kinds of REQ-0257 are the
  precedent for a closed table, not the list: they name digests, not
  ids), an underscore so the whole id stays one shell word and one
  click, the 26 characters of the ULID unchanged, so a decoder that
  strips the prefix keeps REQ-0258.
- Open: whether the file name carries the prefix too
  (`entries/mem_<ulid>.md`) or only what the commands print and accept;
  whether a command still accepts a bare ULID; and which kinds get one
  (every id a person may see, or every ULID the product writes).

Next: a task at the next re-plan, ordered before T174, that writes the
rule and the table of kinds, and changes `julieta memory` (list, show,
edit, add), `romeu memory`, and the index of 08 8.2. This note is a
decision with a task to open, not a deferral; it moves to decisions/
when the task lands.
