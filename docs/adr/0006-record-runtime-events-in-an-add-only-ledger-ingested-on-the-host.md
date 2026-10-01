# 6. Record runtime events in an add-only ledger ingested on the host

Date: 2026-10-01

## Status

Proposed

## Context

At the end of the play the Prince has to work out what happened from
what a friar, a servant and a page remember. Nobody wrote anything
down while it was happening. We would like to do better than Verona:
when something fails inside a sandbox, the failure should be on record
before anyone is asked to remember it.

The maintainer asked for this. julieta saves runtime information that
later feeds back into the sessions, because problems show up when
things execute; the store is one location the sandboxes on a machine
share; nothing edits it except through julieta; and what goes there is
never edited. The maintainer's reason, translated: this looks like
very good data for improving romeu itself and the whole development
environment.

The maintainer then decided these points, and this record transcribes
them:

- One ledger per machine, shared by the projects on it.
- A sandbox may read other projects' events; the full view is on the
  host only.
- The schema is versioned. The maintainer's reason, translated: with a
  version, the pressure to get it right the first time drops a little.
- Events are kept; `doctor` warns past a size; rotation waits.
- **Nothing in the ledger is ever edited or deleted.** The maintainer,
  translated: that no event is edited has to be absolute. No
  exception, no tombstone, no redaction event.
- A v1 event has no free text.
- A secret that still lands in the ledger stays there; the remedy is
  to rotate the credential.
- The guarantee is narrower than the request's wording: the ledger is
  add-only from ingest onward. Nothing inside a sandbox can make
  julieta the one writer of a directory the agent can also write, so
  the time between a write in the sandbox and ingest on the host is a
  residual risk, and the maintainer accepted it as that.
- v1 detects a content mismatch of ingested event bytes and nothing
  else. A removed entry is not detected, so "never deleted" is a
  promise in v1. The maintainer was told this before accepting it.

The rest of the design (the three stores, the fields, the reading
regime, the checks, the bounds) was put to the maintainer as a
proposal and accepted as a whole. It becomes a decision when this
record is accepted.

We cite no outside source for the design. None was checked, and the
constraints it answers are the spec's own: agent output reaches the
host only as data the host validates
([01 1.2](../spec/01-system-model.md#12-sources-of-truth-vs-derived));
host-only state is never mounted (I20); no directory is mounted into
two sandboxes; romeu deletes nothing it does not own (I16); romeu
handles no secret value (I25).

### Alternatives considered

- **A tombstone, or a redaction event, for an entry that should not
  have been stored.** It is the usual answer to a leaked secret in an
  append-only log. Rejected by the maintainer: the rule that nothing
  is edited or deleted has to be absolute. An exception written for
  secrets is a code path that removes history, and each later reason
  to remove something would ask to use it. We make the rule affordable
  from the other side: an event has no field that can hold a sentence,
  and a credential that leaks anyway is rotated.
- **A free-text field**, for the event's message or for a note.
  Rejected for v1. Free text is where a pasted token ends up, and it
  is a string one project's agent could send into another project's
  session through the shared view. Closed sets, valid by membership in
  generated tables, remove both. A free-text note for a project's own
  view is deferred with its trigger.
- **A hash chain over the entries in v1**, so that a removal is
  detected. Deferred, not rejected. The threat model trusts the host,
  and a chain kept on the same disk as the entries is rewritten by
  whoever can remove one. It starts to pay when the ledger leaves the
  machine or gains a second writer, and those are its triggers: the
  ledger is restored from a backup or copied between machines; it is
  used as evidence off the machine; a second writer is proposed. Until
  then the spec says plainly that removal is undetected.
- **A database** (an embedded SQL file, for example). Rejected. It
  would be the product's first database and a new Go dependency on the
  host, which is the trust surface we keep small
  ([12 12.1](../spec/12-engineering.md#121-tech-stack)). An entry that
  is one file is created once by a hard link that fails when the name
  exists, so "never replaced" is a property of one system call and not
  of a schema; `shasum` reproduces its key; and the reader, the caps
  and the atomic-write code already exist for the memory store and
  host state. What a database would buy is queries, and v1 defers
  analysis.
- **One directory shared by the sandboxes**, which is what the request
  literally describes. Rejected: it would be mounted into more than
  one sandbox, and a store an agent can write is not add-only. The
  spool, the ledger and the view split the request into a part the
  agent writes, a part only the host writes, and a part the agent
  reads.
- **A quarantine directory for rejected spool files.** Removed by the
  maintainer as a simplification: the rejected entry already reports a
  bad file once, and julieta deletes the file.
- **A message catalog of the ledger's own.** Removed by the maintainer
  for the same reason: rejection reasons are ids in the one error
  table ([04 4.1](../spec/04-cli.md#41-conventions-both-binaries)).

## Decision

romeu e julieta keeps a runtime ledger, specified in
[13](../spec/13-runtime-ledger.md):

1. **Three stores.** julieta writes events to a per-project spool, a
   read-write mount. romeu ingests them into a per-machine ledger
   under host state, which is never mounted. romeu derives a
   per-project view, a read-only mount.
2. **Add-only from ingest onward.** romeu creates each entry once, by
   a hard link that fails when the name exists, and has no code that
   edits or removes one (I31). No tombstone, no redaction event.
3. **No free text in a v1 event.** `type`, `tool` and `ref` are valid
   by membership in tables generated from the release's own code and
   data, never from a project file. `path` is the one wide value and
   does not cross projects.
4. **The spool is untrusted input** and is read under the regime the
   memory directories have (I32). A file that fails a check becomes a
   rejected entry; a failed ingest never blocks `salvage` or `rm`.
5. **The read policy.** A sandbox reads its own project's entries
   whole and other projects' entries as allowlisted closed fields; the
   full ledger is read on the host (I33).
6. **Versioned.** The event format has a version and romeu ingests
   versions N and N-1. The view policy has its own version.
7. **A leaked secret is handled by rotating the credential.** A
   pattern check rejects an event that matches, as defense in depth,
   and rewrites nothing.
8. **v1 detects a content mismatch of ingested event bytes, and a
   removal goes undetected.** The spec says so where it states the
   rule.

Why this fits us: the product already has every part it is built
from. A direct mount read through `os.Root` with caps, a host
directory that is never mounted, a read-only mount whose writability
julieta checks, struct tags that generate a validator and an
allowlist, one error table, plain sha256 for raw bytes. The ledger
adds a use of them, and seven concepts of its own: the spool, the
ledger, the view, the event schema, ingest, the view policy and the
drain.

## Consequences

- The rendered `sbxenv.yaml` gains two fixed mounts. A project
  rendered before them meets gate 2 once and needs one recreate.
- `internal/ledger` is a new package and a new ask-first surface, and
  three invariants, I31 to I33, each need a guard and a test (S6).
- A secret that reaches the ledger stays there and in machine backups.
  We accept that: the value is dead once rotated, and the alternative
  is a way to delete history.
- An event says less than a log line would. It has no message, and in
  v1 a `hook-failed` event does not name the hook. What an id cannot
  express is not recorded until a new version adds a closed field.
- The ledger only grows, and an agent can mint rejected entries, one
  per distinct invalid file. v1 warns and does not refuse.
- Events between a write in the sandbox and ingest on the host can be
  forged, altered or dropped. The ledger records what passed
  validation, not what happened.
- The bounds and caps are starting values the maintainer has not read
  one by one. They are in one table (13 13.8) and the plan may tune
  them.
- Deferred, each with its trigger, in the spec's
  [Deferred decisions](../spec.md#deferred-decisions): rotation and a
  hard size limit, a hash chain, cross-machine sync, finer visibility
  per project, analysis beyond `query`, a free-text note, keeping
  rejected bytes, a cleaner for skipped spool files, more event types
  and fields, tuning the starting values, and a record of gate runs.
- This record stays Proposed until the maintainer signs it off. What
  holds until then is Q24 in the spec's
  [Open questions](../spec.md#open-questions).
